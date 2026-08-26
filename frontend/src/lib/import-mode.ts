/** Import modes of the student and staff importers (backend `ImportMode`). */
export type ImportMode = "create" | "update" | "upsert";

/** Options of the "Schritt 2" mode switcher on both import pages (#2600). */
export const IMPORT_MODE_ITEMS = [
  { value: "create", label: "Nur neue anlegen" },
  { value: "update", label: "Nur bestehende aktualisieren" },
  { value: "upsert", label: "Beides" },
] as const satisfies readonly { value: ImportMode; label: string }[];

/** One-line explanation shown under the selected mode. */
export const IMPORT_MODE_HINTS: Record<ImportMode, string> = {
  create: "Zeilen, die es schon gibt, werden als Fehler gemeldet.",
  update:
    "Zeilen, die es noch nicht gibt, werden als Fehler gemeldet. Leere Zellen ändern nichts.",
  upsert:
    "Neue Zeilen werden angelegt, bekannte Zeilen aktualisiert. Leere Zellen ändern nichts.",
};
