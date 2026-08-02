export const REDACTED_LOG_VALUE = "[REDACTED]";

const SENSITIVE_LOG_KEY_PARTS = new Set(["password", "pin", "token", "secret"]);

const SENSITIVE_PATH_SEGMENT_PATTERN =
  /((?:^|\/)(?:enroll\/status|accept-guardian-invite|calendar-feed|enrollment\/requests|guardian-invitations)\/)[^/?#\s"']+/gi;
const SENSITIVE_PARAMETER_NAME =
  "(?:password|pin|token|secret|access[_-]?token|refresh[_-]?token|client[_-]?secret|confirm[_-]?password|device[_-]?pin|late[_-]?invite)";
const SENSITIVE_QUERY_PARAMETER_PATTERN = new RegExp(
  String.raw`([?&]${SENSITIVE_PARAMETER_NAME}=)[^&#\s"']*`,
  "gi",
);
const SENSITIVE_TEXT_FIELD_PATTERN = new RegExp(
  String.raw`((?:^|[\s,{])['"]?${SENSITIVE_PARAMETER_NAME}['"]?\s*[:=]\s*['"]?)[^'",}\s&]+`,
  "gi",
);
const BEARER_CREDENTIAL_PATTERN = /(\bBearer\s+)[^\s"',;]+/gi;

function isSensitiveLogKey(key: string): boolean {
  const normalized = key.replace(/([a-z0-9])([A-Z])/g, "$1_$2").toLowerCase();
  return normalized
    .split(/[^a-z0-9]+/)
    .some((part) => SENSITIVE_LOG_KEY_PARTS.has(part));
}

/** Redact credentials embedded in URLs or already-serialized text. */
export function redactSensitiveLogString(value: string): string {
  return value
    .replace(SENSITIVE_PATH_SEGMENT_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(SENSITIVE_QUERY_PARAMETER_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(SENSITIVE_TEXT_FIELD_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(BEARER_CREDENTIAL_PATTERN, `$1${REDACTED_LOG_VALUE}`);
}

/**
 * Return a log-safe copy of a value without mutating the caller's data.
 * Sensitive fields are matched by key at every nesting level so variants such
 * as confirm_password, accessToken, and clientSecret are covered as well.
 */
export function redactSensitiveLogData(
  value: unknown,
  ancestors: WeakSet<object> = new WeakSet(),
): unknown {
  if (typeof value === "string") {
    return redactSensitiveLogString(value);
  }

  if (value === null || typeof value !== "object") {
    return value;
  }

  if (value instanceof Date) {
    return value;
  }

  if (ancestors.has(value)) {
    return "[Circular]";
  }

  ancestors.add(value);

  const redacted = Array.isArray(value)
    ? value.map((item) => redactSensitiveLogData(item, ancestors))
    : Object.fromEntries(
        Object.entries(value).map(([key, nestedValue]) => [
          key,
          isSensitiveLogKey(key)
            ? REDACTED_LOG_VALUE
            : redactSensitiveLogData(nestedValue, ancestors),
        ]),
      );

  ancestors.delete(value);
  return redacted;
}
