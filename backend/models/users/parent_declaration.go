package users

// Erklärungen (#3430). An Erklärung is the third delivery mode of a parent
// announcement: guardians give or refuse their consent for a child (the
// Einverständnis) and every answer is kept as proof. The announcement
// columns carry the settings; the versions freeze what was declared; the
// submissions record who declared what, when.

// ParentAnnouncementDeliveryDeclaration is the delivery mode of an Erklärung
// (mirrors chk_parent_announcements_delivery_mode).
const ParentAnnouncementDeliveryDeclaration = "declaration"

// DeclarationKindConsent is the only declaration kind (mirrors
// chk_parent_announcements_declaration_shape): Zustimmung or Ablehnung. A
// plain read confirmation is the Elternbrief's Lesebestätigung. The kind is
// still stored so every proof names what was asked.
const DeclarationKindConsent = "consent"

// Declaration signer rules: one guardian with the permission per child is
// enough, or every such guardian of the child must declare.
const (
	DeclarationSignersAny = "any"
	DeclarationSignersAll = "all"
)

// Declaration actions (mirrors the submissions' action CHECK).
const (
	DeclarationActionAgreed   = "agreed"
	DeclarationActionDeclined = "declined"
	DeclarationActionRevoked  = "revoked"
)

// DeclarationMethodSimpleElectronic is the only procedure moto offers: a
// simple electronic declaration from the signed-in parent account. It is not
// a qualified electronic signature and replaces no statutory written form.
const DeclarationMethodSimpleElectronic = "simple_electronic"

// Per-child states of a declaration for its current version.
const (
	DeclarationStateOpen     = "open"
	DeclarationStatePartial  = "partial"
	DeclarationStateAgreed   = "agreed"
	DeclarationStateDeclined = "declined"
	DeclarationStateRevoked  = "revoked"
	DeclarationStateNoSigner = "no_signer"
	DeclarationStateExpired  = "expired"
)
