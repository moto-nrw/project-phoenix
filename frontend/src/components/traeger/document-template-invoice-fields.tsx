import { CheckboxCard } from "~/components/ui/checkbox-card";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import type { DocumentTemplateInvoiceLayout } from "~/lib/traeger-document-templates";

/**
 * Die Darstellung der Tabelle der Beträge (#5691): wie `{{positionen}}` die
 * Positionen einer Rechnung zeigt. Betrag, Ermäßigungen, Korrekturen,
 * Gesamtbetrag und die Woche sind fest: Sie sind Pflichtangaben einer
 * Rechnung, die Woche ist der Zeitraum der Leistung. Ein Teil
 * des Vorlagenformulars, weil das Speichern Text und Tabelle zusammen als
 * eine Version ablegt.
 */
export function DocumentTemplateInvoiceFields({
  layout,
  onChange,
  disabled,
  error,
}: Readonly<{
  layout: DocumentTemplateInvoiceLayout;
  onChange: (layout: DocumentTemplateInvoiceLayout) => void;
  disabled: boolean;
  error?: string;
}>) {
  const set = (part: keyof DocumentTemplateInvoiceLayout, value: boolean) =>
    onChange({ ...layout, [part]: value });

  return (
    <fieldset
      aria-describedby={
        error ? "template-invoice-error" : "template-invoice-hint"
      }
      aria-invalid={error ? true : undefined}
      className="border-t border-gray-100 pt-5"
    >
      <legend className="float-left w-full text-sm font-semibold text-gray-900">
        Rechnungstabelle
      </legend>
      <p
        id="template-invoice-hint"
        className="clear-left mt-1 text-sm text-gray-600"
      >
        So zeigt die Tabelle {"{{positionen}}"} die gebuchten Wochen. Woche,
        Betrag und Gesamtbetrag stehen immer darin.
      </p>
      <div className="mt-3 grid gap-2 sm:grid-cols-2 xl:grid-cols-1">
        <CheckboxCard
          checked={layout.groupByChild}
          onChange={(checked) => set("groupByChild", checked)}
          disabled={disabled}
          label="Nach Kind ordnen"
          hint="Der Vorname steht über den Zeilen. Darunter steht die Summe."
        />

        <CheckboxCard
          checked={!layout.hideTimeModels}
          onChange={(checked) => set("hideTimeModels", !checked)}
          disabled={disabled}
          label="Zeitmodell zeigen"
          hint="Zum Beispiel „Vormittag“ oder „Nachmittag“."
        />
      </div>
      <FieldErrorHint id="template-invoice-error" message={error} />
    </fieldset>
  );
}
