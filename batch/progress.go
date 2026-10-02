package batch

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	barWidth       = 30
	redrawInterval = 100 * time.Millisecond
)

// progress tracks finished jobs, drawing a single-line bar on a terminal
// and falling back to one line per file when output is piped or redirected
type progress struct {
	mu          sync.Mutex
	total       int
	done        int
	failed      int
	start       time.Time
	interactive bool
	lastDraw    time.Time
	lastLen     int
}

func newProgress(total int) *progress {
	return &progress{
		total:       total,
		start:       time.Now(),
		interactive: isTerminal(os.Stdout),
	}
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

// record a finished job
func (p *progress) finish(outPath string, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.done++
	if err != nil {
		p.failed++
	}

	if !p.interactive {
		if err == nil {
			fmt.Printf("Saved converted file as: %s\n", outPath)
		}
		return
	}

	//throttle redraws so thousands of tiny files don't flood the terminal
	if p.done < p.total && time.Since(p.lastDraw) < redrawInterval {
		return
	}
	p.draw()
}

func (p *progress) draw() {
	p.lastDraw = time.Now()

	filled := barWidth * p.done / p.total
	bar := strings.Repeat("#", filled) + strings.Repeat("-", barWidth-filled)
	line := fmt.Sprintf("[%s] %d/%d %3d%%  %s", bar, p.done, p.total, 100*p.done/p.total, p.eta())
	if p.failed > 0 {
		line += fmt.Sprintf("  %d failed", p.failed)
	}

	//pad with spaces instead of ANSI clear-line, which older Windows consoles print literally
	pad := max(0, p.lastLen-len(line))
	fmt.Print("\r" + line + strings.Repeat(" ", pad))
	p.lastLen = len(line)
}

func (p *progress) eta() string {
	if p.done == p.total {
		return "done in " + time.Since(p.start).Round(time.Second).String()
	}
	if p.done == 0 {
		return "ETA --"
	}
	elapsed := time.Since(p.start)
	remaining := elapsed / time.Duration(p.done) * time.Duration(p.total-p.done)
	return "ETA " + remaining.Round(time.Second).String()
}

// draw the final state, which a throttled redraw may have skipped, and end the bar line
func (p *progress) close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.interactive && p.done > 0 {
		p.draw()
		fmt.Println()
	}
}
