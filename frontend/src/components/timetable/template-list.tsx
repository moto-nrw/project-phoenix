"use client";

import { Plus } from "lucide-react";
import { useMemo } from "react";

import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { SegmentedControl } from "~/components/ui/segmented-control";
import type { RosterDeviation } from "~/lib/roster-deviation";
import { TemplateCard } from "./template-card";
import type { TimetableTemplate } from "~/lib/timetable-types";
import { timetableSurface, timetableSurfacePadded } from "./timetable-style";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { carePlanConcept } from "~/lib/concepts/carePlan";

type ReviewFilter = "all" | "review";

interface TemplateListProps {
  templates: TimetableTemplate[];
  onCreate: () => void;
  onEdit: (template: TimetableTemplate) => void;
  onApply: (template: TimetableTemplate) => void;
  onArchive: (template: TimetableTemplate) => void;
  /** Leseansicht (#2283): false blendet Anlegen und Karten-Aktionen aus. */
  canManage?: boolean;
  /** Abweichungen von den Anmeldungen je Regeltermin-ID (#5103). */
  deviations?: ReadonlyMap<string, RosterDeviation>;
  onConnect?: (template: TimetableTemplate, deviation: RosterDeviation) => void;
  onKeepManual?: (template: TimetableTemplate) => void;
  /** „Zu prüfen“: nur Regeltermine mit erkannter Abweichung. */
  reviewOnly?: boolean;
  onReviewOnlyChange?: (reviewOnly: boolean) => void;
}

export function TemplateList({
  templates,
  onCreate,
  onEdit,
  onApply,
  onArchive,
  canManage = true,
  deviations,
  onConnect,
  onKeepManual,
  reviewOnly = false,
  onReviewOnlyChange,
}: TemplateListProps) {
  const reviewCount = useMemo(
    () => templates.filter((template) => deviations?.has(template.id)).length,
    [templates, deviations],
  );
  // Zu prüfende Regeltermine stehen vorn; sonst bleibt die Reihenfolge.
  const visible = useMemo(() => {
    const flagged = templates.filter((template) =>
      deviations?.has(template.id),
    );
    if (reviewOnly) return flagged;
    return [
      ...flagged,
      ...templates.filter((template) => !deviations?.has(template.id)),
    ];
  }, [templates, deviations, reviewOnly]);

  if (templates.length === 0) {
    return (
      <div className={`${timetableSurface} border-dashed px-6`}>
        <EmptyState
          icon={
            <span className="flex h-12 w-12 items-center justify-center rounded-full bg-gray-100">
              <MotoConceptIcon concept={carePlanConcept} size={28} />
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
      {onReviewOnlyChange && (reviewCount > 0 || reviewOnly) ? (
        <SegmentedControl<ReviewFilter>
          ariaLabel="Regeltermine zeigen"
          className="mb-4"
          value={reviewOnly ? "review" : "all"}
          onChange={(next) => onReviewOnlyChange(next === "review")}
          items={[
            { value: "all", label: "Alle" },
            { value: "review", label: `Zu prüfen (${reviewCount})` },
          ]}
        />
      ) : null}
      {visible.length === 0 ? (
        <EmptyState
          title="Keine Regeltermine zu prüfen"
          description="Alle Kinderlisten passen zu den Anmeldungen oder werden bewusst von Hand gepflegt."
        />
      ) : (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
          {visible.map((template) => (
            <TemplateCard
              key={template.id}
              template={template}
              onEdit={onEdit}
              onApply={onApply}
              onArchive={onArchive}
              canManage={canManage}
              deviation={deviations?.get(template.id)}
              onConnect={onConnect}
              onKeepManual={onKeepManual}
            />
          ))}
        </div>
      )}
    </div>
  );
}
