package img

import (
	"bytes"
	"image"
	"io"

	"image/gif"  // GIF decoder and encoder
	"image/jpeg" // JPEG decoder and encoder
	"image/png"  // PNG decoder and encoder

	"github.com/disintegration/imaging"

	_ "github.com/biessek/golang-ico" // ICO decoder
	_ "golang.org/x/image/bmp"        // BMP decoder
	_ "golang.org/x/image/webp"       // WEBP decoder
)

// EncodeOptions contains the various encoding options for
// the possible output types.
type EncodeOptions struct {
	Jpeg *jpeg.Options
	Gif  *gif.Options
	Png  *pngOptions
}

type pngOptions struct {
	CompressionLevel png.CompressionLevel
}

// Image is an image.
type Image struct {
	format  string
	m       image.Image
	Options *EncodeOptions
}

// New returns a new Image instance from a reader.
func New(r io.Reader) (*Image, error) {
	// We need to grab the format first, hence this two pass thing
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)
	_, format, _ := image.DecodeConfig(tee)
	m, err := imaging.Decode(
		io.MultiReader(&buf, r),
		imaging.AutoOrientation(true),
	)

	if err != nil {
		return nil, err
	}

	return &Image{
		format: format,
		m:      m,
		Options: &EncodeOptions{
			Jpeg: &jpeg.Options{
				Quality: 80,
			},
			Gif: &gif.Options{
				NumColors: 256,
			},
			Png: &pngOptions{
				CompressionLevel: png.BestCompression,
			},
		},
	}, nil
}

// Encode encodes the image to the given format. If format is an
// empty string it will reuse the original format if possible.
// It fallbacks to jpeg encoding.
func (im *Image) Encode(format string) ([]byte, string, error) {
	if format == "" {
		format = im.format
	}

	var err error
	buf := new(bytes.Buffer)

	switch format {
	case "gif":
		m, ok := im.m.(*image.Paletted)
		numColors := im.Options.Gif.NumColors
		if ok {
			numColors = len(m.Palette)
		}
		options := &gif.Options{}
		*options = *im.Options.Gif
		options.NumColors = numColors

		err = gif.Encode(buf, im.m, options)
	case "png":
		encoder := &png.Encoder{CompressionLevel: im.Options.Png.CompressionLevel}
		err = encoder.Encode(buf, im.m)
	default:
		format = "jpeg"
		err = jpeg.Encode(buf, im.m, im.Options.Jpeg)
	}

	return buf.Bytes(), format, err
}

// Image returns the embedded image.Image instance.
func (im *Image) Image() image.Image {
	return im.m
}

// Format returns the image format.
func (im *Image) Format() string {
	return im.format
}

// Fit resizes the image to a given size, only if
// the given width and height are bigger than the current
// image.
func (im *Image) Fit(w, h int) *Image {
	bounds := im.m.Bounds()
	if w > bounds.Dx() && h > bounds.Dy() {
		return im
	}

	im.m = imaging.Fit(im.m, w, h, imaging.Lanczos)
	return im
}

// Grayscale transforms the image to a grayscale version.
func (im *Image) Grayscale() *Image {
	im.m = imaging.Grayscale(im.m)
	return im
}

// Dither transforms the image to a dithered one.
func (im *Image) Dither(name string, errorMultiplier float32) *Image {
	if errorMultiplier == 0 {
		errorMultiplier = 1.18
	}

	d, ok := dithers[name]
	if !ok {
		panic("Dither not found. " + name)
	}
	im.m = d.Color(im.m, errorMultiplier)
	return im
}

// DitherMono transforms the image to a monochrome dithered one.
func (im *Image) DitherMono(name string, errorMultiplier float32) *Image {
	if errorMultiplier == 0 {
		errorMultiplier = 1.18
	}

	d, ok := dithers[name]
	if !ok {
		panic("Dither not found. " + name)
	}
	im.m = d.Monochrome(im.m, errorMultiplier)
	return im
}
