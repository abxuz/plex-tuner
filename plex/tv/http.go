package tv

import (
	"context"
	"io"
	"net/http"
)

type HttpSteam struct {
	url    string
	resp   *http.Response
	ctx    context.Context
	cancel context.CancelFunc
}

func NewHttpSteam(url string) *HttpSteam {
	s := &HttpSteam{url: url}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	return s
}

func (s *HttpSteam) Start() error {
	request, err := http.NewRequestWithContext(s.ctx, "GET", s.url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	s.resp = resp
	return nil
}

func (s *HttpSteam) Read(b []byte) (int, error) {
	return s.resp.Body.Read(b)
}

func (s *HttpSteam) Close() error {
	s.cancel()
	if s.resp != nil && s.resp.Body != nil {
		io.Copy(io.Discard, s.resp.Body)
		s.resp.Body.Close()
	}
	return nil
}
