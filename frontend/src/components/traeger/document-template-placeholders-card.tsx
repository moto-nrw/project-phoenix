import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import type { DocumentTemplatePlaceholder } from "~/lib/traeger-document-templates";

/** Die Schreibregeln des Brieftexts, wie das Backend sie liest (ADR 0152). */
const RULES: readonly { readonly mark: string; readonly text: string }[] = [
  { mark: "Leere Zeile", text: "beginnt einen neuen Absatz." },
  { mark: "# Text", text: "am Zeilenanfang macht die Zeile zur Überschrift." },
  { mark: "**Text**", text: "um einen ganzen Absatz setzt ihn fett." },
  {
    mark: "{{positionen}}",
    text: "allein in einer Zeile druckt eine Tabelle. Das gilt für jeden Platzhalter mit „(Tabelle)“.",
  },
  {
    mark: "Reihenfolge:",
    text: "Der Brief druckt Absätze und Tabellen in der Reihenfolge des Brieftexts. Verschieben Sie einen Absatz, steht er im Brief an der neuen Stelle.",
  },
  {
    mark: "Leere Platzhalter:",
    text: "Sind alle Platzhalter eines Absatzes leer, fehlt der Absatz im Brief.",
  },
];

function labelOf(placeholder: DocumentTemplatePlaceholder): string {
  return placeholder.table
    ? `${placeholder.label} (Tabelle)`
    : placeholder.label;
}

/**
 * Die Platzhalter einer Dokumentart (#4677). Wer ändern darf, fügt einen
 * Platzhalter mit einem Klick an der Schreibmarke des Brieftexts ein (#5691);
 * sonst ist die Liste nur zum Nachschlagen. Andere Namen lehnt moto beim
 * Speichern ab.
 */
export function DocumentTemplatePlaceholdersCard({
  placeholders,
  onInsert,
  disabled = false,
}: Readonly<{
  placeholders: readonly DocumentTemplatePlaceholder[];
  /** Fügt den Platzhalter in den Brieftext ein; ohne nur zum Nachschlagen. */
  onInsert?: (placeholder: DocumentTemplatePlaceholder) => void;
  disabled?: boolean;
}>) {
  return (
    <SectionCard
      title="Platzhalter"
      description={
        onInsert
          ? "Wählen Sie einen Platzhalter. moto setzt ihn im Brieftext dort ein, wo Sie gerade schreiben. Für jede Familie füllt moto ihn aus."
          : "Schreiben Sie einen Platzhalter mit zwei geschweiften Klammern in den Text. moto füllt ihn für jede Familie aus. Andere Platzhalter nimmt moto nicht an."
      }
    >
      {onInsert ? (
        <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-1">
          {placeholders.map((placeholder) => (
            <li key={placeholder.name} className="min-w-0">
              {/* Der Name kommt aus dem sichtbaren Text, damit er ihn enthält
                  (Sprachsteuerung); die geschweiften Klammern liest die
                  Vorlesehilfe nicht vor. */}
              <Button
                type="button"
                variant="outline"
                size="compact"
                disabled={disabled}
                tooltip="Setzt den Platzhalter an der Schreibmarke in den Brieftext."
                onClick={() => onInsert(placeholder)}
                className="w-full min-w-0 justify-between gap-3"
              >
                <code aria-hidden="true" className="shrink-0">
                  {`{{${placeholder.name}}}`}
                </code>
                <span className="min-w-0 truncate text-gray-600">
                  <span className="sr-only">{`Platzhalter „${labelOf(placeholder)}“ einfügen`}</span>
                  <span aria-hidden="true">{labelOf(placeholder)}</span>
                </span>
              </Button>
            </li>
          ))}
        </ul>
      ) : (
        <DataGrid columns={2}>
          {placeholders.map((placeholder) => (
            <DataField key={placeholder.name} label={labelOf(placeholder)} mono>
              {`{{${placeholder.name}}}`}
            </DataField>
          ))}
        </DataGrid>
      )}
      <h3 className="mt-6 text-sm font-semibold text-gray-900">
        So schreiben Sie den Brieftext
      </h3>
      <ul className="mt-2 space-y-1 text-sm text-gray-600">
        {RULES.map((rule) => (
          <li key={rule.mark}>
            <span className="font-mono text-gray-900">{rule.mark}</span>{" "}
            {rule.text}
          </li>
        ))}
      </ul>
    </SectionCard>
  );
}
