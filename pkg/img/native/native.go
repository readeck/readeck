package native

import (
	"bytes"
	"fmt"
	"image"
	"io"

	"image/gif"  // GIF decoder and encoder
	"image/jpeg" // JPEG decoder and encoder
	"image/png"  // PNG decoder and encoder

	"github.com/disintegration/imaging"

	_ "github.com/biessek/golang-ico" // ICO decoder
	_ "golang.org/x/image/bmp"        // BMP decoder
	_ "golang.org/x/image/webp"       // WEBP decoder

	"github.com/readeck/readeck/pkg/img"
)

func init() {
	img.AddLoader("native", New)
}

// Image is an image.
type Image struct {
	m       image.Image
	format  string
	quality uint
}

// New returns a new Image instance from a reader.
func New(r io.Reader) (img.Image, error) {
	// We need to grab the format first, hence this two pass thing
	var buf bytes.Buffer
	tee := io.TeeReader(r, &buf)

	_, format, err := image.DecodeConfig(tee)
	if err != nil {
		return nil, err
	}

	m, err := imaging.Decode(
		io.MultiReader(&buf, r),
		imaging.AutoOrientation(true),
	)
	if err != nil {
		return nil, err
	}

	return &Image{
		m:       m,
		format:  format,
		quality: 80,
	}, nil
}

// Close must be calles after you're done with your image conversion.
func (im *Image) Close() error {
	return nil
}

// Encode encodes the image to the given format. If format is an
// empty string it will reuse the original format if possible.
// It fallbacks to jpeg encoding.
func (im *Image) Encode(format string) (io.Reader, string, error) {
	if format == "" {
		format = im.format
	}

	var err error
	buf := new(bytes.Buffer)

	switch format {
	case "gif":
		m, ok := im.m.(*image.Paletted)
		numColors := 256
		if ok {
			numColors = len(m.Palette)
		}
		options := &gif.Options{NumColors: 256}
		// *options = *im.Options.Gif
		options.NumColors = numColors

		err = gif.Encode(buf, im.m, options)
	case "png":
		encoder := &png.Encoder{CompressionLevel: png.BestCompression}
		err = encoder.Encode(buf, im.m)
	default:
		format = "jpeg"
		options := &jpeg.Options{Quality: int(im.quality)}
		err = jpeg.Encode(buf, im.m, options)
	}

	return bytes.NewReader(buf.Bytes()), format, err
}

// Format returns the image format.
func (im *Image) Format() string {
	return im.format
}

// Width returns the image width.
func (im *Image) Width() uint {
	return uint(im.m.Bounds().Dx())
}

// Height returns the image height.
func (im *Image) Height() uint {
	return uint(im.m.Bounds().Dy())
}

// SetQuality sets the JPEG quality of final image.
func (im *Image) SetQuality(q uint) {
	im.quality = q
}

// Fit resizes the image to a given size, only if
// the given width and height are bigger than the current
// image.
func (im *Image) Fit(w, h uint) error {
	if w > im.Width() && h > im.Height() {
		return nil
	}

	im.m = imaging.Fit(im.m, int(w), int(h), imaging.Lanczos)
	return nil
}

// Grayscale transforms the image to a grayscale version.
func (im *Image) Grayscale() error {
	im.m = imaging.Grayscale(im.m)
	return nil
}

// Dither transforms the image to a dithered one.
func (im *Image) Dither(uint) error {
	method := "Burkes"
	d, ok := dithers[method]
	if !ok {
		return fmt.Errorf("dither method not found: %s", method)
	}
	im.m = d.Color(im.m, 1.18)
	return nil
}

// DitherMono transforms the image to a monochrome dithered one.
func (im *Image) DitherMono(uint) error {
	method := "Burkes"
	d, ok := dithers[method]
	if !ok {
		return fmt.Errorf("dither method not found: %s", method)
	}
	im.m = d.Monochrome(im.m, 1.18)
	return nil
}
