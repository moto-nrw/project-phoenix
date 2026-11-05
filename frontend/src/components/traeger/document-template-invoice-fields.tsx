import { CheckboxCard } from "~/components/ui/checkbox-card";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import type { DocumentTemplateInvoiceLayout } from "~/lib/traeger-document-templates";

/**
 * Die Darstellung der Tabelle der Beträge (#5691): wie `{{positionen}}` die
 * Positionen einer Rechnung zeigt. Betrag, Ermäßigungen, Korrekturen und
 * Gesamtbetrag sind fest; sie sind das, was die Rechnung verlangt. Ein Teil
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
        So zeigt die Tabelle {"{{positionen}}"} die gebuchten Wochen. Betrag,
        Ermäßigungen und Gesamtbetrag stehen immer darin.
      </p>
      <div className="mt-3 grid gap-2 lg:grid-cols-3">
        <CheckboxCard
          checked={layout.groupByChild}
          onChange={(checked) => set("groupByChild", checked)}
          disabled={disabled}
          label="Nach Kind ordnen"
          hint="Der Vorname steht einmal über den Zeilen, darunter die Summe für das Kind."
        />
        <CheckboxCard
          checked={!layout.hideWeeks}
          onChange={(checked) => set("hideWeeks", !checked)}
          disabled={disabled}
          label="Woche zeigen"
          hint="Zum Beispiel „Woche 19.10. bis 23.10.2026“."
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
