"use client";

import { type ReactNode, useCallback, useId, useMemo, useState } from "react";
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
import { useLocalStorageValue } from "~/lib/hooks/use-local-storage-value";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import { staffAbsenceService, type AbsencePlanGap } from "~/lib/staff-api";
import { useSWRAuth, useTenantMutate } from "~/lib/swr";
import { timetableKeys } from "~/lib/swr/portal-keys";
import { useTenantSlugSafe } from "~/lib/tenant-context";

const logger = createLogger({ component: "AbsencePlanGaps" });

/** Open rows shown before „N weitere anzeigen“, as in the other notice lists. */
const INITIAL_GAP_COUNT = 5;

const SYNC_TOOLTIP =
  "Streicht die Schichten und trägt die Abwesenheit in den Blöcken ein.";
const BUSY_REASON = "Warten Sie, bis die laufende Übernahme fertig ist.";

interface GapPerson {
  readonly firstName: string;
  readonly lastName: string;
}

/**
 * Abwesenheiten, die schon gelten, deren Tage aber noch im Plan stehen
 * (#7021): eingetragen oder genehmigt, bevor jede Abwesenheitsart in
 * Dienstplan und Betreuungsplan wirkte (#6847). Die Leitung übernimmt sie
 * einzeln oder alle auf einmal; abgesagte Schichten, Vertretungen und schon
 * abwesende Blöcke lässt die Übernahme stehen. Soll eine Person an einem
 * Abwesenheitstag doch arbeiten, blendet die Leitung die Abwesenheit aus:
 * sonst stünde sie hier, solange sie gilt. Ausgeblendetes merkt sich der
 * Browser je Schule (eine Bequemlichkeit, kein Datensatz); es wartet in einer
 * eingeklappten Karte und kommt von dort zurück. Die Karten folgen dem Muster
 * „mehrere Hinweise derselben Art“ (frontend-ui-kit). „Alle übernehmen“ fragt
 * vorher nach und geht die gezeigten Abwesenheiten nacheinander durch, je eine
 * Anfrage, weil der Server jede hinter ihren eigenen Sperren schreibt.
 */
export function AbsencePlanGaps({
  enabled,
  staffById,
  onPlansChanged,
  onShowWeek,
}: Readonly<{
  /** Only a reader who may plan shifts loads the list. */
  enabled: boolean;
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
  const [hiddenIds, setHiddenIds] = useHiddenGapIds();

  const { data, error } = useSWRAuth(
    enabled ? timetableKeys.absencePlanGaps : null,
    () => staffAbsenceService.listPlanGaps(),
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
  const open = useMemo(
    () => gaps.filter((gap) => !hiddenIds.has(gap.absenceId)),
    [gaps, hiddenIds],
  );
  const hidden = useMemo(
    () => gaps.filter((gap) => hiddenIds.has(gap.absenceId)),
    [gaps, hiddenIds],
  );

  const finish = useCallback(async () => {
    await reload();
    onPlansChanged();
  }, [onPlansChanged, reload]);

  const setHidden = useCallback(
    (gap: AbsencePlanGap, hide: boolean) => {
      const next = new Set(hiddenIds);
      if (hide) next.add(gap.absenceId);
      else next.delete(gap.absenceId);
      // Nur die Abwesenheiten merken, die die Liste noch kennt.
      setHiddenIds(
        new Set(gaps.map((g) => g.absenceId).filter((id) => next.has(id))),
      );
    },
    [gaps, hiddenIds, setHiddenIds],
  );

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

  const syncAll = useCallback(async () => {
    const batch = [...open];
    let done = 0;
    let applied = 0;
    setProgress(0);
    try {
      for (const gap of batch) {
        if (await staffAbsenceService.syncPlan(gap.absenceId)) applied += 1;
        done += 1;
        setProgress(done);
      }
      toast.success(describeSyncResult(applied, batch.length - applied));
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
  }, [finish, nameOf, open, showActionError, toast]);

  if (error) {
    return <LoadErrorAlert error={loadError} actionLayout="responsive" />;
  }
  if (gaps.length === 0) return null;

  const hiddenCount = Math.max(0, open.length - INITIAL_GAP_COUNT);
  const visible = expanded ? open : open.slice(0, INITIAL_GAP_COUNT);
  const totalShifts = open.reduce((sum, gap) => sum + gap.openShifts, 0);
  const totalBlocks = open.reduce((sum, gap) => sum + gap.openBlocks, 0);
  const running = progress !== null;
  const blocked = running || busyId !== null;

  return (
    <div className="flex flex-col gap-3">
      {open.length > 0 ? (
        <SectionCard
          testId="absence-plan-gaps"
          title="Abwesenheiten fehlen im Plan"
          titleBadge={
            <StatusBadge tone="warning" label={`${open.length} offen`} />
          }
          description="Diese Abwesenheiten gelten schon. Die Personen stehen aber noch im Dienstplan oder in Betreuungsblöcken."
          leading={
            <CalendarX2
              className="text-moto-orange-strong mt-0.5 size-5 shrink-0"
              aria-hidden="true"
            />
          }
          action={
            open.length > 1 ? (
              <Button
                type="button"
                variant="primary"
                size="md"
                tooltip="Übernimmt alle gezeigten Abwesenheiten in den Plan. moto fragt vorher nach."
                onClick={() => setConfirmOpen(true)}
                disabled={blocked}
                disabledReason={BUSY_REASON}
              >
                Alle übernehmen
              </Button>
            ) : undefined
          }
        >
          {/* Einmal über der Liste: gilt für jede Zeile. */}
          <p className="mb-3 max-w-2xl text-sm text-gray-600">
            Abgesagte Schichten und Vertretungen bleiben, wie sie sind. Soll
            eine Person an diesen Tagen doch arbeiten, blenden Sie die
            Abwesenheit aus.
          </p>
          <ResponsiveActionList id={listId}>
            {visible.map((gap) => {
              const name = nameOf(gap);
              return (
                <GapRow key={gap.absenceId} gap={gap} name={name}>
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
                    aria-label={`Abwesenheit von ${name} ausblenden`}
                    tooltip="Lässt den Plan, wie er ist, und blendet die Zeile aus."
                    disabled={blocked}
                    disabledReason={BUSY_REASON}
                    onClick={() => setHidden(gap, true)}
                  >
                    Ausblenden
                  </Button>
                  <Button
                    type="button"
                    variant="surface"
                    size="compact"
                    aria-label={`Abwesenheit von ${name} in den Plan übernehmen`}
                    tooltip={SYNC_TOOLTIP}
                    isLoading={busyId === gap.absenceId}
                    loadingText="Wird übernommen …"
                    disabled={
                      running || (busyId !== null && busyId !== gap.absenceId)
                    }
                    disabledReason={BUSY_REASON}
                    onClick={() => void syncOne(gap)}
                  >
                    In den Plan übernehmen
                  </Button>
                </GapRow>
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
        </SectionCard>
      ) : null}
      {hidden.length > 0 ? (
        <SectionCard
          testId="absence-plan-gaps-hidden"
          title={`Ausgeblendete Abwesenheiten (${hidden.length})`}
          description="Der Plan bleibt hier, wie er ist. Diese Liste merkt sich nur dieser Browser."
          collapsible
          defaultCollapsed
          headingLevel={3}
        >
          <ResponsiveActionList>
            {hidden.map((gap) => {
              const name = nameOf(gap);
              return (
                <GapRow key={gap.absenceId} gap={gap} name={name}>
                  <Button
                    type="button"
                    variant="surface"
                    size="compact"
                    aria-label={`Abwesenheit von ${name} wieder zeigen`}
                    tooltip="Zeigt die Abwesenheit wieder in der Liste oben."
                    onClick={() => setHidden(gap, false)}
                  >
                    Wieder zeigen
                  </Button>
                </GapRow>
              );
            })}
          </ResponsiveActionList>
        </SectionCard>
      ) : null}
      {confirmOpen ? (
        <ConfirmationModal
          isOpen
          onClose={() => setConfirmOpen(false)}
          onConfirm={() => void syncAll()}
          title="Alle Abwesenheiten in den Plan übernehmen?"
          confirmText="Alle übernehmen"
          confirmTooltip={SYNC_TOOLTIP}
          isConfirmLoading={running}
          loadingText="Wird übernommen …"
        >
          <div className="space-y-3 text-sm text-gray-700">
            <p>
              Das betrifft{" "}
              {describeCount(open.length, "Abwesenheit", "Abwesenheiten")}:{" "}
              {describeCount(totalShifts, "Schicht", "Schichten")} und{" "}
              {describeCount(totalBlocks, "Block", "Blöcke")}.
            </p>
            <p>
              Abgesagte Schichten und Vertretungen bleiben. Jede Änderung steht
              danach im Änderungsprotokoll.
            </p>
            {running ? (
              <div className="space-y-1.5">
                <ProgressBar
                  value={progress ?? 0}
                  max={open.length}
                  label="Fortschritt der Übernahme"
                />
                <p className="text-gray-600" aria-live="polite">
                  {progress} von {open.length} übernommen
                </p>
              </div>
            ) : null}
          </div>
        </ConfirmationModal>
      ) : null}
    </div>
  );
}

/** One absence: who, which art, which days and what still stands in the plan. */
function GapRow({
  gap,
  name,
  children,
}: Readonly<{ gap: AbsencePlanGap; name: string; children: ReactNode }>) {
  return (
    <ResponsiveActionRow>
      <div className="min-w-0">
        <p className="text-sm font-semibold text-gray-900">{name}</p>
        <p className="mt-0.5 text-sm text-gray-600">
          {ABSENCE_TYPE_LABEL[gap.absenceType] ?? "Abwesenheit"} ·{" "}
          {describeRange(gap.dateStart, gap.dateEnd)} · noch im Plan:{" "}
          {describePlanRows(gap)}
        </p>
      </div>
      <div className="flex w-full flex-col gap-2 min-[480px]:w-auto min-[480px]:flex-row sm:justify-end">
        {children}
      </div>
    </ResponsiveActionRow>
  );
}

/**
 * The absences hidden in this browser, per school. Reading and writing
 * storage may fail (private window, blocked site data): the list then simply
 * forgets, nothing breaks.
 */
function useHiddenGapIds(): [ReadonlySet<string>, (ids: Set<string>) => void] {
  const slug = useTenantSlugSafe();
  const key = `moto:absence-plan-gaps:hidden:${slug ?? "-"}`;
  const stored = useLocalStorageValue(key, slug !== null);
  const [local, setLocal] = useState<ReadonlySet<string> | null>(null);
  const fromStorage = useMemo(() => parseIds(stored), [stored]);
  const write = useCallback(
    (ids: Set<string>) => {
      setLocal(ids);
      try {
        if (ids.size === 0) globalThis.localStorage.removeItem(key);
        else globalThis.localStorage.setItem(key, JSON.stringify([...ids]));
      } catch {
        // Storage blocked: the choice holds until the page reloads.
      }
    },
    [key],
  );
  return [local ?? fromStorage, write];
}

function parseIds(raw: string | null): ReadonlySet<string> {
  if (!raw) return new Set();
  try {
    const parsed: unknown = JSON.parse(raw);
    return new Set(
      Array.isArray(parsed)
        ? parsed.filter((id): id is string => typeof id === "string")
        : [],
    );
  } catch {
    return new Set();
  }
}

function describeCount(count: number, one: string, many: string): string {
  return `${count} ${count === 1 ? one : many}`;
}

function describeSyncResult(applied: number, skipped: number): string {
  const head =
    applied === 1
      ? "1 Abwesenheit steht jetzt im Plan."
      : `${applied} Abwesenheiten stehen jetzt im Plan.`;
  if (skipped === 0) return head;
  return skipped === 1
    ? `${head} 1 gilt nicht mehr.`
    : `${head} ${skipped} gelten nicht mehr.`;
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
