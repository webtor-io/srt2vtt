package services

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/asticode/go-astisub"

	"github.com/djimenez/iconv-go"

	"github.com/gogs/chardet"
	"github.com/pkg/errors"

	log "github.com/sirupsen/logrus"

	"github.com/webtor-io/lazymap"
)

type SRT2VTT struct {
	lazymap.LazyMap[string]
}

func NewSRT2VTT() *SRT2VTT {
	return &SRT2VTT{
		LazyMap: lazymap.New[string](&lazymap.Config{
			Expire:      60 * time.Second,
			ErrorExpire: 5 * time.Second,
		}),
	}
}

func (s *SRT2VTT) get(src string) (string, error) {
	timeout := 10 * time.Minute
	client := http.Client{
		Timeout: timeout,
	}
	log.Infof("loading sourceURL=%v", src)
	resp, err := client.Get(src)
	if err != nil {
		return "", errors.Wrap(err, "failed to fetch url")
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "failed to read body")
	}
	bodyStr := string(body)
	detector := chardet.NewTextDetector()
	enc, err := detector.DetectBest(body)
	if err != nil {
		return "", errors.Wrap(err, "failed to detect encoding")
	}
	if enc.Charset != "UTF-8" {
		log.Infof("converting source encoding=%v to utf-8", enc.Charset)
		encoded, _ := iconv.ConvertString(bodyStr, enc.Charset, "utf-8")
		bodyStr = encoded
	}
	srt, err := astisub.ReadFromSRT(bytes.NewReader([]byte(bodyStr)))
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

func (s *SRT2VTT) Get(src string) (string, error) {
	return s.LazyMap.Get(src, func() (string, error) {
		return s.get(src)
	})
}
