/**
 * Backend int64 ids on the wire.
 *
 * Endpoints that serialize an id as a JSON STRING (`json:"id,string"`) do it so
 * the value survives the trip: a JSON number is an IEEE-754 double here, so an
 * id beyond 2^53 is rounded during parsing — before any mapper could preserve
 * it — and the rounded value is a valid id for a DIFFERENT row. A number is
 * still accepted so a response from an older server (or a hand-written test
 * fixture) still maps; only the string form is exact, which is why nothing
 * here ever converts an id back to a number.
 */
export type WireID = string | number;

/**
 * Normalizes a wire id to the decimal string the app uses everywhere (see the
 * int64 → string type-mapping rule in CLAUDE.md). A string passes through
 * verbatim — never via `Number()` — so no precision is lost.
 */
export function toIdString(id: WireID): string {
  return typeof id === "string" ? id : String(id);
}

/** Same, for an id the response may omit. */
export function toOptionalIdString(
  id: WireID | null | undefined,
): string | undefined {
  return id === null || id === undefined ? undefined : toIdString(id);
}
