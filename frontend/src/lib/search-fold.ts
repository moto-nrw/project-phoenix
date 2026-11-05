// Letters that carry their accent without a Unicode decomposition, so
// stripping combining marks leaves them unchanged. ß has no single-letter base
// and folds to "ss", as PostgreSQL's unaccent does.
const UNDECOMPOSABLE_LETTERS: Readonly<Record<string, string>> = {
  ł: "l",
  ø: "o",
  đ: "d",
  ı: "i",
  ß: "ss",
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
    .replace(/[łøđıß]/g, (letter) => UNDECOMPOSABLE_LETTERS[letter] ?? letter);
}
