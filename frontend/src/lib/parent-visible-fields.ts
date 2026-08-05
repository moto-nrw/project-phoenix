/**
 * Which child Stammdaten the parent portal mirrors — the single place the
 * Betreuerapp reads when it marks a field as "Für Eltern sichtbar".
 *
 * Backend source of truth (keep in sync in the same PR when a response struct
 * gains or loses a field):
 *   - `ChildMasterData`  — backend/services/parent/parent_master_data_service.go
 *   - `ChildGuardian`    — backend/services/parent/parent_guardian_service.go
 *
 * Deliberately NOT mirrored today, so these fields carry no marker: the child's
 * address, Betreuernotizen, "Elternnotizen" (extra_info), Gruppe, Foto and
 * Foto-Einwilligung, Datenschutz-Angaben, the Zusatzangaben from the enrollment
 * form, the Portalrolle of a guardian, and the notes on the Betreuungsplan.
 *
 * The hints below assert VISIBILITY ONLY, never that parents may change a
 * value. Whether they can depends on the guardian's `parent_portal.*`
 * permission for this child, on tenant settings
 * (`operations.parent_master_data_edit_enabled` /
 * `…_request_enabled`), and per field on further conditions — a departure day
 * set to "accompanied" blocks the request outright. A marker cannot know any of
 * that, so it does not claim it.
 */

/** Default tooltip when a marker gets no field-specific hint. */
export const PARENT_VISIBLE_DEFAULT_HINT =
  "Erziehungsberechtigte sehen diese Angabe im Elternportal.";

/** Field-specific tooltips. Visibility only — see the note above. */
export const PARENT_VISIBLE_HINTS = {
  name: "Erziehungsberechtigte sehen den Namen des Kindes im Elternportal.",
  schoolClass: "Erziehungsberechtigte sehen die Klasse im Elternportal.",
  birthday: "Erziehungsberechtigte sehen das Geburtsdatum im Elternportal.",
  healthInfo:
    "Erziehungsberechtigte sehen die Gesundheitsinformationen im Elternportal.",
  departure:
    "Erziehungsberechtigte sehen die erlaubten Heimwege im Elternportal.",
  // contactProtected() in parent_guardian_service.go strips email, phone and
  // address for account holders, Fachkräfte and familienübergreifende Kontakte,
  // so the hint names that exception rather than over-promising.
  guardianContact:
    "Andere Erziehungsberechtigte dieses Kindes sehen diese Kontaktdaten im Elternportal. Ausgenommen sind Personen mit eigenem Elternkonto, Fachkräfte und familienübergreifend genutzte Kontakte.",
  guardianName:
    "Andere Erziehungsberechtigte dieses Kindes sehen diese Angabe im Elternportal.",
  guardianPermissions:
    "Andere Erziehungsberechtigte dieses Kindes sehen im Elternportal, wer abholberechtigt und wer Notfallkontakt ist.",
} as const;
