import { bffJson } from "~/lib/bff-fetch";

/**
 * The school search (#6954, ADR 0227). The query travels in a POST body and
 * never in a URL, so a phone number or a name stays out of access logs,
 * browser history and error reports.
 */

/** Why a row is in the list. */
export type SchoolSearchMatchField =
  | "name"
  | "class"
  | "group"
  | "parent_note"
  | "enrollment_answer"
  | "phone"
  | "email"
  | "contact";

export interface SchoolSearchMatch {
  readonly field: SchoolSearchMatchField;
  /** The registration question of an answer hit. */
  readonly label?: string;
  /** The matched value as stored: a number, an address, a note. */
  readonly text?: string;
}

export interface SchoolSearchChild {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass: string;
  readonly groupName?: string;
  readonly match: SchoolSearchMatch;
}

export interface SchoolSearchContactChild {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly schoolClass: string;
  readonly relationshipType: string;
}

export interface SchoolSearchContact {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
  readonly match: SchoolSearchMatch;
  readonly children: readonly SchoolSearchContactChild[];
}

export interface SchoolSearchStaff {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
}

export interface SchoolSearchRoom {
  readonly id: string;
  readonly name: string;
  readonly building?: string;
}

export interface SchoolSearchGroup {
  readonly id: string;
  readonly name: string;
}

export interface SchoolSearchResult {
  readonly children: readonly SchoolSearchChild[];
  readonly contacts: readonly SchoolSearchContact[];
  readonly staff: readonly SchoolSearchStaff[];
  readonly rooms: readonly SchoolSearchRoom[];
  readonly groups: readonly SchoolSearchGroup[];
}

/** Shortest query the backend searches; shorter input shows only pages. */
export const SCHOOL_SEARCH_MIN_LENGTH = 2;
/** Longest query the backend accepts. */
export const SCHOOL_SEARCH_MAX_LENGTH = 100;

/** Runs one search through the BFF. */
export function searchSchool(
  query: string,
  signal?: AbortSignal,
): Promise<SchoolSearchResult> {
  return bffJson<SchoolSearchResult>("/api/search", {
    method: "POST",
    body: { query },
    signal,
  });
}

/**
 * Reads a query as a phone number the way the backend does
 * (schoolsearchview.PhoneSearchKey): only digits and the separators people
 * type, at least four digits. The dialog uses it to explain a phone search
 * and to never offer a name search link for a number.
 */
export function looksLikePhoneNumber(query: string): boolean {
  const trimmed = query.trim();
  if (!/^[\d\s+\-/.()]+$/.test(trimmed)) return false;
  return (trimmed.match(/\d/g) ?? []).length >= 4;
}
