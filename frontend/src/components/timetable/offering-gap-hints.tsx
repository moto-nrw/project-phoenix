"use client";

import {
  type ReactNode,
  useCallback,
  useEffect,
  useId,
  useRef,
  useState,
} from "react";
import { ChevronDown, ChevronUp, TriangleAlert } from "lucide-react";

import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ResponsiveActionList } from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { Tooltip } from "~/components/ui/tooltip";
import { PhasesOutsidePeriodNotice } from "~/components/timetable/phases-outside-period-notice";
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
import { timetableKeys } from "~/lib/swr/portal-keys";
import { timetableService } from "~/lib/timetable-api";

const logger = createLogger({ component: "OfferingGapHints" });

/** Open rows shown before „N weitere anzeigen“, as in the parents' to-do list. */
const INITIAL_GAP_COUNT = 5;

/**
 * Offerings whose booked children no Regeltermin of the period takes over
 * (#5140): one collected hint above the Regeltermine, a list in a neutral
 * card with one row per offering and its two equal decisions, the way the
 * staff inbox lists absence requests. A row offers „Regeltermin anlegen“
 * only when the new Regeltermin would plan no child twice (#5343); the
 * reason stands once above the list while such a row is shown (#5390).
 * Only the icon and the count badge carry the warning colour. Decided offerings wait in a folded card with
 * the same rows, from where the hint can come back. Readers without planning
 * rights only see the open hints. A failed check shows the load error with a
 * retry, never an empty list: no hint must not read as "no gaps". Every
 * reload of the template list reloads the hints too, so a new Regeltermin
 * shows at once. Phases that reach beyond the period are named above the
 * list, since their offerings are not checked here at all (#5344).
 */
export function OfferingGapHints({
  periodId,
  canManage,
  canManagePhases,
  onCreateSeries,
  templatesVersion,
}: Readonly<{
  periodId: string | undefined;
  canManage: boolean;
  /** May open the Anmeldephasen to align a phase with the period. */
  canManagePhases: boolean;
  /** Opens a new Regeltermin fed by the offering of the row. */
  onCreateSeries: (gap: OfferingGap) => void;
  templatesVersion: unknown;
}>) {
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const tenantMutate = useTenantMutate();
  const [busyOfferingId, setBusyOfferingId] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(false);
  const openListId = useId();
  const { data, error } = useSWRAuth(
    periodId ? timetableKeys.offeringGaps(periodId) : null,
    () => timetableService.getOfferingGaps(periodId!),
    { keepPreviousData: false },
  );

  const loadError = useSwrLoadError(
    error,
    "die Prüfung der Angebote ohne Regeltermin",
    () => periodId && tenantMutate(timetableKeys.offeringGaps(periodId)),
  );

  const seenVersion = useRef(templatesVersion);
  useEffect(() => {
    if (seenVersion.current === templatesVersion) return;
    const first = seenVersion.current === undefined;
    seenVersion.current = templatesVersion;
    // The first template load runs next to the first hint load already.
    if (!first && templatesVersion !== undefined && periodId) {
      void tenantMutate(timetableKeys.offeringGaps(periodId));
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
        await tenantMutate(timetableKeys.offeringGaps(periodId));
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

  const gaps = data?.gaps ?? [];
  const outside = data?.phasesOutsidePeriod ?? [];
  const open = gaps.filter((gap) => !gap.acknowledged);
  const decided = canManage ? gaps.filter((gap) => gap.acknowledged) : [];
  if (error) {
    return <LoadErrorAlert error={loadError} actionLayout="responsive" />;
  }
  if (open.length === 0 && decided.length === 0 && outside.length === 0) {
    return null;
  }

  const hiddenCount = Math.max(0, open.length - INITIAL_GAP_COUNT);
  const visible = expanded ? open : open.slice(0, INITIAL_GAP_COUNT);
  // Only rows on screen count: a folded row without the button must not
  // explain a gap the reader cannot see (#5390).
  const createWithheld =
    canManage && visible.some((gap) => gap.newSeriesPlansCovered);

  return (
    <div className="flex flex-col gap-3" data-testid="offering-gap-hints">
      <PhasesOutsidePeriodNotice
        phases={outside}
        context="checks"
        canManagePhases={canManagePhases}
      />
      {open.length > 0 ? (
        <SectionCard
          title="Angebote ohne Regeltermin"
          titleBadge={
            <StatusBadge tone="orange" label={`${open.length} offen`} />
          }
          description="Diese Kinder haben ein Angebot gebucht. An den genannten Tagen steht kein passender Regeltermin für sie im Plan."
          leading={
            <TriangleAlert
              className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
              aria-hidden="true"
            />
          }
        >
          {createWithheld ? (
            // Einmal über der Liste, auch bei nur einer solchen Zeile.
            <p className="mb-3 max-w-2xl text-sm text-gray-600">
              Fehlt „Regeltermin anlegen“? Dann sind einige Kinder an diesen
              Tagen schon ganz oder teilweise eingeplant. Oder sie haben noch
              Unterricht. Prüfen Sie ihre Regeltermine und ergänzen Sie, was
              fehlt.
            </p>
          ) : null}
          <ResponsiveActionList id={openListId}>
            {visible.map((gap) => (
              <OfferingGapRow key={gap.offering.id} gap={gap}>
                {canManage ? (
                  <>
                    {gap.newSeriesPlansCovered ? null : (
                      <Button
                        type="button"
                        variant="surface"
                        size="compact"
                        aria-label={`Regeltermin anlegen: „${gap.offering.name}“`}
                        onClick={() => onCreateSeries(gap)}
                        disabled={busyOfferingId === gap.offering.id}
                      >
                        Regeltermin anlegen
                      </Button>
                    )}
                    <Button
                      type="button"
                      variant="surface"
                      size="compact"
                      aria-label={`Kein Regeltermin nötig: „${gap.offering.name}“`}
                      onClick={() => void decide(gap, true)}
                      disabled={busyOfferingId === gap.offering.id}
                    >
                      Kein Regeltermin nötig
                    </Button>
                  </>
                ) : null}
              </OfferingGapRow>
            ))}
          </ResponsiveActionList>
          {hiddenCount > 0 ? (
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="mt-2 min-h-11 w-full px-3 text-sm font-medium"
              aria-expanded={expanded}
              aria-controls={openListId}
              onClick={() => setExpanded((current) => !current)}
            >
              {expanded ? (
                <>
                  Weniger anzeigen
                  <ChevronUp className="size-4" aria-hidden="true" />
                </>
              ) : (
                <>
                  {hiddenCount} weitere anzeigen
                  <ChevronDown className="size-4" aria-hidden="true" />
                </>
              )}
            </Button>
          ) : null}
        </SectionCard>
      ) : null}
      {decided.length > 0 ? (
        <SectionCard
          title={`Ohne eigenen Regeltermin (${decided.length})`}
          description="Sie haben entschieden: Diese Kinder brauchen keinen eigenen Regeltermin."
          collapsible
          defaultCollapsed
          headingLevel={3}
        >
          <ResponsiveActionList>
            {decided.map((gap) => (
              <OfferingGapRow key={gap.offering.id} gap={gap}>
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  aria-label={`Hinweis wieder zeigen: „${gap.offering.name}“`}
                  onClick={() => void decide(gap, false)}
                  disabled={busyOfferingId === gap.offering.id}
                >
                  Hinweis wieder zeigen
                </Button>
              </OfferingGapRow>
            ))}
          </ResponsiveActionList>
        </SectionCard>
      ) : null}
    </div>
  );
}

/**
 * One offering: its name, the short facts (children, classes, days) and the
 * actions. A long class list sits in the kit tooltip, so the facts stay one
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
    <li className="flex flex-col gap-2 py-3 first:pt-0 last:pb-0 sm:col-span-2 sm:grid sm:grid-cols-subgrid sm:items-center">
      <div className="min-w-0">
        <p className="text-sm font-semibold text-gray-900">
          „{gap.offering.name}“
        </p>
        <p className="mt-0.5 text-sm text-gray-600">
          {describeGapChildren(gap.studentCount)} ·{" "}
          {classes.full === classes.short ? (
            classes.short
          ) : (
            <Tooltip content={classes.full}>{classes.short}</Tooltip>
          )}{" "}
          · {describeGapWeekdays(gap.weekdays)}
        </p>
      </div>
      {children ? (
        <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
          {children}
        </div>
      ) : null}
    </li>
  );
}
