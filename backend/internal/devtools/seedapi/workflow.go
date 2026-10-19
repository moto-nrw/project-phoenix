package api

import (
	"context"
	"fmt"
)

type Step interface {
	Name() string
	Run(context.Context, *Runtime) error
}

type Workflow struct {
	Name  string
	Steps []Step
}

type StepError struct {
	Step string
	Err  error
}

func (e *StepError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("%s: %v", e.Step, e.Err)
}

func (e *StepError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (w Workflow) Run(ctx context.Context, rt *Runtime) error {
	for _, step := range w.Steps {
		if err := step.Run(ctx, rt); err != nil {
			return &StepError{
				Step: step.Name(),
				Err:  err,
			}
		}
	}
	return nil
}
