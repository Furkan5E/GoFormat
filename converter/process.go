package converter

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/Furkan5E/GoFormat/v2/format"

	"golang.org/x/image/draw"
)

// ErrOverwritesInput is returned when the output path is the input file itself
var ErrOverwritesInput = errors.New("output would overwrite the input, choose another output directory or format")

// Result describes a successful conversion
// warnings are returned rather than printed so batch mode can report them without breaking the progress bar
type Result struct {
	OutPath string
	//a file was already at OutPath and has been replaced
	Replaced bool
	Warnings []string
}

// Summary is the one line printed for a finished file
func (r Result) Summary() string {
	if r.Replaced {
		return fmt.Sprintf("Saved converted file as: %s (replaced existing file)", r.OutPath)
	}
	return fmt.Sprintf("Saved converted file as: %s", r.OutPath)
}

// Options are the settings shared by every file of a run
type Options struct {
	//target format name, such as jpeg or png
	Format  string
	Quality int
	//target size in pixels, 0 keeps the original
	Width    int
	Height   int
	Pixelart bool
}

func ProcessImage(ctx context.Context, inputPath string, outDir string, opts Options) (Result, error) {
	outPath := generateOutputPath(inputPath, outDir, strings.ToLower(opts.Format))
	return ProcessImageTo(ctx, inputPath, outPath, opts)
}

// ProcessImageTo converts to an exact output path, for callers that choose the file name themselves
func ProcessImageTo(ctx context.Context, inputPath string, outPath string, opts Options) (Result, error) {
	if ctx.Err() != nil {
		return Result{}, ctx.Err()
	}

	replaced, err := checkOutput(inputPath, outPath)
	if err != nil {
		return Result{}, err
	}

	outFormat := strings.ToLower(opts.Format)
	ext := strings.ToLower(filepath.Ext(inputPath))

	//extract historical metadata
	meta := extractMetadata(inputPath)

	if ext == ".gif" && outFormat == "gif" {
		//keep every frame instead of collapsing the animation to a single image
		if err := processGIFToGIF(inputPath, outPath, opts); err != nil {
			return Result{}, fmt.Errorf("failed to process %s: %w", inputPath, err)
		}
	} else {
		enc, err := format.GetEncoder(outFormat)
		if err != nil {
			return Result{}, err
		}

		img, err := loadImage(inputPath)
		if err != nil {
			return Result{}, fmt.Errorf("failed to load %s: %w", inputPath, err)
		}

		//the heic and avif decoders already turn the image themselves
		if ext != ".heic" && ext != ".heif" && ext != ".avif" {
			img = applyOrientation(img, meta.Orientation)
		}

		if opts.Width > 0 || opts.Height > 0 {
			img, err = resizeImage(img, opts.Width, opts.Height, opts.Pixelart)
			if err != nil {
				return Result{}, fmt.Errorf("failed to resize %s: %w", inputPath, err)
			}
		}

		if err := saveImage(img, outPath, enc, opts.Quality); err != nil {
			return Result{}, fmt.Errorf("failed to save %s: %w", outPath, err)
		}
	}

	res := Result{OutPath: outPath, Replaced: replaced}

	//reinject metadata
	if meta.HasMetadata {
		if err := os.Chtimes(outPath, meta.Timestamp, meta.Timestamp); err != nil {
			res.Warnings = append(res.Warnings, fmt.Sprintf("failed to preserve timestamp for %s: %v", outPath, err))
		}
	}

	return res, nil
}

// checkOutput reports whether a file is already at outPath, and refuses to convert a file onto itself
// that happens when the output directory is the input's own and the format is unchanged, and would destroy the original
func checkOutput(inputPath, outPath string) (exists bool, err error) {
	outInfo, err := os.Stat(outPath)
	if err != nil {
		//anything other than a missing file is left for the write to report
		return false, nil
	}
	if outInfo.IsDir() {
		return false, fmt.Errorf("cannot write %s: a directory has that name", outPath)
	}
	if inInfo, err := os.Stat(inputPath); err == nil && os.SameFile(inInfo, outInfo) {
		return false, fmt.Errorf("%s: %w", inputPath, ErrOverwritesInput)
	}
	return true, nil
}

func resizeImage(src image.Image, targetW, targetH int, pixelart bool) (image.Image, error) {
	bounds := src.Bounds()
	origW := bounds.Dx()
	origH := bounds.Dy()

	//empty images are valid for some decoders (e.g. bmp) but cannot be scaled
	if origW == 0 || origH == 0 {
		return nil, fmt.Errorf("cannot resize empty %dx%d image", origW, origH)
	}

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
		return nil, fmt.Errorf("target resolution %dx%d exceeds maximum limit of %d", targetW, targetH, maxDim)
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))

	if pixelart {
		draw.NearestNeighbor.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	} else {
		draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)
	}

	return dst, nil
}

func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

func generateOutputPath(inputPath, outDir, targetFormat string) string {
	ext := filepath.Ext(inputPath)
	baseName := strings.TrimSuffix(filepath.Base(inputPath), ext)
	fileName := fmt.Sprintf("%s.%s", baseName, targetFormat)
	return filepath.Join(outDir, fileName)
}

func saveImage(img image.Image, path string, enc format.Encoder, quality int) error {
	return writeFile(path, func(w io.Writer) error {
		return enc.Encode(w, img, quality)
	})
}

// writeFile encodes into a temporary file next to outPath and renames it into place once it is complete
// a failed conversion then never leaves a partial file behind or damages a file that was already there
func writeFile(outPath string, encode func(w io.Writer) error) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(outPath), filepath.Base(outPath)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	if err = encode(tmp); err != nil {
		return err
	}
	//temporary files are created private to the user, unlike a normal output file
	if err = tmp.Chmod(0o644); err != nil {
		return err
	}
	//a close error means the data may not have reached the disk
	if err = tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), outPath)
}
