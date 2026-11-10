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
  // The kit's one name for the step back to the full list, for a search
  // and for a filter alike (#5034).
  return { description, actionLabel: "Filter zurücksetzen" };
}
