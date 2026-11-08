package records

// KeyCareContractConclusionMode says how the contracts of a school come
// about (#4680, ADR 0088 section 6): "nachtraeglich" (Weg a, the default),
// the Träger-Büro makes a draft and sends it; "bei_annahme" (Weg b), the
// contract is concluded when the school accepts the application, and the
// parents receive it. The Träger sets it for all its schools; a school may
// differ (LevelBoth). The values are the constants of the Betreuungsvertrag.
const KeyCareContractConclusionMode = "care_contract.conclusion_mode"

// KeyCareContractTerminationConfirmation and
// KeyCareContractWithdrawalConfirmation are the Träger's templates of the
// confirmation the parents receive in Textform after a termination or a
// withdrawal (#4682, § 312k BGB). Only the Träger sets them (LevelTraeger);
// the Betreuungsvertrag fills in the fixed placeholders.
// KeyCareContractExtraordinaryConfirmation confirms an extraordinary
// termination, whose end the Träger sets after it checked the reason.
const (
	KeyCareContractTerminationConfirmation   = "care_contract.termination_confirmation"
	KeyCareContractWithdrawalConfirmation    = "care_contract.withdrawal_confirmation"
	KeyCareContractExtraordinaryConfirmation = "care_contract.extraordinary_termination_confirmation"
)

// KeyCareContractEndSetNotice and KeyCareContractReasonRejectedNotice are
// the Träger's templates of the notices the parents receive in Textform
// once the Träger-Büro decided the end of an extraordinary termination
// (#4682, decision of 07.11.2026): it set the last day, or it did not
// accept the reason and the declaration holds as an ordinary termination
// to the next notice date (§ 140 BGB).
const (
	KeyCareContractEndSetNotice         = "care_contract.extraordinary_end_set_notice"
	KeyCareContractReasonRejectedNotice = "care_contract.extraordinary_reason_rejected_notice"
)
