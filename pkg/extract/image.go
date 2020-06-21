package extract

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"

	"net/http"
	"net/url"

	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/pkg/img"
)

// NewRemoteImage loads an image and returns a new img.Image instance.
func NewRemoteImage(src string, client *http.Client) (img.Image, error) {
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

	return img.New(configs.Config.Images.Processor, rsp.Body)
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
func (p *Picture) Load(client *http.Client, size uint, toFormat string) error {
	var format string
	var r io.Reader
	ri, err := NewRemoteImage(p.Href, client)
	if err != nil {
		return err
	}
	defer ri.Close()
	if err = ri.Fit(size, size); err != nil {
		return err
	}
	ri.SetQuality(75)
	if r, format, err = ri.Encode(toFormat); err != nil {
		return err
	}

	if p.bytes, err = ioutil.ReadAll(r); err != nil {
		return err
	}

	p.Size = [2]int{int(ri.Width()), int(ri.Height())}
	p.Type = fmt.Sprintf("image/%s", format)
	return nil
}

// Copy returns a resized copy of the image, as a new Picture instance.
func (p *Picture) Copy(size uint, toFormat string) (*Picture, error) {
	ri, err := img.New(configs.Config.Images.Processor, bytes.NewReader(p.bytes))
	if err != nil {
		return nil, err
	}
	defer ri.Close()

	var format string
	var r io.Reader
	res := &Picture{Href: p.Href}
	if err = ri.Fit(size, size); err != nil {
		return nil, err
	}
	if r, format, err = ri.Encode(toFormat); err != nil {
		return nil, err
	}
	if res.bytes, err = ioutil.ReadAll(r); err != nil {
		return nil, err
	}

	res.Size = [2]int{int(ri.Width()), int(ri.Height())}
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
