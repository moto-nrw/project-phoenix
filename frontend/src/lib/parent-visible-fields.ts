/**
 * Which child Stammdaten the parent portal mirrors — the single place the
 * Betreuerapp reads when it marks a field as "Für Eltern sichtbar".
 *
 * Backend source of truth (keep in sync in the same PR when a response struct
 * gains or loses a field):
 *   - `ChildMasterData`  — backend/services/parent/parent_master_data_service.go
 *   - `ChildGuardian`    — backend/services/parent/parent_guardian_service.go
 *   - care schedule      — backend/api/parent/care_schedule_handlers.go
 *
 * Deliberately NOT mirrored today, so these fields carry no marker: the child's
 * address, Betreuernotizen, "Elternnotizen" (extra_info), Gruppe, Foto and
 * Foto-Einwilligung, Datenschutz-Angaben, and the Zusatzangaben from the
 * enrollment form.
 */

/** Default tooltip when a marker gets no field-specific hint. */
export const PARENT_VISIBLE_DEFAULT_HINT =
  "Erziehungsberechtigte sehen diese Angabe im Elternportal.";

/**
 * Field-specific tooltips. They also say whether parents can change the value
 * themselves, because that is the second question staff ask once they know a
 * field is mirrored.
 */
export const PARENT_VISIBLE_HINTS = {
  name: "Erziehungsberechtigte sehen den Namen im Elternportal und können eine Änderung beantragen.",
  schoolClass:
    "Erziehungsberechtigte sehen die Klasse im Elternportal. Ändern können sie sie dort nicht.",
  birthday:
    "Erziehungsberechtigte sehen das Geburtsdatum im Elternportal und können eine Änderung beantragen.",
  healthInfo:
    "Erziehungsberechtigte sehen die Gesundheitsinformationen im Elternportal und können sie dort selbst ändern.",
  departure:
    "Erziehungsberechtigte sehen die erlaubten Heimwege im Elternportal und können eine Änderung beantragen.",
  careSchedule:
    "Erziehungsberechtigte sehen die Betreuungszeiten im Elternportal und können eine Änderung beantragen.",
  // contactProtected() in parent_guardian_service.go strips email, phone and
  // address for account holders, Fachkräfte and familienübergreifende Kontakte,
  // so the hint names that exception rather than over-promising.
  guardianContact:
    "Andere Erziehungsberechtigte dieses Kindes sehen diese Kontaktdaten im Elternportal. Ausgenommen sind Personen mit eigenem Elternkonto, Fachkräfte und familienübergreifend genutzte Kontakte.",
  guardianName:
    "Andere Erziehungsberechtigte dieses Kindes sehen Namen und Beziehung im Elternportal.",
  guardianPermissions:
    "Andere Erziehungsberechtigte dieses Kindes sehen im Elternportal, wer abholberechtigt und wer Notfallkontakt ist.",
} as const;

export type ParentVisibleHintKey = keyof typeof PARENT_VISIBLE_HINTS;
