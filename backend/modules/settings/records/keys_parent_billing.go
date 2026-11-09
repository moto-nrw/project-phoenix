package records

// The settings of the Elternabrechnung of a Träger (ADR 0088). Only the
// Träger sets them (LevelTraeger).

// KeyParentBillingDebtorNumberOnConfirm is the switch of a Träger (#5621):
// its office enters the debtor number of the family when it confirms a
// holiday registration, and the letter names it as the reference to pay
// with. Only the Träger sets it (LevelTraeger).
const KeyParentBillingDebtorNumberOnConfirm = "parent_billing.debtor_number_on_confirm"

// KeyParentBillingPaymentText is the payment sentence of the confirmation
// letter of a Träger (#5622): its own text with the placeholders {Betrag},
// {Zahlungsfrist}, {Rechnungsnummer} and {Debitorennummer}. Empty keeps the
// built-in sentence. Only the Träger sets it (LevelTraeger).
const KeyParentBillingPaymentText = "parent_billing.payment_text"

// KeyParentBillingSiblingRank says which child of a holiday registration
// counts as the first child (#5664): the one with the highest contribution
// or the oldest. Only the Träger sets it (LevelTraeger).
const KeyParentBillingSiblingRank = "parent_billing.sibling_rank"

// KeyParentBillingCostTakeoverOrder says whether a cost bearer pays what is
// left after the sibling discount or the full price (#5664). Only the Träger
// sets it (LevelTraeger).
const KeyParentBillingCostTakeoverOrder = "parent_billing.cost_takeover_order"

// KeyParentBillingCareMonthChange says how the monthly run bills a month in
// which the contract version changes, starts or ends (#4684): the full
// month by the version on the first day, or every version for its days.
// Only the Träger sets it (LevelTraeger).
const KeyParentBillingCareMonthChange = "parent_billing.care_month_change"

// The changes of a single invoice the review of a monthly run allows
// (#6197): its amount, its recipient, its postal address and its positions.
// The customer has not decided which it needs (SJR item 2); every switch is
// on by default. Only the Träger sets them (LevelTraeger).
const (
	KeyParentBillingCareReviewAmount    = "parent_billing.care_review_amount"
	KeyParentBillingCareReviewRecipient = "parent_billing.care_review_recipient"
	KeyParentBillingCareReviewAddress   = "parent_billing.care_review_address"
	KeyParentBillingCareReviewPositions = "parent_billing.care_review_positions"
)
