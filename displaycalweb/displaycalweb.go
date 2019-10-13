// Package displaycalweb connects to the DisplayCAL web interfaces and retrives the colors that
// DisplayCAL wants to display.
package displaycalweb

import (
	"context"
	"fmt"
	"image/color"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
)

// urlFor returns the URL to get the next color, given the last received color.
func urlFor(base *url.URL, color color.RGBA) string {
	u := *base
	u.Path = "/ajax/messages"

	// We pick a "bad" random number here.  DisplayCAL doesn't read this, it's just a browser
	// hack to bust the cache.
	part := fmt.Sprintf("rgb(%d, %d, %d) %v", color.R, color.G, color.B, rand.Float64())

	// We use PathEscape because they seem to be looking for %20 instead of + when decoding.
	u.RawQuery = url.PathEscape(part)
	return u.String()
}

// parseColor parses a response body from DisplayCAL into a color.RGBA.
func parseColor(body []byte) (color.RGBA, error) {
	if len(body) < 7 {
		return color.RGBA{}, fmt.Errorf("too short: got %s (len %d), want 7 bytes", body, len(body))
	}

	if body[0] != '#' {
		return color.RGBA{}, fmt.Errorf("bad format: expected first byte to be #, got %s", body)
	}

	r, err := strconv.ParseUint(string(body[1:3]), 16, 8)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("red component of %s: %v", body, err)
	}
	g, err := strconv.ParseUint(string(body[3:5]), 16, 8)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("green component of %s: %v", body, err)
	}
	b, err := strconv.ParseUint(string(body[5:7]), 16, 8)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("blue component of %s: %v", body, err)
	}
	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: 255}, nil
}

// Run writes the color that the DisplayCAL web interface at the provided URL wants you to display
// to the channel, running until the context is done.
func Run(ctx context.Context, u *url.URL, ch chan<- color.RGBA) error {
	c := color.RGBA{R: 1, G: 2, B: 3}
	for {
		addr := urlFor(u, c)
		req, err := http.NewRequestWithContext(ctx, "GET", addr, nil)
		if err != nil {
			return fmt.Errorf("create request for %q: %w", addr, err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return err
		}

		// TODO(jrockway): This can potentially block past ctx.Done().
		body, err := ioutil.ReadAll(res.Body)
		res.Body.Close()
		if err != nil {
			return fmt.Errorf("read body %q: %w", addr, err)
		}

		if got, want := res.StatusCode, http.StatusOK; got != want {
			return fmt.Errorf("request %q: http status: %v", addr, got)
		}

		newColor, err := parseColor(body)
		if err != nil {
			return fmt.Errorf("request %q: parse color: %v", addr, err)
		}
		c = newColor

		select {
		case ch <- c:
		case <-ctx.Done():
			return fmt.Errorf("send color: %w", ctx.Err())
		}
	}
}
