"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { SectionCard } from "~/components/ui/section-card";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  describeGapClasses,
  describeOfferingGap,
  type OfferingGap,
} from "~/lib/offering-gap";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "OfferingGapHints" });

export function offeringGapsKey(periodId: string): string {
  return `offering-gaps-${periodId}`;
}

/**
 * Offerings whose booked children no Regeltermin of the period takes over
 * (#5140), above the Regeltermine. Planners create a Regeltermin or decide
 * that these children need none; a decided offering moves into the folded
 * list below, from where the hint can come back. Readers without planning
 * rights only see the open hints. The hint is information next to the plan:
 * a failed read shows nothing and is logged. Every reload of the template
 * list reloads the hints too, so a new Regeltermin shows at once.
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
  );

  useEffect(() => {
    if (!error) return;
    logger.warn("offering_gaps_load_failed", {
      period_id: periodId,
      error: error instanceof Error ? error.message : String(error),
    });
  }, [error, periodId]);

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
  if (open.length === 0 && decided.length === 0) return null;

  return (
    <div className="flex flex-col gap-3" data-testid="offering-gap-hints">
      {open.map((gap) => (
        <Alert
          key={gap.offering.id}
          type="warning"
          // A hint in a list, not an event: it must not interrupt screen readers.
          announce="off"
          message={describeOfferingGap(gap)}
          actionLayout="stacked"
          action={
            canManage ? (
              <span className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  onClick={onCreateSeries}
                >
                  Regeltermin anlegen
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  onClick={() => void decide(gap, true)}
                  disabled={busyOfferingId === gap.offering.id}
                >
                  Kein Regeltermin nötig
                </Button>
              </span>
            ) : undefined
          }
        />
      ))}
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
              <li
                key={gap.offering.id}
                className="flex flex-wrap items-center justify-between gap-2 text-sm text-gray-700"
              >
                <span>
                  „{gap.offering.name}“ ·{" "}
                  {describeGapClasses(gap.schoolClasses)}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  onClick={() => void decide(gap, false)}
                  disabled={busyOfferingId === gap.offering.id}
                >
                  Hinweis wieder zeigen
                </Button>
              </li>
            ))}
          </ul>
        </SectionCard>
      ) : null}
    </div>
  );
}
