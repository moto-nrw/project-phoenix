import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { SectionCard } from "~/components/ui/section-card";
import type { DocumentTemplatePlaceholder } from "~/lib/traeger-document-templates";

/** Die Schreibregeln des Brieftexts, wie das Backend sie liest (ADR 0150). */
const RULES: readonly { readonly mark: string; readonly text: string }[] = [
  { mark: "Leere Zeile", text: "beginnt einen neuen Absatz." },
  { mark: "# Text", text: "am Zeilenanfang macht die Zeile zur Überschrift." },
  { mark: "**Text**", text: "um einen ganzen Absatz setzt ihn fett." },
  {
    mark: "{{positionen}}",
    text: "allein in einer Zeile druckt eine Tabelle. Das gilt für jeden Platzhalter mit „(Tabelle)“.",
  },
  {
    mark: "Leere Platzhalter:",
    text: "Sind alle Platzhalter eines Absatzes leer, fehlt der Absatz im Brief.",
  },
];

/**
 * Die Platzhalter einer Dokumentart (#4677). Nur zum Nachschlagen: andere
 * Namen lehnt moto beim Speichern ab.
 */
export function DocumentTemplatePlaceholdersCard({
  placeholders,
}: Readonly<{ placeholders: readonly DocumentTemplatePlaceholder[] }>) {
  return (
    <SectionCard
      title="Platzhalter"
      description="Schreiben Sie einen Platzhalter mit zwei geschweiften Klammern in den Text. moto füllt ihn für jede Familie aus. Andere Platzhalter nimmt moto nicht an."
    >
      <DataGrid columns={2}>
        {placeholders.map((placeholder) => (
          <DataField
            key={placeholder.name}
            label={
              placeholder.table
                ? `${placeholder.label} (Tabelle)`
                : placeholder.label
            }
            mono
          >
            {`{{${placeholder.name}}}`}
          </DataField>
        ))}
      </DataGrid>
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
