package records

// KeyCareContractSignatureReminderDays is after how many days a Träger
// reminds parents of a care contract they have not signed yet (#4679): the
// days since the last mail or reminder. 0 sends no reminder. Only the
// Träger sets it (LevelTraeger).
const KeyCareContractSignatureReminderDays = "care_contract.signature_reminder_days"
