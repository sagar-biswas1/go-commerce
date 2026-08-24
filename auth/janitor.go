package auth

import (
	"context"
	"log"
	"sync"
	"time"
)

// Janitor deletes refresh tokens that can never be exchanged again.
//
// Without it the table only grows: every login and every rotation adds a row,
// and a busy week of a mobile client refreshing every fifteen minutes leaves
// thousands of dead rows behind the handful of live ones. That is a slower index
// on the one lookup that happens on every refresh.
type Janitor struct {
	tokens TokenStore

	// interval is how often the sweep runs, retention how far past expiry a row
	// is kept. Retention is not politeness: a revoked token is what reuse
	// detection matches against, so deleting it the moment it expires would
	// turn a detectable replay into an unremarkable "not found".
	interval  time.Duration
	retention time.Duration

	stop sync.Once
	done chan struct{}
}

// Defaults chosen so a sweep is cheap and a replay stays detectable for well
// past any plausible client clock skew or retry.
const (
	DefaultCleanupInterval  = 1 * time.Hour
	DefaultCleanupRetention = 30 * 24 * time.Hour
)

var (
	janitorOnce     sync.Once
	janitorInstance *Janitor
)

// GetJanitor returns the process-wide janitor. One is the right number: two
// would sweep the same rows on their own schedules, doing the work twice to
// delete the same nothing.
func GetJanitor(tokens TokenStore) *Janitor {
	janitorOnce.Do(func() {
		janitorInstance = NewJanitor(tokens, DefaultCleanupInterval, DefaultCleanupRetention)
	})
	return janitorInstance
}

func NewJanitor(tokens TokenStore, interval, retention time.Duration) *Janitor {
	if interval <= 0 {
		interval = DefaultCleanupInterval
	}
	if retention <= 0 {
		retention = DefaultCleanupRetention
	}

	return &Janitor{
		tokens:    tokens,
		interval:  interval,
		retention: retention,
		done:      make(chan struct{}),
	}
}

// Start runs the sweep loop until ctx is cancelled or Stop is called. It
// returns immediately; the loop runs in its own goroutine.
//
// It sweeps once on startup rather than waiting out the first interval, so a
// process that is restarted more often than the interval still cleans up.
func (j *Janitor) Start(ctx context.Context) {
	if j == nil || j.tokens == nil {
		return
	}

	go func() {
		ticker := time.NewTicker(j.interval)
		defer ticker.Stop()

		j.sweep(ctx)

		for {
			select {
			case <-ctx.Done():
				log.Printf("[janitor] stopping: %v", ctx.Err())
				return
			case <-j.done:
				log.Print("[janitor] stopping")
				return
			case <-ticker.C:
				j.sweep(ctx)
			}
		}
	}()

	log.Printf("[janitor] sweeping expired refresh tokens every %s, keeping %s past expiry",
		j.interval, j.retention)
}

// Stop ends the loop. It is safe to call more than once, and from more than one
// goroutine, which is what makes it usable from a shutdown path that may itself
// be racing a context cancellation.
func (j *Janitor) Stop() {
	if j == nil {
		return
	}
	j.stop.Do(func() { close(j.done) })
}

// sweepTimeout bounds one deletion. It is far shorter than the interval so a
// slow sweep cannot still be running when the next one starts, and so a stuck
// query is abandoned rather than held for an hour.
const sweepTimeout = 2 * time.Minute

// sweep runs one deletion. A failure is logged and the loop continues: the
// janitor is housekeeping, and a database hiccup should not end it for the life
// of the process.
func (j *Janitor) sweep(ctx context.Context) {
	sweepCtx, cancel := context.WithTimeout(ctx, sweepTimeout)
	defer cancel()

	deleted, err := j.tokens.DeleteExpired(sweepCtx, j.retention)
	switch {
	case err != nil:
		log.Printf("[janitor] sweep failed: %v", err)
	case deleted > 0:
		log.Printf("[janitor] deleted %d expired refresh token(s)", deleted)
	}
}
