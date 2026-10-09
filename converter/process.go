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
	"time"

	"github.com/Furkan5E/GoFormat/v2/format"

	"golang.org/x/image/draw"
)

// ErrOverwritesInput is returned when the output path is the input file itself
var ErrOverwritesInput = errors.New("output would overwrite the input, choose another output directory or format")

// ErrTooLarge is returned for an input with more pixels than Options.MaxPixels
var ErrTooLarge = errors.New("raise the limit with -max-megapixels to convert it")

// Result describes a successful conversion
// warnings are returned rather than printed so batch mode can report them without breaking the progress bar
type Result struct {
	InPath  string
	OutPath string
	//nothing was written, the result says what a real run would do
	DryRun bool
	//a file was already at OutPath and has been replaced
	Replaced bool
	//a file was already at OutPath and has been left alone
	Skipped  bool
	Warnings []string

	//pixel sizes before and after, file sizes in bytes and the time taken, shown in verbose output
	SrcSize  image.Point
	OutSize  image.Point
	InBytes  int64
	OutBytes int64
	Elapsed  time.Duration
}

// Verbose is the detailed line printed for a finished file with -v
func (r Result) Verbose() string {
	if r.DryRun || r.Skipped {
		return r.Summary()
	}
	line := fmt.Sprintf("Converted %s -> %s (%dx%d -> %dx%d, %s -> %s, %s)", r.InPath, r.OutPath,
		r.SrcSize.X, r.SrcSize.Y, r.OutSize.X, r.OutSize.Y,
		formatBytes(r.InBytes), formatBytes(r.OutBytes), r.Elapsed.Round(time.Millisecond))
	if r.Replaced {
		line += " (replaced existing file)"
	}
	return line
}

func formatBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f kB", float64(n)/(1<<10))
	}
	return fmt.Sprintf("%d B", n)
}

// Summary is the one line printed for a finished file
func (r Result) Summary() string {
	switch {
	case r.DryRun && r.Skipped:
		return fmt.Sprintf("Would skip existing file: %s", r.OutPath)
	case r.DryRun && r.Replaced:
		return fmt.Sprintf("Would convert: %s -> %s (replacing existing file)", r.InPath, r.OutPath)
	case r.DryRun:
		return fmt.Sprintf("Would convert: %s -> %s", r.InPath, r.OutPath)
	case r.Skipped:
		return fmt.Sprintf("Skipped existing file: %s", r.OutPath)
	}
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
	//write lossless webp instead of lossy
	Lossless bool
	//target size in pixels, 0 keeps the original
	Width    int
	Height   int
	Pixelart bool
	//leave files that already exist at the output path alone instead of replacing them
	SkipExisting bool
	//work out what would happen to each file without writing anything
	DryRun bool
	//largest input accepted, in pixels (width times height), 0 for no limit
	MaxPixels int64
	//shared by concurrent conversions to bound their combined memory use, may be nil
	Memory *Budget
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

	replaced, err := checkOutput(inputPath, outPath, opts)
	if err != nil {
		return Result{}, err
	}
	if replaced && opts.SkipExisting {
		return Result{InPath: inputPath, OutPath: outPath, Skipped: true, DryRun: opts.DryRun}, nil
	}
	if opts.DryRun {
		return Result{InPath: inputPath, OutPath: outPath, Replaced: replaced, DryRun: true}, nil
	}

	//the size is read from the header, so an oversized image is turned away before any of it is decoded
	cfg, err := loadConfig(inputPath)
	if err != nil {
		return Result{}, fmt.Errorf("failed to load %s: %w", inputPath, err)
	}
	pixels := int64(cfg.Width) * int64(cfg.Height)
	if opts.MaxPixels > 0 && pixels > opts.MaxPixels {
		return Result{}, fmt.Errorf("%s is %dx%d (%d megapixels), over the limit of %d megapixels: %w",
			inputPath, cfg.Width, cfg.Height, pixels/1_000_000, opts.MaxPixels/1_000_000, ErrTooLarge)
	}
	if opts.Memory != nil {
		reserved, err := opts.Memory.Acquire(ctx, pixels)
		if err != nil {
			return Result{}, err
		}
		defer opts.Memory.Release(reserved)
	}

	start := time.Now()
	res := Result{InPath: inputPath, OutPath: outPath, Replaced: replaced}
	outFormat := strings.ToLower(opts.Format)
	ext := strings.ToLower(filepath.Ext(inputPath))

	//extract historical metadata
	meta := extractMetadata(inputPath)

	if ext == ".gif" && outFormat == "gif" {
		//keep every frame instead of collapsing the animation to a single image
		res.SrcSize, res.OutSize, err = processGIFToGIF(inputPath, outPath, opts)
		if err != nil {
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
		res.SrcSize = img.Bounds().Size()

		if opts.Width > 0 || opts.Height > 0 {
			img, err = resizeImage(img, opts.Width, opts.Height, opts.Pixelart)
			if err != nil {
				return Result{}, fmt.Errorf("failed to resize %s: %w", inputPath, err)
			}
		}
		res.OutSize = img.Bounds().Size()

		if err := saveImage(img, outPath, enc, format.Options{Quality: opts.Quality, Lossless: opts.Lossless}); err != nil {
			return Result{}, fmt.Errorf("failed to save %s: %w", outPath, err)
		}
	}

	if info, err := os.Stat(inputPath); err == nil {
		res.InBytes = info.Size()
	}
	if info, err := os.Stat(outPath); err == nil {
		res.OutBytes = info.Size()
	}
	res.Elapsed = time.Since(start)

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
func checkOutput(inputPath, outPath string, opts Options) (exists bool, err error) {
	outInfo, err := os.Stat(outPath)
	if err != nil {
		//anything other than a missing file is left for the write to report
		return false, nil
	}
	if outInfo.IsDir() {
		return false, fmt.Errorf("cannot write %s: a directory has that name", outPath)
	}
	//a skipped file is not written to, so it is safe even when it is the input
	if opts.SkipExisting {
		return true, nil
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

func loadConfig(path string) (image.Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return image.Config{}, err
	}
	defer file.Close()

	cfg, _, err := image.DecodeConfig(file)
	return cfg, err
}

func generateOutputPath(inputPath, outDir, targetFormat string) string {
	ext := filepath.Ext(inputPath)
	baseName := strings.TrimSuffix(filepath.Base(inputPath), ext)
	fileName := fmt.Sprintf("%s.%s", baseName, targetFormat)
	return filepath.Join(outDir, fileName)
}

func saveImage(img image.Image, path string, enc format.Encoder, opts format.Options) error {
	return writeFile(path, func(w io.Writer) error {
		return enc.Encode(w, img, opts)
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
