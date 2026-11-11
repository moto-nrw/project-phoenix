// Wege zur Ferienanmeldung im Elternportal (#6671): der Link eines
// Ferienzeitraums und der Rückweg dorthin nach der Anmeldung am Konto.

const HOLIDAY_REGISTRATION_PATH = /^\/ferienanmeldung\/[A-Za-z0-9_-]+$/;

/** Pfad des Anmeldeformulars eines Ferienzeitraums auf dem Elternportal. */
export function holidayRegistrationPath(token: string): string {
  return `/ferienanmeldung/${encodeURIComponent(token)}`;
}

/** Anmeldeseite, die nach dem Login zum Formular des Zeitraums zurückführt. */
export function holidayRegistrationLoginPath(token: string): string {
  return `/login?next=${encodeURIComponent(holidayRegistrationPath(token))}`;
}

/**
 * Das Ziel nach dem Login, wenn `next` genau ein Anmeldeformular der
 * Ferienbetreuung nennt, sonst `null`. Jeder andere Wert, auch eine fremde
 * Adresse, führt wie bisher zur Startseite.
 */
export function holidayRegistrationReturnPath(
  next: string | null,
): string | null {
  return next && HOLIDAY_REGISTRATION_PATH.test(next) ? next : null;
}
