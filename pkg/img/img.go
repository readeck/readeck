package img

import (
	"fmt"
	"io"
)

// Image describes the interface of an image manipulation object.
type Image interface {
	Close() error
	Encode(format string) (io.Reader, string, error)
	Format() string
	Width() uint
	Height() uint
	SetQuality(uint)
	Fit(w, h uint) error
	Grayscale() error
	Dither(numColors uint) error
	DitherMono(numColors uint) error
}

var loaders = map[string]func(io.Reader) (Image, error){}

// AddLoader adds a new image loader to the available loaders.
func AddLoader(name string, fn func(io.Reader) (Image, error)) {
	loaders[name] = fn
}

// New loads an image using the given loader.
func New(loader string, r io.Reader) (Image, error) {
	fn, ok := loaders[loader]
	if !ok {
		return nil, fmt.Errorf("loaders %s not found", loader)
	}

	return fn(r)
}
