// The following implements dither algorithms for image.
// The original code, MIT licensed was found on:
// https://github.com/esimov/dithergo/

package img

import (
	"image"
	"image/color"
)

// DitherFilter defines a dither matrix
type DitherFilter [][]float32

var dithers = map[string]DitherFilter{
	"Atkinson": [][]float32{
		{0.0, 0.0, 1.0 / 8.0, 1.0 / 8.0},
		{1.0 / 8.0, 1.0 / 8.0, 1.0 / 8.0, 0.0},
		{0.0, 1.0 / 8.0, 0.0, 0.0},
	},

	"Burkes": [][]float32{
		{0.0, 0.0, 0.0, 8.0 / 32.0, 4.0 / 32.0},
		{2.0 / 32.0, 4.0 / 32.0, 8.0 / 32.0, 4.0 / 32.0, 2.0 / 32.0},
		{0.0, 0.0, 0.0, 0.0, 0.0},
	},

	"FloydSteinberg": [][]float32{
		{0.0, 0.0, 0.0, 7.0 / 48.0, 5.0 / 48.0},
		{3.0 / 48.0, 5.0 / 48.0, 7.0 / 48.0, 5.0 / 48.0, 3.0 / 48.0},
		{1.0 / 48.0, 3.0 / 48.0, 5.0 / 48.0, 3.0 / 48.0, 1.0 / 48.0},
	},

	"Sierra2": [][]float32{
		{0.0, 0.0, 0.0, 4.0 / 16.0, 3.0 / 16.0},
		{1.0 / 16.0, 2.0 / 16.0, 3.0 / 16.0, 2.0 / 16.0, 1.0 / 16.0},
		{0.0, 0.0, 0.0, 0.0, 0.0},
	},

	"Sierra3": [][]float32{
		{0.0, 0.0, 0.0, 5.0 / 32.0, 3.0 / 32.0},
		{2.0 / 32.0, 4.0 / 32.0, 5.0 / 32.0, 4.0 / 32.0, 2.0 / 32.0},
		{0.0, 2.0 / 32.0, 3.0 / 32.0, 2.0 / 32.0, 0.0},
	},

	"SierraLite": [][]float32{
		{0.0, 0.0, 2.0 / 4.0},
		{1.0 / 4.0, 1.0 / 4.0, 0.0},
		{0.0, 0.0, 0.0},
	},

	"Stucki": [][]float32{
		{0.0, 0.0, 0.0, 8.0 / 42.0, 4.0 / 42.0},
		{2.0 / 42.0, 4.0 / 42.0, 8.0 / 42.0, 4.0 / 42.0, 2.0 / 42.0},
		{1.0 / 42.0, 2.0 / 42.0, 4.0 / 42.0, 2.0 / 42.0, 1.0 / 42.0},
	},
}

// Monochrome converts an image to a monochrome, dithered one.
func (d DitherFilter) Monochrome(input image.Image, errorMultiplier float32) image.Image {
	bounds := input.Bounds()
	img := image.NewGray(bounds)
	for x := bounds.Min.X; x < bounds.Dx(); x++ {
		for y := bounds.Min.Y; y < bounds.Dy(); y++ {
			pixel := input.At(x, y)
			img.Set(x, y, pixel)
		}
	}
	dx, dy := img.Bounds().Dx(), img.Bounds().Dy()

	// Pre-populate multidimensional slice
	errors := make([][]float32, dx)
	for x := 0; x < dx; x++ {
		errors[x] = make([]float32, dy)
		for y := 0; y < dy; y++ {
			errors[x][y] = 0
		}
	}

	for x := 0; x < dx; x++ {
		for y := 0; y < dy; y++ {
			pix := float32(img.GrayAt(x, y).Y)
			pix -= errors[x][y] * errorMultiplier

			var quantError float32
			// Diffuse the error of each calculation to the neighboring pixels
			if pix < 128 {
				quantError = -pix
				pix = 0
			} else {
				quantError = 255 - pix
				pix = 255
			}

			img.SetGray(x, y, color.Gray{Y: uint8(pix)})

			// Diffuse error in two dimension
			ydim := len(d) - 1
			xdim := len(d[0]) / 2
			for xx := 0; xx < ydim+1; xx++ {
				for yy := -xdim; yy <= xdim-1; yy++ {
					if y+yy < 0 || dy <= y+yy || x+xx < 0 || dx <= x+xx {
						continue
					}
					// Adds the error of the previous pixel to the current pixel
					errors[x+xx][y+yy] += quantError * d[xx][yy+ydim]
				}
			}
		}
	}
	return img
}

// Color converts an image to a dithered one.
func (d DitherFilter) Color(input image.Image, errorMultiplier float32) image.Image {
	bounds := input.Bounds()
	img := image.NewRGBA(bounds)
	for x := bounds.Min.X; x < bounds.Dx(); x++ {
		for y := bounds.Min.Y; y < bounds.Dy(); y++ {
			pixel := input.At(x, y)
			img.Set(x, y, pixel)
		}
	}
	dx, dy := img.Bounds().Dx(), img.Bounds().Dy()

	// Prepopulate multidimensional slices
	redErrors := make([][]float32, dx)
	greenErrors := make([][]float32, dx)
	blueErrors := make([][]float32, dx)
	for x := 0; x < dx; x++ {
		redErrors[x] = make([]float32, dy)
		greenErrors[x] = make([]float32, dy)
		blueErrors[x] = make([]float32, dy)
		for y := 0; y < dy; y++ {
			redErrors[x][y] = 0
			greenErrors[x][y] = 0
			blueErrors[x][y] = 0
		}
	}

	var qrr, qrg, qrb float32
	for x := 0; x < dx; x++ {
		for y := 0; y < dy; y++ {
			r32, g32, b32, a := img.At(x, y).RGBA()
			r, g, b := float32(uint8(r32)), float32(uint8(g32)), float32(uint8(b32))
			r -= redErrors[x][y] * errorMultiplier
			g -= greenErrors[x][y] * errorMultiplier
			b -= blueErrors[x][y] * errorMultiplier

			// Diffuse the error of each calculation to the neighboring pixels
			if r < 128 {
				qrr = -r
				r = 0
			} else {
				qrr = 255 - r
				r = 255
			}
			if g < 128 {
				qrg = -g
				g = 0
			} else {
				qrg = 255 - g
				g = 255
			}
			if b < 128 {
				qrb = -b
				b = 0
			} else {
				qrb = 255 - b
				b = 255
			}
			img.Set(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), uint8(a)})

			// Diffuse error in two dimension
			ydim := len(d) - 1
			xdim := len(d[0]) / 2
			for xx := 0; xx < ydim+1; xx++ {
				for yy := -xdim; yy <= xdim-1; yy++ {
					if y+yy < 0 || dy <= y+yy || x+xx < 0 || dx <= x+xx {
						continue
					}
					// Adds the error of the previous pixel to the current pixel
					redErrors[x+xx][y+yy] += qrr * d[xx][yy+ydim]
					greenErrors[x+xx][y+yy] += qrg * d[xx][yy+ydim]
					blueErrors[x+xx][y+yy] += qrb * d[xx][yy+ydim]
				}
			}
		}
	}
	return img
}
