package converter

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

type ImageMetadata struct {
	Timestamp   time.Time
	HasMetadata bool
	//EXIF orientation tag (1 to 8), or 0 when the image has none
	Orientation int
}

func extractMetadata(path string) ImageMetadata {
	file, err := os.Open(path)
	if err != nil {
		return ImageMetadata{HasMetadata: false}
	}
	defer file.Close()

	//parse EXIF data
	x, err := exif.Decode(file)
	if err != nil {
		return ImageMetadata{HasMetadata: false}
	}

	var meta ImageMetadata

	if tag, err := x.Get(exif.Orientation); err == nil {
		if v, err := tag.Int(0); err == nil {
			meta.Orientation = v
		}
	}

	//extract original DateTime capture
	if tm, err := x.DateTime(); err == nil {
		meta.Timestamp = tm
		meta.HasMetadata = true
	}

	return meta
}