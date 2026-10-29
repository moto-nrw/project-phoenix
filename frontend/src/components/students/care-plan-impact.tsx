"use client";

import { type ReactNode, useState } from "react";
import { useSession } from "next-auth/react";
import { Button, ButtonLink } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { hasPermission } from "~/lib/auth-utils";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  describeAdjustment,
  formatWeekdayAdverbs,
  type ManualRosterTemplate,
  removeStudentTemplateDays,
  type RosterImpact,
} from "~/lib/roster-impact";
import { useTenantAwarePath } from "~/lib/tenant-path";

const logger = createLogger({ component: "CarePlanImpact" });

interface CarePlanImpactProps {
  readonly impact: RosterImpact;
  /** Ohne Kind-ID gibt es nichts zu entfernen; die Liste bleibt sichtbar. */
  readonly studentId?: string;
  /** Vorname, wenn eine Seite mehrere Kinder zeigt. */
  readonly childName?: string;
}

/**
 * „Auswirkung auf den Betreuungsplan“ nach einer Buchungsänderung (#5102):
 * was moto automatisch angepasst hat und welche von Hand gepflegten
 * Regeltermine das Kind an weggefallenen Tagen weiter einplanen. Plant ein
 * Regeltermin über Klasse, Jahrgang oder Gruppe, hält ein Entfernen nicht;
 * dort führt der Hinweis zu „Mit Angebot verbinden“ (#5137).
 */
export function CarePlanImpact({
  impact,
  studentId,
  childName,
}: CarePlanImpactProps) {
  const { data: session } = useSession();
  const canManage = hasPermission(session, "schedules:manage");
  const canRemove = studentId !== undefined && canManage;
  const [removed, setRemoved] = useState<ReadonlySet<string>>(new Set());
  const [target, setTarget] = useState<ManualRosterTemplate | null>(null);
  const [removing, setRemoving] = useState(false);
  const errors = useApiFormError();
  const toast = useToast();
  const tenantPath = useTenantAwarePath();
  const stichtag = formatDate(impact.effectiveFrom);

  if (impact.adjusted.length === 0 && impact.notAdjusted.length === 0) {
    return (
      <p className="text-sm text-gray-600">
        Im Betreuungsplan musste kein Regeltermin angepasst werden.
      </p>
    );
  }

  const planLink = tenantPath(
    `/betreuungsplan?view=serien&d=${encodeURIComponent(impact.effectiveFrom)}`,
  );
  const connectLink = (templateId: string) =>
    `${planLink}&verbinden=${encodeURIComponent(templateId)}`;
  // Ohne Recht fehlt an einem Eintrag eine Aktion, die andere sehen würden.
  const actionMissing =
    !canManage &&
    impact.notAdjusted.some(
      (template) => template.viaTarget || studentId !== undefined,
    );

  const closeConfirm = () => {
    if (removing) return;
    setTarget(null);
    errors.clear();
  };

  const confirmRemoval = async () => {
    if (!target || studentId === undefined) return;
    setRemoving(true);
    errors.clear();
    try {
      await removeStudentTemplateDays({
        templateId: target.templateId,
        studentId,
        weekdays: target.weekdays,
        effectiveFrom: impact.effectiveFrom,
      });
      setRemoved((current) => new Set(current).add(target.templateId));
      toast.success(`Das Kind ist aus „${target.name}“ entfernt.`);
      setTarget(null);
    } catch (err) {
      logger.warn("care_plan_impact_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
        template_id: target.templateId,
      });
      await errors.show(err, { object: "das Entfernen aus dem Regeltermin" });
    } finally {
      setRemoving(false);
    }
  };

  return (
    <SectionCard
      title={`Auswirkung auf den Betreuungsplan${childName ? ` für ${childName}` : ""} ab ${stichtag}`}
      headingLevel={4}
      namedRegion
      titleClassName="text-sm"
      bodyClassName="mt-3 flex flex-col gap-5"
    >
      {impact.adjusted.length > 0 ? (
        <SectionCard
          title="Automatisch angepasst"
          headingLevel={5}
          titleClassName="text-sm font-medium text-gray-600"
          bare
          className="space-y-2"
        >
          <ul className="divide-y divide-gray-100 text-sm text-gray-700">
            {impact.adjusted.map((template) => (
              <li
                key={template.templateId}
                className="py-2 first:pt-0 last:pb-0"
              >
                <span className="font-medium text-gray-900">
                  {template.name}
                </span>
                : {describeAdjustment(template)}
              </li>
            ))}
          </ul>
        </SectionCard>
      ) : null}
      {impact.notAdjusted.length > 0 ? (
        <SectionCard
          title="Nicht angepasst, weil von Hand gepflegt"
          description="Hier steht das Kind weiter im Plan. Prüfen Sie, ob das so bleiben soll."
          headingLevel={5}
          titleClassName="text-sm font-medium text-gray-600"
          bare
          className="space-y-3"
        >
          <ManualTemplateList>
            {impact.notAdjusted.map((template) => (
              <ManualTemplateEntry
                key={template.templateId}
                template={template}
                removed={removed.has(template.templateId)}
                canRemove={canRemove}
                canConnect={canManage}
                onRemove={setTarget}
                planLink={planLink}
                connectLink={connectLink(template.templateId)}
              />
            ))}
          </ManualTemplateList>
          {actionMissing ? (
            <p className="text-xs text-gray-500">
              Ändern kann nur, wer den Betreuungsplan bearbeiten darf.
            </p>
          ) : null}
        </SectionCard>
      ) : null}
      <ConfirmDeleteModal
        isOpen={target !== null}
        title="Aus dem Regeltermin entfernen?"
        description={
          target ? (
            <p>
              Das Kind wird ab {stichtag}{" "}
              {formatWeekdayAdverbs(target.weekdays)} aus{" "}
              <span className="font-medium text-gray-900">„{target.name}“</span>{" "}
              entfernt. Das gilt auch für schon geplante Termine. Andere Tage
              und vergangene Termine bleiben.
            </p>
          ) : null
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Jetzt entfernen"
        loadingLabel="Wird entfernt …"
        onConfirm={confirmRemoval}
        onClose={closeConfirm}
        loading={removing}
        error={errors.error}
      />
    </SectionCard>
  );
}

/**
 * The Regeltermine with a dividing line between them, as in the list of
 * offerings without a Regeltermin (#5181). From `sm` on the actions sit in
 * one column of their own (the rows share it through `subgrid`), so every
 * row's buttons start at the same place. Below `sm` each row stacks.
 */
function ManualTemplateList({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <ul className="divide-y divide-gray-100 sm:grid sm:grid-cols-[minmax(0,1fr)_auto] sm:gap-x-4">
      {children}
    </ul>
  );
}

/**
 * One Regeltermin and its actions. Every action is the same kit button,
 * none the lesser one; its accessible name starts with the visible label
 * and adds the Regeltermin, so a screen reader's list tells the rows apart.
 */
function ManualTemplateEntry({
  template,
  removed,
  canRemove,
  canConnect,
  onRemove,
  planLink,
  connectLink,
}: Readonly<{
  template: ManualRosterTemplate;
  removed: boolean;
  canRemove: boolean;
  canConnect: boolean;
  onRemove: (template: ManualRosterTemplate) => void;
  planLink: string;
  connectLink: string;
}>) {
  const about = `„${template.name}“`;
  return (
    <li className="flex flex-col gap-2 py-3 text-sm text-gray-700 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
      <div className="min-w-0">
        <p>
          <span className="font-medium text-gray-900">{template.name}</span>,{" "}
          {formatWeekdayAdverbs(template.weekdays)}:{" "}
          {removed ? "Kind entfernt." : "Kind bleibt eingeplant."}
        </p>
        {template.viaTarget ? (
          <p className="mt-0.5 text-gray-600">
            Plant alle Kinder einer Klasse, eines Jahrgangs oder einer Gruppe.
            Bei der nächsten Planung steht das Kind wieder im Plan.
            {canConnect
              ? " Verbinden Sie den Regeltermin mit dem Angebot."
              : null}
          </p>
        ) : null}
      </div>
      <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row min-[480px]:items-center sm:justify-end">
        {removed ? (
          <span className="flex">
            <StatusBadge tone="green" label="Entfernt" />
          </span>
        ) : template.viaTarget ? (
          canConnect ? (
            <ButtonLink
              href={connectLink}
              variant="surface"
              size="compact"
              aria-label={`Mit Angebot verbinden: ${about}`}
            >
              Mit Angebot verbinden
            </ButtonLink>
          ) : null
        ) : canRemove ? (
          <Button
            type="button"
            variant="surface"
            size="compact"
            aria-label={`Dort auch entfernen: ${about}`}
            onClick={() => onRemove(template)}
          >
            Dort auch entfernen
          </Button>
        ) : null}
        <ButtonLink
          href={planLink}
          variant="surface"
          size="compact"
          aria-label={`Im Betreuungsplan ansehen: ${about}`}
        >
          Im Betreuungsplan ansehen
        </ButtonLink>
      </div>
    </li>
  );
}
