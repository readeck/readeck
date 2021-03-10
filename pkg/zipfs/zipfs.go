package zipfs

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// HTTPZipFile serves a zip file as an HTTP directory (without listing)
// It properly handles If-Modified-Since header and can serve the compressed
// content when deflate is in Accept-Encoding and the content is compressed
// with deflate.
type HTTPZipFile string

func (f HTTPZipFile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" && r.Method != "HEAD" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	filename := r.URL.Path

	fd, size, err := f.OpenZip()
	if err != nil {
		f.error(w, err)
		return
	}
	defer fd.Close()

	entry, err := f.GetEntry(filename, fd, size)
	if err != nil {
		f.error(w, err)
		return
	}

	f.serveEntry(w, r, fd, entry)
}

// OpenZip opens a file ready
func (f HTTPZipFile) OpenZip() (*os.File, int64, error) {
	zipfile := string(f)

	st, err := os.Stat(zipfile)
	if err != nil {
		return nil, 0, err
	}
	if st.IsDir() {
		return nil, 0, os.ErrNotExist
	}

	fd, err := os.Open(zipfile)
	if err != nil {
		return nil, 0, err
	}
	s, err := fd.Stat()
	if err != nil {
		fd.Close()
		return nil, 0, err
	}

	return fd, s.Size(), err
}

// GetEntry returns a zip.File instance for "name", or an error if the
// entry does not exist or is a directory.
func (f HTTPZipFile) GetEntry(name string, fd *os.File, size int64) (*zip.File, error) {
	zr, err := zip.NewReader(fd, size)
	if err != nil {
		return nil, err
	}

	for _, x := range zr.File {
		if strings.HasSuffix(x.Name, "/") {
			continue
		}
		if x.Name != name {
			continue
		}

		return x, nil
	}

	return nil, os.ErrNotExist
}

func (f HTTPZipFile) error(w http.ResponseWriter, err error) {
	if os.IsNotExist(err) {
		http.Error(w, http.StatusText(404), 404)
		return
	}

	http.Error(w, http.StatusText(500), 500)
}

func (f HTTPZipFile) serveEntry(w http.ResponseWriter, r *http.Request, fd *os.File, z *zip.File) {
	modtime := z.Modified.UTC()

	if f.checkIfModifiedSince(r, modtime) {
		w.WriteHeader(http.StatusNotModified)
		return
	}

	w.Header().Set("Last-Modified", modtime.Format(http.TimeFormat))

	fp, err := z.Open()
	if err != nil {
		http.Error(w, http.StatusText(500), 500)
		return
	}
	defer fp.Close()

	// Sniff the content
	var buf [512]byte
	n, _ := io.ReadFull(fp, buf[:])
	ctype := http.DetectContentType(buf[:n])

	w.Header().Set("Content-Type", ctype)

	if r.Method == "HEAD" {
		return
	}

	// Directly send the compressed data when possible
	ae := r.Header.Get("Accept-Encoding")
	if strings.Contains(ae, "deflate") && z.Method == zip.Deflate {
		offset, err := z.DataOffset()
		if err != nil {
			panic(err)
		}
		if err == nil {
			if _, err := fd.Seek(offset, 0); err != nil {
				panic(err)
			}

			w.Header().Set("Content-Encoding", "deflate")
			w.Header().Set("Content-Length", strconv.FormatUint(z.CompressedSize64, 10))

			w.WriteHeader(http.StatusOK)
			if _, err := io.CopyN(w, fd, int64(z.CompressedSize64)); err != nil {
				panic(err)
			}
			return
		}
	}

	w.Header().Set("Content-Length", strconv.FormatUint(z.UncompressedSize64, 10))
	w.WriteHeader(http.StatusOK)
	_, err = io.Copy(w, io.MultiReader(bytes.NewReader(buf[:n]), fp))
	if err != nil && !errors.Is(err, syscall.EPIPE) {
		panic(err)
	}
}

func (f HTTPZipFile) checkIfModifiedSince(r *http.Request, modtime time.Time) bool {
	ius := r.Header.Get("If-Modified-Since")
	if ius == "" {
		return false
	}
	t, err := http.ParseTime(ius)
	if err != nil {
		return false
	}

	modtime = modtime.Truncate(time.Second)
	if modtime.Before(t) || modtime.Equal(t) {
		return true
	}
	return false
}
