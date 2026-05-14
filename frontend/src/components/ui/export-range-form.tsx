"use client";

import { useState } from "react";
import type { DateRange } from "react-day-picker";

import {
  buildDefaultPresets,
  RangeCalendarInline,
} from "~/components/ui/date-range-picker";

function addDays(d: Date, days: number): Date {
  const r = new Date(d);
  r.setDate(r.getDate() + days);
  return r;
}

function toIsoDate(d: Date): string {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

// Shared export form: inline date-range calendar + CSV/Excel buttons. The
// caller passes the proxy URL — used for both the MA-Sicht ("/api/time-tracking
// /export") and the admin-Sicht ("/api/staff/{id}/time-tracking/export").
export function ExportRangeForm({
  exportUrl,
  anchor,
  initialRange,
  onDone,
}: {
  readonly exportUrl: string;
  readonly anchor: Date;
  readonly initialRange?: DateRange;
  readonly onDone?: () => void;
}) {
  const today = new Date();
  const [range, setRange] = useState<DateRange | undefined>(
    () => initialRange ?? { from: addDays(today, -29), to: today },
  );

  const handleExport = (format: "csv" | "xlsx") => {
    if (!range?.from || !range?.to) return;
    const from = toIsoDate(range.from);
    const to = toIsoDate(range.to);
    const url = `${exportUrl}?from=${from}&to=${to}&format=${format}`;
    globalThis.location.href = url;
    onDone?.();
  };

  const hasRange = Boolean(range?.from && range?.to);

  return (
    <div className="w-full sm:w-fit">
      <p className="mb-3 px-4 pt-4 text-sm font-semibold text-gray-800">
        Zeitraum exportieren
      </p>
      <div className="px-2">
        <RangeCalendarInline
          value={range}
          onChange={setRange}
          presets={buildDefaultPresets(anchor, today)}
          fromMin={anchor}
          toMax={today}
        />
      </div>
      <div className="flex gap-2 border-t border-gray-100 p-4">
        <button
          type="button"
          onClick={() => handleExport("csv")}
          disabled={!hasRange}
          className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50"
        >
          CSV
        </button>
        <button
          type="button"
          onClick={() => handleExport("xlsx")}
          disabled={!hasRange}
          className="flex-1 rounded-lg bg-gray-900 px-3 py-2 text-sm font-medium text-white transition-colors hover:bg-gray-700 disabled:cursor-not-allowed disabled:opacity-50"
        >
          Excel
        </button>
      </div>
    </div>
  );
}
