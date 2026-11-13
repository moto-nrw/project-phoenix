/**
 * The name of a person in the Träger-Büro (#4693). The export protocol and
 * the history of every invoice show it instead of an e-mail address. The
 * backend trims it and allows 1 to 100 characters; the fields mirror that.
 */

/** Longest name the backend accepts, after trimming. */
export const TRAEGER_NAME_MAX_LENGTH = 100;

/** Who sees the name and where; under every name field. */
export const TRAEGER_NAME_HINT =
  "Personen im Träger-Büro sehen ihn im Exportprotokoll und im Verlauf jeder Rechnung.";

/** The field error of an empty own name. */
export const OWN_NAME_REQUIRED = "Bitte geben Sie Ihren Namen ein.";

/** The field error of an empty name of another person. */
export const MEMBER_NAME_REQUIRED = "Bitte geben Sie einen Namen ein.";
