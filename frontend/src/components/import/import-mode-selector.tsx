import { RefreshCw } from "lucide-react";
import { SegmentedControl } from "~/components/ui/segmented-control";

/** Import modes of the student and staff importers (backend `ImportMode`). */
export type ImportMode = "create" | "update" | "upsert";

const IMPORT_MODE_ITEMS = [
  { value: "create", label: "Nur neue anlegen" },
  { value: "update", label: "Nur bestehende aktualisieren" },
  { value: "upsert", label: "Beides" },
] as const satisfies readonly { value: ImportMode; label: string }[];

interface ImportModeSelectorProps {
  readonly value: ImportMode;
  readonly onChange: (next: ImportMode) => void;
  /** How an existing row is recognised, e.g. "Vorname, Nachname und Klasse". */
  readonly matchHint: string;
}

const MODE_HINTS: Record<ImportMode, string> = {
  create: "Zeilen, die es schon gibt, werden als Fehler gemeldet.",
  update:
    "Zeilen, die es noch nicht gibt, werden als Fehler gemeldet. Leere Zellen ändern nichts.",
  upsert:
    "Neue Zeilen werden angelegt, bekannte Zeilen aktualisiert. Leere Zellen ändern nichts.",
};

/**
 * Lets the user decide what the import does with rows that already exist
 * (#2600). Shared by the child and the staff import page.
 */
export function ImportModeSelector({
  value,
  onChange,
  matchHint,
}: ImportModeSelectorProps) {
  return (
    <div className="rounded-xl border border-gray-100 bg-white p-6">
      <h3 className="mb-4 flex items-center gap-2 text-sm font-semibold text-gray-900">
        <RefreshCw className="h-5 w-5 text-gray-600" aria-hidden="true" />
        Schritt 2: Was soll der Import tun?
      </h3>
      <SegmentedControl
        items={IMPORT_MODE_ITEMS}
        value={value}
        onChange={onChange}
        fullWidth
        ariaLabel="Import-Modus"
      />
      <p className="mt-3 text-sm text-gray-600">{MODE_HINTS[value]}</p>
      <p className="mt-1 text-sm text-gray-500">
        Bekannt ist eine Zeile über {matchHint}.
      </p>
    </div>
  );
}
