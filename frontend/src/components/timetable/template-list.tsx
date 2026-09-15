"use client";

import { Plus } from "lucide-react";

import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
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
      <div className={`${timetableSurface} border-dashed px-6`}>
        <EmptyState
          icon={
            <span className="flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
              <MotoConceptIcon concept="carePlan" size={28} />
            </span>
          }
          title="Keine Regeltermine in diesem Zeitraum"
          description="Regeltermine sind wiederkehrende Termine, zum Beispiel Mensa jeden Montag oder Lernzeit alle zwei Wochen."
          action={
            canManage ? (
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={onCreate}
                className="gap-2"
              >
                <Plus className="h-5 w-5 stroke-[2.5]" aria-hidden />
                Regeltermin anlegen
              </Button>
            ) : undefined
          }
        />
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
