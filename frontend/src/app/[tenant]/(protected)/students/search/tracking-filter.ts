import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";

export type TrackingFilter = "all" | "none_visited" | `missing:${number}`;

export type ParsedTrackingFilter =
  | { readonly kind: "all" }
  | { readonly kind: "none_visited" }
  | { readonly kind: "missing"; readonly idx: number }
  | { readonly kind: "invalid" };

const MISSING_PATTERN = /^missing:(\d+)$/;

export function parseTrackingFilter(value: string): ParsedTrackingFilter {
  if (value === "all") return { kind: "all" };
  if (value === "none_visited") return { kind: "none_visited" };
  const match = MISSING_PATTERN.exec(value);
  // \d+ guarantees a non-empty digit string, so Number() cannot produce NaN here.
  if (match) return { kind: "missing", idx: Number(match[1]) };
  return { kind: "invalid" };
}

interface StudentLike {
  readonly id: string;
  readonly has_full_access?: boolean;
}

// Returns true if the student should be shown under the given tracking filter.
// Redacted students (has_full_access === false) are always hidden when the filter
// is active, matching the card-render rule that hides indicators for them.
// Students whose tracking row hasn't arrived yet (SWR keepPreviousData during
// revalidation) are treated as unknown → hidden, to avoid false positives.
export function matchesTrackingFilter(
  student: StudentLike,
  filter: TrackingFilter,
  trackingData: TrackingIndicatorsResponse | undefined,
): boolean {
  const parsed = parseTrackingFilter(filter);
  if (parsed.kind === "all") return true;
  if (parsed.kind === "invalid") return false;

  if (student.has_full_access === false) return false;

  const results = trackingData?.results?.[student.id];
  if (!results) return false;

  const labelCount = trackingData?.labels?.length ?? 0;

  if (parsed.kind === "none_visited") {
    if (labelCount === 0) return false;
    return results.every((r) => !r);
  }

  // parsed.kind === "missing"
  if (parsed.idx < 0 || parsed.idx >= labelCount) return false;
  return results[parsed.idx] !== true;
}

// Returns the German chip label for an active tracking filter, or null when
// the filter is "all" / invalid / points at a label index that no longer exists.
// Centralises the chip copy so the dropdown option label and the active-filter
// chip can't drift apart.
export function trackingFilterChipLabel(
  filter: TrackingFilter,
  labels: readonly string[],
): string | null {
  const parsed = parseTrackingFilter(filter);
  if (parsed.kind === "none_visited") return "Noch nichts erledigt";
  if (parsed.kind === "missing") {
    const lbl = labels[parsed.idx];
    return lbl ? `Noch nicht in ${lbl}` : null;
  }
  return null;
}

// Decides what trackingFilter should be after the configured labels change
// (admin add/remove/disable). Returns the input unchanged while tracking data
// is still loading so a pre-set filter isn't wiped on mount. Callers should
// only invoke setTrackingFilter when the return value differs from `filter`.
export function resolveTrackingFilterAfterLabelChange(
  filter: TrackingFilter,
  trackingData: TrackingIndicatorsResponse | undefined,
): TrackingFilter {
  if (trackingData === undefined) return filter;
  const labelCount = trackingData.labels?.length ?? 0;
  const parsed = parseTrackingFilter(filter);
  if (parsed.kind === "all") return filter;
  if (parsed.kind === "invalid" || labelCount === 0) return "all";
  if (parsed.kind === "missing" && parsed.idx >= labelCount) return "all";
  return filter;
}
