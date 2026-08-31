/**
 * Extracts a user-friendly error message from an API error.
 * Handles common HTTP status codes with German translations.
 */
export function getApiErrorMessage(
  err: unknown,
  action: string,
  entityType: string,
  defaultMessage: string,
): string {
  if (!(err instanceof Error)) {
    return defaultMessage;
  }

  const message = err.message;

  if (message.includes("user is not authenticated")) {
    return `Sie müssen angemeldet sein, um ${entityType} zu ${action}.`;
  }
  if (message.includes("401")) {
    return "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
  }
  if (
    message.includes("403") &&
    (message.includes("you can only modify") ||
      message.includes("created or supervise"))
  ) {
    return `Sie können diese ${entityType} nicht ${action}, da Sie sie nicht erstellt haben und kein Betreuer sind.`;
  }
  // Mitarbeiter-Vorschau (#2893): das Backend blockt jede Schreibaktion
  // eines Vorschau-Tokens mit diesem Code bzw. Text.
  if (
    message.includes("read_only_preview") ||
    message.includes("In der Vorschau können Sie nur lesen")
  ) {
    return "In der Vorschau können Sie nur lesen. Beenden Sie die Vorschau, um etwas zu ändern.";
  }
  if (message.includes("403")) {
    return `Sie haben keine Berechtigung, diese ${entityType} zu ${action}.`;
  }
  if (message.includes("400")) {
    return "Ungültige Eingabedaten. Bitte überprüfen Sie Ihre Eingaben.";
  }

  return message;
}

/**
 * Turns the message of a failed API call into one a school user can read: the
 * client prefixes its errors with the status code („HTTP 409: …"), and a
 * status code in front of a German sentence tells the reader nothing they can
 * act on. Backends send their message lowercase (staticcheck ST1005), so the
 * sentence is capitalised and closed here.
 *
 * Returns null when nothing readable is left — the caller then keeps its own
 * wording instead of showing an empty line.
 */
export function readableApiMessage(err: unknown): string | null {
  const raw = err instanceof Error ? err.message : "";
  const text = raw.replace(/^HTTP\s+\d+:\s*/i, "").trim();
  if (text === "") return null;
  const sentence = text.charAt(0).toUpperCase() + text.slice(1);
  return /[.!?]$/.test(sentence) ? sentence : `${sentence}.`;
}
