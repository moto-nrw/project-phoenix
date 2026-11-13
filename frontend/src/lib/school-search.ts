import { bffJson } from "~/lib/bff-fetch";

/**
 * The school search (#6954, ADR 0231). The query travels in a POST body and
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

interface SchoolSearchContactChild {
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

interface SchoolSearchStaff {
  readonly id: string;
  readonly firstName: string;
  readonly lastName: string;
}

interface SchoolSearchRoom {
  readonly id: string;
  readonly name: string;
  readonly building?: string;
}

interface SchoolSearchGroup {
  readonly id: string;
  readonly name: string;
}

/**
 * What this account may find, as the server decided it from its rights. It
 * depends on the account only, never on the hits.
 */
export interface SchoolSearchScope {
  readonly children: boolean;
  readonly childDetails: boolean;
  readonly contacts: boolean;
  readonly staff: boolean;
  readonly rooms: boolean;
  readonly groups: boolean;
}

export interface SchoolSearchResult {
  readonly children: readonly SchoolSearchChild[];
  readonly contacts: readonly SchoolSearchContact[];
  readonly staff: readonly SchoolSearchStaff[];
  readonly rooms: readonly SchoolSearchRoom[];
  readonly groups: readonly SchoolSearchGroup[];
  readonly scope: SchoolSearchScope;
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

/** Fewest digits a phone query keeps after its prefixes go. */
const MIN_PHONE_DIGITS = 4;
/** From this length a leading 49 without + or 00 is the country code. */
const MIN_FULL_GERMAN_DIGITS = 11;

// Separators between the digits of a number, as the backend's
// isPhoneSeparator: every Unicode space, every dash, the minus sign and
// + / . ( ).
const PHONE_QUERY = /^[0-9\p{White_Space}\p{Pd}\u2212+/.()]+$/u;
const TRUNK_ZERO_MARKER = /\( *0 *\)/g;

/**
 * The digits a phone query is compared with, or "" when the query is no
 * phone number: the backend's schoolsearchview.PhoneSearchKey, with the same
 * cases in both tests. The key drops the bracketed trunk zero, the
 * international 00, the German country code 49 and the German trunk zero; a
 * foreign number keeps its country code. A leading 49 without + or 00 goes
 * only from eleven digits on. The key needs at least four digits.
 */
export function phoneSearchKey(query: string): string {
  if (!PHONE_QUERY.test(query)) return "";
  const text = query.replace(TRUNK_ZERO_MARKER, "");
  const marks = text.replace(/[^0-9+]/g, "");
  let key = text.replace(/[^0-9]/g, "");
  const international = marks.startsWith("+") || key.startsWith("00");
  if (key.startsWith("00")) key = key.slice(2);
  if (
    key.startsWith("49") &&
    (international || key.length >= MIN_FULL_GERMAN_DIGITS)
  ) {
    key = key.slice(2);
  }
  if (!international || key.startsWith("0")) key = key.replace(/^0+/, "");
  return key.length >= MIN_PHONE_DIGITS ? key : "";
}

/**
 * Whether the backend reads the query as a phone number. The dialog uses it
 * to explain a phone search the account cannot run.
 */
export function looksLikePhoneNumber(query: string): boolean {
  return phoneSearchKey(query) !== "";
}

/**
 * A registration question as the label in front of its answer: its own end
 * mark goes, so "Mein Kind darf schwimmen." reads "Mein Kind darf schwimmen:
 * Ja" and "Seepferdchen?" reads "Seepferdchen: Ja".
 */
export function answerQuestionLabel(question: string): string {
  return question.replace(/[\s.:?!]+$/, "");
}
