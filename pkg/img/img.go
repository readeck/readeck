package img

import (
	"image/color"
	"io"
)

// Gray16Palette is a 16 level b&w palette.
var Gray16Palette = []color.Color{
	color.RGBA{0x00, 0x00, 0x00, 0xff},
	color.RGBA{0x11, 0x11, 0x11, 0xff},
	color.RGBA{0x22, 0x22, 0x22, 0xff},
	color.RGBA{0x33, 0x33, 0x33, 0xff},
	color.RGBA{0x44, 0x44, 0x44, 0xff},
	color.RGBA{0x55, 0x55, 0x55, 0xff},
	color.RGBA{0x66, 0x66, 0x66, 0xff},
	color.RGBA{0x77, 0x77, 0x77, 0xff},
	color.RGBA{0x88, 0x88, 0x88, 0xff},
	color.RGBA{0x99, 0x99, 0x99, 0xff},
	color.RGBA{0xaa, 0xaa, 0xaa, 0xff},
	color.RGBA{0xbb, 0xbb, 0xbb, 0xff},
	color.RGBA{0xcc, 0xcc, 0xcc, 0xff},
	color.RGBA{0xdd, 0xdd, 0xdd, 0xff},
	color.RGBA{0xee, 0xee, 0xee, 0xff},
	color.RGBA{0xff, 0xff, 0xff, 0xff},
}

// ImageCompression is the compression level used for PNG images
type ImageCompression uint8

const (
	// CompressionFast is a fast method.
	CompressionFast ImageCompression = iota

	// CompressionBest is the best space saving method.
	CompressionBest
)

// Image describes the interface of an image manipulation object.
type Image interface {
	Close() error
	Format() string
	Width() uint
	Height() uint
	SetFormat(string) error
	SetCompression(ImageCompression) error
	SetQuality(uint8) error
	Resize(uint, uint) error
	Fit(uint, uint) error
	Grayscale() error
	Gray16() error
	Pipeline(...ImageFilter) error
	Encode(io.Writer) error
}

// ImageFilter is a filter application function used by the
// Pipeline method of an Image instance.
type ImageFilter func(Image) error

// New create a new Image instance, using the ImageNative implementation.
// Since there's no other implementation at the moment,
// let's keep it this way for now.
func New(r io.Reader) (Image, error) {
	return NewNativeImage(r)
}
