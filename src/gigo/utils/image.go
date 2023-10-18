package utils

import (
	"fmt"
	"golang.org/x/image/draw"
	"image"
	"image/jpeg"
	"image/png"
	"io"
)

// PrepImageFile
//
//	Reads an image file from the disk, validates that it is an image file and writes the cleaned image file
//	to the disk as a jpeg so that no EXIF data is stored within the image
//
//	Args
//		src		- io.ReadCloser, source file that will be loaded as an image
//		dst 	- io.WriteCloser, destination file that the sanitized image will be written to (in jpeg format)
func PrepImageFile(src io.ReadCloser, dst io.WriteCloser) error {
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
				return err
			}
			return fmt.Errorf("failed to decode image file: %v", err)
		}
		img = pngImg
	}

	// create a new image to hold the resized image matrix of 500x500 pixels
	resized := image.NewRGBA(image.Rect(0, 0, 896, 504))

	// resize the image to 500x500
	draw.CatmullRom.Scale(resized, resized.Rect, img, img.Bounds(), draw.Over, nil)

	// write the image as a new jpeg file
	err = jpeg.Encode(dst, resized, &jpeg.Options{Quality: 85})
	if err != nil {
		return fmt.Errorf("failed to encode image file: %v", err)
	}

	return nil
}
