// Wege zur Ferienanmeldung im Elternportal (#6671): der Link eines
// Ferienzeitraums und der Rückweg dorthin nach der Anmeldung am Konto.

const HOLIDAY_REGISTRATION_PATH = /^\/ferienanmeldung\/[A-Za-z0-9_-]+$/;

/** Die eigenen Ferienanmeldungen in der Eltern-App (#6673). */
const HOLIDAY_REGISTRATIONS_PATH = "/ferienanmeldungen";

/** Pfad des Anmeldeformulars eines Ferienzeitraums auf dem Elternportal. */
export function holidayRegistrationPath(token: string): string {
  return `/ferienanmeldung/${encodeURIComponent(token)}`;
}

/** Anmeldeseite, die nach dem Login zum Formular des Zeitraums zurückführt. */
export function holidayRegistrationLoginPath(token: string): string {
  return `/login?next=${encodeURIComponent(holidayRegistrationPath(token))}`;
}

/**
 * Anmeldeseite, die nach dem Login zu den eigenen Ferienanmeldungen führt:
 * der Weg nach dem Einrichten des Elternkontos (#6673).
 */
export function holidayRegistrationsLoginPath(): string {
  return `/login?next=${encodeURIComponent(HOLIDAY_REGISTRATIONS_PATH)}`;
}

/**
 * Das Ziel nach dem Login, wenn `next` genau ein Anmeldeformular der
 * Ferienbetreuung oder die eigenen Ferienanmeldungen nennt, sonst `null`.
 * Jeder andere Wert, auch eine fremde Adresse, führt wie bisher zur
 * Startseite.
 */
export function holidayRegistrationReturnPath(
  next: string | null,
): string | null {
  if (next === HOLIDAY_REGISTRATIONS_PATH) return next;
  return next && HOLIDAY_REGISTRATION_PATH.test(next) ? next : null;
}
