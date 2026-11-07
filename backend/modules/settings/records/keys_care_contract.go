package records

// KeyCareContractConclusionMode says how the contracts of a school come
// about (#4680, ADR 0088 section 6): "nachtraeglich" (Weg a, the default),
// the Träger-Büro makes a draft and sends it; "bei_annahme" (Weg b), the
// contract is concluded when the school accepts the application, and the
// parents receive it. The Träger sets it for all its schools; a school may
// differ (LevelBoth). The values are the constants of the Betreuungsvertrag.
const KeyCareContractConclusionMode = "care_contract.conclusion_mode"
