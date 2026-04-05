package testutil

import (
	"io"
	"net/http"
	"sync/atomic"
)

// TrackTransport wraps an http.RoundTripper and counts the number of times
// response bodies are closed. Use this in adapter tests to verify that every
// streaming goroutine calls resp.Body.Close() (FD-leak detection).
//
//	tt := &testutil.TrackTransport{}
//	client := &http.Client{Transport: tt}
//	// ... run N requests ...
//	if tt.Closes() != N { t.Errorf(...) }
type TrackTransport struct {
	Base       http.RoundTripper // nil → uses http.DefaultTransport
	closeCount int64             // accessed atomically
}

// RoundTrip delegates to Base and wraps the response body with a close counter.
func (t *TrackTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	resp, err := base.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	resp.Body = &trackBody{ReadCloser: resp.Body, counter: &t.closeCount}
	return resp, nil
}

// Closes returns the total number of times a wrapped response body was closed.
func (t *TrackTransport) Closes() int64 {
	return atomic.LoadInt64(&t.closeCount)
}

type trackBody struct {
	io.ReadCloser
	counter *int64
}

func (b *trackBody) Close() error {
	atomic.AddInt64(b.counter, 1)
	return b.ReadCloser.Close()
}
