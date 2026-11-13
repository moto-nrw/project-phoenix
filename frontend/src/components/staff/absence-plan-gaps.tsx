"use client";

import { useCallback, useId, useMemo, useState } from "react";
import { CalendarX2, ChevronDown, ChevronUp } from "lucide-react";

import { Button } from "~/components/ui/button";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { ProgressBar } from "~/components/ui/progress-bar";
import {
  ResponsiveActionList,
  ResponsiveActionRow,
} from "~/components/ui/responsive-action-list";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { ABSENCE_TYPE_LABEL } from "~/lib/absence-helpers";
import { formatDate } from "~/lib/date-helpers";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { staffAbsenceService, type AbsencePlanGap } from "~/lib/staff-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableKeys } from "~/lib/swr/portal-keys";

const logger = createLogger({ component: "AbsencePlanGaps" });

/** Open rows shown before „N weitere anzeigen“, as in the other notice lists. */
const INITIAL_GAP_COUNT = 5;

const ROW_TOOLTIP =
  "Streicht die Schichten und trägt die Person in den Blöcken als abwesend ein.";

interface GapPerson {
  readonly firstName: string;
  readonly lastName: string;
}

/**
 * Abwesenheiten, die schon gelten, deren Tage aber noch im Plan stehen
 * (#7021): eingetragen oder genehmigt, bevor jede Abwesenheitsart in
 * Dienstplan und Betreuungsplan wirkte (#6847). Die Leitung übernimmt sie
 * einzeln oder alle auf einmal; von Hand geplante Tage (abgesagte Schichten,
 * Vertretungen, schon abwesende Blöcke) lässt die Übernahme stehen. Die Karte
 * folgt dem Muster „mehrere Hinweise derselben Art“ (frontend-ui-kit) und
 * verschwindet, sobald nichts mehr offen ist. „Alle übernehmen“ fragt vorher
 * nach und geht die Abwesenheiten nacheinander durch, je eine Anfrage, weil
 * der Server jede Abwesenheit hinter ihren eigenen Sperren schreibt.
 */
export function AbsencePlanGaps({
  staffById,
  onPlansChanged,
  onShowWeek,
}: Readonly<{
  /** Names of the staff list the Dienstplan loaded; the gaps carry ids. */
  staffById: ReadonlyMap<string, GapPerson>;
  /** Reloads the plan surfaces after a take-over. */
  onPlansChanged: () => void;
  /** Shows the week of a day in the Dienstplan, to check it first. */
  onShowWeek: (day: string) => void;
}>) {
  const toast = useToast();
  const { show: showActionError } = useApiErrorDisplay();
  const tenantMutate = useTenantMutate();
  const listId = useId();
  const [expanded, setExpanded] = useState(false);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const [progress, setProgress] = useState<number | null>(null);

  const { data, error } = useSWRAuth(timetableKeys.absencePlanGaps, () =>
    staffAbsenceService.listPlanGaps(),
  );
  const reload = useCallback(
    () => tenantMutate(timetableKeys.absencePlanGaps),
    [tenantMutate],
  );
  const loadError = useSwrLoadError(
    error,
    "die Prüfung der Abwesenheiten im Plan",
    reload,
  );

  const nameOf = useCallback(
    (gap: AbsencePlanGap) => {
      const person = staffById.get(gap.staffId);
      return person ? `${person.firstName} ${person.lastName}` : "Unbekannt";
    },
    [staffById],
  );

  const finish = useCallback(async () => {
    await reload();
    onPlansChanged();
  }, [onPlansChanged, reload]);

  const syncOne = useCallback(
    async (gap: AbsencePlanGap) => {
      setBusyId(gap.absenceId);
      try {
        const applied = await staffAbsenceService.syncPlan(gap.absenceId);
        toast.success(
          applied
            ? `Die Abwesenheit von ${nameOf(gap)} steht jetzt im Plan.`
            : `Die Abwesenheit von ${nameOf(gap)} gilt nicht mehr.`,
        );
      } catch (err) {
        logger.error("absence_plan_sync_failed", {
          absence_id: gap.absenceId,
          error: err instanceof Error ? err.message : String(err),
        });
        void showActionError(err, {
          object: `die Übernahme der Abwesenheit von ${nameOf(gap)}`,
        });
      } finally {
        setBusyId(null);
        await finish();
      }
    },
    [finish, nameOf, showActionError, toast],
  );

  // Die nächste Abwesenheit zuerst: sie braucht die Entscheidung am ehesten.
  const gaps = useMemo(
    () =>
      [...(data ?? [])].sort(
        (a, b) =>
          a.dateStart.localeCompare(b.dateStart) ||
          a.dateEnd.localeCompare(b.dateEnd) ||
          a.absenceId.localeCompare(b.absenceId),
      ),
    [data],
  );

  const syncAll = useCallback(async () => {
    const batch = [...gaps];
    let done = 0;
    setProgress(0);
    try {
      for (const gap of batch) {
        await staffAbsenceService.syncPlan(gap.absenceId);
        done += 1;
        setProgress(done);
      }
      toast.success(
        batch.length === 1
          ? "Die Abwesenheit steht jetzt im Plan."
          : `${batch.length} Abwesenheiten stehen jetzt im Plan.`,
      );
    } catch (err) {
      const failed = batch[done];
      logger.error("absence_plan_sync_all_failed", {
        absence_id: failed?.absenceId,
        done,
        total: batch.length,
        error: err instanceof Error ? err.message : String(err),
      });
      void showActionError(err, {
        object: failed
          ? `die Übernahme der Abwesenheit von ${nameOf(failed)}`
          : "die Übernahme der Abwesenheiten",
        messageSuffix:
          done > 0
            ? `${done} von ${batch.length} stehen schon im Plan.`
            : undefined,
      });
    } finally {
      setProgress(null);
      setConfirmOpen(false);
      await finish();
    }
  }, [finish, gaps, nameOf, showActionError, toast]);

  if (error) {
    return <LoadErrorAlert error={loadError} actionLayout="responsive" />;
  }
  if (gaps.length === 0) return null;

  const hiddenCount = Math.max(0, gaps.length - INITIAL_GAP_COUNT);
  const visible = expanded ? gaps : gaps.slice(0, INITIAL_GAP_COUNT);
  const totalShifts = gaps.reduce((sum, gap) => sum + gap.openShifts, 0);
  const totalBlocks = gaps.reduce((sum, gap) => sum + gap.openBlocks, 0);
  const running = progress !== null;

  return (
    <SectionCard
      testId="absence-plan-gaps"
      title="Abwesenheiten fehlen im Plan"
      titleBadge={<StatusBadge tone="warning" label={`${gaps.length} offen`} />}
      description="Diese Abwesenheiten gelten schon. Die Personen stehen aber noch im Dienstplan oder in Betreuungsblöcken."
      leading={
        <CalendarX2
          className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
          aria-hidden="true"
        />
      }
      action={
        gaps.length > 1 ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            tooltip="Übernimmt alle Abwesenheiten in den Plan. moto fragt vorher nach."
            onClick={() => setConfirmOpen(true)}
            disabled={busyId !== null}
            disabledReason="Warten Sie, bis die laufende Übernahme fertig ist."
          >
            Alle übernehmen
          </Button>
        ) : undefined
      }
    >
      {/* Einmal über der Liste: gilt für jede Zeile. */}
      <p className="mb-3 max-w-2xl text-sm text-gray-600">
        Von Hand geplante Tage bleiben, wie sie sind. Abgesagte Schichten und
        Vertretungen ändert moto nicht.
      </p>
      <ResponsiveActionList id={listId}>
        {visible.map((gap) => {
          const name = nameOf(gap);
          return (
            <ResponsiveActionRow key={gap.absenceId}>
              <div className="min-w-0">
                <p className="text-sm font-semibold text-gray-900">{name}</p>
                <p className="mt-0.5 text-sm text-gray-600">
                  {ABSENCE_TYPE_LABEL[gap.absenceType] ?? "Abwesenheit"} ·{" "}
                  {describeRange(gap.dateStart, gap.dateEnd)} · noch im Plan:{" "}
                  {describePlanRows(gap)}
                </p>
              </div>
              <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  aria-label={`Woche der Abwesenheit von ${name} ansehen`}
                  tooltip="Zeigt die erste Woche der Abwesenheit im Dienstplan."
                  onClick={() => onShowWeek(gap.dateStart)}
                >
                  Woche ansehen
                </Button>
                <Button
                  type="button"
                  variant="surface"
                  size="compact"
                  aria-label={`Abwesenheit von ${name} in den Plan übernehmen`}
                  tooltip={ROW_TOOLTIP}
                  isLoading={busyId === gap.absenceId}
                  loadingText="Wird übernommen …"
                  disabled={
                    running || (busyId !== null && busyId !== gap.absenceId)
                  }
                  disabledReason="Warten Sie, bis die laufende Übernahme fertig ist."
                  onClick={() => void syncOne(gap)}
                >
                  In den Plan übernehmen
                </Button>
              </div>
            </ResponsiveActionRow>
          );
        })}
      </ResponsiveActionList>
      {hiddenCount > 0 ? (
        <Button
          type="button"
          variant="ghost"
          size="md"
          className="mt-2 w-full"
          noTooltip="Klappt nur die Liste auf oder zu."
          aria-expanded={expanded}
          aria-controls={listId}
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
      {confirmOpen ? (
        <ConfirmationModal
          isOpen
          onClose={() => setConfirmOpen(false)}
          onConfirm={() => void syncAll()}
          title="Alle Abwesenheiten in den Plan übernehmen?"
          confirmText="Alle übernehmen"
          confirmTooltip="Streicht die Schichten und trägt die Personen in den Blöcken als abwesend ein."
          isConfirmLoading={running}
          loadingText="Wird übernommen …"
        >
          <div className="space-y-3 text-sm text-gray-700">
            <p>
              Das betrifft {gaps.length} Abwesenheiten:{" "}
              {describeCount(totalShifts, "Schicht", "Schichten")} und{" "}
              {describeCount(totalBlocks, "Block", "Blöcke")}.
            </p>
            <p>
              Von Hand geplante Tage bleiben, wie sie sind. Jede Änderung steht
              danach im Änderungsprotokoll.
            </p>
            {running ? (
              <div className="space-y-1.5">
                <ProgressBar
                  value={progress ?? 0}
                  max={gaps.length}
                  label="Fortschritt der Übernahme"
                />
                <p className="text-gray-600" aria-live="polite">
                  {progress} von {gaps.length} übernommen
                </p>
              </div>
            ) : null}
          </div>
        </ConfirmationModal>
      ) : null}
    </SectionCard>
  );
}

function describeCount(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

/** „4 Schichten und 6 Blöcke“; a zero part is left out. */
export function describePlanRows(gap: AbsencePlanGap): string {
  const parts: string[] = [];
  if (gap.openShifts > 0) {
    parts.push(describeCount(gap.openShifts, "Schicht", "Schichten"));
  }
  if (gap.openBlocks > 0) {
    parts.push(describeCount(gap.openBlocks, "Block", "Blöcke"));
  }
  return parts.join(" und ");
}

function describeRange(start: string, end: string): string {
  if (start === end) return formatDate(start);
  return `${formatDate(start)} bis ${formatDate(end)}`;
}
