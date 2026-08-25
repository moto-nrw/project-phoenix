package defaults

import (
	"github.com/moto-nrw/project-phoenix/models/config"
)

const (
	contractReadPermission  = "config:read"
	contractWritePermission = "config:manage"

	contractTab = "vertrag"

	contractCategoryContract = "vertrag"
	contractCategoryBilling  = "rechnung"
	contractCategoryContact  = "kontakt"
)

// Contract settings (#1459 demo surface): the commercial facts of a school's
// moto contract.
//
// Every key is AccessOperatorOnly. The moto team maintains these values in the
// operator portal (the auto-generated "Vertrag" tab in the per-school settings
// editor); the school reads them back read-only on /vertrag. A school admin
// must never be able to raise their own tier or child contingent, so the
// tenant schema hides them entirely and api/config rejects direct writes.
//
// Every default is the empty value on purpose. An unconfigured school shows
// "noch nicht hinterlegt" instead of an invented tier or price — the same
// reasoning as the DATEV Lohnarten in payroll.go: a plausible-looking preset
// would be read as fact and billed on.
//
// The payment schedule is NOT here. Due dates and payment status are a list,
// one row per invoice, and live in platform.school_invoices.
func init() {
	emailPattern := `^[^@\s]+@[^@\s]+\.[^@\s]+$`
	customerNumberPattern := `^[A-Za-z0-9._\-/ ]{1,32}$`

	minChildren, maxChildren := 0.0, 10000.0
	minPrice, maxPrice := 0.0, 1000000.0

	config.Register(config.Definition{
		Key:   config.KeyContractTier,
		Label: "Tarif",
		Description: "Der gebuchte moto-Tarif. Diesen Wert pflegt das moto-Team; " +
			"die Schule sieht ihn unter „Vertrag“ nur zur Information.",
		Type:            config.FieldSelect,
		Default:         config.ContractTierUnset,
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContract,
		SortOrder:       10,
		AccessPolicy:    config.AccessOperatorOnly,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Noch nicht hinterlegt", Value: config.ContractTierUnset},
				{Label: "Testphase", Value: config.ContractTierTest},
				{Label: "Basis", Value: config.ContractTierBasis},
				{Label: "Plus", Value: config.ContractTierPlus},
				{Label: "Premium", Value: config.ContractTierPremium},
			},
		},
	})

	config.Register(config.Definition{
		Key:   config.KeyContractBookedChildren,
		Label: "Gebuchte Kinder",
		Description: "Wie viele Kinder der Vertrag abdeckt. 0 bedeutet: noch nicht " +
			"hinterlegt. Die Schule sieht daneben, wie viele Kinder heute wirklich " +
			"aktiv sind. Es wird nichts gesperrt, wenn die Zahl überschritten wird.",
		Type:            config.FieldNumber,
		Default:         0,
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContract,
		SortOrder:       20,
		AccessPolicy:    config.AccessOperatorOnly,
		Validation:      &config.ValidationRules{Min: &minChildren, Max: &maxChildren},
	})

	config.Register(config.Definition{
		Key:   config.KeyContractPricePerChildCents,
		Label: "Preis pro Kind und Monat (in Cent)",
		Description: "In Cent eintragen, damit nichts gerundet wird. Beispiel: " +
			"100 bedeutet 1,00 €. 0 bedeutet: noch nicht hinterlegt.",
		Type:            config.FieldNumber,
		Default:         0,
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContract,
		SortOrder:       30,
		AccessPolicy:    config.AccessOperatorOnly,
		Validation:      &config.ValidationRules{Min: &minPrice, Max: &maxPrice},
	})

	config.Register(config.Definition{
		Key:             config.KeyContractBillingCycle,
		Label:           "Zahlungsrhythmus",
		Description:     "Wie oft die Schule eine Rechnung bekommt.",
		Type:            config.FieldSelect,
		Default:         config.ContractCycleUnset,
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContract,
		SortOrder:       40,
		AccessPolicy:    config.AccessOperatorOnly,
		Options: &config.SelectOptions{
			Static: []config.SelectOption{
				{Label: "Noch nicht hinterlegt", Value: config.ContractCycleUnset},
				{Label: "Monatlich", Value: config.ContractCycleMonthly},
				{Label: "Quartalsweise", Value: config.ContractCycleQuarterly},
				{Label: "Jährlich", Value: config.ContractCycleYearly},
			},
		},
	})

	config.Register(config.Definition{
		Key:             config.KeyContractTermStart,
		Label:           "Vertrag läuft ab",
		Description:     "Erster Tag der Vertragslaufzeit. Leer lassen, wenn noch kein Datum feststeht.",
		Type:            config.FieldDate,
		Default:         "",
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContract,
		SortOrder:       50,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeyContractTermEnd,
		Label:           "Vertrag läuft bis",
		Description:     "Letzter Tag der Vertragslaufzeit. Leer lassen, wenn der Vertrag unbefristet ist.",
		Type:            config.FieldDate,
		Default:         "",
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContract,
		SortOrder:       60,
		AccessPolicy:    config.AccessOperatorOnly,
	})

	config.Register(config.Definition{
		Key:             config.KeyContractInvoiceRecipient,
		Label:           "Rechnung geht an (E-Mail)",
		Description:     "An diese Adresse schickt das moto-Team die Rechnungen.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryBilling,
		SortOrder:       10,
		AccessPolicy:    config.AccessOperatorOnly,
		Validation:      &config.ValidationRules{Pattern: &emailPattern, AllowEmpty: true},
	})

	config.Register(config.Definition{
		Key:             config.KeyContractCustomerNumber,
		Label:           "Kundennummer",
		Description:     "Nummer der Schule in der Buchhaltung des moto-Teams. Steht auf jeder Rechnung.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryBilling,
		SortOrder:       20,
		AccessPolicy:    config.AccessOperatorOnly,
		Validation:      &config.ValidationRules{Pattern: &customerNumberPattern, AllowEmpty: true},
	})

	config.Register(config.Definition{
		Key:   config.KeyContractSupportEmail,
		Label: "Fragen zur Rechnung an (E-Mail)",
		Description: "Diese Adresse zeigt moto der Schule als Ansprechpartner an. " +
			"Ohne sie steht auf der Seite „Vertrag“ kein Kontakt.",
		Type:            config.FieldText,
		Default:         "",
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContact,
		SortOrder:       10,
		AccessPolicy:    config.AccessOperatorOnly,
		Validation:      &config.ValidationRules{Pattern: &emailPattern, AllowEmpty: true},
	})

	config.Register(config.Definition{
		Key:   config.KeyContractNote,
		Label: "Hinweis an die Schule",
		Description: "Freier Text, den die Schule auf der Seite „Vertrag“ liest. " +
			"Zum Beispiel eine Absprache zur Laufzeit. Leer lassen, wenn es nichts zu sagen gibt.",
		Type:            config.FieldTextarea,
		Default:         "",
		ReadPermission:  contractReadPermission,
		WritePermission: contractWritePermission,
		Tab:             contractTab,
		Category:        contractCategoryContact,
		SortOrder:       20,
		AccessPolicy:    config.AccessOperatorOnly,
	})
}
