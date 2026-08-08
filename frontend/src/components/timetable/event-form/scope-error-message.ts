/**
 * German user text for failures of a series-scoped Regeltermin save.
 *
 * Issue #2187: a split series is a chain of templates, and the client may
 * still hold the id of a capped predecessor segment. Saving against such an
 * id used to surface the raw backend text ("template not found"), which reads
 * like data loss to a school user. The backend now resolves the chain forward;
 * everything it still rejects gets a message that says what to do next.
 *
 * Informative backend 400 texts are passed through on purpose — they name the
 * concrete field or conflict and are more useful than a generic sentence.
 */
export function timetableSeriesErrorMessage(
  err: unknown,
  fallback: string,
): string {
  const raw = err instanceof Error ? err.message : "";

  if (raw.includes("effective_date must not be in the past")) {
    return "Der Stichtag liegt in der Vergangenheit. Bitte einen künftigen Termin der Serie wählen.";
  }

  // Duck-typed: TimetableApiError is not exported, and tests mock the module.
  const code = readStringProp(err, "code");
  if (code === "template_not_found") return TEMPLATE_GONE_MESSAGE;

  const httpStatus = readNumberProp(err, "httpStatus");
  if (httpStatus !== undefined) {
    if (httpStatus === 401) {
      return "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
    }
    if (httpStatus === 403) {
      return "Sie haben keine Berechtigung für diese Änderung.";
    }
    if (httpStatus === 404) return TEMPLATE_GONE_MESSAGE;
    if (httpStatus === 409) {
      return "Der Regeltermin wurde zwischenzeitlich geändert. Bitte laden Sie die Seite neu.";
    }
    if (httpStatus >= 500) return fallback;
  }

  return raw !== "" ? raw : fallback;
}

const TEMPLATE_GONE_MESSAGE =
  "Der Regeltermin konnte nicht geladen werden. Bitte laden Sie die Seite neu und öffnen Sie den Termin erneut.";

function readStringProp(err: unknown, key: string): string | undefined {
  if (typeof err !== "object" || err === null) return undefined;
  const value = (err as Record<string, unknown>)[key];
  return typeof value === "string" ? value : undefined;
}

function readNumberProp(err: unknown, key: string): number | undefined {
  if (typeof err !== "object" || err === null) return undefined;
  const value = (err as Record<string, unknown>)[key];
  return typeof value === "number" ? value : undefined;
}
