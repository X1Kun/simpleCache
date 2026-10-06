package cache

import "context"

// One limiter is shared by all groups created by a server.
type SourceLimiter struct{ slots chan struct{} }

func NewSourceLimiter(limit int) *SourceLimiter {
	if limit <= 0 {
		panic("source concurrency must be positive")
	}
	return &SourceLimiter{slots: make(chan struct{}, limit)}
}
func (l *SourceLimiter) acquire(ctx context.Context) error {
	select {
	case l.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func (l *SourceLimiter) release() { <-l.slots }
