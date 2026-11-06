// Letters that carry their accent without a Unicode decomposition, so
// stripping combining marks leaves them unchanged. ß has no single-letter base
// and folds to "ss", the ligatures æ and œ to "ae" and "oe" (#5791), as
// PostgreSQL's unaccent does; ǽ decomposes to æ first.
const UNDECOMPOSABLE_LETTERS: Readonly<Record<string, string>> = {
  ł: "l",
  ø: "o",
  đ: "d",
  ı: "i",
  ß: "ss",
  æ: "ae",
  œ: "oe",
};

/**
 * Reduces a text to the form a name search compares: lowercase and without
 * accents, so „gomez“ finds „Gómez“ and „ozdem“ finds „Özdemir“ (#4949).
 * Umlauts fold to their base letter: „Müller“ becomes „muller“, not
 * „mueller“. Apply it to both the searched text and the query. It matches
 * `public.f_unaccent(…) ILIKE` in the backend and `collation.FoldForSearch`.
 */
export function foldForSearch(value: string | null | undefined): string {
  if (!value) return "";
  return value
    .toLocaleLowerCase("de")
    .normalize("NFD")
    .replace(/\p{Mn}/gu, "")
    .replace(
      /[łøđıßæœ]/g,
      (letter) => UNDECOMPOSABLE_LETTERS[letter] ?? letter,
    );
}
