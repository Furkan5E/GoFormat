package converter

import (
	"fmt"
	"image"
	"strings"

	"golang.org/x/image/draw"
)

// Fit says how an image is fitted into a box when both Width and Height are set
type Fit string

const (
	//resize to exactly the box, distorting the image if its proportions differ
	FitStretch Fit = "stretch"
	//keep the proportions and the whole image, making it as large as fits inside the box
	FitContain Fit = "contain"
	//keep the proportions and fill the box exactly, cropping what does not fit from the edges
	FitCover Fit = "cover"
)

// ParseFit turns a name into a Fit, with the empty name meaning FitStretch
func ParseFit(name string) (Fit, error) {
	switch f := Fit(strings.ToLower(name)); f {
	case "":
		return FitStretch, nil
	case FitStretch, FitContain, FitCover:
		return f, nil
	}
	return "", fmt.Errorf("unknown fit '%s', choose stretch, contain or cover", name)
}

// resizePlan is what the size options come to for one image:
// the size of the output, and the part of the source that is scaled to fill it
type resizePlan struct {
	width, height int
	//relative to the top left corner of the source
	crop image.Rectangle
}

func planResize(origW, origH int, opts Options) (resizePlan, error) {
	//empty images are valid for some decoders (e.g. bmp) but cannot be scaled
	if origW == 0 || origH == 0 {
		return resizePlan{}, fmt.Errorf("cannot resize empty %dx%d image", origW, origH)
	}

	targetW, targetH := opts.Width, opts.Height

	//keep aspect ratio, rounding to nearest and never below 1px so thin images stay encodable
	if targetW == 0 {
		targetW = max(1, (origW*targetH+origH/2)/origH)
	}
	if targetH == 0 {
		targetH = max(1, (origH*targetW+origW/2)/origW)
	}

	//prevent massive allocations
	const maxDim = 16384
	if targetW > maxDim || targetH > maxDim {
		return resizePlan{}, fmt.Errorf("target resolution %dx%d exceeds maximum limit of %d", targetW, targetH, maxDim)
	}

	crop := image.Rect(0, 0, origW, origH)

	//with both sides given the box rarely has the image's proportions, Fit says what gives way
	//the source is wider than the box when origW/origH > targetW/targetH, compared here without dividing
	wider := origW*targetH > origH*targetW
	if opts.Width > 0 && opts.Height > 0 {
		switch opts.Fit {
		case FitContain:
			//the side that would overflow fills the box, the other one shrinks to keep the proportions
			if wider {
				targetH = max(1, (origH*targetW+origW/2)/origW)
			} else {
				targetW = max(1, (origW*targetH+origH/2)/origH)
			}
		case FitCover:
			//keep the middle of the image in the box's proportions and trim the rest equally from both sides
			if wider {
				w := min(origW, max(1, (origH*targetW+targetH/2)/targetH))
				crop.Min.X = (origW - w) / 2
				crop.Max.X = crop.Min.X + w
			} else {
				h := min(origH, max(1, (origW*targetH+targetW/2)/targetW))
				crop.Min.Y = (origH - h) / 2
				crop.Max.Y = crop.Min.Y + h
			}
		}
	}

	return resizePlan{width: targetW, height: targetH, crop: crop}, nil
}

func resizeImage(src image.Image, opts Options) (image.Image, error) {
	bounds := src.Bounds()
	plan, err := planResize(bounds.Dx(), bounds.Dy(), opts)
	if err != nil {
		return nil, err
	}

	dst := image.NewRGBA(image.Rect(0, 0, plan.width, plan.height))
	from := plan.crop.Add(bounds.Min)

	if opts.Pixelart {
		draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, from, draw.Over, nil)
	} else {
		draw.BiLinear.Scale(dst, dst.Bounds(), src, from, draw.Over, nil)
	}

	return dst, nil
}
