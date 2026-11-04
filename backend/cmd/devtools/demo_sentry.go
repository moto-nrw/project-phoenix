package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/getsentry/sentry-go"
)

// The demo process reports an order that failed for good to Sentry: the
// visitor waits on a link that leads nowhere, and a log line alone went
// unnoticed for hours on 2026-10-27. Without SENTRY_DSN (development, CI)
// it reports nothing.

const demoSentryFlushTimeout = 2 * time.Second

// initDemoSentry starts the Sentry client for the demo process under the
// Sentry environment APP_ENV, as the server does. The returned func flushes
// pending events.
func initDemoSentry(dsn, appEnv string) (func(), error) {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return func() {}, nil
	}
	if err := sentry.Init(sentry.ClientOptions{Dsn: dsn, Environment: strings.TrimSpace(appEnv)}); err != nil {
		return nil, fmt.Errorf("initialize sentry: %w", err)
	}
	return func() { sentry.Flush(demoSentryFlushTimeout) }, nil
}

var (
	demoFailedStep = regexp.MustCompile(`workflow step (.+?) failed`)
	demoFailedCode = regexp.MustCompile(`\(([a-z_]+\.[a-z_]+)\)`)
)

// demoOrderFailure names what failed without the error text: that text
// carries request paths with link tokens and seeded e-mail addresses, which
// must not leave for Sentry. Step and code are enough to find the cause.
func demoOrderFailure(err error) (step, code string) {
	if err == nil {
		return "", ""
	}
	text := err.Error()
	if match := demoFailedStep.FindStringSubmatch(text); match != nil {
		step = match[1]
	}
	if match := demoFailedCode.FindStringSubmatch(text); match != nil {
		code = match[1]
	}
	return step, code
}

// reportDemoOrderFailed sends one event for an order closed as failed.
func reportDemoOrderFailed(slug string, err error) {
	step, code := demoOrderFailure(err)
	hub := sentry.CurrentHub().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetLevel(sentry.LevelError)
		scope.SetTag("component", "demo-runtime")
		scope.SetTag("demo_school", slug)
		scope.SetTag("seed_step", step)
		scope.SetTag("error_code", code)
		scope.SetFingerprint([]string{"demo-school-order-failed", step, code})
		hub.CaptureMessage(fmt.Sprintf("demo school order failed: %s (%s)", step, code))
	})
}
