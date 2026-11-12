/**
 * Texts that several buttons of the supervision pages share (#6755): the
 * same action reads the same everywhere. A module without imports, so tests
 * that mock the components using them still get the real text.
 */

/** „Rückgängig“ in the banner and „Wieder öffnen“ on an ended block. */
export const REOPEN_TOOLTIP =
  "Lässt die Aktivität mit denselben Kindern wieder laufen.";

/** Every „Betreuer hinzufügen“ entry point. */
export const ADD_SUPERVISOR_HINT =
  "Öffnet ein Fenster, um eine weitere Aufsicht einzutragen.";

/** Every „Beaufsichtigen“ for the Schulhof. */
export const SCHULHOF_SUPERVISE_TOOLTIP =
  "Trägt Sie sofort als Aufsicht auf dem Schulhof ein.";
