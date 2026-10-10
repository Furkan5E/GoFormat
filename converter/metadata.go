package converter

import (
	"os"
	"time"

	"github.com/rwcarlsen/goexif/exif"
)

type ImageMetadata struct {
	//when the picture was taken, or when the file was last modified if the image does not say
	Timestamp    time.Time
	HasTimestamp bool
	//EXIF orientation tag (1 to 8), or 0 when the image has none
	Orientation int
}

func extractMetadata(path string) ImageMetadata {
	var meta ImageMetadata

	file, err := os.Open(path)
	if err != nil {
		return meta
	}
	defer file.Close()

	//png, gif, bmp and most other formats carry no capture date, their file time is the best there is
	if info, err := file.Stat(); err == nil {
		meta.Timestamp = info.ModTime()
		meta.HasTimestamp = true
	}

	//parse EXIF data
	x, err := exif.Decode(file)
	if err != nil {
		return meta
	}

	if tag, err := x.Get(exif.Orientation); err == nil {
		if v, err := tag.Int(0); err == nil {
			meta.Orientation = v
		}
	}

	//extract original DateTime capture
	if tm, err := x.DateTime(); err == nil {
		meta.Timestamp = tm
	}

	return meta
}
