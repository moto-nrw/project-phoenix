/**
 * Standardsatz am Bestätigen-Knopf eines Bestätigungs- oder Löschdialogs
 * (#6744): Der Dialog hat schon nachgefragt. Eine eigene Datei, damit Tests,
 * die `modal` ersetzen, den Löschdialog nicht mit ersetzen müssen.
 */
export const CONFIRM_TOOLTIP =
  "Führt die Aktion sofort aus, ohne weitere Nachfrage.";

/** Erster Schritt von `ConfirmDeleteModal` `twoStep`: er löscht noch nichts. */
export const FIRST_STEP_TOOLTIP =
  "Löscht noch nicht, moto fragt ein letztes Mal nach.";
