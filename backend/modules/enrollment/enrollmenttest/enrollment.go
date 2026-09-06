// Package enrollmenttest composes the Enrollment owner for behavior tests.
package enrollmenttest

import (
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
)

func New() *enrollment.Module { return compose.New() }

type Module = enrollment.Module

// Phase and Date let shared fixtures use owner values without a production dependency.
type Phase = enrollment.Phase
type Date = enrollment.Date

type Request = enrollment.Request
type RequestChild = enrollment.RequestChild
type RequestChildOffering = enrollment.RequestChildOffering

const ChildStatusSubmitted = enrollment.ChildStatusSubmitted
