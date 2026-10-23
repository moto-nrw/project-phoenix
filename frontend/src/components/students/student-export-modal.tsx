"use client";

import { useEffect, useMemo, useState } from "react";
import { Info } from "lucide-react";
import {
  BIRTHDAY_MONTH_OPTIONS,
  HEALTH_LIST_FIXED_COLUMNS,
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
import { Checkbox } from "~/components/ui/checkbox";
import { ExportDialog } from "~/components/ui/export-dialog";
import { Input } from "~/components/ui/input";
import { useExportRun } from "~/hooks/use-export-run";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { ToggleChip } from "~/components/ui/toggle-chip";

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
  /** Filters of the calling page as readable labels („Klasse 3a“). */
  readonly appliedFilters?: readonly string[];
  readonly onClose: () => void;
}

const STUDENT_EXPORT_FORMATS: readonly StudentExportFormat[] = [
  "pdf",
  "docx",
  "xlsx",
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
  appliedFilters,
  onClose,
}: StudentExportModalProps) {
  const [preset, setPreset] = useState<StudentExportPreset>("ogs_weekly");
  const [title, setTitle] = useState("OGS Wochenliste");
  const [columns, setColumns] = useState<StudentExportColumn[]>(
    STUDENT_EXPORT_PRESETS[0]?.columns ?? [],
  );
  const [groupByClass, setGroupByClass] = useState(false);
  const [months, setMonths] = useState<string[]>([]);
  const [includeWithoutHealthInfo, setIncludeWithoutHealthInfo] =
    useState(false);
  const isHealthList = preset === "health_list";

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
    setIncludeWithoutHealthInfo(false);
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
  // catalog would put a weekly matrix on a birthday list. The Gesundheitsliste
  // is held to its own columns even in the template picker, because the
  // backend prints nothing else on it, and its fixed columns get no checkbox.
  const availableColumns = useMemo(() => {
    if (!lockedPreset && !isHealthList) return columnCatalog;
    const allowed = new Set<StudentExportColumn>(activePreset?.columns ?? []);
    return columnCatalog.filter(
      (column) =>
        allowed.has(column.id) &&
        !(isHealthList && HEALTH_LIST_FIXED_COLUMNS.has(column.id)),
    );
  }, [activePreset, columnCatalog, isHealthList, lockedPreset]);

  // "Alle Kinder" would contradict the Gesundheitsliste's default scope,
  // which leaves children without a note off the list. The Kindersuche only
  // knows its general result count, so it must not report that count for
  // the narrower health-list scope.
  const isDefaultHealthListScope = isHealthList && !includeWithoutHealthInfo;
  // A table selection (#3834) is not "the current filtering": name it as
  // what it is, the children the user marked.
  const selectedCount = filters.student_ids?.length;
  const scopeWithoutCount = isDefaultHealthListScope
    ? selectedCount !== undefined
      ? "Ausgewählte Kinder mit hinterlegten Gesundheitsinformationen."
      : resultCount === undefined
        ? "Kinder der Schule mit hinterlegten Gesundheitsinformationen."
        : "Kinder aus der aktuellen Filterung mit hinterlegten Gesundheitsinformationen."
    : "Alle Kinder der Schule.";
  const showsResultCount =
    resultCount !== undefined && !isDefaultHealthListScope;

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

  const validate = () =>
    columns.length === 0
      ? "Bitte wählen Sie mindestens eine Spalte aus."
      : null;

  const request = (format: StudentExportFormat) => ({
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
      ...(isHealthList
        ? { include_without_health_info: includeWithoutHealthInfo }
        : {}),
    },
    columns,
  });

  const download = (format: StudentExportFormat) =>
    exportStudents(request(format));

  const description = showsResultCount
    ? selectedCount !== undefined
      ? `${selectedCount} ${selectedCount === 1 ? "ausgewähltes Kind" : "ausgewählte Kinder"}.`
      : `${resultCount} Kinder aus der aktuellen Filterung.`
    : scopeWithoutCount;

  const run = useExportRun({
    isOpen,
    onDownload: download,
    onPrint: (target) => exportStudents(request("pdf"), target),
    validate,
    errorObject: "die Exportdatei",
    onDone: onClose,
    logName: "student_list",
  });

  return (
    <ExportDialog
      isOpen={isOpen}
      onClose={onClose}
      title={heading}
      description={description}
      appliedFilters={appliedFilters}
      formats={STUDENT_EXPORT_FORMATS}
      run={run}
      widthClass="sm:w-[640px]"
    >
      <section>
        <Input
          id="student-export-title-input"
          label="Titel"
          value={title}
          onChange={(event) => setTitle(event.target.value)}
        />
      </section>

      {!lockedPreset && (
        <section>
          <p className="text-sm font-medium text-gray-900">Vorlage</p>
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {presets.map((item) => (
              <ChoiceTile
                key={item.id}
                as="button"
                aria-pressed={preset === item.id}
                selected={preset === item.id}
                onClick={() => setPreset(item.id)}
                className="block p-3"
              >
                <span className="block text-sm font-semibold text-gray-900">
                  {item.label}
                </span>
                <span className="mt-1 block text-xs font-normal text-gray-500">
                  {item.description}
                </span>
              </ChoiceTile>
            ))}
          </div>
        </section>
      )}

      {preset === "birthday_list" && (
        <section>
          <div className="flex items-start justify-between gap-3">
            <div>
              <p className="text-sm font-medium text-gray-900">Geburtsmonate</p>
              <p className="text-xs text-gray-500">
                {months.length === 0
                  ? "Alle Geburtstage des Jahres werden aufgelistet."
                  : "Nur Kinder, die in den gewählten Monaten Geburtstag haben."}
              </p>
            </div>
            <ToggleChip
              pressed={months.length === 0}
              onPressedChange={() => setMonths([])}
              className="shrink-0"
            >
              Ganzes Jahr
            </ToggleChip>
          </div>
          <div className="mt-2 grid grid-cols-4 gap-2 sm:grid-cols-6">
            {BIRTHDAY_MONTH_OPTIONS.map((month) => {
              const selected = months.includes(month.value);
              return (
                <ToggleChip
                  key={month.value}
                  pressed={selected}
                  onPressedChange={() => toggleMonth(month.value)}
                  ariaLabel={month.label}
                >
                  {month.label.slice(0, 3)}
                </ToggleChip>
              );
            })}
          </div>
        </section>
      )}

      {isHealthList && (
        <section>
          <p className="text-sm font-medium text-gray-900">Kinder</p>
          <p className="text-xs text-gray-500">
            {includeWithoutHealthInfo
              ? "Alle Kinder. Ohne Eintrag steht „Nicht hinterlegt“ in der Liste."
              : "Nur Kinder mit hinterlegten Gesundheitsinformationen."}
          </p>
          <div className="mt-2">
            <ExportToggleCheckbox
              checked={includeWithoutHealthInfo}
              label="Auch Kinder ohne Eintrag"
              description="So sehen Sie auf einen Blick, bei wem noch nichts eingetragen ist."
              onChange={() =>
                setIncludeWithoutHealthInfo((current) => !current)
              }
            />
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
              {isHealthList
                ? "Name und Gesundheitsinformationen sind immer enthalten."
                : `${activePreset?.label ?? "Vorlage"} kann angepasst werden.`}
            </p>
          </div>
          <span className="shrink-0 rounded-full bg-gray-100 px-2 py-1 text-xs font-medium whitespace-nowrap text-gray-600">
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
    </ExportDialog>
  );
}

function sortColumns(columns: StudentExportColumn[]): StudentExportColumn[] {
  const order = new Map(
    STUDENT_EXPORT_COLUMNS.map((column, index) => [column.id, index]),
  );
  // A column outside the shared catalog (the Gesundheitsliste's health note)
  // stays at the end, where that list prints it.
  const rank = (column: StudentExportColumn) =>
    order.get(column) ?? Number.MAX_SAFE_INTEGER;
  return [...columns].sort((a, b) => rank(a) - rank(b));
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
