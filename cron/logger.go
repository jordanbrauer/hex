package cron

import (
	robcron "github.com/robfig/cron/v3"

	hexlog "github.com/jordanbrauer/hex/log"
)

// logger adapts hex/log to robfig/cron's Logger interface so scheduler
// internals (job runs, panics, missed ticks) flow through hex's logger.
type logger struct{}

func newLogger() robcron.Logger { return logger{} }

// Info forwards to hexlog.Debug — the underlying library emits routine "job
// scheduled" / "wake" messages at Info, which is too loud for typical apps.
// Consumers who want them can lift the level via --log-level=debug.
func (logger) Info(msg string, keysAndValues ...any) {
	hexlog.Debug("cron: "+msg, keysAndValues...)
}

// Error forwards to hexlog.Error with the error tagged as a key.
func (logger) Error(err error, msg string, keysAndValues ...any) {
	kv := append([]any{"error", err}, keysAndValues...)
	hexlog.Error("cron: "+msg, kv...)
}
