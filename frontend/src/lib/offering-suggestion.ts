import type { OfferingSelection } from "./roster-deviation";

/**
 * Children per ISO weekday of a Regeltermin that is not saved yet (#5104):
 * what the editor would plan, used to suggest an offering and to preview
 * the switch to it.
 */
export type DraftRoster = Record<number, string[]>;

/** An offering whose bookings match the children the editor plans. */
export interface OfferingSuggestion {
  offering: { id: string; name: string };
  /** Klassen- or Jahrgangsfilter, taken from the Zielgruppe. */
  gradeLevels: number[];
  schoolClasses: string[];
  unbookedCount: number;
  missingCount: number;
  /** Every offering that passes the rule, best first. */
  candidates: Array<{ id: string; name: string }>;
  /** Several candidates fit fully (#5138): the school picks one. */
  ambiguous: boolean;
}

export interface OfferingSuggestionQuery {
  periodId: string;
  type: string;
  listKind?: string;
  roster: DraftRoster;
  schoolClasses: string[];
  gradeLevels: number[];
}

interface BackendOfferingRef {
  id: number;
  name: string;
}

export interface BackendOfferingSuggestionResponse {
  suggestion?: {
    offering: BackendOfferingRef;
    grade_levels?: number[] | null;
    school_classes?: string[] | null;
    unbooked_count: number;
    missing_count: number;
    candidates?: BackendOfferingRef[] | null;
    ambiguous?: boolean;
  } | null;
}

function appendRoster(params: URLSearchParams, roster: DraftRoster): void {
  const weekdays = Object.keys(roster)
    .map(Number)
    .sort((a, b) => a - b);
  for (const weekday of weekdays) {
    params.append("roster", `${weekday}:${(roster[weekday] ?? []).join(",")}`);
  }
}

export function suggestionQuery(
  query: OfferingSuggestionQuery,
): URLSearchParams {
  const params = new URLSearchParams();
  params.set("period_id", query.periodId);
  params.set("type", query.type);
  if (query.listKind) params.set("list_kind", query.listKind);
  appendRoster(params, query.roster);
  if (query.schoolClasses.length > 0) {
    params.set("school_classes", query.schoolClasses.join(","));
  }
  if (query.gradeLevels.length > 0) {
    params.set("grade_levels", query.gradeLevels.join(","));
  }
  return params;
}

export function draftPreviewQuery(
  periodId: string,
  selection: OfferingSelection,
  roster: DraftRoster,
  from?: string,
): URLSearchParams {
  const params = new URLSearchParams();
  params.set("period_id", periodId);
  params.set("offering_ids", selection.offeringIds.join(","));
  if (selection.gradeLevels.length > 0) {
    params.set("grade_levels", selection.gradeLevels.join(","));
  }
  if (selection.schoolClasses.length > 0) {
    params.set("school_classes", selection.schoolClasses.join(","));
  }
  appendRoster(params, roster);
  if (from) params.set("from", from);
  return params;
}

const offeringRef = (offering: BackendOfferingRef) => ({
  id: String(offering.id),
  name: offering.name,
});

export function mapOfferingSuggestion(
  raw: BackendOfferingSuggestionResponse,
): OfferingSuggestion | null {
  const suggestion = raw.suggestion;
  if (!suggestion) return null;
  return {
    offering: offeringRef(suggestion.offering),
    gradeLevels: suggestion.grade_levels ?? [],
    schoolClasses: suggestion.school_classes ?? [],
    unbookedCount: suggestion.unbooked_count,
    missingCount: suggestion.missing_count,
    candidates: (suggestion.candidates ?? []).map(offeringRef),
    ambiguous: suggestion.ambiguous ?? false,
  };
}

export function suggestionSelection(
  suggestion: OfferingSuggestion,
): OfferingSelection {
  return {
    offeringIds: [suggestion.offering.id],
    gradeLevels: suggestion.gradeLevels,
    schoolClasses: suggestion.schoolClasses,
  };
}
