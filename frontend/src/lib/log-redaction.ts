export const REDACTED_LOG_VALUE = "[REDACTED]";

const SENSITIVE_LOG_KEY_PARTS = new Set([
  "jwt",
  "password",
  "pin",
  "token",
  "secret",
]);
const SENSITIVE_LOG_KEYS = new Set([
  "apikey",
  "api_key",
  "authorization",
  "cookie",
  "cookies",
  "proxy_authorization",
  "set_cookie",
]);

const SENSITIVE_PATH_SEGMENT_PATTERN =
  /((?:^|\/)(?:enroll\/status|accept-guardian-invite|calendar-feed|enrollment\/requests|guardian-invitations)\/)[^/?#\s"']+/gi;
const SENSITIVE_CREDENTIAL_NAME =
  "password|pin|token|secret|jwt|api[_-]?key|access[_-]?token|refresh[_-]?token|client[_-]?secret|confirm[_-]?password|device[_-]?pin|late[_-]?invite";
const SENSITIVE_TEXT_FIELD_NAME = `(?:${SENSITIVE_CREDENTIAL_NAME})`;
const SENSITIVE_PARAMETER_NAME = `(?:${SENSITIVE_CREDENTIAL_NAME}|authorization|cookies?)`;
const SENSITIVE_QUERY_PARAMETER_PATTERN = new RegExp(
  String.raw`([?&]${SENSITIVE_PARAMETER_NAME}=)[^&#\s"']*`,
  "gi",
);
const SENSITIVE_QUOTED_TEXT_FIELD_PATTERN = new RegExp(
  String.raw`((?:^|[\s,{])['"]?${SENSITIVE_TEXT_FIELD_NAME}['"]?\s*[:=]\s*)(["'])(?:\\.|(?!\2)[^\\])*\2`,
  "gi",
);
const SENSITIVE_UNQUOTED_TEXT_FIELD_PATTERN = new RegExp(
  String.raw`((?:^|[\s,{])['"]?${SENSITIVE_TEXT_FIELD_NAME}['"]?\s*[:=]\s*)[^'",}\s&]+`,
  "gi",
);
const SENSITIVE_QUOTED_HEADER_FIELD_PATTERN =
  /((?:^|[\s,{])["']?(?:authorization|cookies?|(?:x[-_])?api[-_]?key)["']?\s*:\s*)(["'])(?:\\.|(?!\2)[^\\])*\2/gi;
const AUTHORIZATION_CREDENTIAL_PATTERN =
  /(\b(?:Bearer|Basic|Digest|ApiKey)\s+)[^\s"',;]+/gi;
const AUTHORIZATION_HEADER_PATTERN =
  /(\b(?:Proxy-)?Authorization\s*:\s*)[^\r\n]+/gi;
const API_KEY_HEADER_PATTERN =
  /(\b(?:X[-_])?API[-_]?Key["']?\s*:\s*["']?)[^"',}\s]+/gi;
const COOKIE_HEADER_PATTERN = /(\b(?:Set-)?Cookie\s*:\s*)[^\r\n]+/gi;
const SAFE_TOKEN_PRESENCE_KEY_PATTERN = /^has_(?:[a-z0-9]+_)*tokens?$/;
const SAFE_TOKEN_EXPIRY_KEY_PATTERN =
  /^(?:[a-z0-9]+_)*token_(?:expiry|expires_at)$/;
const SAFE_DURATION_PATTERN = /^\d+(?:\.\d+)?(?:ms|s|m|h|d)$/;
const ISO_TIMESTAMP_PATTERN =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:\d{2})$/;

function normalizeLogKey(key: string): string {
  return key
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .toLowerCase()
    .split(/[^a-z0-9]+/)
    .filter(Boolean)
    .join("_");
}

function isSensitiveLogKey(key: string): boolean {
  const normalized = normalizeLogKey(key);
  const parts = normalized.split("_");

  return (
    SENSITIVE_LOG_KEYS.has(normalized) ||
    parts.some((part) => SENSITIVE_LOG_KEY_PARTS.has(part)) ||
    parts.some((part, index) => part === "api" && parts[index + 1] === "key")
  );
}

function isSafeTokenMetadata(key: string, value: unknown): boolean {
  const normalized = normalizeLogKey(key);

  if (SAFE_TOKEN_PRESENCE_KEY_PATTERN.test(normalized)) {
    return typeof value === "boolean";
  }

  if (!SAFE_TOKEN_EXPIRY_KEY_PATTERN.test(normalized)) {
    return false;
  }

  return (
    value === null ||
    value instanceof Date ||
    (typeof value === "number" && Number.isFinite(value)) ||
    (typeof value === "string" &&
      (value === "not set" ||
        SAFE_DURATION_PATTERN.test(value) ||
        ISO_TIMESTAMP_PATTERN.test(value)))
  );
}

/** Redact credentials embedded in URLs or already-serialized text. */
export function redactSensitiveLogString(value: string): string {
  return value
    .replace(SENSITIVE_PATH_SEGMENT_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(SENSITIVE_QUERY_PARAMETER_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(
      SENSITIVE_QUOTED_HEADER_FIELD_PATTERN,
      `$1$2${REDACTED_LOG_VALUE}$2`,
    )
    .replace(SENSITIVE_QUOTED_TEXT_FIELD_PATTERN, `$1$2${REDACTED_LOG_VALUE}$2`)
    .replace(SENSITIVE_UNQUOTED_TEXT_FIELD_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(AUTHORIZATION_CREDENTIAL_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(AUTHORIZATION_HEADER_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(API_KEY_HEADER_PATTERN, `$1${REDACTED_LOG_VALUE}`)
    .replace(COOKIE_HEADER_PATTERN, `$1${REDACTED_LOG_VALUE}`);
}

/**
 * Return a log-safe copy of a value without mutating the caller's data.
 * Sensitive fields are matched by key at every nesting level so variants such
 * as confirm_password, accessToken, apiKey, and clientSecret are covered as
 * well.
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
          isSensitiveLogKey(key) && !isSafeTokenMetadata(key, nestedValue)
            ? REDACTED_LOG_VALUE
            : redactSensitiveLogData(nestedValue, ancestors),
        ]),
      );

  ancestors.delete(value);
  return redacted;
}
