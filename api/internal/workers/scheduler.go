package workers

import (
	"context"
	"log/slog"
	"sync"

	"github.com/robfig/cron/v3"
)

type Job func(ctx context.Context) error

type Scheduler struct {
	mu     sync.Mutex
	cron   *cron.Cron
	logger *slog.Logger
	ctx    context.Context
}

func NewScheduler() *Scheduler {
	return &Scheduler{
		cron:   cron.New(),
		logger: slog.Default(),
	}
}

func (s *Scheduler) WithLogger(l *slog.Logger) *Scheduler {
	s.logger = l
	return s
}

func (s *Scheduler) Add(spec string, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.cron.AddFunc(spec, func() {
		ctx := s.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		if err := job(ctx); err != nil {
			s.logger.Error("scheduled job failed", "err", err, "spec", spec)
		}
	})
	return err
}

func (s *Scheduler) Start(ctx context.Context) {
	s.mu.Lock()
	s.ctx = ctx
	s.mu.Unlock()
	s.cron.Start()
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
}
