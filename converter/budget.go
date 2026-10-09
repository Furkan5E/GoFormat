package converter

import (
	"context"
	"sync"
)

// DefaultMaxPixels is the largest image decoded unless the caller raises it
// a decoded image takes about four bytes per pixel, so this is roughly 1 GB
const DefaultMaxPixels = 250_000_000

// Budget limits how many decoded pixels are held in memory at once
// workers share one, so a batch of huge images is converted a few at a time instead of all together
type Budget struct {
	mu    sync.Mutex
	cond  *sync.Cond
	limit int64
	used  int64
}

func NewBudget(limit int64) *Budget {
	b := &Budget{limit: limit}
	b.cond = sync.NewCond(&b.mu)
	return b
}

// Acquire waits until pixels fit in the budget and returns how many were reserved, to be handed back to Release
// an image larger than the whole budget reserves all of it, so it is converted on its own rather than never
func (b *Budget) Acquire(ctx context.Context, pixels int64) (int64, error) {
	pixels = min(pixels, b.limit)

	//wake the wait below when the run is cancelled
	stop := context.AfterFunc(ctx, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		b.cond.Broadcast()
	})
	defer stop()

	b.mu.Lock()
	defer b.mu.Unlock()
	for b.used+pixels > b.limit {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		b.cond.Wait()
	}
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	b.used += pixels
	return pixels, nil
}

func (b *Budget) Release(pixels int64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.used -= pixels
	b.cond.Broadcast()
}
