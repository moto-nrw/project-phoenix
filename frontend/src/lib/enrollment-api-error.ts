import { apiErrorFromText, type ApiError } from "~/lib/api-error";

type EnrollmentErrorLogger = {
  error: (message: string, context?: Record<string, unknown>) => void;
  warn?: (message: string, context?: Record<string, unknown>) => void;
};

/** The backend sentence, kept for the log only (ADR 0006: never shown). */
function backendDiagnosis(text: string): string | undefined {
  try {
    const body = JSON.parse(text) as { error?: unknown; message?: unknown };
    const raw = body.error ?? body.message;
    return typeof raw === "string" ? raw : undefined;
  } catch {
    return text || undefined;
  }
}

/**
 * Turns a failed enrollment response into an `ApiError` (#2515): code, field
 * errors, details and request ID survive for the shared error path, which
 * picks the text from the catalog. The backend sentence goes to the log only.
 */
export async function readEnrollmentError(
  response: Response,
  fallback: string,
  logger: EnrollmentErrorLogger,
  event: string,
): Promise<ApiError> {
  let text = "";
  try {
    text = await response.text();
  } catch {
    // Body already consumed or the stream broke: the status still classifies.
  }
  const error = apiErrorFromText(
    `${fallback} (HTTP ${response.status})`,
    response.status,
    text,
  );
  const context = {
    status: response.status,
    code: error.code,
    requestId: error.requestId,
    rawMessage: backendDiagnosis(text),
  };
  if (response.status >= 500 || !logger.warn) {
    logger.error(event, context);
  } else {
    logger.warn(event, context);
  }
  return error;
}
