"use client";

import type { ReactNode } from "react";
import { Archive, CalendarClock, DoorOpen, Pencil, Users } from "lucide-react";

import {
  getActivityColor,
  getActivityTypeBadge,
  getGermanWeekdayShort,
} from "~/lib/timetable-helpers";
import type { TimetableTemplate } from "~/lib/timetable-types";

interface TemplateListProps {
  templates: TimetableTemplate[];
  selectedId?: string | null;
  onSelect: (template: TimetableTemplate) => void;
  onCreate: () => void;
  onEdit: (template: TimetableTemplate) => void;
  onArchive: (template: TimetableTemplate) => void;
}

export function TemplateList({
  templates,
  selectedId,
  onSelect,
  onCreate,
  onEdit,
  onArchive,
}: TemplateListProps) {
  if (templates.length === 0) {
    return (
      <div className="rounded-xl border border-dashed border-slate-300 bg-slate-50 p-6 text-center">
        <h3 className="text-base font-bold text-slate-900">
          Noch keine Vorlagen
        </h3>
        <p className="mt-1 text-sm text-slate-500">
          Vorlagen beschreiben, was regelmäßig passieren soll. Daraus entstehen
          später konkrete Termine im Kalender.
        </p>
        <button
          type="button"
          onClick={onCreate}
          className="mt-4 rounded-md bg-[#5080D8] px-4 py-2 text-sm font-semibold text-white hover:bg-[#4070c8]"
        >
          Vorlage anlegen
        </button>
      </div>
    );
  }

  return (
    <div className="grid gap-3 lg:grid-cols-[minmax(0,1fr)_360px]">
      <div className="grid gap-2">
        {templates.map((template) => {
          const badge = getActivityTypeBadge(template.type);
          const color = getActivityColor(template.type);
          const selected = selectedId === template.id;
          return (
            <button
              key={template.id}
              type="button"
              onClick={() => onSelect(template)}
              className={`rounded-lg border bg-white p-3 text-left shadow-sm transition-colors hover:border-[#5080D8] ${
                selected
                  ? "border-[#5080D8] ring-2 ring-[#5080D8]/20"
                  : "border-slate-200"
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <span
                      className="h-2.5 w-2.5 rounded-full"
                      style={{ backgroundColor: color }}
                    />
                    <h3 className="truncate text-sm font-bold text-slate-900">
                      {template.name}
                    </h3>
                    {badge && (
                      <span
                        className="rounded px-1.5 py-0.5 text-[9px] font-bold text-white"
                        style={{ backgroundColor: badge.bg }}
                      >
                        {badge.label}
                      </span>
                    )}
                  </div>
                  <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-500">
                    <span className="inline-flex items-center gap-1">
                      <CalendarClock className="h-3.5 w-3.5" />
                      {formatSchedules(template)}
                    </span>
                    <span className="inline-flex items-center gap-1">
                      <DoorOpen className="h-3.5 w-3.5" />
                      {template.roomName || "Kein Raum"}
                    </span>
                    <span className="inline-flex items-center gap-1">
                      <Users className="h-3.5 w-3.5" />
                      {template.enrollmentCount} Kinder ·{" "}
                      {template.supervisorCount} Personal
                    </span>
                  </div>
                </div>
              </div>
            </button>
          );
        })}
      </div>

      <TemplateDetail
        template={templates.find((t) => t.id === selectedId) ?? templates[0]!}
        onEdit={onEdit}
        onArchive={onArchive}
      />
    </div>
  );
}

function TemplateDetail({
  template,
  onEdit,
  onArchive,
}: {
  template: TimetableTemplate;
  onEdit: (template: TimetableTemplate) => void;
  onArchive: (template: TimetableTemplate) => void;
}) {
  return (
    <aside className="rounded-xl border border-slate-200 bg-white p-4 shadow-sm">
      <div className="text-xs font-bold text-slate-400 uppercase">Vorlage</div>
      <h3 className="mt-1 text-lg font-bold text-slate-900">{template.name}</h3>
      <dl className="mt-4 space-y-3 text-sm">
        <Row label="Kategorie">{template.categoryName || "Ohne Kategorie"}</Row>
        <Row label="Raum">{template.roomName || "Kein Raum hinterlegt"}</Row>
        <Row label="Zeiten">{formatSchedules(template)}</Row>
        <Row label="Kinder">{template.enrollmentCount}</Row>
        <Row label="Personal">{template.supervisorCount}</Row>
      </dl>
      <div className="mt-5 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => onEdit(template)}
          className="inline-flex items-center gap-2 rounded-md border border-[#5080D8] bg-[#5080D8] px-3 py-2 text-xs font-semibold text-white hover:bg-[#4070c8]"
        >
          <Pencil className="h-3.5 w-3.5" />
          Bearbeiten
        </button>
        <button
          type="button"
          onClick={() => onArchive(template)}
          className="inline-flex items-center gap-2 rounded-md border border-[#FECACA] bg-white px-3 py-2 text-xs font-semibold text-[#991B1B] hover:bg-[#FEF2F2]"
        >
          <Archive className="h-3.5 w-3.5" />
          Archivieren
        </button>
      </div>
    </aside>
  );
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div>
      <dt className="text-[11px] font-semibold text-slate-500">{label}</dt>
      <dd className="font-medium text-slate-900">{children}</dd>
    </div>
  );
}

function formatSchedules(template: TimetableTemplate): string {
  if (template.schedules.length === 0) return "Keine Zeiten";
  const byTime = new Map<string, number[]>();
  for (const schedule of template.schedules) {
    const key = `${schedule.startTime}–${schedule.endTime}`;
    const days = byTime.get(key) ?? [];
    days.push(schedule.weekday);
    byTime.set(key, days);
  }
  return [...byTime.entries()]
    .map(([time, days]) => `${days.map(weekdayLabel).join(", ")} ${time}`)
    .join(" · ");
}

function weekdayLabel(iso: number): string {
  const ref = new Date(2024, 0, 1);
  ref.setDate(ref.getDate() + (iso - 1));
  return getGermanWeekdayShort(ref);
}
