"use client";

import { useEffect, useState } from "react";
import { Download } from "lucide-react";
import type { DateRange } from "react-day-picker";

import {
  buildDefaultPresets,
  RangeCalendarInline,
} from "~/components/ui/date-range-picker";
import { ExportDialog } from "~/components/ui/export-dialog";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { useExportRun } from "~/hooks/use-export-run";
import {
  berlinTodayISO,
  formatDate,
  parseISODate,
  toISODate,
} from "~/lib/date-helpers";
import {
  exportStaffTimeTracking,
  type TimeTrackingExportFormat,
} from "~/lib/time-tracking-export-api";

// PDF ist der gestaltete Zeitnachweis (#1568) und deshalb vorne; Excel und
// CSV bleiben die Datenformate.
const TIME_TRACKING_FORMATS: readonly TimeTrackingExportFormat[] = [
  "pdf",
  "xlsx",
  "csv",
];

function addDays(date: Date, days: number): Date {
  const next = new Date(date);
  next.setDate(next.getDate() + days);
  return next;
}

function defaultRange(yearStart: Date): DateRange {
  const today = parseISODate(berlinTodayISO());
  const from = addDays(today, -29);
  return { from: from < yearStart ? yearStart : from, to: today };
}

/**
 * Zeitnachweis einer Person (eigene Zeiterfassung und Personal, Reiter
 * Zeiterfassung). Der Eintrag im Menü der Kopfkarte öffnet den gemeinsamen
 * Export-Dialog mit dem Zeitraum als Option.
 */
export function StaffExportButton({
  staffId,
  yearStart,
}: {
  readonly staffId: string;
  readonly yearStart: Date;
}) {
  const yearStartISO = toISODate(yearStart);
  const [open, setOpen] = useState(false);
  const [range, setRange] = useState<DateRange | undefined>(() =>
    defaultRange(yearStart),
  );

  // Nur beim Öffnen oder bei einer anderen Datumsgrenze neu vorbelegen.
  useEffect(() => {
    if (open) setRange(defaultRange(parseISODate(yearStartISO)));
  }, [open, yearStartISO]);

  const run = useExportRun({
    isOpen: open,
    onDownload: (format: TimeTrackingExportFormat) =>
      range?.from && range.to
        ? exportStaffTimeTracking(
            staffId,
            toISODate(range.from),
            toISODate(range.to),
            format,
          )
        : Promise.resolve(),
    onPrint: (target) =>
      range?.from && range.to
        ? exportStaffTimeTracking(
            staffId,
            toISODate(range.from),
            toISODate(range.to),
            "pdf",
            target,
          )
        : Promise.resolve(),
    validate: () =>
      range?.from && range.to ? null : "Bitte wählen Sie einen Zeitraum aus.",
    errorObject: "den Zeitnachweis",
    onDone: () => setOpen(false),
    logName: "time_tracking",
  });

  const today = parseISODate(berlinTodayISO());
  const hasRange = Boolean(range?.from && range.to);

  return (
    <>
      <OverflowMenu
        ariaLabel="Menü öffnen"
        items={[
          {
            label: "Exportieren",
            icon: <Download className="h-4 w-4" />,
            onClick: () => setOpen(true),
          },
        ]}
      />
      <ExportDialog
        isOpen={open}
        onClose={() => setOpen(false)}
        title="Zeitnachweis exportieren"
        description={
          range?.from && range.to
            ? `Arbeitszeiten vom ${formatDate(toISODate(range.from))} bis ${formatDate(toISODate(range.to))}.`
            : "Arbeitszeiten für den gewählten Zeitraum."
        }
        formats={TIME_TRACKING_FORMATS}
        run={run}
        blocked={!hasRange}
        widthClass="sm:w-[640px]"
      >
        <section>
          <p className="text-sm font-medium text-gray-900">Zeitraum</p>
          <div className="mt-2">
            <RangeCalendarInline
              value={range}
              onChange={setRange}
              presets={buildDefaultPresets(yearStart, today)}
              fromMin={yearStart}
              toMax={today}
            />
          </div>
        </section>
      </ExportDialog>
    </>
  );
}
