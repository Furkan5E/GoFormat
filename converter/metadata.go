package converter

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

type ImageMetadata struct {
	Timestamp   time.Time
	HasMetadata bool
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

	//extract original DateTime capture
	tm, err := x.DateTime()
	if err != nil {
		return ImageMetadata{HasMetadata: false}
	}

	return ImageMetadata{
		Timestamp:   tm,
		HasMetadata: true,
	}
}