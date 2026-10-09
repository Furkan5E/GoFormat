package format

import (
	"image"
	"image/color"
	"sort"
)

// PaletteFor returns a GIF palette for a flattened image, with TransparentIndex reserved
// exact is true when every colour of the image is in it, otherwise the image needs dithering
func PaletteFor(flat *image.NRGBA) (pal color.Palette, exact bool) {
	b := NewPaletteBuilder()
	for i := 0; i < len(flat.Pix) && !b.Full(); i += 4 {
		b.Add(color.NRGBA{flat.Pix[i], flat.Pix[i+1], flat.Pix[i+2], flat.Pix[i+3]})
	}
	if pal, exact := b.Palette(); exact {
		return pal, true
	}
	return adaptivePalette(flat), false
}

// cell is a group of near-identical colours and how many pixels have them
type cell struct {
	r, g, b uint64 //sums over the pixels, divided by n for the average
	n       uint64
}

func (c cell) channel(i int) uint64 {
	return [3]uint64{c.r, c.g, c.b}[i] / c.n
}

// adaptivePalette picks the 255 colours that represent the image best, using median cut
// the colours are split into boxes, always halving the box that covers the most pixels and the widest range,
// so areas the image uses heavily get more palette entries than a fixed palette would give them
func adaptivePalette(flat *image.NRGBA) color.Palette {
	//colours are grouped by the top five bits of each channel to keep the work independent of image size
	grid := make([]cell, 1<<15)
	for i := 0; i < len(flat.Pix); i += 4 {
		if flat.Pix[i+3] == 0 {
			continue
		}
		r, g, b := flat.Pix[i], flat.Pix[i+1], flat.Pix[i+2]
		c := &grid[int(r>>3)<<10|int(g>>3)<<5|int(b>>3)]
		c.r += uint64(r)
		c.g += uint64(g)
		c.b += uint64(b)
		c.n++
	}

	var cells []cell
	for _, c := range grid {
		if c.n > 0 {
			cells = append(cells, c)
		}
	}

	boxes := [][]cell{cells}
	for len(boxes) < 255 {
		//pick the box most worth splitting, and the channel it spreads furthest along
		best, bestAxis, bestScore := -1, 0, uint64(0)
		for i, box := range boxes {
			if len(box) < 2 {
				continue
			}
			axis, spread := widestAxis(box)
			if score := (spread + 1) * pixelCount(box); score > bestScore {
				best, bestAxis, bestScore = i, axis, score
			}
		}
		if best < 0 {
			break
		}

		box := boxes[best]
		sort.Slice(box, func(i, j int) bool { return box[i].channel(bestAxis) < box[j].channel(bestAxis) })

		//cut where half the pixels are on each side, keeping at least one cell in both halves
		half, seen, cut := pixelCount(box)/2, uint64(0), 1
		for i, c := range box[:len(box)-1] {
			seen += c.n
			cut = i + 1
			if seen >= half {
				break
			}
		}
		boxes[best] = box[:cut]
		boxes = append(boxes, box[cut:])
	}

	pal := color.Palette{color.NRGBA{}}
	for _, box := range boxes {
		var sum cell
		for _, c := range box {
			sum.r += c.r
			sum.g += c.g
			sum.b += c.b
			sum.n += c.n
		}
		pal = append(pal, color.NRGBA{uint8(sum.r / sum.n), uint8(sum.g / sum.n), uint8(sum.b / sum.n), 255})
	}
	return pal
}

func pixelCount(box []cell) uint64 {
	var n uint64
	for _, c := range box {
		n += c.n
	}
	return n
}

// widestAxis returns the channel (0 red, 1 green, 2 blue) with the largest range in box, and that range
func widestAxis(box []cell) (axis int, spread uint64) {
	for i := 0; i < 3; i++ {
		lo, hi := uint64(255), uint64(0)
		for _, c := range box {
			v := c.channel(i)
			lo, hi = min(lo, v), max(hi, v)
		}
		if hi-lo > spread {
			axis, spread = i, hi-lo
		}
	}
	return axis, spread
}
