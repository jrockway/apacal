package displaycalweb

import (
	"context"
	"image/color"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

type testHandler struct {
	ch chan []byte
}

// Based on https://sourceforge.net/p/dispcalgui/code/HEAD/tree/trunk/DisplayCAL/webwin.py
func (h *testHandler) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if req.URL.Path != "/ajax/messages" {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	ctx := req.Context()
	select {
	case <-ctx.Done():
		http.Error(w, ctx.Err().Error(), http.StatusInternalServerError)
		return
	case bytes := <-h.ch:
		w.Header().Add("content-type", "text/plain; charset=UTF-8")
		w.Header().Add("cache-control", "no-cache")
		w.Write(bytes)
		return
	}
	return
}

func TestRun(t *testing.T) {
	h := &testHandler{ch: make(chan []byte)}
	s := httptest.NewServer(h)
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatal(err)
	}

	ctx, kill := context.WithCancel(context.Background())
	go func() {
		h.ch <- []byte("#FFFFFF")
	}()

	doneCh := make(chan error)
	resultCh := make(chan color.RGBA)
	go func() {
		doneCh <- Run(ctx, u, resultCh)
	}()

	timeoutCh := time.After(100 * time.Millisecond)

	var c color.RGBA
	select {
	case <-timeoutCh:
		t.Fatal("timeout waiting for color")
	case c = <-resultCh:

	}
	if got, want := c, (color.RGBA{R: 255, G: 255, B: 255, A: 255}); got != want {
		t.Errorf("recv color:\n  got: %v\n want: %v", got, want)
	}
	kill()

	select {
	case <-timeoutCh:
		t.Fatal("timeout waiting for Run to stop")
	case err := <-doneCh:
		if err == nil {
			t.Errorf("got no error, wanted 'context cancelled'")
		}
	}
}
