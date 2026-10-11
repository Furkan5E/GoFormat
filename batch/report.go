package batch

import (
	"encoding/json"
	"io"

	"github.com/Furkan5E/GoFormat/v2/converter"
)

// what happened to a file, or would happen on a dry run
const (
	StatusConverted    = "converted"
	StatusSkipped      = "skipped"
	StatusFailed       = "failed"
	StatusWouldConvert = "would_convert"
	StatusWouldSkip    = "would_skip"
)

// Report is the machine-readable account of a run, written as JSON for scripts to read
type Report struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Format string `json:"format"`
	//nothing was written, the statuses say what a real run would do
	DryRun bool `json:"dry_run"`
	//the run was interrupted, so files may be missing from the list
	Cancelled bool `json:"cancelled"`
	//set when the input directory could not be read in full
	Error     string       `json:"error,omitempty"`
	ElapsedMs int64        `json:"elapsed_ms"`
	Summary   Summary      `json:"summary"`
	Files     []FileReport `json:"files"`
}

type Summary struct {
	//images found, each of which is one entry in the file list
	Total int `json:"total"`
	//converted, or on a dry run the number that would be
	Converted int `json:"converted"`
	//how many of the converted files replaced one that already existed
	Replaced int `json:"replaced"`
	//left alone because the output already existed
	Skipped int `json:"skipped"`
	Failed  int `json:"failed"`
	//files in the input that are not images GoFormat can read
	Unsupported int `json:"unsupported"`
	Warnings    int `json:"warnings"`
}

type FileReport struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Status string `json:"status"`
	//the output path already held a file, which was replaced
	Replaced bool     `json:"replaced"`
	Error    string   `json:"error,omitempty"`
	Warnings []string `json:"warnings,omitempty"`

	//sizes in pixels and bytes before and after, left out when the file was not converted
	SourceWidth  int   `json:"source_width,omitempty"`
	SourceHeight int   `json:"source_height,omitempty"`
	Width        int   `json:"width,omitempty"`
	Height       int   `json:"height,omitempty"`
	InputBytes   int64 `json:"input_bytes,omitempty"`
	OutputBytes  int64 `json:"output_bytes,omitempty"`
	ElapsedMs    int64 `json:"elapsed_ms"`
}

// NewFileReport describes the outcome of converting input to output
func NewFileReport(input, output string, res converter.Result, err error) FileReport {
	f := FileReport{Input: input, Output: output}
	switch {
	case err != nil:
		f.Status = StatusFailed
		f.Error = err.Error()
		return f
	case res.DryRun && res.Skipped:
		f.Status = StatusWouldSkip
	case res.DryRun:
		f.Status = StatusWouldConvert
	case res.Skipped:
		f.Status = StatusSkipped
	default:
		f.Status = StatusConverted
	}

	f.Replaced = res.Replaced
	f.Warnings = res.Warnings
	f.SourceWidth, f.SourceHeight = res.SrcSize.X, res.SrcSize.Y
	f.Width, f.Height = res.OutSize.X, res.OutSize.Y
	f.InputBytes, f.OutputBytes = res.InBytes, res.OutBytes
	f.ElapsedMs = res.Elapsed.Milliseconds()
	return f
}

// Add appends a file and counts it in the summary
func (r *Report) Add(f FileReport) {
	r.Files = append(r.Files, f)
	r.Summary.Total++
	r.Summary.Warnings += len(f.Warnings)
	switch f.Status {
	case StatusFailed:
		r.Summary.Failed++
	case StatusSkipped, StatusWouldSkip:
		r.Summary.Skipped++
	default:
		r.Summary.Converted++
		if f.Replaced {
			r.Summary.Replaced++
		}
	}
}

// Write prints the report as indented JSON
func (r Report) Write(w io.Writer) error {
	//an empty list reads better than null for whoever parses it
	if r.Files == nil {
		r.Files = []FileReport{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
