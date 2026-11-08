//go:build testhooks

package api

import "github.com/moto-nrw/project-phoenix/modules/carecontract"

// ContractChangeDecisionForTest keeps cross-owner review fixtures on the typed root.
type ContractChangeDecisionForTest = carecontract.ChangeDecision

type ContractChangeDecisionInputForTest = carecontract.ChangeDecisionInput

type ContractChangeLineForTest = carecontract.ChangeLine

func ContractChangeNotFoundForTest() error { return carecontract.ErrChangeNotFound }
