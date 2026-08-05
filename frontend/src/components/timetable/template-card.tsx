"use client";

/**
 * TemplateCard — single recurring series tile.
 *
 * Replaces the legacy two-column list+aside in `template-list.tsx` with a
 * self-contained card that exposes the highest-signal info inline (weekday
 * pattern visualization, time, room, counts) plus two quick actions:
 * "Bearbeiten" (opens existing modal) and "Termine erzeugen" (materializes
 * the series' calendar period). Visual style follows shadcn conventions —
 * single border, no default shadow, brand accent reserved for the color
 * bar that ties the card back to the activity type.
 */

import { Archive, Clock } from "lucide-react";

import { Button } from "~/components/ui/button";
import {
  getActivityColor,
  getGermanWeekdayShort,
} from "~/lib/timetable-helpers";
import type { TimetableTemplate } from "~/lib/timetable-types";
import { capacityTone, TimetableRatioPill } from "./timetable-ratio-pill";
import { timetableSurface } from "./timetable-style";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

interface TemplateCardProps {
  template: TimetableTemplate;
  onEdit: (template: TimetableTemplate) => void;
  onApply: (template: TimetableTemplate) => void;
  onArchive: (template: TimetableTemplate) => void;
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

export function TemplateCard({
  template,
  onEdit,
  onApply,
  onArchive,
}: TemplateCardProps) {
  const color = getActivityColor(template.type);
  const activeWeekdays = new Set(template.schedules.map((s) => s.weekday));
  const timeRange = summarizeTimeRange(template);

  return (
    <article
      className={`${timetableSurface} group relative flex flex-col overflow-hidden transition-[border-color,box-shadow] hover:border-gray-300 hover:shadow-md`}
    >
      <div
        className="absolute top-0 left-0 h-full w-1"
        style={{ backgroundColor: color }}
        aria-hidden
      />

      <div className="flex flex-col gap-3 p-4 pl-5">
        <div className="flex items-start justify-between gap-2">
          <div className="min-w-0 flex-1">
            <h3 className="truncate text-sm font-semibold text-gray-900">
              {template.name}
            </h3>
            <p className="mt-0.5 text-[11px] font-medium tracking-wide text-gray-500 uppercase">
              {TYPE_LABELS[template.type]}
              {template.categoryName ? ` · ${template.categoryName}` : ""}
            </p>
            {template.shiftTypeName ? (
              <span
                className="mt-1 inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-medium"
                style={{
                  backgroundColor: `${template.shiftTypeColor || "#6B7280"}1A`,
                  color: template.shiftTypeColor || "#6B7280",
                }}
                title={`Schichtart: ${template.shiftTypeName}`}
              >
                <span
                  className="h-2 w-2 rounded-full"
                  style={{
                    backgroundColor: template.shiftTypeColor || "#6B7280",
                  }}
                  aria-hidden
                />
                {template.shiftTypeName}
              </span>
            ) : null}
          </div>
        </div>

        <div className="flex gap-1">
          {WEEKDAYS.map((wd) => {
            const active = activeWeekdays.has(wd);
            return (
              <div
                key={wd}
                className={`flex h-6 w-7 items-center justify-center rounded-full text-[10px] font-semibold ${
                  active
                    ? "bg-gray-900 text-white"
                    : "bg-gray-100 text-gray-400"
                }`}
              >
                {weekdayShort(wd)}
              </div>
            );
          })}
        </div>

        <dl className="space-y-1.5 text-xs text-gray-600">
          <div className="flex items-center gap-2">
            <Clock className="h-3.5 w-3.5 text-gray-400" aria-hidden />
            <span className="tabular-nums">
              {timeRange ?? "Keine Zeiten hinterlegt"}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <MotoConceptIcon concept="rooms" size={16} />
            <span className="truncate">{template.roomName ?? "Kein Raum"}</span>
          </div>
          <div className="flex items-center gap-2">
            <MotoConceptIcon concept="children" size={16} />
            <span>
              {template.enrollmentCount}{" "}
              {template.enrollmentCount === 1 ? "Kind" : "Kinder"}
              {" · "}
              {template.supervisorCount} Personal
            </span>
          </div>
          {template.requiredStaffCount > 0 && (
            <div className="flex items-center gap-2">
              <MotoConceptIcon concept="supervision" size={16} />
              <TimetableRatioPill
                icon={<MotoConceptIcon concept="supervision" size={16} />}
                label="Besetzung"
                value={`${template.assignedStaffCount}/${template.requiredStaffCount}`}
                tone={capacityTone(
                  template.assignedStaffCount,
                  template.requiredStaffCount,
                )}
                variant="compact"
              />
            </div>
          )}
        </dl>

        {!template.roomId && (
          <p className="flex items-center gap-1.5 text-xs text-gray-600">
            <span
              className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#EAB308]"
              aria-hidden
            />
            Raum fehlt – wird nicht eingeplant
          </p>
        )}
      </div>

      <div className="flex flex-col gap-2 border-t border-gray-100 px-4 py-2.5 pl-5">
        <div className="flex items-center justify-between gap-2">
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => onEdit(template)}
          >
            Bearbeiten
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={() => onArchive(template)}
          >
            <Archive className="h-3.5 w-3.5" aria-hidden />
            Archivieren
          </Button>
        </div>
        <Button
          type="button"
          variant="primary"
          size="compact"
          className="w-full"
          onClick={() => onApply(template)}
        >
          Termine erzeugen
        </Button>
      </div>
    </article>
  );
}
