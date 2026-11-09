"use client";

// Zuordnung der Spalten beim Mandatsimport (#6198): je Feld eines Mandats die
// Spalte der Datei, aus der es kommt. moto schlägt sie aus den Überschriften
// vor; jede Änderung liest die Datei neu. Fehlt die Spalte mit der
// Gläubiger-ID, bestätigt das Büro ausdrücklich, dass die Datei seine
// Mandate enthält.

import { CheckboxCard } from "~/components/ui/checkbox-card";
import { CustomSelect } from "~/components/ui/custom-select";
import { FieldLabel } from "~/components/ui/field-label";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type {
  ImportField,
  ImportMapping,
  PayerKey,
} from "~/lib/traeger-sepa-import-api";
import {
  IMPORT_FIELD_HINTS,
  IMPORT_FIELD_LABELS,
  IMPORT_FIELDS,
  PAYER_KEY_LABELS,
  REQUIRED_IMPORT_FIELDS,
} from "./sepa-import-format";

const UNMAPPED = "";

const PAYER_KEYS: { value: PayerKey; label: string }[] = [
  { value: "email", label: PAYER_KEY_LABELS.email },
  { value: "contract_id", label: PAYER_KEY_LABELS.contract_id },
];

export function SepaImportMappingCard({
  headers,
  mapping,
  unmapped,
  creditorId,
  disabled,
  onChange,
}: Readonly<{
  headers: readonly string[];
  mapping: ImportMapping;
  unmapped: readonly ImportField[];
  creditorId: string;
  disabled: boolean;
  onChange: (mapping: ImportMapping) => void;
}>) {
  const options = [
    { value: UNMAPPED, label: "Nicht zuordnen" },
    ...headers.map((header) => ({ value: header, label: header })),
  ];
  const setColumn = (field: ImportField, header: string) =>
    onChange({ ...mapping, columns: { ...mapping.columns, [field]: header } });
  const missing = new Set(unmapped);
  const withoutCreditorColumn = mapping.columns.creditor_id === UNMAPPED;

  return (
    <SectionCard
      title="Spalten zuordnen"
      description="Wählen Sie für jedes Feld die Spalte aus Ihrer Datei. moto hat schon passende Spalten vorgeschlagen."
    >
      <div className="grid gap-x-5 gap-y-4 sm:grid-cols-2 xl:grid-cols-3">
        {IMPORT_FIELDS.map((field) => (
          <CustomSelect
            key={field}
            id={`sepa-import-${field}`}
            label={IMPORT_FIELD_LABELS[field]}
            required={REQUIRED_IMPORT_FIELDS.has(field)}
            value={mapping.columns[field]}
            options={options}
            onChange={(header) => setColumn(field, header)}
            disabled={disabled}
            hint={IMPORT_FIELD_HINTS[field]}
            error={
              missing.has(field) && field !== "creditor_id"
                ? "Bitte eine Spalte wählen."
                : null
            }
          />
        ))}
      </div>
      <div className="mt-5 grid gap-4 border-t border-gray-100 pt-5 lg:grid-cols-2">
        <div>
          <FieldLabel id="sepa-import-payer-key">
            Die Spalte „Zahler“ enthält
          </FieldLabel>
          <SegmentedControl
            ariaLabel="Die Spalte Zahler enthält"
            items={PAYER_KEYS.map((item) => ({ ...item, disabled }))}
            value={mapping.payerKey}
            onChange={(payerKey) => onChange({ ...mapping, payerKey })}
          />
          <p className="mt-2 text-xs text-gray-500">
            {mapping.payerKey === "email"
              ? "Die E-Mail-Adresse aus dem Vertrag der Familie."
              : "Die Nummer des Vertrags in moto, zum Beispiel 1042."}
          </p>
        </div>
        {withoutCreditorColumn ? (
          <CheckboxCard
            checked={mapping.creditorFromTraeger}
            onChange={(creditorFromTraeger) =>
              onChange({ ...mapping, creditorFromTraeger })
            }
            disabled={disabled}
            label={`Alle Mandate gehören zu unserer Gläubiger-ID ${creditorId}`}
            hint="Ihre Datei hat keine Spalte mit der Gläubiger-ID. Bestätigen Sie, dass jedes Mandat für diese Gläubiger-ID unterschrieben wurde."
          />
        ) : null}
      </div>
    </SectionCard>
  );
}
