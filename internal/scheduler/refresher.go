package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/robfig/cron/v3"

	"datablox/internal/service"
)

type Refresher struct {
	log  *slog.Logger
	svc  *service.ExperienceService
	cron *cron.Cron
}

func New(log *slog.Logger, svc *service.ExperienceService, interval time.Duration, enabled bool) *Refresher {
	c := cron.New(cron.WithSeconds())
	r := &Refresher{log: log, svc: svc, cron: c}
	if enabled && interval > 0 {
		spec := fmtEverySeconds(interval)
		if _, err := c.AddFunc(spec, r.run); err != nil {
			log.Warn("scheduler failed to start", "err", err)
		}
		c.Start()
	}
	return r
}

func (r *Refresher) Stop() context.Context {
	return r.cron.Stop()
}

func (r *Refresher) run() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	n, err := r.svc.RefreshAll(ctx)
	if err != nil {
		r.log.Warn("refresh all failed", "err", err)
		return
	}
	if n > 0 {
		r.log.Info("background refresh selesai", "updated", n)
	}
}

// fmtEverySeconds builds a cron spec firing every d (rounded to seconds, min 1s).
func fmtEverySeconds(d time.Duration) string {
	secs := int(d.Round(time.Second) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return fmt.Sprintf("@every %ds", secs)
}