"use client";

import { useId, useState } from "react";
import { useSession } from "next-auth/react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import NavigationLink from "~/components/ui/navigation-link";
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
 * Regeltermine das Kind an weggefallenen Tagen weiter einplanen.
 */
export function CarePlanImpact({
  impact,
  studentId,
  childName,
}: CarePlanImpactProps) {
  const headingId = useId();
  const { data: session } = useSession();
  const canRemove =
    studentId !== undefined && hasPermission(session, "schedules:manage");
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
    <section
      aria-labelledby={headingId}
      className="flex flex-col gap-3 border-t border-gray-100 pt-3"
    >
      <h4 id={headingId} className="text-sm font-semibold text-gray-900">
        Auswirkung auf den Betreuungsplan{childName ? ` für ${childName}` : ""}{" "}
        ab {stichtag}
      </h4>
      {impact.adjusted.length > 0 ? (
        <div>
          <h5 className="text-xs font-medium text-gray-500">
            Automatisch angepasst
          </h5>
          <ul className="mt-1 flex flex-col gap-1 text-sm text-gray-700">
            {impact.adjusted.map((template) => (
              <li key={template.templateId}>
                <span className="font-medium text-gray-900">
                  {template.name}
                </span>
                : {describeAdjustment(template)}
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {impact.notAdjusted.length > 0 ? (
        <div>
          <h5 className="text-xs font-medium text-gray-500">
            Nicht angepasst, weil von Hand gepflegt
          </h5>
          <p className="mt-1 text-sm text-gray-600">
            Hier steht das Kind weiter im Plan. Prüfen Sie, ob das so bleiben
            soll.
          </p>
          <ul className="mt-2 flex flex-col divide-y divide-gray-100">
            {impact.notAdjusted.map((template) => (
              <ManualTemplateEntry
                key={template.templateId}
                template={template}
                removed={removed.has(template.templateId)}
                canRemove={canRemove}
                onRemove={setTarget}
                planLink={planLink}
              />
            ))}
          </ul>
          {canRemove || studentId === undefined ? null : (
            <p className="mt-2 text-xs text-gray-500">
              Entfernen kann nur, wer den Betreuungsplan bearbeiten darf.
            </p>
          )}
        </div>
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
        warningSlot={
          target?.viaTarget ? (
            <Alert
              type="warning"
              message="Bei einer neuen Planung kommt das Kind wieder dazu. Der Regeltermin plant alle Kinder der Klasse, des Jahrgangs oder der Gruppe. Soll es dauerhaft fehlen, ändern Sie die Kinder im Regeltermin."
            />
          ) : undefined
        }
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Jetzt entfernen"
        loadingLabel="Wird entfernt …"
        onConfirm={confirmRemoval}
        onClose={closeConfirm}
        loading={removing}
        error={errors.error}
      />
    </section>
  );
}

function ManualTemplateEntry({
  template,
  removed,
  canRemove,
  onRemove,
  planLink,
}: Readonly<{
  template: ManualRosterTemplate;
  removed: boolean;
  canRemove: boolean;
  onRemove: (template: ManualRosterTemplate) => void;
  planLink: string;
}>) {
  return (
    <li className="flex flex-col gap-1.5 py-2 text-sm text-gray-700 first:pt-0 last:pb-0">
      <span>
        <span className="font-medium text-gray-900">{template.name}</span>,{" "}
        {formatWeekdayAdverbs(template.weekdays)}:{" "}
        {removed ? "Kind entfernt." : "Kind bleibt eingeplant."}
      </span>
      <span className="flex flex-wrap items-center gap-x-4 gap-y-2">
        {removed ? (
          <StatusBadge tone="green" label="Entfernt" />
        ) : canRemove ? (
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => onRemove(template)}
          >
            Dort auch entfernen
          </Button>
        ) : null}
        <NavigationLink
          href={planLink}
          className="text-sm font-medium text-gray-700 underline underline-offset-2 hover:text-gray-900"
        >
          Im Betreuungsplan ansehen
        </NavigationLink>
      </span>
    </li>
  );
}
