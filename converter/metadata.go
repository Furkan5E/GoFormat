package converter

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"time"

	"github.com/Furkan5E/GoFormat/v2/format"

	"github.com/rwcarlsen/goexif/exif"
)

type ImageMetadata struct {
	//when the picture was taken, or when the file was last modified if the image does not say
	Timestamp    time.Time
	HasTimestamp bool
	//EXIF orientation tag (1 to 8), or 0 when the image has none
	Orientation int
	//the EXIF block and colour profile as stored in the file, for copying into the output
	Raw format.Metadata
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

	meta.Raw = format.ReadMetadataFile(path)

	//parse EXIF data, from the block found above or by letting the parser search the file itself (it knows tiff)
	var x *exif.Exif
	if len(meta.Raw.EXIF) > 0 {
		x, err = exif.Decode(bytes.NewReader(meta.Raw.EXIF))
	} else {
		x, err = exif.Decode(file)
	}
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

// outputMetadata is what of the source's metadata still holds for the converted image
func outputMetadata(meta ImageMetadata, decoded image.Image) format.Metadata {
	out := format.Metadata{
		//the pixels are saved upright, so the block must not tell viewers to turn them again
		EXIF: format.UprightEXIF(meta.Raw.EXIF),
	}
	if profileFits(meta.Raw.ICC, decoded) {
		out.ICC = meta.Raw.ICC
	}
	return out
}

// profileFits reports whether a colour profile still describes the image once it is saved
// output is always written as RGB, so a profile for print (CMYK) or greyscale values would be wrong for it
func profileFits(icc []byte, decoded image.Image) bool {
	//bytes 16 to 19 of a profile name the kind of colour values it is for
	if len(icc) < 20 || string(icc[16:20]) != "RGB " {
		return false
	}
	switch decoded.ColorModel() {
	case color.CMYKModel, color.GrayModel, color.Gray16Model:
		return false
	}
	return true
}
