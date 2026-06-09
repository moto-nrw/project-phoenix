"use client";

import { CalendarDays } from "lucide-react";

import { Button } from "~/components/ui/button";
import { TemplateCard } from "./template-card";
import type { TimetableTemplate } from "~/lib/timetable-types";

interface TemplateListProps {
  templates: TimetableTemplate[];
  onCreate: () => void;
  onEdit: (template: TimetableTemplate) => void;
  onApply: (template: TimetableTemplate) => void;
  onArchive: (template: TimetableTemplate) => void;
}

export function TemplateList({
  templates,
  onCreate,
  onEdit,
  onApply,
  onArchive,
}: TemplateListProps) {
  if (templates.length === 0) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 rounded-2xl border border-dashed border-gray-200 bg-gray-50/40 px-6 py-16 text-center">
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
          <CalendarDays className="h-6 w-6 text-gray-400" aria-hidden />
        </div>
        <div className="flex flex-col gap-1">
          <h3 className="text-base font-semibold text-gray-900">
            Keine Serien für diese Periode
          </h3>
          <p className="max-w-sm text-sm text-gray-500">
            Serien sind wiederkehrende Termine, die automatisch im Kalender
            erscheinen.
          </p>
        </div>
        <Button
          type="button"
          variant="primary"
          size="sm"
          onClick={onCreate}
          className="mt-2"
        >
          + Serientermin anlegen
        </Button>
      </div>
    );
  }

  return (
    <div className="moto-content-surface rounded-2xl border p-4 sm:p-6">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {templates.map((template) => (
          <TemplateCard
            key={template.id}
            template={template}
            onEdit={onEdit}
            onApply={onApply}
            onArchive={onArchive}
          />
        ))}
      </div>
    </div>
  );
}
