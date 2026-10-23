"use client";

import { useEffect, useMemo, useState } from "react";

import { ISODatePicker } from "~/components/ui/date-picker";
import { ExportDialog } from "~/components/ui/export-dialog";
import { Radio } from "~/components/ui/radio";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { useExportRun } from "~/hooks/use-export-run";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import {
  exportPlan,
  PLAN_EXPORT_TEMPLATES,
  PLAN_EXPORT_VARIANTS,
  type PlanExportFormat,
  type PlanExportPlan,
  type PlanExportTemplate,
  type PlanExportVariant,
} from "~/lib/plan-export-api";

const PLAN_EXPORT_FORMATS: readonly PlanExportFormat[] = ["pdf", "xlsx"];

// The backend widens any range to whole Monday-Friday weeks and caps it at
// eight; the dialog states the same limit rather than letting the user
// discover it as an error.
const MAX_WEEKS = 8;

interface PlanExportModalProps {
  readonly isOpen: boolean;
  readonly plan: PlanExportPlan;
  /** Any day of the exported week ("YYYY-MM-DD"). */
  readonly weekDay: string;
  /**
   * Whether that week is the one on screen. False in the Monats-, Serien- und
   * Halbjahresansicht: dort steht keine Woche im Bild, `weekDay` ist dann nur
   * der Datumsanker der Seite. Der Umschalter darf die Woche in diesem Fall
   * nicht "angezeigt" nennen.
   */
  readonly isWeekOnScreen: boolean;
  /** Internal exports can contain sensitive absence and cancellation details. */
  readonly canExportInternal: boolean;
  readonly onClose: () => void;
}

type RangeMode = "week" | "range";

/**
 * Export dialog for the two weekly plans (#2079).
 *
 * Shared by the Dienstplan and the Betreuungsplan: the row axis is the only
 * thing that differs between them, and it comes from the plan's template
 * list, so neither page carries its own copy of this form.
 */
export function PlanExportModal({
  isOpen,
  plan,
  weekDay,
  isWeekOnScreen,
  canExportInternal,
  onClose,
}: PlanExportModalProps) {
  const templates = PLAN_EXPORT_TEMPLATES[plan];
  const variants = canExportInternal
    ? PLAN_EXPORT_VARIANTS
    : PLAN_EXPORT_VARIANTS.filter((item) => item.id === "aushang");
  const defaultTemplate = templates[0]?.id ?? "persons";

  const [template, setTemplate] = useState<PlanExportTemplate>(defaultTemplate);
  const [variant, setVariant] = useState<PlanExportVariant>("aushang");
  const [rangeMode, setRangeMode] = useState<RangeMode>("week");
  const [from, setFrom] = useState(weekDay);
  const [to, setTo] = useState(weekDay);
  // Reopening the dialog on a different week must not export the week the
  // user looked at last time.
  useEffect(() => {
    if (!isOpen) return;
    setTemplate(defaultTemplate);
    setVariant("aushang");
    setRangeMode("week");
    setFrom(weekDay);
    setTo(weekDay);
  }, [isOpen, defaultTemplate, weekDay]);

  const range = useMemo(
    () =>
      rangeMode === "week" ? { from: weekDay, to: weekDay } : { from, to },
    [rangeMode, weekDay, from, to],
  );

  const weekCount = useMemo(() => countWeeks(range.from, range.to), [range]);
  const rangeError = useMemo(() => {
    if (rangeMode === "week") return null;
    if (!from || !to) return "Bitte wählen Sie Von und Bis aus.";
    if (weekCount === null) return "Das Bis-Datum liegt vor dem Von-Datum.";
    if (weekCount > MAX_WEEKS)
      return `Es lassen sich höchstens ${MAX_WEEKS} Wochen auf einmal drucken.`;
    return null;
  }, [rangeMode, from, to, weekCount]);

  // Die Stundenübersicht (#3819) druckt nur Zahlen, keine Gründe. Eine
  // Wahl zwischen Aushang und interner Fassung ändert daran nichts.
  const showsVariant = template !== "hours";

  const request = () => ({
    from: range.from,
    to: range.to,
    template,
    variant: showsVariant ? variant : "aushang",
  });

  const run = useExportRun({
    isOpen,
    onDownload: (format: PlanExportFormat) =>
      exportPlan(plan, request(), format, "download", null),
    onPrint: (target) => exportPlan(plan, request(), "pdf", "print", target),
    validate: () => rangeError,
    errorObject: (kind) =>
      kind === "print" ? "die Druckfassung des Plans" : "die Datei des Plans",
    successMessage: "Der Plan wurde erstellt.",
    onDone: onClose,
    logName: plan,
  });

  return (
    <ExportDialog
      isOpen={isOpen}
      onClose={onClose}
      title={
        plan === "dienstplan"
          ? "Dienstplan drucken oder exportieren"
          : "Tages- und Wochenplan drucken oder exportieren"
      }
      formats={PLAN_EXPORT_FORMATS}
      run={run}
      blocked={rangeError !== null}
    >
      {templates.length > 1 && (
        <section>
          <p className="text-sm font-medium text-gray-900">Vorlage</p>
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {templates.map((item) => (
              <RadioOption
                key={item.id}
                id={`plan-export-template-${item.id}`}
                name="plan-export-template"
                selected={template === item.id}
                label={item.label}
                description={item.description}
                onClick={() => setTemplate(item.id)}
              />
            ))}
          </div>
        </section>
      )}

      {showsVariant && (
        <section>
          <p className="text-sm font-medium text-gray-900">Fassung</p>
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {variants.map((item) => (
              <RadioOption
                key={item.id}
                id={`plan-export-variant-${item.id}`}
                name="plan-export-variant"
                selected={variant === item.id}
                label={item.label}
                description={item.description}
                onClick={() => setVariant(item.id)}
              />
            ))}
          </div>
        </section>
      )}

      <section>
        <p className="text-sm font-medium text-gray-900">Zeitraum</p>
        <SegmentedControl
          ariaLabel="Zeitraum des Exports"
          value={rangeMode}
          onChange={(next) => setRangeMode(next as RangeMode)}
          items={[
            {
              value: "week",
              label: isWeekOnScreen ? "Angezeigte Woche" : "Einzelne Woche",
            },
            { value: "range", label: "Mehrere Wochen" },
          ]}
        />

        {rangeMode === "range" && (
          <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
            <div>
              <label
                htmlFor="plan-export-from"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Von
              </label>
              <ISODatePicker
                id="plan-export-from"
                controlSize="lg"
                value={from}
                onChange={setFrom}
                calendarLayout="popover"
              />
            </div>
            <div>
              <label
                htmlFor="plan-export-to"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Bis
              </label>
              <ISODatePicker
                id="plan-export-to"
                controlSize="lg"
                value={to}
                min={from || undefined}
                onChange={setTo}
                calendarLayout="popover"
              />
            </div>
          </div>
        )}

        <p className="mt-2 text-xs text-gray-500">
          {rangeError ??
            describeRange(range.from, range.to, weekCount, template)}
        </p>
      </section>
    </ExportDialog>
  );
}

/**
 * Whole Monday-Friday weeks the range covers, or null when it is inverted.
 * Mirrors the backend, which widens any range to full weeks.
 */
function countWeeks(from: string, to: string): number | null {
  if (!from || !to) return null;
  const first = startOfWeek(parseISODate(from));
  const last = startOfWeek(parseISODate(to));
  const days = Math.round(
    (last.getTime() - first.getTime()) / (24 * 60 * 60 * 1000),
  );
  if (days < 0) return null;
  return Math.floor(days / 7) + 1;
}

function startOfWeek(date: Date): Date {
  const monday = new Date(date);
  // getDay(): Sunday = 0, so shift it to the end of the week.
  monday.setDate(monday.getDate() - ((monday.getDay() + 6) % 7));
  monday.setHours(0, 0, 0, 0);
  return monday;
}

/** States which days actually land on the sheet, not which ones were picked. */
function describeRange(
  from: string,
  to: string,
  weekCount: number | null,
  template: PlanExportTemplate,
): string {
  if (!from || !to || weekCount === null) return "";
  const monday = startOfWeek(parseISODate(from));
  const friday = startOfWeek(parseISODate(to));
  friday.setDate(friday.getDate() + 4);
  const span = `${formatDate(toISODate(monday))} bis ${formatDate(toISODate(friday))}`;
  // Samstag und Sonntag stehen nicht im Text, weil sie nicht vom Zeitraum
  // abhängen, sondern davon, ob dort überhaupt etwas geplant ist.
  // Die Stundenübersicht hat keine Tagesspalten; ihre Summen zählen die
  // ganze Woche.
  const weekend =
    template === "hours"
      ? "Die Stunden zählen die ganze Woche, auch Samstag und Sonntag."
      : "Samstag und Sonntag nur, wenn dort etwas geplant ist.";
  return weekCount === 1
    ? `Gedruckt wird die Woche vom ${span}, Montag bis Freitag. ${weekend}`
    : `Gedruckt werden ${weekCount} Wochen (${span}), je Woche ein Blatt. ${weekend}`;
}

function RadioOption({
  id,
  name,
  selected,
  label,
  description,
  onClick,
}: Readonly<{
  id: string;
  name: string;
  selected: boolean;
  label: string;
  description: string;
  onClick: () => void;
}>) {
  return (
    <label
      htmlFor={id}
      className={`flex cursor-pointer gap-3 rounded-lg border px-3 py-3 text-left shadow-sm transition-colors ${
        selected
          ? "border-gray-900 bg-gray-950 text-white"
          : "border-gray-200 bg-white text-gray-800 hover:bg-gray-50"
      }`}
    >
      <Radio id={id} name={name} checked={selected} onChange={onClick} />
      <span>
        <span className="block text-sm font-semibold">{label}</span>
        <span
          className={`mt-1 block text-xs ${selected ? "text-gray-200" : "text-gray-500"}`}
        >
          {description}
        </span>
      </span>
    </label>
  );
}
