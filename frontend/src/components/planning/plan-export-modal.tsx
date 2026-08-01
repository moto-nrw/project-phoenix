"use client";

import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Download, FileSpreadsheet, Printer } from "lucide-react";

import { Modal } from "~/components/ui/modal";
import { ISODatePicker } from "~/components/ui/date-picker";
import { useToast } from "~/contexts/ToastContext";
import { formatDate, parseISODate, toISODate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  exportPlan,
  PLAN_EXPORT_TEMPLATES,
  PLAN_EXPORT_VARIANTS,
  type PlanExportFormat,
  type PlanExportMode,
  type PlanExportPlan,
  type PlanExportTemplate,
  type PlanExportVariant,
} from "~/lib/plan-export-api";

const logger = createLogger({ component: "PlanExportModal" });

// The backend widens any range to whole Monday-Friday weeks and caps it at
// eight; the dialog states the same limit rather than letting the user
// discover it as an error.
const MAX_WEEKS = 8;

interface PlanExportModalProps {
  readonly isOpen: boolean;
  readonly plan: PlanExportPlan;
  /** Any day of the week currently on screen ("YYYY-MM-DD"). */
  readonly weekDay: string;
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
  onClose,
}: PlanExportModalProps) {
  const toast = useToast();
  const templates = PLAN_EXPORT_TEMPLATES[plan];
  const defaultTemplate = templates[0]?.id ?? "persons";

  const [template, setTemplate] = useState<PlanExportTemplate>(defaultTemplate);
  const [variant, setVariant] = useState<PlanExportVariant>("aushang");
  const [rangeMode, setRangeMode] = useState<RangeMode>("week");
  const [from, setFrom] = useState(weekDay);
  const [to, setTo] = useState(weekDay);
  const [busy, setBusy] = useState<string | null>(null);

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
    if (!from || !to) return "Bitte Von und Bis auswählen.";
    if (weekCount === null) return "Das Bis-Datum liegt vor dem Von-Datum.";
    if (weekCount > MAX_WEEKS)
      return `Es lassen sich höchstens ${MAX_WEEKS} Wochen auf einmal drucken.`;
    return null;
  }, [rangeMode, from, to, weekCount]);

  if (!isOpen) return null;

  const run = async (
    key: string,
    format: PlanExportFormat,
    mode: PlanExportMode,
  ) => {
    if (rangeError) {
      toast.error(rangeError);
      return;
    }
    // The print tab has to open while the click's user activation is still
    // valid: after the fetch, popup blockers refuse.
    let printTarget: Window | null = null;
    if (mode === "print") {
      printTarget = globalThis.open("", "_blank");
      if (!printTarget) {
        toast.error("Der Druckdialog konnte nicht geöffnet werden.");
        return;
      }
    }

    setBusy(key);
    try {
      await exportPlan(
        plan,
        { from: range.from, to: range.to, template, variant },
        format,
        mode,
        printTarget,
      );
      if (mode === "download") toast.success("Der Plan wurde erstellt.");
      onClose();
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Export fehlgeschlagen";
      logger.error("plan_export_failed", { plan, format, error: message });
      toast.error(message);
    } finally {
      setBusy(null);
    }
  };

  const footer = (
    <>
      <ActionButton
        onClick={() => void run("print", "pdf", "print")}
        disabled={busy !== null || rangeError !== null}
        icon={<Printer className="h-4 w-4" aria-hidden />}
      >
        {busy === "print" ? "Wird geöffnet..." : "Drucken"}
      </ActionButton>
      <ActionButton
        onClick={() => void run("xlsx", "xlsx", "download")}
        disabled={busy !== null || rangeError !== null}
        icon={<FileSpreadsheet className="h-4 w-4" aria-hidden />}
      >
        {busy === "xlsx" ? "Erstellt..." : "XLSX"}
      </ActionButton>
      <ActionButton
        primary
        onClick={() => void run("pdf", "pdf", "download")}
        disabled={busy !== null || rangeError !== null}
        icon={<Download className="h-4 w-4" aria-hidden />}
      >
        {busy === "pdf" ? "Erstellt..." : "PDF"}
      </ActionButton>
    </>
  );

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      title={
        plan === "dienstplan"
          ? "Dienstplan drucken oder exportieren"
          : "Betreuungsplan drucken oder exportieren"
      }
      closeLabel="Export schließen"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-2xl"
      isDismissDisabled={busy !== null}
      footer={footer}
    >
      <div className="space-y-5">
        {templates.length > 1 && (
          <section>
            <p className="text-sm font-medium text-gray-900">Vorlage</p>
            <div className="mt-2 grid gap-2 sm:grid-cols-2">
              {templates.map((item) => (
                <OptionCard
                  key={item.id}
                  selected={template === item.id}
                  label={item.label}
                  description={item.description}
                  onClick={() => setTemplate(item.id)}
                />
              ))}
            </div>
          </section>
        )}

        <section>
          <p className="text-sm font-medium text-gray-900">Fassung</p>
          <div className="mt-2 grid gap-2 sm:grid-cols-2">
            {PLAN_EXPORT_VARIANTS.map((item) => (
              <OptionCard
                key={item.id}
                selected={variant === item.id}
                label={item.label}
                description={item.description}
                onClick={() => setVariant(item.id)}
              />
            ))}
          </div>
        </section>

        <section>
          <p className="text-sm font-medium text-gray-900">Zeitraum</p>
          <div className="mt-2 flex flex-wrap gap-2">
            <Pill
              selected={rangeMode === "week"}
              onClick={() => setRangeMode("week")}
            >
              Angezeigte Woche
            </Pill>
            <Pill
              selected={rangeMode === "range"}
              onClick={() => setRangeMode("range")}
            >
              Mehrere Wochen
            </Pill>
          </div>

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
            {rangeError ?? describeRange(range.from, range.to, weekCount)}
          </p>
        </section>
      </div>
    </Modal>
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
): string {
  if (!from || !to || weekCount === null) return "";
  const monday = startOfWeek(parseISODate(from));
  const friday = startOfWeek(parseISODate(to));
  friday.setDate(friday.getDate() + 4);
  const span = `${formatDate(toISODate(monday))} bis ${formatDate(toISODate(friday))}`;
  return weekCount === 1
    ? `Gedruckt wird die Woche vom ${span}, Montag bis Freitag.`
    : `Gedruckt werden ${weekCount} Wochen (${span}), je Woche ein Blatt.`;
}

function OptionCard({
  selected,
  label,
  description,
  onClick,
}: Readonly<{
  selected: boolean;
  label: string;
  description: string;
  onClick: () => void;
}>) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={`rounded-lg border px-3 py-3 text-left shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        selected
          ? "border-gray-900 bg-gray-950 text-white"
          : "border-gray-200 bg-white text-gray-800 hover:bg-gray-50"
      }`}
    >
      <span className="block text-sm font-semibold">{label}</span>
      <span
        className={`mt-1 block text-xs ${selected ? "text-gray-200" : "text-gray-500"}`}
      >
        {description}
      </span>
    </button>
  );
}

function Pill({
  selected,
  onClick,
  children,
}: Readonly<{
  selected: boolean;
  onClick: () => void;
  children: ReactNode;
}>) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className={`inline-flex h-9 items-center gap-2 rounded-lg border px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${
        selected
          ? "border-gray-900 bg-gray-950 text-white"
          : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
      }`}
    >
      {children}
    </button>
  );
}

function ActionButton({
  onClick,
  disabled,
  icon,
  primary = false,
  children,
}: Readonly<{
  onClick: () => void;
  disabled: boolean;
  icon: ReactNode;
  primary?: boolean;
  children: ReactNode;
}>) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`inline-flex h-9 items-center justify-center gap-2 rounded-lg px-3 text-sm font-medium shadow-sm transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50 ${
        primary
          ? "bg-gray-900 text-white hover:bg-gray-700"
          : "border border-gray-200 bg-white text-gray-700 hover:bg-gray-50"
      }`}
    >
      {icon}
      {children}
    </button>
  );
}
