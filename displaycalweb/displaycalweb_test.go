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

// Based on https://sourceforge.net/p/dispcalgui/code/HEAD/tree/trunk/DisplayCAL/webwin.py.
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

func TestParseColor(t *testing.T) {
	testData := []struct {
		input   string
		want    color.RGBA
		wantErr bool
	}{
		{"#000000", color.RGBA{0x00, 0x00, 0x00, 0xff}, false},
		{"#FFFFFF", color.RGBA{0xff, 0xff, 0xff, 0xff}, false},
		{"#ffffff", color.RGBA{0xff, 0xff, 0xff, 0xff}, false},
		{"#0Aa1F0", color.RGBA{0x0a, 0xa1, 0xf0, 0xff}, false},

		{"", color.RGBA{}, true},
		{"!FFFFFF", color.RGBA{}, true},
		{"#foobar", color.RGBA{}, true},
		{"#f", color.RGBA{}, true},
		{"hello there", color.RGBA{}, true},
	}

	for i, test := range testData {
		got, err := parseColor([]byte(test.input))
		if want := test.want; got != want {
			t.Errorf("test %d: color:\n  got: %v\n want: %v", i, got, want)
		}
		if test.wantErr && err == nil {
			t.Errorf("test %d: expected error", i)
		}
		if !test.wantErr && err != nil {
			t.Errorf("test %d: unexpected error: %v", i, err)
		}
	}
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
		h.ch <- []byte("#010203")
	}()

	doneCh := make(chan error)
	resultCh := make(chan color.RGBA)
	go func() {
		doneCh <- Run(ctx, u, resultCh)
	}()

	timeoutCh := time.After(100 * time.Millisecond)

	select {
	case <-timeoutCh:
		t.Fatal("timeout waiting for color")
	case c := <-resultCh:
		if got, want := c, (color.RGBA{R: 255, G: 255, B: 255, A: 255}); got != want {
			t.Errorf("recv color:\n  got: %v\n want: %v", got, want)
		}
	}
	select {
	case <-timeoutCh:
		t.Fatal("timeout waiting for second color")
	case c := <-resultCh:
		if got, want := c, (color.RGBA{R: 1, G: 2, B: 3, A: 255}); got != want {
			t.Errorf("recv color:\n  got: %v\n want: %v", got, want)
		}
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
