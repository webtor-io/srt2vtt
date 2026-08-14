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
	detector := chardet.NewTextDetector()
	enc, err := detector.DetectBest(body)
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
	srt, err := astisub.ReadFromSRT(bytes.NewReader(body))
	if err != nil {
		return "", errors.Wrap(err, "failed to read srt")
	}
	log.Infof("writing to vtt")
	var buf = &bytes.Buffer{}
	err = srt.WriteToWebVTT(buf)
	if err != nil {
		return "", errors.Wrap(err, "failed to write vtt")
	}
	return buf.String(), nil
}

func (s *SRT2VTT) Get(ctx context.Context, src string) (string, error) {
	return s.LazyMap.Get(src, func() (string, error) {
		return s.get(ctx, src)
	})
}
