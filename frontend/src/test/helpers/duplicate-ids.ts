/**
 * IDs that occur more than once in the whole document. Reads `document`, not
 * a render container, so portaled menus and listboxes count too.
 */
export function duplicateIds(): string[] {
  const counts = new Map<string, number>();
  for (const element of document.querySelectorAll("[id]")) {
    counts.set(element.id, (counts.get(element.id) ?? 0) + 1);
  }
  return [...counts].filter(([, count]) => count > 1).map(([id]) => id);
}
