// Shared by the Arbeitszeitmodell tab and its Sonderarbeitszeiten section, so
// the section does not import the tab that renders it.

// A work-time model save rewrites the contractual Soll, so every cache that
// prices days against it goes stale. Two portals read the same staff member's
// targets: the admin staff-detail tabs key them by staff id
// (staff-schedule-targets- / staff-month-summary-), while a manager editing
// their OWN model also has the own-service portal's caches open, which key
// WITHOUT an id (time-tracking-schedule-targets- for the daily table and the
// weekly KPI, time-tracking-month-summary- for the Monatskarte). Both sets must
// be invalidated, or the self-service daily table keeps showing the old
// Soll/Saldo while the monthly summary has already updated (#1842). Everything
// is recomputed live on the server, so a plain invalidation suffices. useSWRAuth
// prefixes keys with the tenant slug, so we match with includes, not startsWith
// — the same convention as staff-session-table's handleSaved.
export function isStaleAfterModelSave(key: unknown): boolean {
  return (
    typeof key === "string" &&
    (key.includes("staff-schedule-targets-") ||
      key.includes("staff-month-summary-") ||
      key.includes("time-tracking-schedule-targets-") ||
      key.includes("time-tracking-month-summary-"))
  );
}

const MAX_DAILY_HOURS = 12;
const DECIMAL_HOURS_FORMAT = new Intl.NumberFormat("de-DE", {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
});

export type DecimalHoursParseResult =
  | { status: "empty" }
  | { status: "invalid" }
  | { status: "valid"; minutes: number };

export function parseDecimalHours(value: string): DecimalHoursParseResult {
  const normalized = value.trim().replace(",", ".");
  if (normalized === "") return { status: "empty" };
  if (!/^(?:\d+(?:\.\d*)?|\.\d+)$/.test(normalized)) {
    return { status: "invalid" };
  }

  const [wholePart = "0", fractionPart = ""] = normalized.split(".");
  const scale = 10n ** BigInt(fractionPart.length);
  const decimalUnits =
    BigInt(wholePart || "0") * scale + BigInt(fractionPart || "0");
  if (decimalUnits > BigInt(MAX_DAILY_HOURS) * scale) {
    return { status: "invalid" };
  }
  const minutes = Number((decimalUnits * 60n * 2n + scale) / (scale * 2n));
  return { status: "valid", minutes };
}

export function formatDecimalHours(minutes: number): string {
  return DECIMAL_HOURS_FORMAT.format(minutes / 60);
}
