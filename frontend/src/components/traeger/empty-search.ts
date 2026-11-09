/**
 * One wording for a search or filter without a hit, on every Träger list:
 * what is active, and the button that clears it.
 */
export function emptySearchText(
  search: string,
  filterLabels: readonly string[],
): { readonly description: string; readonly actionLabel: string } {
  const term = search.trim();
  const description = [
    term ? `Gesucht: „${term}“.` : "",
    filterLabels.length > 0 ? `Filter: ${filterLabels.join(", ")}.` : "",
  ]
    .filter(Boolean)
    .join(" ");
  return {
    description,
    actionLabel:
      filterLabels.length > 0 ? "Filter zurücksetzen" : "Suche zurücksetzen",
  };
}
