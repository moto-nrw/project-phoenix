"use client";

import { Plus } from "lucide-react";

import { Button } from "~/components/ui/button";
import { TemplateCard } from "./template-card";
import type { TimetableTemplate } from "~/lib/timetable-types";
import { timetableSurface, timetableSurfacePadded } from "./timetable-style";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";

interface TemplateListProps {
  templates: TimetableTemplate[];
  onCreate: () => void;
  onEdit: (template: TimetableTemplate) => void;
  onApply: (template: TimetableTemplate) => void;
  onArchive: (template: TimetableTemplate) => void;
  /** Leseansicht (#2283): false blendet Anlegen und Karten-Aktionen aus. */
  canManage?: boolean;
}

export function TemplateList({
  templates,
  onCreate,
  onEdit,
  onApply,
  onArchive,
  canManage = true,
}: TemplateListProps) {
  if (templates.length === 0) {
    return (
      <div
        className={`${timetableSurface} flex flex-col items-center justify-center gap-3 border-dashed px-6 py-16 text-center`}
      >
        <div className="flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
          <MotoConceptIcon concept="carePlan" size={28} />
        </div>
        <div className="flex flex-col gap-1">
          <h3 className="text-base font-semibold text-gray-900">
            Keine Regeltermine in diesem Zeitraum
          </h3>
          <p className="max-w-sm text-sm text-gray-500">
            Regeltermine sind wiederkehrende Termine, zum Beispiel Mensa jeden
            Montag oder Lernzeit alle zwei Wochen.
          </p>
        </div>
        {canManage && (
          <Button
            type="button"
            variant="primary"
            size="sm"
            onClick={onCreate}
            className="mt-2 gap-2"
          >
            <Plus className="h-5 w-5 stroke-[2.5]" aria-hidden />
            Regeltermin anlegen
          </Button>
        )}
      </div>
    );
  }

  return (
    <div className={timetableSurfacePadded}>
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {templates.map((template) => (
          <TemplateCard
            key={template.id}
            template={template}
            onEdit={onEdit}
            onApply={onApply}
            onArchive={onArchive}
            canManage={canManage}
          />
        ))}
      </div>
    </div>
  );
}
