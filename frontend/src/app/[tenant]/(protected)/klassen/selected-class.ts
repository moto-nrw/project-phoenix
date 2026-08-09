// Auswahl gegen die aktuelle Klassenliste abgleichen: verschwindet die
// gewählte Klasse, während die Seite offen ist (Admin entzieht oder benennt
// sie um, die Fokus-Revalidierung zieht die neue Liste nach), fällt die
// Ansicht auf die erste Klasse zurück, statt mit einer Auswahl ohne Report
// und ohne Fehlerzustand leer zu bleiben.
export function reconcileSelectedClass(
  selected: string,
  classes: readonly string[] | undefined,
): string {
  return selected && classes?.includes(selected)
    ? selected
    : (classes?.[0] ?? "");
}
