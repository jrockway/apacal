// apacal connects to the DisplayCAL web interface and sets an APA102 LED to the color that DisplayCAL
// has requested.
package main

import (
	"flag"
	"fmt"
	"image/color"
	"log"
	"net/url"
	"os"
	"os/signal"

	"periph.io/x/extra/hostextra"
	"periph.io/x/periph/conn/spi/spireg"
	"periph.io/x/periph/devices/apa102"
)

var (
	webAddr   = flag.String("web", "http://localhost:8080", "address of the DisplayCAL web interface")
	numPixels = flag.Int("pixels", 1, "number of LEDs to drive")
	intensity = flag.Int("intensity", 15, "upper bound on brightness (0-255); see apa102.Opts.Intensity")
	spiPort   string
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
func setColor(leds *apa102.Dev, n int, c color.RGBA) (int, error) {
	pixels := make([]byte, 0, n*3)
	for i := 0; i < n; i++ {
		pixels = append(pixels, c.R, c.G, c.B)
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

	webURL, err := url.Parse(*webAddr)
	if err != nil {
		log.Fatalf("unable to parse provided web URL %q: %v", *webAddr, err)
	}

	p, err := spireg.Open(spiPort)
	if err != nil {
		log.Fatalf("opening spi port %q: %v", spiPort, err)
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
		log.Printf("initialilzing apa102 device: %v", err)
		return
	}
	if _, err := setColor(leds, *numPixels, color.RGBA{255, 255, 255, 255}); err != nil {
		log.Printf("setting leds to white: %v", err)
		return
	}

	log.Printf("waiting for colors from the web interface at %s, press C-c to abort", webURL)
	colorCh := make(chan color.RGBA)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
loop:
	for {
		select {
		case c := <-colorCh:
			log.Printf("received color %v", c)
		case <-sigCh:
			log.Printf("interrupt")
			break loop
		}
	}
	log.Printf("done")
	if _, err := setColor(leds, *numPixels, color.RGBA{0, 0, 0, 255}); err != nil {
		log.Printf("setting leds to black: %v", err)
		return
	}
	return
}
