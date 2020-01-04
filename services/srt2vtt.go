package services

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/asticode/go-astisub"

	"github.com/djimenez/iconv-go"

	"github.com/gogs/chardet"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"
)

type SRT2VTT struct {
	sourceURL string
	vttText   string
	inited    bool
	err       error
	mux       sync.Mutex
}

func NewSRT2VTT(url string) *SRT2VTT {
	return &SRT2VTT{sourceURL: url, inited: false}
}

func (s *SRT2VTT) get() (string, error) {
	timeout := time.Duration(10 * time.Minute)
	client := http.Client{
		Timeout: timeout,
	}
	log.Infof("Loading sourceURL=%v", s.sourceURL)
	resp, err := client.Get(s.sourceURL)
	if err != nil {
		return "", errors.Wrap(err, "Failed to fetch url")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read body")
	}
	bodyStr := string(body)
	detector := chardet.NewTextDetector()
	enc, err := detector.DetectBest(body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to detect encoding")
	}
	if enc.Charset != "UTF-8" {
		log.Infof("Converting source encoding=%v to utf-8", enc.Charset)
		encoded, _ := iconv.ConvertString(bodyStr, enc.Charset, "utf-8")
		bodyStr = encoded
	}
	srt, err := astisub.ReadFromSRT(bytes.NewReader([]byte(bodyStr)))
	if err != nil {
		return "", errors.Wrap(err, "Failed to read srt")
	}
	log.Infof("Writing to vtt")
	var buf = &bytes.Buffer{}
	err = srt.WriteToWebVTT(buf)
	if err != nil {
		return "", errors.Wrap(err, "Failed to write vtt")
	}
	return buf.String(), nil
}

func (s *SRT2VTT) Get() (string, error) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.inited {
		return s.vttText, s.err
	}
	s.vttText, s.err = s.get()
	s.inited = true
	return s.vttText, s.err
}
