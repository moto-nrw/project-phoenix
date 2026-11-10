/**
 * One wording for a search or filter without a hit, on every Träger list:
 * what is active, what to do, and the button that clears it.
 */
export function emptySearchText(
  search: string,
  filterLabels: readonly string[],
  /** The page's own next step for a search alone, e.g. what else to try. */
  searchHint = "Prüfen Sie die Schreibweise.",
): { readonly description: string; readonly actionLabel: string } {
  const term = search.trim();
  const filtered = filterLabels.length > 0;
  // What is active, then what to do about it.
  const next = term
    ? filtered
      ? "Ändern Sie die Suche oder den Filter."
      : searchHint
    : filtered
      ? "Ändern Sie den Filter."
      : "";
  const description = [
    term ? `Gesucht: „${term}“.` : "",
    filtered ? `Filter: ${filterLabels.join(", ")}.` : "",
    next,
  ]
    .filter(Boolean)
    .join(" ");
  // The kit's one name for the step back to the full list, for a search
  // and for a filter alike (#5034).
  return { description, actionLabel: "Filter zurücksetzen" };
}
