package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/identityaccess/internal/domain"
)

// CommandTransactions selects the identity application unit of work. Global
// credential/operator commands are explicit; ordinary school reads never
// choose administrative access when their school is absent.
type CommandTransactions struct {
	attach       func(context.Context) context.Context
	current      func(context.Context) (any, bool)
	school       func(context.Context) int64
	withinSchool func(context.Context, func(context.Context) error) error
	withinAdmin  func(context.Context, func(context.Context) error) error
}

func NewCommandTransactions(attach func(context.Context) context.Context, current func(context.Context) (any, bool), school func(context.Context) int64, withinSchool, withinAdmin func(context.Context, func(context.Context) error) error) CommandTransactions {
	return CommandTransactions{attach: attach, current: current, school: school, withinSchool: withinSchool, withinAdmin: withinAdmin}
}
func (r CommandTransactions) RunWrite(ctx context.Context, fn func(context.Context) error) error {
	ctx = r.attach(ctx)
	if _, ok := r.current(ctx); ok {
		return fn(ctx)
	}
	if r.school(ctx) <= 0 {
		return domain.ErrTenantRequired
	}
	return r.withinSchool(ctx, fn)
}
func (r CommandTransactions) RunRead(ctx context.Context, fn func(context.Context) error) error {
	return r.RunWrite(ctx, fn)
}
func (r CommandTransactions) RunPlatform(ctx context.Context, fn func(context.Context) error) error {
	ctx = r.attach(ctx)
	if _, ok := r.current(ctx); ok {
		return fn(ctx)
	}
	if r.school(ctx) <= 0 {
		return r.withinAdmin(ctx, fn)
	}
	return r.withinSchool(ctx, fn)
}
func (r CommandTransactions) RunAdmin(ctx context.Context, fn func(context.Context) error) error {
	ctx = r.attach(ctx)
	if _, ok := r.current(ctx); ok {
		return fn(ctx)
	}
	return r.withinAdmin(ctx, fn)
}
