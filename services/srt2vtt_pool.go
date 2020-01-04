package services

import "sync"

type SRT2VTTPool struct {
	sm sync.Map
}

func NewSRT2VTTPool() *SRT2VTTPool {
	return &SRT2VTTPool{}
}

func (s *SRT2VTTPool) Get(url string) (string, error) {
	v, loaded := s.sm.LoadOrStore(url, NewSRT2VTT(url))
	if !loaded {
		defer s.sm.Delete(url)
	}
	return v.(*SRT2VTT).Get()
}
