package bookmarks

import (
	"archive/zip"
	"compress/flate"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type zipper struct {
	fp *os.File
	zp *zip.Writer
}

func newZipper(name string) (*zipper, error) {
	dirname := filepath.Dir(name)

	stat, err := os.Stat(dirname)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := os.MkdirAll(dirname, 0750); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	} else if !stat.IsDir() {
		return nil, fmt.Errorf(`"%s" is not a directory`, dirname)
	}

	fp, err := os.Create(name)
	if err != nil {
		return nil, err
	}

	zp := zip.NewWriter(fp)
	zp.RegisterCompressor(zip.Deflate, func(out io.Writer) (io.WriteCloser, error) {
		return flate.NewWriter(out, flate.BestSpeed)
	})

	return &zipper{fp: fp, zp: zp}, nil
}

func (z *zipper) addFile(name string, data []byte) error {
	f, err := z.zp.CreateHeader(&zip.FileHeader{
		Method:   zip.Store,
		Name:     name,
		Modified: time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

func (z *zipper) addCompressedFile(name string, data []byte) error {
	f, err := z.zp.CreateHeader(&zip.FileHeader{
		Method:   zip.Deflate,
		Name:     name,
		Modified: time.Now(),
	})
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	return err
}

func (z *zipper) addDirectory(name string) error {
	if !strings.HasSuffix(name, "/") {
		name += "/"
	}

	_, err := z.zp.CreateHeader(&zip.FileHeader{
		Method:   zip.Deflate,
		Name:     name,
		Modified: time.Now(),
	})
	return err
}

func (z *zipper) close() error {
	err := z.zp.Close()
	if err != nil {
		if err := z.fp.Close(); err != nil {
			panic(err)
		}
		return err
	}

	return z.fp.Close()
}
