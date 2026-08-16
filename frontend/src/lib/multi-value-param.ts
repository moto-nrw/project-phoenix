/**
 * Encoding for filters a school may pick several values for (#2218): school
 * class, group and school year travel as one comma-separated parameter, in the
 * URL, in the stored last-used filters, in the SWR cache key and in the export
 * request body.
 *
 * **Why the values are escaped.** `users.students.school_class` is free text, so
 * a class may be called "A,B" — and then a plain join is ambiguous: the two
 * separate classes A and B and the single class "A,B" both read back as
 * `school_class=A,B` (#2218 review). That is not only a wrong filter, it also
 * collides in the cache key, where two different selections would share one
 * entry. So a comma that belongs to the value is written `\,` and a literal
 * backslash `\\`; splitting cuts at unescaped commas only.
 *
 * A value carrying neither character encodes to itself, so `?school_class=3a,4b`
 * and every link, bookmark and stored filter that predates this keep working —
 * and the backend reads the same grammar (api/students/list_helpers.go).
 */

const SEPARATOR = ",";
const ESCAPE = "\\";

/** Trims, drops blanks and collapses duplicates, keeping the caller's order. */
export function normalizeMultiValues(values: readonly string[]): string[] {
  return [
    ...new Set(values.map((value) => value.trim()).filter((v) => v !== "")),
  ];
}

/**
 * Splits one parameter at its unescaped commas. Accepts the plain unescaped
 * form too, which is what a hand-written URL contains.
 */
export function parseMultiValueParam(
  value: string | null | undefined,
): string[] {
  if (!value) return [];

  const values: string[] = [];
  let current = "";
  let escaped = false;
  for (const char of value) {
    if (escaped) {
      current += char;
      escaped = false;
    } else if (char === ESCAPE) {
      escaped = true;
    } else if (char === SEPARATOR) {
      values.push(current);
      current = "";
    } else {
      current += char;
    }
  }
  values.push(current);

  return normalizeMultiValues(values);
}

/**
 * Renders a selection as one parameter value. The inverse of
 * {@link parseMultiValueParam} for every input: encode → parse returns the
 * normalized selection unchanged, commas and backslashes included.
 */
export function encodeMultiValueParam(values: readonly string[]): string {
  return normalizeMultiValues(values)
    .map((value) =>
      value
        .replaceAll(ESCAPE, ESCAPE + ESCAPE)
        .replaceAll(SEPARATOR, ESCAPE + SEPARATOR),
    )
    .join(SEPARATOR);
}
