"use client";

import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Download, FileSpreadsheet, FileText, Info } from "lucide-react";
import {
  BIRTHDAY_MONTH_OPTIONS,
  STUDENT_EXPORT_COLUMNS,
  STUDENT_EXPORT_PRESETS,
  buildStudentExportColumns,
  buildStudentExportPresets,
  currentBirthdayMonth,
  exportStudents,
  type StudentExportColumn,
  type StudentExportColumnOption,
  type StudentExportFilters,
  type StudentExportFormat,
  type StudentExportPreset,
} from "~/lib/student-export-api";
import { parseMultiValueParam } from "~/lib/multi-value-param";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { Modal } from "~/components/ui/modal";

const logger = createLogger({ component: "StudentExportModal" });

interface StudentExportModalProps {
  readonly isOpen: boolean;
  readonly filters: StudentExportFilters;
  /**
   * Number of children the caller's filtering produced. Omit where there is no
   * filtered list to speak of (the central export page), so the modal describes
   * the scope honestly instead of reporting "0 Kinder".
   */
  readonly resultCount?: number;
  /** Modal heading. Defaults to the Kindersuche wording. */
  readonly heading?: string;
  /**
   * Restricts the dialog to one list: the template grid and the class grouping
   * disappear, and only that template's columns are offered. Opening
   * "Geburtstagsliste" should not present a weekly matrix. Omit for the
   * general-purpose export that lets users pick any template. The central
   * export page locks every card to its list; the Kindersuche leaves this
   * unset to keep the full template picker.
   */
  readonly lockedPreset?: StudentExportPreset;
  readonly onClose: () => void;
}

const FORMAT_OPTIONS: Array<{
  value: StudentExportFormat;
  label: string;
  icon: ReactNode;
}> = [
  { value: "pdf", label: "PDF", icon: <FileText className="h-4 w-4" /> },
  { value: "docx", label: "DOCX", icon: <FileText className="h-4 w-4" /> },
  {
    value: "xlsx",
    label: "XLSX",
    icon: <FileSpreadsheet className="h-4 w-4" />,
  },
];

// A class filter may now name several classes (#2218). Only a selection of
// exactly ONE class makes per-class separation pointless — with two classes in
// the list the school wants them apart, which is the same situation as an
// unfiltered export.
//
// The value follows the shared escaping grammar, so it is parsed with the
// shared parser: a single class literally called "A,B" arrives as "A\,B" and
// must count as one class, not two (#2218 review).
function isSingleClassSelection(schoolClass: string | undefined): boolean {
  return parseMultiValueParam(schoolClass).length === 1;
}

export function StudentExportModal({
  isOpen,
  filters,
  resultCount,
  heading = "Alle Kinder exportieren",
  lockedPreset,
  onClose,
}: StudentExportModalProps) {
  const [preset, setPreset] = useState<StudentExportPreset>("ogs_weekly");
  const [format, setFormat] = useState<StudentExportFormat>("pdf");
  const [title, setTitle] = useState("OGS Wochenliste");
  const [columns, setColumns] = useState<StudentExportColumn[]>(
    STUDENT_EXPORT_PRESETS[0]?.columns ?? [],
  );
  const [groupByClass, setGroupByClass] = useState(false);
  const [months, setMonths] = useState<string[]>([]);
  const [exporting, setExporting] = useState(false);
  const toast = useToast();

  useEffect(() => {
    if (!isOpen) return;
    setPreset(
      lockedPreset ?? (filters.school_class ? "class_roster" : "ogs_weekly"),
    );
  }, [filters.school_class, lockedPreset, isOpen]);

  // The month a birthday list is most often wanted for is the current one.
  useEffect(() => {
    if (!isOpen) return;
    setMonths([currentBirthdayMonth()]);
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    setGroupByClass(
      preset === "class_roster" &&
        !isSingleClassSelection(filters.school_class),
    );
  }, [filters.school_class, isOpen, preset]);

  useEffect(() => {
    if (!isOpen) return;
    const presetConfig = STUDENT_EXPORT_PRESETS.find(
      (item) => item.id === preset,
    );
    if (!presetConfig) return;
    setTitle(presetConfig.label);
    setColumns(presetConfig.columns);
  }, [isOpen, preset]);

  // Day-scoped copy (preset + column descriptions) names the selected planning
  // day rather than "heute" when a non-today date is exported (#1939). The
  // column ids and preset column sets are identical to the base constants, so
  // all selection logic below is unaffected — only the displayed wording
  // changes. filters.date is set only for a non-today export.
  const presets = useMemo(
    () => buildStudentExportPresets(filters.date),
    [filters.date],
  );
  const columnCatalog = useMemo(
    () => buildStudentExportColumns(filters.date),
    [filters.date],
  );

  const activePreset = useMemo(
    () => presets.find((item) => item.id === preset),
    [presets, preset],
  );

  // A locked dialog offers only the columns its list is made of; the full
  // catalog would put a weekly matrix on a birthday list.
  const availableColumns = useMemo(() => {
    if (!lockedPreset) return columnCatalog;
    const allowed = new Set<StudentExportColumn>(activePreset?.columns ?? []);
    return columnCatalog.filter((column) => allowed.has(column.id));
  }, [activePreset, columnCatalog, lockedPreset]);

  if (!isOpen) return null;

  const toggleColumn = (column: StudentExportColumn) => {
    setColumns((current) =>
      current.includes(column)
        ? current.filter((item) => item !== column)
        : sortColumns([...current, column]),
    );
  };

  const toggleMonth = (month: string) => {
    setMonths((current) =>
      current.includes(month)
        ? current.filter((item) => item !== month)
        : [...current, month].sort((a, b) => a.localeCompare(b)),
    );
  };

  const handleExport = async () => {
    if (columns.length === 0) {
      toast.error("Bitte wähle mindestens eine Spalte aus.");
      return;
    }
    setExporting(true);
    try {
      await exportStudents({
        format,
        preset,
        title,
        filters: {
          ...filters,
          ...(groupByClass && !isSingleClassSelection(filters.school_class)
            ? { group_by_class: true }
            : {}),
          // A birthday list is only readable in calendar order, so the preset
          // carries its own sort rather than inheriting the page's.
          ...(preset === "birthday_list" ? { months, sort: "birthday" } : {}),
        },
        columns,
      });
      toast.success("Export wurde erstellt.");
      onClose();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Export fehlgeschlagen";
      logger.error("student_export_failed", { error: message });
      toast.error(message);
    } finally {
      setExporting(false);
    }
  };

  const footer = (
    <>
      <button
        type="button"
        onClick={onClose}
        disabled={exporting}
        className="inline-flex h-9 items-center justify-center rounded-lg border border-gray-200 bg-white px-3 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        Abbrechen
      </button>
      <button
        type="button"
        onClick={() => void handleExport()}
        disabled={exporting}
        className="inline-flex h-9 items-center justify-center gap-2 rounded-lg bg-gray-900 px-3 text-sm font-medium text-white shadow-sm transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
      >
        <Download className="h-4 w-4" aria-hidden />
        {exporting ? "Exportiert..." : "Exportieren"}
      </button>
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={heading}
      closeLabel="Export schließen"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      isDismissDisabled={exporting}
      footer={footer}
    >
      <p className="mb-5 text-sm text-gray-500">
        {resultCount === undefined
          ? "Alle Kinder der Schule."
          : `${resultCount} Kinder aus der aktuellen Filterung.`}
      </p>
      <div className="space-y-5">
        <section>
          <label
            htmlFor="student-export-title-input"
            className="text-sm font-medium text-gray-900"
          >
            Titel
          </label>
          <input
            id="student-export-title-input"
            value={title}
            onChange={(event) => setTitle(event.target.value)}
            className="mt-2 h-10 w-full rounded-lg border border-gray-200 bg-white px-3 text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
          />
        </section>

        {!lockedPreset && (
          <section>
            <p className="text-sm font-medium text-gray-900">Vorlage</p>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              {presets.map((item) => (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setPreset(item.id)}
                  className={`rounded-lg border px-3 py-3 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                    preset === item.id
                      ? "border-gray-900 bg-gray-950 text-white"
                      : "border-gray-200 bg-white text-gray-800 hover:bg-gray-50"
                  }`}
                >
                  <span className="block text-sm font-semibold">
                    {item.label}
                  </span>
                  <span
                    className={`mt-1 block text-xs ${
                      preset === item.id ? "text-gray-200" : "text-gray-500"
                    }`}
                  >
                    {item.description}
                  </span>
                </button>
              ))}
            </div>
          </section>
        )}

        <section>
          <p className="text-sm font-medium text-gray-900">Format</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {FORMAT_OPTIONS.map((item) => (
              <button
                key={item.value}
                type="button"
                onClick={() => setFormat(item.value)}
                className={`inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                  format === item.value
                    ? "border-gray-900 bg-gray-950 text-white"
                    : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                }`}
              >
                {item.icon}
                {item.label}
              </button>
            ))}
          </div>
        </section>

        {preset === "birthday_list" && (
          <section>
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-sm font-medium text-gray-900">
                  Geburtsmonate
                </p>
                <p className="text-xs text-gray-500">
                  {months.length === 0
                    ? "Alle Geburtstage des Jahres werden aufgelistet."
                    : "Nur Kinder, die in den gewählten Monaten Geburtstag haben."}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setMonths([])}
                aria-pressed={months.length === 0}
                className={`inline-flex h-8 shrink-0 items-center rounded-md border px-2.5 text-xs font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                  months.length === 0
                    ? "border-gray-900 bg-gray-950 text-white"
                    : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                }`}
              >
                Ganzes Jahr
              </button>
            </div>
            <div className="mt-2 grid grid-cols-4 gap-2 sm:grid-cols-6">
              {BIRTHDAY_MONTH_OPTIONS.map((month) => {
                const selected = months.includes(month.value);
                return (
                  <button
                    key={month.value}
                    type="button"
                    onClick={() => toggleMonth(month.value)}
                    aria-label={month.label}
                    aria-pressed={selected}
                    className={`inline-flex h-9 items-center justify-center rounded-lg border text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
                      selected
                        ? "border-gray-900 bg-gray-950 text-white"
                        : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
                    }`}
                  >
                    {month.label.slice(0, 3)}
                  </button>
                );
              })}
            </div>
          </section>
        )}

        {!lockedPreset && !isSingleClassSelection(filters.school_class) && (
          <section>
            <p className="text-sm font-medium text-gray-900">Gliederung</p>
            <div className="mt-2">
              <ExportToggleCheckbox
                checked={groupByClass}
                label="Nach Klassen getrennt"
                description="Jede Klasse beginnt mit einer eigenen Überschrift, im PDF auf einer neuen Seite."
                onChange={() => setGroupByClass((current) => !current)}
              />
            </div>
          </section>
        )}

        <section>
          <div className="flex items-center justify-between gap-3">
            <div>
              <p className="text-sm font-medium text-gray-900">Spalten</p>
              <p className="text-xs text-gray-500">
                {activePreset?.label ?? "Vorlage"} kann angepasst werden.
              </p>
            </div>
            <span className="rounded-full bg-gray-100 px-2 py-1 text-xs font-medium text-gray-600">
              {columns.length} aktiv
            </span>
          </div>
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            {availableColumns.map((column) => {
              const checked = columns.includes(column.id);
              return (
                <ExportColumnCheckbox
                  key={column.id}
                  checked={checked}
                  column={column}
                  onChange={() => toggleColumn(column.id)}
                />
              );
            })}
          </div>
        </section>
      </div>
    </Modal>
  );
}

function sortColumns(columns: StudentExportColumn[]): StudentExportColumn[] {
  const order = new Map(
    STUDENT_EXPORT_COLUMNS.map((column, index) => [column.id, index]),
  );
  return [...columns].sort((a, b) => (order.get(a) ?? 0) - (order.get(b) ?? 0));
}

function ExportToggleCheckbox({
  checked,
  label,
  description,
  onChange,
}: Readonly<{
  checked: boolean;
  label: string;
  description: string;
  onChange: () => void;
}>) {
  return (
    <ChoiceTile selected={checked} className="min-h-11">
      <Checkbox checked={checked} onChange={onChange} />
      <span className="min-w-0 flex-1 leading-snug">
        <span>{label}</span>
        <span className="mt-0.5 block text-xs leading-snug font-normal text-gray-500">
          {description}
        </span>
      </span>
    </ChoiceTile>
  );
}

function ExportColumnCheckbox({
  checked,
  column,
  onChange,
}: Readonly<{
  checked: boolean;
  column: StudentExportColumnOption;
  onChange: () => void;
}>) {
  return (
    <ChoiceTile
      title={column.description}
      selected={checked}
      className="min-h-11"
    >
      <Checkbox checked={checked} onChange={onChange} />
      <span className="min-w-0 flex-1 leading-snug">
        <span className="flex items-center gap-1.5">
          <span>{column.label}</span>
          <Info className="h-3.5 w-3.5 text-gray-400" aria-hidden />
        </span>
        <span className="mt-0.5 block text-xs leading-snug font-normal text-gray-500">
          {column.description}
        </span>
      </span>
    </ChoiceTile>
  );
}
