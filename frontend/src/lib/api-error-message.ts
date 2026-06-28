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
  if (message.includes("403")) {
    return `Sie haben keine Berechtigung, diese ${entityType} zu ${action}.`;
  }
  if (message.includes("400")) {
    return "Ungültige Eingabedaten. Bitte überprüfen Sie Ihre Eingaben.";
  }

  return message;
}
