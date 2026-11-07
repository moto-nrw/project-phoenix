import "server-only";
import { env } from "~/env";

/** Nanoseconds per unit, as Go's time.ParseDuration knows them. */
const UNIT_NS: Record<string, number> = {
  ns: 1,
  us: 1e3,
  µs: 1e3, // U+00B5 micro sign
  μs: 1e3, // U+03BC Greek mu
  ms: 1e6,
  s: 1e9,
  m: 60e9,
  h: 3600e9,
};

const COMPONENT = /(\d*)(?:\.(\d*))?(ns|us|µs|μs|ms|s|m|h)/y;

/**
 * Milliseconds of a duration in the grammar of Go's time.ParseDuration:
 * the backend reads AUTH_JWT_EXPIRY and AUTH_JWT_REFRESH_EXPIRY with it, so
 * this accepts exactly what the backend accepts (`15m`, `1h30m`, `1.5h`)
 * and fails at startup on what the backend refuses too (`7d`).
 */
function goDurationMs(duration: string): number | null {
  let rest = duration;
  let sign = 1;
  if (rest.startsWith("-") || rest.startsWith("+")) {
    sign = rest.startsWith("-") ? -1 : 1;
    rest = rest.slice(1);
  }
  if (rest === "0") return 0;
  if (rest === "") return null;

  let ns = 0;
  COMPONENT.lastIndex = 0;
  while (COMPONENT.lastIndex < rest.length) {
    const start = COMPONENT.lastIndex;
    const match = COMPONENT.exec(rest);
    if (!match || match.index !== start) return null;
    const [, whole = "", fraction = ""] = match;
    if (whole === "" && fraction === "") return null;
    const value = Number(`${whole || "0"}.${fraction || "0"}`);
    ns += value * UNIT_NS[match[3]!]!;
  }
  return (sign * ns) / 1e6;
}

/**
 * A malformed value fails at startup instead of becoming another lifetime.
 * The value is missing only while env validation is skipped (`next build`);
 * the server validates its env at startup (instrumentation.ts).
 */
export function parseDurationToMs(
  name: string,
  duration: string | undefined,
): number {
  if (duration === undefined) return Number.NaN;
  const ms = goDurationMs(duration);
  if (ms === null) {
    throw new Error(
      `${name} must be a Go duration like "15m", "1h30m" or "168h", got "${duration}"`,
    );
  }
  return ms;
}

export const accessTokenExpiry = parseDurationToMs(
  "AUTH_JWT_EXPIRY",
  env.AUTH_JWT_EXPIRY,
);
export const refreshTokenExpiry = parseDurationToMs(
  "AUTH_JWT_REFRESH_EXPIRY",
  env.AUTH_JWT_REFRESH_EXPIRY,
);

/** Access tokens inside this window are refreshed by the JWT callback. */
export const ACCESS_TOKEN_REFRESH_BUFFER_MS = 5 * 60 * 1000;
