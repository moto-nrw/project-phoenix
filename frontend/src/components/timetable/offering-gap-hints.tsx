"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";
import { DecisionActions } from "~/components/timetable/decision-hint";
import { Tooltip } from "~/components/ui/tooltip";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  describeGapChildren,
  describeGapWeekdays,
  summarizeGapClasses,
  type OfferingGap,
} from "~/lib/offering-gap";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "OfferingGapHints" });

function offeringGapsKey(periodId: string): string {
  return `offering-gaps-${periodId}`;
}

/**
 * Offerings whose booked children no Regeltermin of the period takes over
 * (#5140): one warning above the Regeltermine with a short row per offering
 * and the two equal decisions (DecisionActions, shared with the roster
 * review). Decided offerings wait in a folded card, from where the hint can
 * come back. Readers without planning
 * rights only see the open hints. A failed check shows the load error with a
 * retry, never an empty list: no hint must not read as "no gaps". Every
 * reload of the template list reloads the hints too, so a new Regeltermin
 * shows at once.
 */
export function OfferingGapHints({
  periodId,
  canManage,
  onCreateSeries,
  templatesVersion,
}: Readonly<{
  periodId: string | undefined;
  canManage: boolean;
  onCreateSeries: () => void;
  templatesVersion: unknown;
}>) {
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const tenantMutate = useTenantMutate();
  const [busyOfferingId, setBusyOfferingId] = useState<string | null>(null);
  const { data, error } = useSWRAuth(
    periodId ? offeringGapsKey(periodId) : null,
    () => timetableService.getOfferingGaps(periodId!),
    { keepPreviousData: false },
  );

  const loadError = useSwrLoadError(
    error,
    "die Prüfung der Angebote ohne Regeltermin",
    () => periodId && tenantMutate(offeringGapsKey(periodId)),
  );

  const seenVersion = useRef(templatesVersion);
  useEffect(() => {
    if (seenVersion.current === templatesVersion) return;
    const first = seenVersion.current === undefined;
    seenVersion.current = templatesVersion;
    // The first template load runs next to the first hint load already.
    if (!first && templatesVersion !== undefined && periodId) {
      void tenantMutate(offeringGapsKey(periodId));
    }
  }, [templatesVersion, periodId, tenantMutate]);

  const decide = useCallback(
    async (gap: OfferingGap, acknowledged: boolean) => {
      if (!periodId) return;
      setBusyOfferingId(gap.offering.id);
      try {
        await timetableService.setOfferingGapAcknowledged(
          gap.offering.id,
          periodId,
          acknowledged,
        );
        toast.success(
          acknowledged
            ? `Der Hinweis zu „${gap.offering.name}“ ist ausgeblendet.`
            : `Der Hinweis zu „${gap.offering.name}“ wird wieder gezeigt.`,
        );
        await tenantMutate(offeringGapsKey(periodId));
      } catch (err) {
        logger.error("offering_gap_decision_failed", {
          offering_id: gap.offering.id,
          acknowledged,
          error: err instanceof Error ? err.message : String(err),
        });
        void showActionError(err, {
          object: acknowledged
            ? "das Ausblenden des Hinweises"
            : "das Einblenden des Hinweises",
        });
      } finally {
        setBusyOfferingId(null);
      }
    },
    [periodId, showActionError, tenantMutate, toast],
  );

  const gaps = data ?? [];
  const open = gaps.filter((gap) => !gap.acknowledged);
  const decided = canManage ? gaps.filter((gap) => gap.acknowledged) : [];
  if (error) return <LoadErrorAlert error={loadError} />;
  if (open.length === 0 && decided.length === 0) return null;

  return (
    <div className="flex flex-col gap-3" data-testid="offering-gap-hints">
      {open.length > 0 ? (
        <Alert
          type="warning"
          // A hint in a list, not an event: it must not interrupt screen readers.
          announce="off"
          title={
            open.length === 1
              ? "1 Angebot ohne Regeltermin"
              : `${open.length} Angebote ohne Regeltermin`
          }
          message="Diese Kinder haben ein Angebot gebucht, aber kein Regeltermin übernimmt sie."
          details={
            <ul className="mt-2 flex flex-col gap-2">
              {open.map((gap) => (
                <OfferingGapRow key={gap.offering.id} gap={gap}>
                  {canManage ? (
                    <DecisionActions
                      primary={{
                        label: "Regeltermin anlegen",
                        onClick: onCreateSeries,
                      }}
                      secondary={{
                        label: "Kein Regeltermin nötig",
                        onClick: () => void decide(gap, true),
                      }}
                      busy={busyOfferingId === gap.offering.id}
                    />
                  ) : null}
                </OfferingGapRow>
              ))}
            </ul>
          }
        />
      ) : null}
      {decided.length > 0 ? (
        <SectionCard
          title={`Ohne eigenen Regeltermin (${decided.length})`}
          description="Sie haben entschieden: Diese Kinder brauchen keinen eigenen Regeltermin."
          collapsible
          defaultCollapsed
          headingLevel={3}
        >
          <ul className="flex flex-col gap-2">
            {decided.map((gap) => (
              <OfferingGapRow key={gap.offering.id} gap={gap}>
                <DecisionActions
                  primary={{
                    label: "Hinweis wieder zeigen",
                    onClick: () => void decide(gap, false),
                  }}
                  busy={busyOfferingId === gap.offering.id}
                />
              </OfferingGapRow>
            ))}
          </ul>
        </SectionCard>
      ) : null}
    </div>
  );
}

/**
 * One offering: its name, the short facts (children, classes, days) and the
 * actions. A long class list sits in the kit tooltip, so the row stays one
 * line on a desktop.
 */
function OfferingGapRow({
  gap,
  children,
}: Readonly<{
  gap: OfferingGap;
  children?: ReactNode;
}>) {
  const classes = summarizeGapClasses(gap.schoolClasses);
  return (
    <li className="flex flex-wrap items-center justify-between gap-x-4 gap-y-1">
      <div className="min-w-0">
        <span className="font-medium">„{gap.offering.name}“</span>{" "}
        <span>
          {describeGapChildren(gap.studentCount)} ·{" "}
          {classes.full === classes.short ? (
            classes.short
          ) : (
            <Tooltip content={classes.full}>{classes.short}</Tooltip>
          )}{" "}
          · {describeGapWeekdays(gap.weekdays)}
        </span>
      </div>
      {children}
    </li>
  );
}
