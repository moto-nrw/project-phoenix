"use client";

import { Check, Eye, FileText, Pencil } from "lucide-react";
import { GuideStep } from "~/components/enrollment/enrollment-form-overview";
import {
  FormPreview,
  PreviewLinkCard,
  useTemplatePreviewHref,
} from "~/components/enrollment/form-template-preview";
import { BackAction } from "~/components/ui/back-link";
import { Button, ButtonLink } from "~/components/ui/button";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StatCard } from "~/components/ui/stat-card";
import { calendarPeriodsConcept } from "~/lib/concepts/calendarPeriods";
import { childrenConcept } from "~/lib/concepts/children";
import { enrollmentsConcept } from "~/lib/concepts/enrollments";
import { formatBerlinDate } from "~/lib/date-helpers";
import type { FormSchema } from "~/lib/enrollment-form-schema-api";
import type { Phase } from "~/lib/enrollment-phase-api";

export function FormTemplateDetail({
  schema,
  onBack,
  onEdit,
  assignedPhases,
}: Readonly<{
  schema: FormSchema;
  onBack: () => void;
  onEdit: () => void;
  assignedPhases: Phase[];
}>) {
  const previewHref = useTemplatePreviewHref();
  const requiredCount = schema.fields.filter((field) =>
    Boolean(field.required),
  ).length;
  const childFieldCount = schema.fields.filter((field) =>
    Boolean(field.applies_to_child),
  ).length;

  return (
    // Flex-Spalte, siehe die Übersicht derselben Seite.
    <div className="flex flex-col space-y-5">
      <section className="moto-content-surface flex flex-col overflow-hidden rounded-2xl border">
        <div className="grid flex-1 gap-0 lg:grid-cols-[minmax(0,1fr)_360px] xl:grid-cols-[minmax(0,1fr)_400px]">
          <div>
            <div className="border-b border-gray-100 px-5 py-3 sm:px-6">
              <BackAction onClick={onBack} label="Zurück zur Übersicht" />
            </div>
            <div className="space-y-5 p-5 sm:p-6">
              <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
                <div>
                  <h2 className="text-xl font-semibold text-gray-900">
                    {schema.name}
                  </h2>
                  <p className="mt-2 max-w-3xl text-sm leading-6 text-gray-600">
                    Prüfen Sie die Formularvorschau und ordnen Sie diese Vorlage
                    einer Anmeldephase zu, wenn Eltern die angepassten
                    Pflichtangaben oder Zusatzfragen sehen sollen.
                  </p>
                </div>
                <Button
                  type="button"
                  onClick={onEdit}
                  variant="primary"
                  size="md"
                  className="inline-flex items-center justify-center gap-2 whitespace-nowrap"
                >
                  <Pencil className="h-4 w-4" aria-hidden="true" />
                  Bearbeiten
                </Button>
              </div>

              <div className="grid grid-cols-2 gap-3 xl:grid-cols-4">
                <StatCard
                  variant="inline"
                  icon={<FileText className="h-4 w-4" aria-hidden="true" />}
                  value={formatBerlinDate(schema.created_at)}
                  label="Zuletzt gespeichert"
                />
                <StatCard
                  variant="inline"
                  icon={
                    <MotoConceptIcon concept={enrollmentsConcept} size={16} />
                  }
                  value={schema.fields.length.toString()}
                  label="Zusatzfragen"
                />
                <StatCard
                  variant="inline"
                  icon={<Check className="h-4 w-4" aria-hidden="true" />}
                  value={requiredCount.toString()}
                  label="Pflicht-Zusatzfragen"
                />
                <StatCard
                  variant="inline"
                  icon={<MotoConceptIcon concept={childrenConcept} size={16} />}
                  value={childFieldCount.toString()}
                  label="Pro Kind"
                />
              </div>

              <FormPreview
                fields={schema.fields}
                coreRequirements={schema.core_requirements ?? {}}
                legalBlocks={schema.legal_blocks ?? []}
                templateName={schema.name}
                isActive={schema.is_active}
                isSaved
                previewHref={previewHref(schema.id)}
                assignedPhaseCount={assignedPhases.length}
                sticky={false}
              />
            </div>
          </div>

          <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-5 sm:p-6 lg:border-t-0 lg:border-l">
            <div className="sticky top-6 space-y-4">
              <div>
                <h2 className="text-base font-semibold text-gray-900">
                  {assignedPhases.length > 0
                    ? "In Anmeldephase verwendet"
                    : "In Anmeldephase verwenden"}
                </h2>
                <p className="mt-2 text-sm leading-6 text-gray-600">
                  {assignedPhases.length > 0
                    ? `Diese Vorlage ist in ${assignedPhases.length} Anmeldephase ausgewählt.`
                    : "Eine Vorlage wird erst für Eltern relevant, wenn sie in einer Anmeldephase ausgewählt ist. Ohne Auswahl nutzt die Phase das Basisformular."}
                </p>
              </div>

              <ButtonLink
                href={
                  assignedPhases.length > 0
                    ? "/enrollment-phases"
                    : `/enrollment-phases?assignForm=${encodeURIComponent(schema.id)}`
                }
                size="md"
                className="w-full gap-2"
              >
                <MotoConceptIcon
                  concept={calendarPeriodsConcept}
                  colorMode="inherit"
                  size={18}
                />
                {assignedPhases.length > 0
                  ? "Anmeldephasen öffnen"
                  : "In Anmeldephase auswählen"}
              </ButtonLink>

              <PreviewLinkCard
                href={previewHref(schema.id)}
                description="Öffnet die Formularvorschau in einem neuen Tab."
              />

              <div className="moto-content-surface rounded-2xl border p-4">
                <h3 className="text-sm font-semibold text-gray-900">Status</h3>
                <div className="mt-4 space-y-3">
                  <GuideStep
                    icon={<Check className="h-4 w-4" aria-hidden="true" />}
                    title="Vorlage gespeichert"
                    done
                  />
                  <GuideStep
                    icon={<Eye className="h-4 w-4" aria-hidden="true" />}
                    title="Vorschau prüfen"
                    done
                  />
                  <GuideStep
                    icon={
                      <MotoConceptIcon
                        concept={calendarPeriodsConcept}
                        size={18}
                      />
                    }
                    title="In Anmeldephase auswählen"
                    done={assignedPhases.length > 0}
                  />
                </div>
              </div>
            </div>
          </aside>
        </div>
      </section>
    </div>
  );
}
