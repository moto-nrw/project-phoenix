package api

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/parentbilling"
	configModels "github.com/moto-ogs/project-phoenix/modules/settings/records"
)

// TraegerBrandingSettings is the Träger level of the settings system as the
// letters read the branding of a Träger (ADR 0112), the Elternabrechnung
// its switches (#5621) and the holiday registration its retention (#6691).
type TraegerBrandingSettings interface {
	TraegerInts(ctx context.Context, organizationID int64, keys []string) (map[string]int, error)
	TraegerTexts(ctx context.Context, organizationID int64, keys []string) (map[string]string, error)
	TraegerBools(ctx context.Context, organizationID int64, keys []string) (map[string]bool, error)
}

// parentBillingSettings binds the Elternabrechnung's settings port to the
// Träger level of the settings system: whether the Träger enters the debtor
// number of the family when it confirms a holiday registration (#5621), its
// payment text, which the billing run checks before the release (#5622), and
// its choices in the chain of the holiday contribution (#5664).
type parentBillingSettings struct {
	settings TraegerBrandingSettings
}

func (s parentBillingSettings) AsksDebtorNumber(ctx context.Context, organizationID int64) (bool, error) {
	values, err := s.settings.TraegerBools(ctx, organizationID, []string{configModels.KeyParentBillingDebtorNumberOnConfirm})
	if err != nil {
		return false, err
	}
	return values[configModels.KeyParentBillingDebtorNumberOnConfirm], nil
}

// DebtorNumberFormat reads the format of the Träger's debtor numbers
// (#6675): its three values of parent_billing.debtor_number_* or their
// defaults, the two lengths in either order.
func (s parentBillingSettings) DebtorNumberFormat(ctx context.Context, organizationID int64) (parentbilling.DebtorNumberFormat, error) {
	switches, err := s.settings.TraegerBools(ctx, organizationID, []string{configModels.KeyParentBillingDebtorNumberDigitsOnly})
	if err != nil {
		return parentbilling.DebtorNumberFormat{}, err
	}
	if !switches[configModels.KeyParentBillingDebtorNumberDigitsOnly] {
		return parentbilling.DebtorNumberFormat{}, nil
	}
	lengths, err := s.settings.TraegerInts(ctx, organizationID, []string{
		configModels.KeyParentBillingDebtorNumberMinDigits, configModels.KeyParentBillingDebtorNumberMaxDigits,
	})
	if err != nil {
		return parentbilling.DebtorNumberFormat{}, err
	}
	return parentbilling.NewDebtorNumberFormat(true, lengths[configModels.KeyParentBillingDebtorNumberMinDigits],
		lengths[configModels.KeyParentBillingDebtorNumberMaxDigits]), nil
}

func (s parentBillingSettings) PaymentText(ctx context.Context, organizationID int64) (string, error) {
	texts, err := s.settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyParentBillingPaymentText})
	if err != nil {
		return "", err
	}
	return texts[configModels.KeyParentBillingPaymentText], nil
}

func (s parentBillingSettings) HolidayChain(ctx context.Context, organizationID int64) (parentbilling.HolidayChainSettings, error) {
	texts, err := s.settings.TraegerTexts(ctx, organizationID, []string{
		configModels.KeyParentBillingSiblingRank, configModels.KeyParentBillingCostTakeoverOrder,
	})
	if err != nil {
		return parentbilling.HolidayChainSettings{}, err
	}
	return parentbilling.HolidayChainSettings{
		SiblingRank:       texts[configModels.KeyParentBillingSiblingRank],
		CostTakeoverOrder: texts[configModels.KeyParentBillingCostTakeoverOrder],
	}, nil
}

// CareMonthRule is the Träger's value of parent_billing.care_month_change
// or its default (#4684).
func (s parentBillingSettings) CareMonthRule(ctx context.Context, organizationID int64) (string, error) {
	texts, err := s.settings.TraegerTexts(ctx, organizationID, []string{configModels.KeyParentBillingCareMonthChange})
	if err != nil {
		return "", err
	}
	return texts[configModels.KeyParentBillingCareMonthChange], nil
}

// CareAmendable reads which changes of a single invoice the review of a
// monthly run allows (#6197): the Träger's switches or their defaults.
func (s parentBillingSettings) CareAmendable(ctx context.Context, organizationID int64) (parentbilling.CareAmendable, error) {
	values, err := s.settings.TraegerBools(ctx, organizationID, []string{
		configModels.KeyParentBillingCareReviewAmount, configModels.KeyParentBillingCareReviewRecipient,
		configModels.KeyParentBillingCareReviewAddress, configModels.KeyParentBillingCareReviewPositions,
	})
	if err != nil {
		return parentbilling.CareAmendable{}, err
	}
	return parentbilling.CareAmendable{
		Amount:    values[configModels.KeyParentBillingCareReviewAmount],
		Recipient: values[configModels.KeyParentBillingCareReviewRecipient],
		Address:   values[configModels.KeyParentBillingCareReviewAddress],
		Positions: values[configModels.KeyParentBillingCareReviewPositions],
	}, nil
}

// DatevSettings reads the DATEV settings of the Träger (#4694): its own
// values or the registry defaults, each with its key.
func (s parentBillingSettings) DatevSettings(ctx context.Context, organizationID int64) (parentbilling.DatevSettings, error) {
	keys := []string{
		configModels.KeyParentBillingDatevConsultantNumber, configModels.KeyParentBillingDatevClientNumber,
		configModels.KeyParentBillingDatevAccountLength, configModels.KeyParentBillingDatevChartOfAccounts,
		configModels.KeyParentBillingDatevFiscalYearStart, configModels.KeyParentBillingDatevRevenueAccount,
	}
	texts, err := s.settings.TraegerTexts(ctx, organizationID, keys)
	if err != nil {
		return parentbilling.DatevSettings{}, err
	}
	locked, err := s.settings.TraegerBools(ctx, organizationID, []string{
		configModels.KeyParentBillingDatevLocked, configModels.KeyParentBillingDatevCostCenterRequired,
	})
	if err != nil {
		return parentbilling.DatevSettings{}, err
	}
	setting := func(key string) parentbilling.DatevSetting {
		return parentbilling.DatevSetting{Key: key, Value: texts[key]}
	}
	return parentbilling.DatevSettings{
		ConsultantNumber: setting(configModels.KeyParentBillingDatevConsultantNumber),
		ClientNumber:     setting(configModels.KeyParentBillingDatevClientNumber),
		AccountLength:    setting(configModels.KeyParentBillingDatevAccountLength),
		ChartOfAccounts:  setting(configModels.KeyParentBillingDatevChartOfAccounts),
		FiscalYearStart:  setting(configModels.KeyParentBillingDatevFiscalYearStart),
		RevenueAccount:   setting(configModels.KeyParentBillingDatevRevenueAccount),
		Locked:           locked[configModels.KeyParentBillingDatevLocked],
		// A booking needs a KOST1 (#4821).
		CostCenterRequired: locked[configModels.KeyParentBillingDatevCostCenterRequired],
	}, nil
}
