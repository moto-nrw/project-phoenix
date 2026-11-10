"use client";

// Die Feldzuordnung der Sollstellung (#4693): welche Spalten die Datei für
// die Kämmerei hat, in welcher Reihenfolge, mit welcher Überschrift, mit
// welchen Trennzeichen und unter welchem Namen. Die Vorschau zeigt die
// ersten Zeilen der Datei mit einer erfundenen Rechnung und ihrem Storno.

import { useRef, useState } from "react";
import { Plus } from "lucide-react";
import {
  SOLL_DATES,
  SOLL_DECIMALS,
  SOLL_DELIMITERS,
  SOLL_ENCODINGS,
  SOLL_FIELDS,
  SOLL_FIELD_LABELS,
  sollFileName,
  sollPreview,
} from "~/components/traeger/finance-export-format";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { EditActions } from "~/components/ui/edit-actions";
import { FieldErrorHint } from "~/components/ui/field-error-hint";
import { FieldLabel } from "~/components/ui/field-label";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { InsetPanel } from "~/components/ui/inset-panel";
import { Input } from "~/components/ui/input";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { SwitchRow } from "~/components/ui/switch";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  type ExportSettings,
  type SollColumn,
  type SollField,
  type SollMapping,
  saveSollMapping,
} from "~/lib/traeger-finance-exports-api";

const logger = createLogger({ component: "SollMappingEditor" });

const FIELD_OPTIONS = SOLL_FIELDS.map((field) => ({
  value: field,
  label: SOLL_FIELD_LABELS[field],
}));

const FILE_NAME_PATTERN = /^[A-Za-z0-9._-]*\{NR\}[A-Za-z0-9._-]*$/;

/** The hints of a mapping the backend would refuse, keyed like its errors. */
export function sollMappingErrors(
  mapping: SollMapping,
): Record<string, string> {
  const errors: Record<string, string> = {};
  if (mapping.columns.length === 0) {
    errors.columns = "Bitte mindestens eine Spalte anlegen.";
  }
  mapping.columns.forEach((column, index) => {
    if (mapping.headerRow && column.header.trim() === "") {
      errors[`columns.${index}.header`] = "Bitte einen Spaltennamen eingeben.";
    }
  });
  if (mapping.decimalSeparator === mapping.delimiter) {
    errors.decimal_separator =
      "Das Dezimalzeichen muss sich vom Trennzeichen unterscheiden.";
  }
  if (!FILE_NAME_PATTERN.test(mapping.fileNamePattern.trim())) {
    errors.file_name_pattern =
      "Der Name braucht einmal {NR} und nur Buchstaben, Ziffern, Punkt, Strich und Unterstrich.";
  }
  return errors;
}

function ColumnRow({
  column,
  index,
  count,
  headerRow,
  error,
  onChange,
  onMove,
  onRemove,
}: Readonly<{
  column: SollColumn;
  index: number;
  count: number;
  headerRow: boolean;
  error: (name: string) => string | undefined;
  onChange: (column: SollColumn) => void;
  onMove: (offset: -1 | 1) => void;
  onRemove: () => void;
}>) {
  const fieldLabel = SOLL_FIELD_LABELS[column.field];
  const position = index + 1;
  const menuItems = [
    ...(index > 0
      ? [{ label: "Nach links schieben", onClick: () => onMove(-1) }]
      : []),
    ...(index < count - 1
      ? [{ label: "Nach rechts schieben", onClick: () => onMove(1) }]
      : []),
    ...(count > 1
      ? [{ label: "Spalte entfernen", destructive: true, onClick: onRemove }]
      : []),
  ];
  return (
    <li className="flex items-start gap-3 py-4 first:pt-0 last:pb-0">
      <div className="grid min-w-0 flex-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
        <CustomSelect
          label={`Inhalt der Spalte ${position}`}
          name={`columns.${index}.field`}
          value={column.field}
          options={FIELD_OPTIONS}
          error={error(`columns.${index}.field`)}
          onChange={(field) =>
            onChange({
              ...column,
              field: field as SollField,
              header:
                column.header === SOLL_FIELD_LABELS[column.field] ||
                column.header === ""
                  ? SOLL_FIELD_LABELS[field as SollField]
                  : column.header,
            })
          }
        />
        <Input
          controlSize="compact"
          label="Spaltenname"
          name={`columns.${index}.header`}
          value={column.header}
          maxLength={60}
          required={headerRow}
          hint={headerRow ? undefined : "Ohne Kopfzeile nicht in der Datei."}
          error={error(`columns.${index}.header`)}
          onChange={(event) =>
            onChange({ ...column, header: event.target.value })
          }
        />
        {column.field === "constant" ? (
          <Input
            controlSize="compact"
            label="Fester Wert"
            name={`columns.${index}.value`}
            value={column.value}
            maxLength={60}
            hint="Steht in jeder Zeile, etwa ein Konto."
            error={error(`columns.${index}.value`)}
            onChange={(event) =>
              onChange({ ...column, value: event.target.value })
            }
          />
        ) : null}
      </div>
      <div className="mt-7 shrink-0">
        {menuItems.length > 0 ? (
          <OverflowMenu
            ariaLabel={`Spalte ${position} (${fieldLabel}) ändern`}
            items={menuItems}
          />
        ) : null}
      </div>
    </li>
  );
}

/** The field mapping of the Sollstellung, saved as a whole. */
export function SollMappingEditor({
  settings,
  onCancel,
  onSaved,
}: Readonly<{
  settings: ExportSettings;
  onCancel: () => void;
  onSaved: (settings: ExportSettings) => void;
}>) {
  const [draft, setDraft] = useState<SollMapping>(settings.mapping);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const { success } = useToast();
  const fieldError = (name: string) => formErrors.fieldError(name) ?? undefined;
  const decimalError = fieldError("decimal_separator");

  const set = (patch: Partial<SollMapping>) =>
    setDraft((current) => ({ ...current, ...patch }));
  const setColumns = (change: (columns: SollColumn[]) => SollColumn[]) =>
    setDraft((current) => ({
      ...current,
      columns: change([...current.columns]),
    }));

  const submit = async () => {
    formErrors.clear();
    const errors = sollMappingErrors(draft);
    if (Object.keys(errors).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Felder.", errors);
      return;
    }
    setSaving(true);
    try {
      const saved = await saveSollMapping({
        ...draft,
        fileNamePattern: draft.fileNamePattern.trim(),
      });
      success("Die Feldzuordnung ist gespeichert.");
      onSaved(saved);
    } catch (err) {
      logger.error("soll_mapping_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "die Feldzuordnung" });
    } finally {
      setSaving(false);
    }
  };

  const preview = sollPreview(draft, settings.nextFileNumber);

  return (
    <form
      ref={formRef}
      noValidate
      className="flex flex-col gap-6"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      <SectionCard
        title="Spalten"
        description="So stehen die Spalten in der Datei, von links nach rechts. Fehlt Ihrer Kämmerei ein Wert, legen Sie eine Spalte mit festem Wert an."
        action={
          <Button
            type="button"
            variant="outline"
            size="md"
            disabled={draft.columns.length >= 40}
            onClick={() =>
              setColumns((columns) => [
                ...columns,
                {
                  field: "invoice_number",
                  header: "Rechnungsnummer",
                  value: "",
                },
              ])
            }
          >
            <Plus className="mr-1.5 h-4 w-4" aria-hidden="true" />
            Spalte
          </Button>
        }
      >
        <ol className="divide-y divide-gray-100">
          {draft.columns.map((column, index) => (
            <ColumnRow
              key={`${index}-${column.field}`}
              column={column}
              index={index}
              count={draft.columns.length}
              headerRow={draft.headerRow}
              error={fieldError}
              onChange={(next) =>
                setColumns((columns) =>
                  columns.map((current, at) => (at === index ? next : current)),
                )
              }
              onMove={(offset) =>
                setColumns((columns) => {
                  const target = index + offset;
                  const moved = columns[index];
                  const other = columns[target];
                  if (!moved || !other) return columns;
                  return columns.map((current, at) =>
                    at === index ? other : at === target ? moved : current,
                  );
                })
              }
              onRemove={() =>
                setColumns((columns) => columns.filter((_, at) => at !== index))
              }
            />
          ))}
        </ol>
      </SectionCard>

      <SectionCard
        title="Datei"
        description="Fragen Sie Ihre Kämmerei, welche Form ihr Programm liest."
      >
        <div className="flex flex-col gap-5">
          <div className="flex flex-col gap-2">
            <FieldLabel id="soll-delimiter">Trennzeichen</FieldLabel>
            <SegmentedControl
              ariaLabel="Trennzeichen"
              items={SOLL_DELIMITERS}
              value={draft.delimiter}
              onChange={(delimiter) => set({ delimiter })}
              fullWidth
              wrap
            />
          </div>
          <div className="grid gap-5 sm:grid-cols-2">
            <div className="flex flex-col gap-2">
              <FieldLabel id="soll-decimal">Beträge</FieldLabel>
              {/* A form error focuses the first option after this field. */}
              <input
                type="hidden"
                name="decimal_separator"
                value={draft.decimalSeparator}
              />
              <SegmentedControl
                ariaLabel="Beträge"
                ariaDescribedBy={
                  decimalError ? "soll-decimal-error" : undefined
                }
                items={SOLL_DECIMALS}
                value={draft.decimalSeparator}
                onChange={(decimalSeparator) => set({ decimalSeparator })}
              />
              <FieldErrorHint id="soll-decimal-error" message={decimalError} />
            </div>
            <div className="flex flex-col gap-2">
              <FieldLabel id="soll-date">Datum</FieldLabel>
              <SegmentedControl
                ariaLabel="Datum"
                items={SOLL_DATES}
                value={draft.dateFormat}
                onChange={(dateFormat) => set({ dateFormat })}
              />
            </div>
          </div>
          <div className="grid gap-5 wrap-anywhere sm:grid-cols-2">
            <CustomSelect
              label="Zeichensatz"
              name="encoding"
              value={draft.encoding}
              options={SOLL_ENCODINGS}
              hint="Zeigt die Kämmerei falsche Umlaute, wählen Sie Windows-1252."
              onChange={(encoding) =>
                set({ encoding: encoding as SollMapping["encoding"] })
              }
            />
            <Input
              controlSize="compact"
              label="Dateiname"
              name="file_name_pattern"
              value={draft.fileNamePattern}
              maxLength={120}
              required
              hint={`{NR} wird zur Dateinummer, etwa ${sollFileName(draft.fileNamePattern.trim() || "{NR}", settings.nextFileNumber)}.`}
              error={fieldError("file_name_pattern")}
              onChange={(event) => set({ fileNamePattern: event.target.value })}
            />
          </div>
          <SwitchRow
            label="Kopfzeile mit den Spaltennamen"
            description="Die erste Zeile der Datei nennt die Spalten."
            checked={draft.headerRow}
            onCheckedChange={(headerRow) => set({ headerRow })}
          />
        </div>
      </SectionCard>

      <SectionCard
        title="Vorschau"
        description="So beginnt die Datei. Die Zeilen zeigen eine erfundene Rechnung und ihr Storno."
      >
        <InsetPanel padding="md" className="overflow-x-auto">
          <pre
            className="font-mono text-sm leading-6 text-gray-800"
            aria-label="Vorschau der ersten Zeilen der Datei"
          >
            {preview.join("\n")}
          </pre>
        </InsetPanel>
        <p className="mt-3 text-sm text-gray-500">
          Nächste Datei:{" "}
          <span className="font-mono wrap-anywhere text-gray-800">
            {sollFileName(
              draft.fileNamePattern.trim(),
              settings.nextFileNumber,
            )}
          </span>
        </p>
      </SectionCard>

      <FormErrorAlert message={formErrors.error} />
      <EditActions onCancel={onCancel} saving={saving} />
    </form>
  );
}
