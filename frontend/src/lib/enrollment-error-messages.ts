type EnrollmentErrorLogger = {
  error: (message: string, context?: Record<string, unknown>) => void;
  warn?: (message: string, context?: Record<string, unknown>) => void;
};

interface BackendErrorEnvelope {
  error?: string;
  message?: string;
  code?: string;
}

export type EnrollmentAPIError = Error & {
  status?: number;
  code?: string;
  rawMessage?: string;
};

export const CARE_OFFERING_TEMPLATE_PERIOD_MISMATCH_MESSAGE =
  "Der Planungszeitraum des gewählten Regeltermins muss den gesamten Betreuungszeitraum der Anmeldephase abdecken. Wähle einen passenden Regeltermin oder entferne die Verknüpfung.";

const ENROLLMENT_CODE_MESSAGES: Record<string, string> = {
  "enrollment.care_offering_missing":
    "Bitte wähle für jedes Kind mindestens ein Betreuungsangebot aus.",
  "enrollment.care_offering_exactly_one":
    "Bitte wähle für jedes Kind genau ein Betreuungsangebot aus.",
  "enrollment.care_offering_unavailable":
    "Ein ausgewähltes Betreuungsangebot ist für die Klassenstufe dieses Kindes nicht verfügbar. Bitte prüfe die Klassenstufe und die Auswahl.",
  "enrollment.required_care_offering_missing":
    "Für jedes Kind muss ein verpflichtendes Betreuungsangebot ausgewählt sein.",
  "enrollment.care_offering_full":
    "Eines der ausgewählten Betreuungsangebote ist bereits voll und kann derzeit keine weiteren Anmeldungen aufnehmen. Bitte wähle ein anderes Angebot oder wende dich an die Schule.",
  "enrollment.care_offering_template_period_mismatch":
    CARE_OFFERING_TEMPLATE_PERIOD_MISMATCH_MESSAGE,
  "enrollment.disabled":
    "Die Online-Anmeldung ist für diese Schule aktuell nicht freigeschaltet. Bitte wende dich an die Schulleitung.",
  "enrollment.window_closed":
    "Die Anmeldefrist für diese Anmeldephase ist abgelaufen oder noch nicht geöffnet. Bitte wende dich an die Schule.",
  "enrollment.invalid_phone": "Bitte gib eine gültige Telefonnummer ein.",
  "enrollment.invalid_email": "Bitte gib eine gültige E-Mail-Adresse ein.",
  "enrollment.pickup_time_not_allowed":
    "Bitte wähle bei den Abholzeiten nur Uhrzeiten aus der vorgegebenen Liste. Die markierte Zeit ist nicht mehr verfügbar.",
  "enrollment.selected_day_not_available":
    "Der gewählte Wochentag ist für dieses Angebot nicht verfügbar. Bitte wähle nur die angezeigten Tage aus.",
  "enrollment.day_selection_required":
    "Bitte wähle für dieses Angebot mindestens einen Wochentag aus.",
  "enrollment.day_selection_not_allowed":
    "Für dieses Angebot sind die Betreuungstage fest vorgegeben und können nicht ausgewählt werden.",
  "enrollment.care_offering_days_required":
    "Bitte wähle mindestens einen Wochentag für das Angebot aus.",
  "enrollment.late_invite_invalid":
    "Dieser Nachzügler-Link ist ungültig, abgelaufen oder wurde bereits verwendet. Bitte wende dich an die Schule.",
  "enrollment.phase_not_eligible":
    "Diese Anmeldephase ist für dein Konto nicht verfügbar. Bitte melde dich im Elternportal an oder wende dich an die Schule.",
  "enrollment.class_not_eligible":
    "Diese Anmeldephase ist auf bestimmte Klassen beschränkt. Bitte prüfe die Klassenangabe deines Kindes.",
  "enrollment.child_already_enrolled":
    "Dieses Kind ist an der Schule bereits angemeldet. Diese Phase richtet sich nur an neue Kinder.",
  "enrollment.child_not_enrolled":
    "Dieses Kind ist an der Schule nicht angemeldet. Diese Phase richtet sich nur an bereits angemeldete Kinder.",
  "enrollment.schema_has_phases":
    "Diese Formularvorlage wird noch in einer Anmeldephase verwendet.",
  "enrollment.schema_has_requests":
    "Diese Formularvorlage wurde bereits für Anmeldungen verwendet und kann nicht gelöscht werden.",
  "enrollment.schema_name_exists":
    "Es gibt bereits ein Formular mit diesem Namen.",
  "rollover.source_not_found": "Die Quellphase wurde nicht gefunden.",
  "rollover.invalid_request":
    "Die Eingaben sind unvollständig oder ungültig. Bitte alle Pflichtfelder prüfen.",
  "rollover.review_invalid": "Die Aktion ist in diesem Zustand nicht erlaubt.",
  "rollover.review_not_found":
    "Dieser Eintrag in der Prüfliste existiert nicht mehr.",
  "rollover.duplicate_name":
    "Es existiert bereits eine Phase mit diesem Namen. Bitte einen anderen Namen wählen.",
  "rollover.source_already_rolled":
    "Aus dieser Phase wurde bereits eine Anschlussphase erstellt. Bitte zuerst die bestehende Anschlussphase löschen, bevor du eine neue anlegst.",
};

const RAW_MESSAGE_TRANSLATIONS: Array<[RegExp, string]> = [
  [
    /phase validation: .*phase name is required|phase name is required/i,
    "Bitte gib einen Namen für die Anmeldephase ein.",
  ],
  [
    /service_start_date is required/i,
    "Bitte gib den Beginn des Betreuungszeitraums ein.",
  ],
  [
    /service_end_date is required/i,
    "Bitte gib das Ende des Betreuungszeitraums ein.",
  ],
  [
    /service_start_date must be YYYY-MM-DD/i,
    "Der Beginn des Betreuungszeitraums muss ein gültiges Datum sein.",
  ],
  [
    /service_end_date must be YYYY-MM-DD/i,
    "Das Ende des Betreuungszeitraums muss ein gültiges Datum sein.",
  ],
  [
    /service_end_date must be on or after service_start_date/i,
    "Das Ende des Betreuungszeitraums muss am gleichen Tag oder nach dem Beginn liegen.",
  ],
  [
    /enrollment_open_at must be RFC3339/i,
    "Die Öffnung des Anmeldefensters muss ein gültiger Zeitpunkt sein.",
  ],
  [
    /enrollment_close_at must be RFC3339/i,
    "Die Schließung des Anmeldefensters muss ein gültiger Zeitpunkt sein.",
  ],
  [
    /enrollment_close_at must be after enrollment_open_at/i,
    "Die Schließung des Anmeldefensters muss nach der Öffnung liegen.",
  ],
  [
    /form_schema_id must be a positive integer string/i,
    "Die ausgewählte Formularvorlage ist ungültig.",
  ],
  [
    /phase kind must be one of/i,
    "Bitte wähle einen gültigen Typ für die Anmeldephase aus.",
  ],
  [
    /care_overflow_mode must be/i,
    "Bitte wähle ein gültiges Verhalten bei vollen Betreuungsangeboten aus.",
  ],
  [
    /care_offering_selection_mode must be/i,
    "Bitte wähle eine gültige Auswahlregel für Betreuungsangebote aus.",
  ],
  [
    /care offering name is required/i,
    "Bitte gib einen Namen für das Betreuungsangebot ein.",
  ],
  [/phase_id is required/i, "Bitte wähle eine Anmeldephase aus."],
  [
    /invalid phase_id|phase_id must be positive/i,
    "Die ausgewählte Anmeldephase ist ungültig.",
  ],
  [
    /days_of_week_mode must be/i,
    "Bitte wähle eine gültige Tagesauswahl für das Betreuungsangebot aus.",
  ],
  [/capacity must be non-negative/i, "Die Kapazität darf nicht negativ sein."],
  [/price_cents must be non-negative/i, "Der Preis darf nicht negativ sein."],
  [
    /selection_rule .* is invalid|care offerings in the same selection group must share one selection rule/i,
    "Bitte prüfe die Auswahlregel des Betreuungsangebots.",
  ],
  [
    /a required care offering must not have a capacity limit/i,
    "Ein verpflichtendes Betreuungsangebot darf keine Kapazitätsgrenze haben.",
  ],
  [
    /schema name is required|form schema name is required/i,
    "Bitte gib einen Namen für die Formularvorlage ein.",
  ],
  [
    /schema with name .* already exists/i,
    "Es existiert bereits eine Formularvorlage mit diesem Namen.",
  ],
  [/duplicate name/i, "Dieser Name ist bereits vergeben."],
  [
    /form field key is required/i,
    "Bitte gib jedem Formularfeld einen technischen Schlüssel.",
  ],
  [
    /form field label is required/i,
    "Bitte gib jedem Formularfeld eine Beschriftung.",
  ],
  [
    /form field key must be at most/i,
    "Der technische Schlüssel eines Formularfelds ist zu lang.",
  ],
  [
    /form field key .* must be lowercase letters/i,
    "Der technische Schlüssel darf nur Kleinbuchstaben, Zahlen und Unterstriche enthalten.",
  ],
  [
    /duplicate form field key/i,
    "Jeder technische Feldschlüssel darf nur einmal vorkommen.",
  ],
  [
    /duplicate legal block key/i,
    "Jeder Rechtstext-Schlüssel darf nur einmal vorkommen.",
  ],
  [
    /select field .* must declare at least one option/i,
    "Auswahlfelder brauchen mindestens eine Option.",
  ],
  [
    /non-select field .* must not declare options/i,
    "Optionen sind nur bei Auswahlfeldern erlaubt.",
  ],
  [
    /field type .* must declare a target/i,
    "Dieses Formularfeld braucht eine Zuordnung.",
  ],
  [/phone_number is required/i, "Bitte gib eine Telefonnummer ein."],
  [
    /phone_type .* must be/i,
    "Bitte wähle einen gültigen Telefonnummer-Typ aus.",
  ],
  [/weekday .* must be one of/i, "Bitte wähle gültige Wochentage aus."],
  [
    /weekday .* time .* must be HH:MM/i,
    "Bitte gib Uhrzeiten im Format HH:MM ein.",
  ],
  [
    /contact first_name and last_name are required/i,
    "Bitte gib Vor- und Nachname des Kontakts ein.",
  ],
  [
    /contact requires at least one of email or phone_numbers/i,
    "Bitte gib für den Kontakt eine E-Mail-Adresse oder Telefonnummer an.",
  ],
  [
    /emergency_priority must be non-negative/i,
    "Die Notfall-Priorität darf nicht negativ sein.",
  ],
  [
    /legal block key is required/i,
    "Bitte gib jedem Rechtstext einen technischen Schlüssel.",
  ],
  [
    /legal block key must be at most/i,
    "Der technische Schlüssel eines Rechtstexts ist zu lang.",
  ],
  [
    /legal block key .* must be lowercase letters/i,
    "Der Rechtstext-Schlüssel darf nur Kleinbuchstaben, Zahlen und Unterstriche enthalten.",
  ],
  [
    /legal block source .* must be/i,
    "Bitte wähle eine gültige Quelle für den Rechtstext aus.",
  ],
  [
    /notice legal block .* cannot be required/i,
    "Ein Hinweistext darf nicht als Pflichtzustimmung markiert sein.",
  ],
  [
    /enrollment window is closed/i,
    "Die Anmeldefrist für diese Anmeldephase ist abgelaufen oder noch nicht geöffnet. Bitte wende dich an die Schule.",
  ],
  [
    /invalid schema/i,
    "Die Formularvorlage ist ungültig. Bitte prüfe die Felder.",
  ],
  [
    /guardian first name is required/i,
    "Bitte gib den Vornamen des Elternteils ein.",
  ],
  [
    /guardian last name is required/i,
    "Bitte gib den Nachnamen des Elternteils ein.",
  ],
  [
    /guardian email is required/i,
    "Bitte gib die E-Mail-Adresse des Elternteils ein.",
  ],
  [
    /guardian phone is required/i,
    "Bitte gib die Telefonnummer des Elternteils ein.",
  ],
  [
    /guardian phone number has an invalid format/i,
    "Bitte gib eine gültige Telefonnummer ein.",
  ],
  [
    /guardian email has an invalid format/i,
    "Bitte gib eine gültige E-Mail-Adresse ein.",
  ],
  [/at least one child is required/i, "Bitte füge mindestens ein Kind hinzu."],
  [
    /child \d+ missing name/i,
    "Bitte gib für jedes Kind Vor- und Nachname ein.",
  ],
  [
    /child \d+ missing date_of_birth/i,
    "Bitte gib für jedes Kind das Geburtsdatum ein.",
  ],
  [
    /child \d+ missing target_grade_level/i,
    "Bitte gib für jedes Kind die Ziel-Klassenstufe ein.",
  ],
  [
    /child \d+: invalid date_of_birth/i,
    "Bitte gib das Geburtsdatum im Format JJJJ-MM-TT ein.",
  ],
  [
    /consent .* is required/i,
    "Bitte bestätige alle verpflichtenden Zustimmungen.",
  ],
  [/field .* is required/i, "Bitte fülle alle Pflichtfelder aus."],
  [
    /an active enrollment already exists/i,
    "Für dieses Kind liegt in dieser Anmeldephase bereits eine aktive Anmeldung vor.",
  ],
  [/captcha token is required/i, "Bitte bestätige die Sicherheitsabfrage."],
  [
    /captcha verification failed/i,
    "Die Sicherheitsabfrage konnte nicht bestätigt werden. Bitte versuche es erneut.",
  ],
  [/status token is required/i, "Der Status-Link ist ungültig."],
  [/invalid child_id/i, "Die ausgewählte Anmeldung ist ungültig."],
  [
    /child is already in a terminal status/i,
    "Diese Anmeldung ist bereits abgeschlossen.",
  ],
  [/invalid decision status/i, "Bitte wähle eine gültige Entscheidung aus."],
  [
    /request_id required|child_id required/i,
    "Die Anmeldung konnte nicht eindeutig zugeordnet werden.",
  ],
  [/phase not found/i, "Die Anmeldephase wurde nicht gefunden."],
  [
    /enrollment request not found|request child not found/i,
    "Die Anmeldung wurde nicht gefunden.",
  ],
  [/tenant not found/i, "Die Schule wurde nicht gefunden."],
  [
    /missing actor/i,
    "Deine Sitzung konnte nicht zugeordnet werden. Bitte melde dich erneut an.",
  ],
];

const GERMAN_MESSAGE_HINT =
  /[äöüß]|\b(bitte|der|die|das|diese|dieser|dieses|nicht|konnte|konnten|ungültig|erforderlich|abgelaufen|zurückgenommen|anmeldung|anmeldephase|betreuungsangebot|formular|phase)\b/i;

export function translateEnrollmentErrorMessage(
  rawMessage?: string,
  code?: string,
): string | undefined {
  if (code) {
    const codedMessage = ENROLLMENT_CODE_MESSAGES[code];
    if (codedMessage) return codedMessage;
  }
  if (!rawMessage) return undefined;
  for (const [pattern, message] of RAW_MESSAGE_TRANSLATIONS) {
    if (pattern.test(rawMessage)) return message;
  }
  if (GERMAN_MESSAGE_HINT.test(rawMessage)) return rawMessage;
  return undefined;
}

async function readBackendErrorPayload(
  response: Response,
): Promise<BackendErrorEnvelope> {
  try {
    return (await response.clone().json()) as BackendErrorEnvelope;
  } catch {
    const text = await response.text().catch(() => "");
    return text ? { error: text } : {};
  }
}

export async function readEnrollmentError(
  response: Response,
  fallback: string,
  logger: EnrollmentErrorLogger,
  event: string,
): Promise<EnrollmentAPIError> {
  const payload = await readBackendErrorPayload(response);
  const rawMessage = payload.error ?? payload.message;
  const message =
    translateEnrollmentErrorMessage(rawMessage, payload.code) ??
    `${fallback} (HTTP ${response.status})`;
  const context = {
    status: response.status,
    message,
    rawMessage,
    code: payload.code,
  };
  if (response.status >= 500 || !logger.warn) {
    logger.error(event, context);
  } else {
    logger.warn(event, context);
  }
  const err = new Error(message) as EnrollmentAPIError;
  err.status = response.status;
  err.code = payload.code;
  err.rawMessage = rawMessage;
  return err;
}
