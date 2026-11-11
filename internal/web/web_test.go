package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGetRejectsBodiesOverTheCap(t *testing.T) {
	old := MaxBodySize
	MaxBodySize = 10
	defer func() { MaxBodySize = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 11)))
	}))
	defer srv.Close()

	_, err := Get(context.Background(), srv.Client(), srv.URL, nil)
	if !errors.Is(err, ErrBodyTooLarge) {
		t.Fatalf("err = %v, want ErrBodyTooLarge", err)
	}
}

func TestGetAcceptsBodiesAtTheCap(t *testing.T) {
	old := MaxBodySize
	MaxBodySize = 10
	defer func() { MaxBodySize = old }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(strings.Repeat("x", 10)))
	}))
	defer srv.Close()

	resp, err := Get(context.Background(), srv.Client(), srv.URL, nil)
	if err != nil || len(resp.Body) != 10 {
		t.Fatalf("resp=%v err=%v, want 10-byte body", resp, err)
	}
}
