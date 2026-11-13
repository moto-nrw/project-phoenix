"use client";

// Die Feldzuordnung der Sollstellung (#4693): welche Spalten die Datei für
// die Kämmerei hat, in welcher Reihenfolge, mit welcher Überschrift, mit
// welchen Trennzeichen und unter welchem Namen. Die Vorschau zeigt die
// ersten Zeilen der Datei mit einer erfundenen Rechnung und ihrem Storno.

import { useRef, useState } from "react";
import { Plus } from "lucide-react";
import {
  BOOKING_TEXT_PLACEHOLDERS,
  bookingTextSample,
  SOLL_DATES,
  SOLL_DECIMALS,
  SOLL_DELIMITERS,
  SOLL_ENCODINGS,
  SOLL_FIELDS,
  SOLL_FIELD_LABELS,
  sollFileName,
  sollPreview,
  sollSplitsLines,
  unknownBookingTextPlaceholder,
} from "~/components/traeger/finance-export-format";
import { BOOKING_ACCOUNTS_PATH } from "~/components/traeger/finance-export-datev";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
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
import { traegerPath } from "~/lib/traeger-url";
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
    const unknown =
      column.field === "booking_text"
        ? unknownBookingTextPlaceholder(column.value)
        : null;
    if (unknown) {
      errors[`columns.${index}.value`] =
        `${unknown} kennt moto nicht. Wählen Sie einen Platzhalter darunter.`;
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

/**
 * The pattern of a booking text (#7008): fixed words and placeholders that
 * moto fills per row. A click on a placeholder puts it where the cursor
 * stands; the example shows the text of the sample invoice.
 */
function BookingTextField({
  column,
  index,
  error,
  onChange,
}: Readonly<{
  column: SollColumn;
  index: number;
  error: string | undefined;
  onChange: (column: SollColumn) => void;
}>) {
  const inputRef = useRef<HTMLInputElement>(null);
  const caret = useRef<{ start: number; end: number } | null>(null);
  const remember = (field: HTMLInputElement) => {
    caret.current = {
      start: field.selectionStart ?? field.value.length,
      end: field.selectionEnd ?? field.value.length,
    };
  };
  const insert = (token: string) => {
    const pattern = column.value;
    const end = Math.min(caret.current?.end ?? pattern.length, pattern.length);
    const start = Math.min(caret.current?.start ?? pattern.length, end);
    const before = start > 0 && pattern[start - 1] !== " " ? " " : "";
    const after = end < pattern.length && pattern[end] !== " " ? " " : "";
    const text = before + token + after;
    const value = pattern.slice(0, start) + text + pattern.slice(end);
    if (value.length > 60) return;
    onChange({ ...column, value });
    const next = start + before.length + token.length;
    caret.current = { start: next, end: next };
    requestAnimationFrame(() => {
      inputRef.current?.focus();
      inputRef.current?.setSelectionRange(next, next);
    });
  };
  const example = bookingTextSample(column.value).trim();
  const helpId = `booking-text-${index}-help`;
  return (
    <>
      <Input
        ref={inputRef}
        controlSize="compact"
        label="Text der Buchung"
        name={`columns.${index}.value`}
        value={column.value}
        maxLength={60}
        placeholder="SKB {Monat} {Kind}"
        aria-describedby={helpId}
        error={error}
        onChange={(event) => {
          onChange({ ...column, value: event.target.value });
          remember(event.target);
        }}
        onSelect={(event) => remember(event.currentTarget)}
      />
      <InsetPanel
        padding="md"
        className="flex min-w-0 flex-col gap-3 sm:col-span-2 lg:col-span-3"
      >
        <p className="text-sm text-gray-600" id={helpId}>
          Feste Wörter schreiben Sie selbst. Einen Platzhalter füllt moto für
          jede Rechnung aus. Ein Klick setzt ihn ein.
        </p>
        <ul className="flex flex-wrap gap-2" aria-label="Platzhalter">
          {BOOKING_TEXT_PLACEHOLDERS.map((placeholder) => (
            <li key={placeholder.token} className="max-w-full min-w-0">
              <Button
                type="button"
                variant="outline"
                size="compact"
                tooltip="Fügt diesen Platzhalter in den Buchungstext ein."
                disabled={column.value.length + placeholder.token.length >= 60}
                disabledReason="Der Text hat schon fast 60 Zeichen."
                onClick={() => insert(placeholder.token)}
                className="max-w-full gap-2"
              >
                <code aria-hidden="true" className="shrink-0 text-gray-900">
                  {placeholder.token}
                </code>
                <span className="min-w-0 truncate text-gray-500">
                  <span className="sr-only">{`Platzhalter „${placeholder.label}“ einfügen`}</span>
                  <span aria-hidden="true">{placeholder.label}</span>
                </span>
              </Button>
            </li>
          ))}
        </ul>
        <p className="flex min-w-0 flex-col gap-1 border-t border-gray-200 pt-3 text-sm sm:flex-row sm:items-baseline sm:gap-2">
          <span className="shrink-0 text-gray-500">
            So steht es in der Datei:
          </span>
          <span
            className="min-w-0 font-mono wrap-anywhere text-gray-900"
            aria-live="polite"
          >
            {example || "–"}
          </span>
        </p>
      </InsetPanel>
    </>
  );
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
        {column.field === "booking_text" ? (
          <BookingTextField
            column={column}
            index={index}
            error={error(`columns.${index}.value`)}
            onChange={onChange}
          />
        ) : null}
        {column.field === "credit_amount" ? (
          <p className="text-sm text-gray-600 sm:col-span-2 lg:col-span-1 lg:mt-7">
            Stornos und Gutschriften stehen hier ohne Minus. Bei ihnen bleibt
            die Spalte „Betrag“ leer.
          </p>
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
        {sollSplitsLines(draft) ? (
          <Alert
            type="info"
            className="mt-4"
            message="Mit Erlöskonto oder Kostenstelle hat die Datei eine Zeile je Konto einer Rechnung. Der Betrag ist dann der Teil der Rechnung auf diesem Konto."
            action={
              <ButtonLink
                href={traegerPath(BOOKING_ACCOUNTS_PATH)}
                variant="ghost_tone"
                size="compact"
              >
                Konten ansehen
              </ButtonLink>
            }
          />
        ) : null}
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
