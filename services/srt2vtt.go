package services

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/asticode/go-astisub"

	"github.com/gogs/chardet"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/htmlindex"
	"golang.org/x/text/encoding/unicode/utf32"

	"github.com/webtor-io/lazymap"
)

type SRT2VTT struct {
	lazymap.LazyMap[string]
	cl *http.Client
}

func NewSRT2VTT(cl *http.Client) *SRT2VTT {
	return &SRT2VTT{
		cl: cl,
		LazyMap: lazymap.New[string](&lazymap.Config{
			Expire:      60 * time.Second,
			ErrorExpire: 5 * time.Second,
		}),
	}
}

// getEncoding resolves a charset name as reported by chardet to an encoding.
// htmlindex covers every charset chardet can return except UTF-32, which it
// deliberately omits as it is not a WHATWG encoding, so it is handled here.
func getEncoding(charset string) (encoding.Encoding, error) {
	switch charset {
	case "UTF-32LE":
		return utf32.UTF32(utf32.LittleEndian, utf32.IgnoreBOM), nil
	case "UTF-32BE":
		return utf32.UTF32(utf32.BigEndian, utf32.IgnoreBOM), nil
	}
	return htmlindex.Get(charset)
}

func (s *SRT2VTT) get(ctx context.Context, src string) (string, error) {
	log.Infof("loading sourceURL=%v", src)
	req, err := http.NewRequest(http.MethodGet, src, nil)
	if err != nil {
		return "", errors.Wrap(err, "failed to make request")
	}
	resp, err := s.cl.Do(req.WithContext(ctx))
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch url")
	}
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read body")
	}
	// For SSA/ASS, detect the charset from the dialogue text alone: the
	// large ASCII section/format scaffolding otherwise drowns out the
	// non-ASCII signal and chardet settles on ISO-8859-1 for e.g. a
	// windows-1251 file. The "[Script Info]" marker itself is ASCII, so
	// the raw-byte sniff works for any 8-bit charset (UTF-16/32 files
	// fail this sniff, get detected structurally from the full body, and
	// are re-sniffed after decoding below).
	detectionSample := body
	if isSSA(body) {
		if dialogue := ssaDialogueText(body); len(dialogue) > 0 {
			detectionSample = dialogue
		}
	}
	detector := chardet.NewTextDetector()
	enc, err := detector.DetectBest(detectionSample)
	if err != nil {
		return "", errors.Wrap(err, "failed to detect encoding")
	}
	if enc.Charset != "UTF-8" {
		log.Infof("converting source encoding=%v to utf-8", enc.Charset)
		e, err := getEncoding(enc.Charset)
		if err != nil {
			return "", errors.Wrap(err, "failed to find encoding")
		}
		body, err = e.NewDecoder().Bytes(body)
		if err != nil {
			return "", errors.Wrap(err, "failed to convert encoding")
		}
	}
	var subs *astisub.Subtitles
	if isSSA(body) {
		log.Infof("detected ssa/ass content")
		subs, err = astisub.ReadFromSSA(bytes.NewReader(body))
		if err != nil {
			return "", errors.Wrap(err, "failed to read ssa")
		}
	} else {
		subs, err = astisub.ReadFromSRT(bytes.NewReader(body))
		if err != nil {
			return "", errors.Wrap(err, "failed to read srt")
		}
	}
	log.Infof("writing to vtt")
	var buf = &bytes.Buffer{}
	err = subs.WriteToWebVTT(buf)
	if err != nil {
		return "", errors.Wrap(err, "failed to write vtt")
	}
	return buf.String(), nil
}

// isSSA sniffs SubStation Alpha content (both SSA v4 and ASS v4+) by its
// mandatory "[Script Info]" section header, skipping the ";"-prefixed
// comment lines many authoring tools emit above it. Sniffing the content
// instead of trusting the URL extension matters because callers pass
// arbitrary upstream URLs that often have no meaningful extension at
// all. body is already UTF-8 at this point. A plain SRT can contain
// brackets inside cue text, but its first non-blank line is a numeric
// index, never this header.
func isSSA(body []byte) bool {
	head := bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	for _, line := range bytes.SplitN(head, []byte("\n"), 32) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 || line[0] == ';' {
			continue
		}
		return bytes.HasPrefix(bytes.ToLower(line), []byte("[script info]"))
	}
	return false
}

// ssaDialogueText extracts the free-text field of every "Dialogue:" line
// (everything after the 9th comma per the SSA/ASS event format) so charset
// detection can run on actual prose instead of ASCII scaffolding.
func ssaDialogueText(body []byte) []byte {
	var out [][]byte
	for _, line := range bytes.Split(body, []byte("\n")) {
		line = bytes.TrimSpace(line)
		if !bytes.HasPrefix(line, []byte("Dialogue:")) {
			continue
		}
		rest := line[len("Dialogue:"):]
		fields := bytes.SplitN(rest, []byte(","), 10)
		if len(fields) < 10 {
			continue
		}
		out = append(out, fields[9])
	}
	return bytes.Join(out, []byte("\n"))
}

func (s *SRT2VTT) Get(ctx context.Context, src string) (string, error) {
	return s.LazyMap.Get(src, func() (string, error) {
		return s.get(ctx, src)
	})
}
