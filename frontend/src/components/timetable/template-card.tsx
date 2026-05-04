"use client";

/**
 * TemplateCard — single recurring-activity template tile.
 *
 * Replaces the legacy two-column list+aside in `template-list.tsx` with a
 * self-contained card that exposes the highest-signal info inline (weekday
 * pattern visualization, time, room, counts) plus two quick actions:
 * "Bearbeiten" (opens existing modal) and "Anwenden" (materializes the
 * template's calendar period). Visual style follows shadcn conventions —
 * single border, no default shadow, brand accent reserved for the color
 * bar that ties the card back to the activity type.
 */

import { Clock, DoorOpen, Users } from "lucide-react";

import {
  getActivityColor,
  getGermanWeekdayShort,
} from "~/lib/timetable-helpers";
import type { TimetableTemplate } from "~/lib/timetable-types";

interface TemplateCardProps {
  template: TimetableTemplate;
  onEdit: (template: TimetableTemplate) => void;
  onApply: (template: TimetableTemplate) => void;
}

const WEEKDAYS = [1, 2, 3, 4, 5] as const;

const TYPE_LABELS: Record<TimetableTemplate["type"], string> = {
  care: "Betreuung",
  activity: "AG",
  external: "Extern",
};

function weekdayShort(iso: number): string {
  // 2024-01-01 was a Monday — anchor for getGermanWeekdayShort.
  const ref = new Date(2024, 0, 1);
  ref.setDate(ref.getDate() + (iso - 1));
  return getGermanWeekdayShort(ref);
}

function summarizeTimeRange(template: TimetableTemplate): string | null {
  if (template.schedules.length === 0) return null;
  // All schedules typically share the same time-of-day window; show the
  // first and append "+1" if any other schedule diverges.
  const first = template.schedules[0]!;
  const allSame = template.schedules.every(
    (s) => s.startTime === first.startTime && s.endTime === first.endTime,
  );
  const base = `${first.startTime} – ${first.endTime}`;
  return allSame ? base : `${base} +`;
}

export function TemplateCard({ template, onEdit, onApply }: TemplateCardProps) {
  const color = getActivityColor(template.type);
  const activeWeekdays = new Set(template.schedules.map((s) => s.weekday));
  const timeRange = summarizeTimeRange(template);

  return (
    <article className="group relative flex flex-col overflow-hidden rounded-lg border border-slate-200 bg-white transition-shadow hover:shadow-sm">
      <div
        className="absolute top-0 left-0 h-full w-1"
        style={{ backgroundColor: color }}
        aria-hidden
      />

      <div className="flex flex-col gap-3 p-4 pl-5">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <h3 className="truncate text-sm font-semibold text-slate-900">
              {template.name}
            </h3>
            <p className="mt-0.5 text-[11px] font-medium tracking-wide text-slate-500 uppercase">
              {TYPE_LABELS[template.type]}
              {template.categoryName ? ` · ${template.categoryName}` : ""}
            </p>
          </div>
        </div>

        <div className="flex gap-1">
          {WEEKDAYS.map((wd) => {
            const active = activeWeekdays.has(wd);
            return (
              <div
                key={wd}
                className={`flex h-6 w-7 items-center justify-center rounded text-[10px] font-semibold ${
                  active
                    ? "bg-slate-900 text-white"
                    : "bg-slate-100 text-slate-400"
                }`}
              >
                {weekdayShort(wd)}
              </div>
            );
          })}
        </div>

        <dl className="space-y-1.5 text-[12px] text-slate-600">
          <div className="flex items-center gap-2">
            <Clock className="h-3.5 w-3.5 text-slate-400" aria-hidden />
            <span className="tabular-nums">
              {timeRange ?? "Keine Zeiten hinterlegt"}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <DoorOpen className="h-3.5 w-3.5 text-slate-400" aria-hidden />
            <span className="truncate">{template.roomName ?? "Kein Raum"}</span>
          </div>
          <div className="flex items-center gap-2">
            <Users className="h-3.5 w-3.5 text-slate-400" aria-hidden />
            <span>
              {template.enrollmentCount}{" "}
              {template.enrollmentCount === 1 ? "Kind" : "Kinder"}
              {" · "}
              {template.supervisorCount} Personal
            </span>
          </div>
        </dl>
      </div>

      <div className="flex items-center justify-between gap-2 border-t border-slate-100 px-4 py-2.5 pl-5">
        <button
          type="button"
          onClick={() => onEdit(template)}
          className="text-[12px] font-medium text-slate-600 transition-colors hover:text-slate-900"
        >
          Bearbeiten
        </button>
        <button
          type="button"
          onClick={() => onApply(template)}
          className="inline-flex items-center gap-1 rounded-md bg-slate-900 px-2.5 py-1.5 text-[12px] font-medium text-white transition-colors hover:bg-slate-700"
        >
          Anwenden
        </button>
      </div>
    </article>
  );
}
