// Package displaycalweb connects to the DisplayCAL web interfaces and retrives the colors that
// DisplayCAL wants to display.
package displaycalweb

import (
	"context"
	"fmt"
	"image/color"
	"net/url"
)

// Run writes the color that the DisplayCAL web interface at the provided URL wants you to display
// to the channel, running until the context is done.
func Run(ctx context.Context, u *url.URL, ch <-chan color.RGBA) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("sending color: %w", err)
		}
	}
}
