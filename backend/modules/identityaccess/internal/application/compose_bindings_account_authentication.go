package application

import (
	"github.com/moto-nrw/project-phoenix/tenant"
)

func NewTransactionRunner() *tenant.TransactionRunner { return tenant.NewTransactionRunner() }
