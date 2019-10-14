// apacal connects to the DisplayCAL web interface and sets an APA102 LED to the color that DisplayCAL
// has requested.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/jrockway/apacal/displaycalweb"
	"periph.io/x/extra/hostextra"
	"periph.io/x/periph/conn/spi/spireg"
	"periph.io/x/periph/devices/apa102"
)

var (
	webAddr    = flag.String("web", "http://localhost:8080", "address of the DisplayCAL web interface")
	numPixels  = flag.Int("n", 1, "number of LEDs to drive")
	firstPixel = flag.Int("from", 0, "first pixel to illuminate; 0 is the first pixel")
	lastPixel  = flag.Int("to", -1, "last pixel to illuminate (exclusive); -1 to use all pixels")
	intensity  = flag.Int("i", 80, "upper bound on brightness (0-255); see apa102.Opts.Intensity")
	spiPort    string
)

func setupSPIFlag() {
	var def string
	var choices []string
	devices := spireg.All()
	if len(devices) > 0 {
		def = devices[0].Name
	}
	for _, d := range devices {
		choices = append(choices, d.Name)
	}
	flag.StringVar(&spiPort, "spi", def, fmt.Sprintf("spi bus to use; one of %v", choices))
}

// setColor sets the entire LED strip/matrix to the provided color.  The alpha of the color is
// ignored.
func setColor(leds *apa102.Dev, c color.RGBA) (int, error) {
	pixels := make([]byte, *numPixels*3)
	for i := 0; i < *numPixels; i++ {
		pixels[i*3] = 0
		pixels[i*3+1] = 0
		pixels[i*3+2] = 0
	}
	for i := *firstPixel; i < *lastPixel; i++ {
		pixels[i*3] = c.R
		pixels[i*3+1] = c.G
		pixels[i*3+2] = c.B
	}
	return leds.Write(pixels)
}

func main() {
	// We do initialization before parsing flags so that we can include a reasonable default for
	// the SPI bus name.
	if _, err := hostextra.Init(); err != nil {
		log.Fatalf("periph.io hostextra initialization: %v", err)
	}
	setupSPIFlag()
	flag.Parse()

	if *intensity < 0 || *intensity > 255 {
		log.Fatalf("intensity value %d out of range 0-255", *intensity)
	}

	if *lastPixel == -1 {
		*lastPixel = *numPixels
	}
	if *lastPixel > *numPixels {
		log.Fatalf("-to is greater than the number of pixels (%d > %d)", *lastPixel, *numPixels)
	}
	if *lastPixel < 0 {
		log.Fatalf("-to must be > 0 (%d)", *lastPixel)
	}
	if *firstPixel < 0 {
		log.Fatalf("-from must be > 0 (%d)", *firstPixel)
	}
	if *firstPixel > *lastPixel {
		log.Fatalf("-from and -to out of range (%d > %d)", *firstPixel, *lastPixel)
	}

	webURL, err := url.Parse(*webAddr)
	if err != nil {
		log.Fatalf("unable to parse provided web URL %q: %v", *webAddr, err)
	}

	p, err := spireg.Open(spiPort)
	if err != nil {
		log.Fatalf("open spi port %q: %v", spiPort, err)
	}
	defer p.Close()
	opts := &apa102.Opts{
		NumPixels:        *numPixels,
		Intensity:        uint8(*intensity),
		Temperature:      apa102.NeutralTemp, // Disable color correction, because the reason you're running this script is to do your own.
		DisableGlobalPWM: true,
	}
	leds, err := apa102.New(p, opts)
	if err != nil {
		log.Printf("init apa102 device: %v", err)
		return
	}
	if _, err := setColor(leds, color.RGBA{255, 255, 255, 255}); err != nil {
		log.Printf("set leds to white: %v", err)
		return
	}

	log.Printf("Waiting for colors from the web interface at %s, press C-c to abort", webURL)

	ctx, kill := context.WithCancel(context.Background())
	doneCh := make(chan error)
	colorCh := make(chan color.RGBA)
	go func() {
		for {
			err := displaycalweb.Run(ctx, webURL, colorCh)
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				doneCh <- err
				return
			}
			log.Printf("read color: %v", err)
			time.Sleep(time.Second)
		}
	}()
	var exited bool

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
loop:
	for {
		select {
		case c := <-colorCh:
			log.Printf("received color %v", c)
			if _, err := setColor(leds, c); err != nil {
				log.Printf("set leds: %v", err)
			}
		case err := <-doneCh:
			if err != nil {
				log.Printf("read color: %v", err)
			}
			exited = true
			close(colorCh)
			close(doneCh)
			break loop
		case <-sigCh:
			log.Printf("interrupt")
			break loop
		}
	}
	kill()
	log.Printf("done")

	if !exited {
		select {
		case err := <-doneCh:
			if err != nil {
				log.Printf("read color: %v", err)
			}
			close(colorCh)
			close(doneCh)
		case <-time.After(time.Second):
			log.Printf("timeout waiting for web interface loop to stop")
		}
	}

	if _, err := setColor(leds, color.RGBA{0, 0, 0, 255}); err != nil {
		log.Printf("set leds to black: %v", err)
		return
	}
	return
}
