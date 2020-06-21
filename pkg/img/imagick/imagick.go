package img

import (
	"bytes"
	"io"
	"io/ioutil"
	"strings"
	"sync"

	"gopkg.in/gographics/imagick.v2/imagick"

	"github.com/readeck/readeck/pkg/img"
)

var lock sync.Mutex

func init() {
	imagick.Initialize()

	img.AddLoader("imagick", New)
}

// Image is an image.
type Image struct {
	mw      *imagick.MagickWand
	format  string
	quality uint
}

// New returns a new Image instance from a reader.
func New(r io.Reader) (img.Image, error) {
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	lock.Lock()
	mw := imagick.NewMagickWand()
	err = mw.ReadImageBlob(b)
	if err != nil {
		lock.Unlock()
		mw.Destroy()
		return nil, err
	}

	return &Image{
		mw:      mw,
		format:  strings.ToLower(mw.GetImageFormat()),
		quality: 80,
	}, nil
}

// Close destroys the underlying image resource. It must be called
// after you're done with your image conversion.
func (im *Image) Close() error {
	lock.Unlock()
	im.mw.Destroy()
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
	var buf bytes.Buffer

	switch format {
	case "gif":
		im.mw.SetFormat("gif")
	case "png":
		im.mw.SetFormat("png")
	default:
		im.mw.SetFormat("jpeg")
		err = im.mw.SetImageCompressionQuality(im.quality)
		if err != nil {
			return nil, "", err
		}
	}

	if _, err = buf.Write(im.mw.GetImageBlob()); err != nil {
		return nil, "", err
	}

	return bytes.NewReader(buf.Bytes()), im.mw.GetFormat(), nil
}

// Format returns the image format.
func (im *Image) Format() string {
	return im.format
}

// Width returns the image width.
func (im *Image) Width() uint {
	return im.mw.GetImageWidth()
}

// Height returns the image height.
func (im *Image) Height() uint {
	return im.mw.GetImageHeight()
}

// SetQuality sets the JPEG quality of final image.
func (im *Image) SetQuality(q uint) {
	im.quality = q
}

// Fit resizes the image to a given size, only if
// the given width and height are bigger than the current
// image.
func (im *Image) Fit(w, h uint) error {
	ow := im.mw.GetImageWidth()
	oh := im.mw.GetImageHeight()

	if w > ow && h > oh {
		return nil
	}

	srcAspectRatio := float64(ow) / float64(oh)
	maxAspectRatio := float64(w) / float64(h)

	var nw, nh uint
	if srcAspectRatio > maxAspectRatio {
		nw = w
		nh = uint(float64(nw) / srcAspectRatio)
	} else {
		nh = h
		nw = uint(float64(nh) * srcAspectRatio)
	}

	return im.mw.ResizeImage(nw, nh, imagick.FILTER_LANCZOS, 1)
}

// Grayscale transforms the image to a grayscale version.
func (im *Image) Grayscale() error {
	return im.mw.QuantizeImages(256, imagick.COLORSPACE_GRAY, 0, false, false)
}

// Dither transforms the image to a dithered one.
func (im *Image) Dither(numColors uint) error {
	return im.mw.QuantizeImages(numColors, imagick.COLORSPACE_RGB, 0, true, false)
}

// DitherMono transforms the image to a monochrome dithered one.
func (im *Image) DitherMono(numColors uint) error {
	return im.mw.QuantizeImages(numColors, imagick.COLORSPACE_GRAY, 0, true, false)
}
