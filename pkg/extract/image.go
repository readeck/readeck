package extract

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"io"
	"net/http"
	"net/url"

	"github.com/disintegration/imaging"

	"image/gif"  // GIF decoder and encoder
	"image/jpeg" // JPEG decoder and encoder
	"image/png"  // PNG decoder and encoder

	_ "github.com/biessek/golang-ico" // ICO decoder
	_ "golang.org/x/image/bmp"        // BMP decoder
	_ "golang.org/x/image/webp"       // WEBP decoder
)

// RemoteImage is a remote image that can be loaded and manipulated.
type RemoteImage struct {
	format string
	m      image.Image
}

// NewRemoteImage loads an image and returns a new RemoteImage instance.
func NewRemoteImage(src string, client *http.Client) (*RemoteImage, error) {
	if client == nil {
		client = http.DefaultClient
	}

	if src == "" {
		return nil, fmt.Errorf("No image URL")
	}

	rsp, err := client.Get(src)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Invalid response status (%d)", rsp.StatusCode)
	}

	return remoteImageFromReader(rsp.Body)
}

func remoteImageFromReader(r io.Reader) (*RemoteImage, error) {
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

	return &RemoteImage{
		format: format,
		m:      m,
	}, nil
}

// Image returns the embeded image.Image instance.
func (ri *RemoteImage) Image() image.Image {
	return ri.m
}

// Format returns the image format.
func (ri *RemoteImage) Format() string {
	return ri.format
}

// Fit resizes the image to a given size, only if
// the given width and height are bigger than the current
// image.
func (ri *RemoteImage) Fit(w, h int) *RemoteImage {
	bounds := ri.m.Bounds()
	if w > bounds.Dx() && h > bounds.Dy() {
		return ri
	}

	ri.m = imaging.Fit(ri.m, w, h, imaging.Lanczos)
	return ri
}

// Encode encodes the image to the given format. If format is an
// empty string it will reuse the original format if possible.
// It fallbacks to jpeg encoding.
func (ri *RemoteImage) Encode(format string) ([]byte, string, error) {
	if format == "" {
		format = ri.format
	}

	var err error
	buf := new(bytes.Buffer)

	switch format {
	case "gif":
		m, ok := ri.m.(*image.Paletted)
		numColors := 256
		if ok {
			numColors = len(m.Palette)
		}
		err = gif.Encode(buf, ri.m, &gif.Options{
			NumColors: numColors,
		})
	case "png":
		encoder := &png.Encoder{CompressionLevel: png.BestCompression}
		err = encoder.Encode(buf, ri.m)
	default:
		format = "jpeg"
		err = jpeg.Encode(buf, ri.m, &jpeg.Options{
			Quality: 80,
		})
	}

	return buf.Bytes(), format, err
}

// Picture is a remote picture
type Picture struct {
	Href  string
	Type  string
	Size  [2]int
	bytes []byte
}

// NewPicture returns a new Picture instance from a given
// URL and its base.
func NewPicture(src string, base *url.URL) (*Picture, error) {
	href, err := base.Parse(src)
	if err != nil {
		return nil, err
	}

	return &Picture{
		Href: href.String(),
	}, nil
}

// Load loads the image remotely and fit it into the given
// boundaries size.
func (p *Picture) Load(client *http.Client, size int, toFormat string) error {
	var format string
	ri, err := NewRemoteImage(p.Href, client)
	if err != nil {
		return err
	}
	if p.bytes, format, err = ri.Fit(size, size).Encode(toFormat); err != nil {
		return err
	}

	p.Size = [2]int{ri.Image().Bounds().Dx(), ri.Image().Bounds().Dy()}
	p.Type = fmt.Sprintf("image/%s", format)
	return nil
}

// Copy returns a resized copy of the image, as a new Picture instance.
func (p *Picture) Copy(size int, toFormat string) (*Picture, error) {
	ri, err := remoteImageFromReader(bytes.NewReader(p.bytes))
	if err != nil {
		return nil, err
	}

	var format string
	res := &Picture{Href: p.Href}
	if res.bytes, format, err = ri.Fit(size, size).Encode(toFormat); err != nil {
		return nil, err
	}

	res.Size = [2]int{ri.Image().Bounds().Dx(), ri.Image().Bounds().Dy()}
	res.Type = fmt.Sprintf("image/%s", format)
	return res, nil
}

// Name returns the given name of the picture with the correct
// extension.
func (p *Picture) Name(name string) string {
	return fmt.Sprintf("%s.%s", name, p.Type[6:])
}

// Bytes returns the image data.
func (p *Picture) Bytes() []byte {
	return p.bytes
}

// Encoded returns a base64 encoded string of the image.
func (p *Picture) Encoded() string {
	if len(p.bytes) == 0 {
		return ""
	}

	return base64.StdEncoding.EncodeToString(p.bytes)
}
