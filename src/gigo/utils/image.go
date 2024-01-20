package utils

import (
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"io"
	"sort"

	"github.com/gage-technologies/prominentcolor"
	"golang.org/x/image/draw"
)

// PrepImageFile
//
//		Reads an image file from the disk, validates that it is an image file and writes the cleaned image file
//		to the disk as a jpeg so that no EXIF data is stored within the image. The function also returns the
//	 most dominant color of the image as a hex code
//
//		Args
//			src		- io.ReadCloser, source file that will be loaded as an image
//			dst 	- io.WriteCloser, destination file that the sanitized image will be written to (in jpeg format)
func PrepImageFile(src io.ReadCloser, dst io.WriteCloser, vertical bool, color bool) (string, error) {
	// defer closure of source file
	defer src.Close()
	// defer closure of destination file
	defer dst.Close()

	// attempt to decode the image file
	img, _, err := image.Decode(src)
	if err != nil {
		pngImg, err2 := png.Decode(src)
		if err2 != nil {
			if err == image.ErrFormat {
				return "", err
			}
			return "", fmt.Errorf("failed to decode image file: %v", err)
		}
		img = pngImg
	}

	// calculate new dimensions based on aspect ratio
	width := float64(img.Bounds().Max.X)
	height := float64(img.Bounds().Max.Y)
	if vertical {
		width = height * 9 / 16
	} else {
		height = width * 9 / 16
	}

	// get the dominant color of the image
	dominantColor := "#29C18C"
	if color {
		dominantColor, err = getDominantColor(img)
		if err != nil {
			return "", fmt.Errorf("failed to retrieve dominant color for image: %v", err)
		}
	}

	// create a new image to hold the resized image matrix
	resized := image.NewRGBA(image.Rect(0, 0, int(width), int(height)))

	// resize the image to the new dimensions
	draw.CatmullRom.Scale(resized, resized.Rect, img, img.Bounds(), draw.Over, nil)

	// write the image as a new jpeg file
	err = jpeg.Encode(dst, resized, &jpeg.Options{Quality: 85})
	if err != nil {
		return "", fmt.Errorf("failed to encode image file: %v", err)
	}

	return dominantColor, nil
}

func getDominantColor(img image.Image) (string, error) {
	// get the dominant color of the image
	resizeSize := uint(prominentcolor.DefaultSize)
	bgmasks := prominentcolor.GetDefaultMasks()
	dominantColors, err := prominentcolor.KmeansWithAll(3, img, prominentcolor.ArgumentDefault, resizeSize, bgmasks)
	if err != nil {
		return "", fmt.Errorf("failed to determine dominant color for image: %v", err)
	}
	if len(dominantColors) == 0 {
		return "", fmt.Errorf("no colors found in image")
	}

	// sort descending by count
	sort.Slice(dominantColors, func(i, j int) bool {
		return dominantColors[i].Cnt > dominantColors[j].Cnt
	})

	// select both the top overall and the top within brightness range
	top := dominantColors[0].Color
	for _, c := range dominantColors {
		if isWithinBrightnessRange(rgbaColor(c.Color), 0.2, 0.8) {
			top = c.Color
			break
		}
	}

	r, g, b, _ := top.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", uint8(r), uint8(g), uint8(b)), nil
}

func rgbaColor(c color.Color) color.RGBA {
	r, g, b, _ := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), 0xff}
}

func isWithinBrightnessRange(c color.RGBA, minBrightness, maxBrightness float64) bool {
	brightness := calculateBrightness(c)
	return brightness >= minBrightness && brightness <= maxBrightness
}

func calculateBrightness(c color.RGBA) float64 {
	return 0.299*float64(c.R) + 0.587*float64(c.G) + 0.114*float64(c.B)/255
}
