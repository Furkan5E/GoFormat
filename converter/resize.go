package converter

import (
	"fmt"
	"image"

	"golang.org/x/image/draw"
)

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

	return resizePlan{width: targetW, height: targetH, crop: image.Rect(0, 0, origW, origH)}, nil
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
