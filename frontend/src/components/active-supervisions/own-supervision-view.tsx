"use client";

import { useCallback, useMemo } from "react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import { AddUnplannedStudentModal } from "~/components/active-supervisions/add-unplanned-student-modal";
import { CompleteInstanceModal } from "~/components/active-supervisions/complete-instance-modal";
import { PlannedNowSection } from "~/components/active-supervisions/planned-now-section";
import { TimetableRosterContent } from "~/components/active-supervisions/timetable-roster";
import {
  reopenBannerMessage,
  useReopenBanner,
} from "~/components/active-supervisions/use-reopen-banner";
import { useTimetableActions } from "~/components/active-supervisions/use-timetable-actions";
import { useTimetableRoster } from "~/components/active-supervisions/use-timetable-roster";
import { rosterRunState } from "~/components/active-supervisions/view-model";
import type { ActiveGroup } from "~/lib/usercontext-helpers";
import { userContextService } from "~/lib/usercontext-api";
import { formatChildCount } from "~/lib/activity-occupancy";
import { BELOW_LG, useMediaQuery } from "~/lib/hooks/use-media-query";
import { hasPermission } from "~/lib/auth-utils";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { useSWRAuth } from "~/lib/swr";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import { timetableOperationsApi } from "~/lib/timetable-operations-api";
import type { PlannedTimetableInstance } from "~/lib/timetable-operations-types";
import { useTenantRouter } from "~/lib/tenant-router";

const NO_ROOMS = [] as const;
const LIST_REFRESH_MS = 60 * 1000;

function sessionHref(activeGroupId: string): string {
  return `/active-supervisions?session=${activeGroupId}`;
}

/** Eigene laufende Blöcke und tatsächlich übernommene zusätzliche Aufsichten. */
function ownRunningBlocks(
  day: readonly PlannedTimetableInstance[],
  sessions: readonly ActiveGroup[],
): {
  id: string | null;
  title: string;
  activeGroupId?: string | null;
}[] {
  // Der eigene Endpunkt liefert ausschließlich offene, selbst beaufsichtigte Sessions.
  const ownSessions = sessions;
  const ownIds = new Set(ownSessions.map((session) => session.id));
  const planned = day
    .filter(
      (block) =>
        block.status === "active" &&
        block.activeGroupId != null &&
        (block.isAssigned || ownIds.has(block.activeGroupId)),
    )
    .sort((a, b) => a.startTime.localeCompare(b.startTime));
  const plannedIds = new Set(planned.map((block) => block.activeGroupId));
  return [
    ...planned,
    ...ownSessions
      .filter((session) => !plannedIds.has(session.id))
      .map((session) => ({
        id: null,
        title: session.name ?? session.room?.name ?? "Aktuelle Aufsicht",
        activeGroupId: session.id,
      })),
  ];
}

/**
 * „Aktuelle Aufsicht“ ohne `groups:read` (#4573, AG-Leitung): nur die
 * eigenen laufenden Blöcke, ihre Kinderliste mit An- und Abmelden und
 * Beenden, darüber „Als Nächstes“ zum Starten. Die eigene Aufsicht ergänzt
 * die Betriebs-API des Betreuungsplans (`schedules:read`); das Backend prüft
 * jede Aktion an der Einteilung oder tatsächlichen Aufsicht. Die schulweite Übersicht, Gruppen, offene
 * Räume und der Schulhof fehlen bewusst: sie zeigen die Anwesenheit der
 * ganzen Schule.
 */
export function OwnSupervisionView() {
  const { data: session } = useSession();
  const router = useTenantRouter();
  const searchParams = useSearchParams();
  const sessionParam = searchParams.get("session");
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const showTimetableCounts = useShowTimetableCounts();
  const isDesktop = !useMediaQuery(BELOW_LG);

  const day = useSWRAuth<PlannedTimetableInstance[]>(
    "active-supervisions-own-day",
    () =>
      timetableOperationsApi.plannedNow({ scope: "day", includeRoster: false }),
    { refreshInterval: LIST_REFRESH_MS },
  );
  const upcoming = useSWRAuth<PlannedTimetableInstance[]>(
    "active-supervisions-own-planned-now",
    () => timetableOperationsApi.plannedNow({ includeRoster: true }),
    { refreshInterval: LIST_REFRESH_MS },
  );
  const ownSessions = useSWRAuth<ActiveGroup[]>(
    "active-supervisions-own-sessions",
    () => userContextService.getMySupervisedGroups(),
    { refreshInterval: LIST_REFRESH_MS },
  );
  const { mutate: mutateOwnSessions } = ownSessions;
  const { mutate: mutateDay } = day;
  const { mutate: mutateUpcoming } = upcoming;
  const running = useMemo(
    () => ownRunningBlocks(day.data ?? [], ownSessions.data ?? []),
    [day.data, ownSessions.data],
  );
  const selected =
    running.find((block) => block.activeGroupId === sessionParam) ??
    running[0] ??
    null;

  const roster = useTimetableRoster({
    selectedTimetableInstanceId: selected?.id ?? null,
    currentRoomId: selected?.activeGroupId,
  });
  const { currentTimetableRoster } = roster;
  const loadError = useSwrLoadError(
    day.error ?? upcoming.error ?? ownSessions.error ?? roster.rosterError,
    "Ihre Aufsicht",
    () =>
      Promise.all([
        mutateDay(),
        mutateUpcoming(),
        mutateOwnSessions(),
        roster.mutateRoster(),
      ]),
  );
  const reopen = useReopenBanner();

  const reloadLists = useCallback(
    () => Promise.all([mutateDay(), mutateUpcoming(), mutateOwnSessions()]),
    [mutateDay, mutateUpcoming, mutateOwnSessions],
  );
  // Die Auswahl folgt der Adresse; die Seite merkt sich keinen Block selbst.
  const adoptSession = useCallback(
    (activeGroupId: string) => sessionHref(activeGroupId),
    [],
  );
  const keepSelection = useCallback(() => undefined, []);

  const actions = useTimetableActions({
    allRooms: NO_ROOMS,
    currentStaffId: undefined,
    activeTimetableInstanceId: roster.activeTimetableInstanceId,
    currentTimetableRoster,
    mutateRoster: roster.mutateRoster,
    mutateDashboard: reloadLists,
    adoptSession,
    setSelectedTimetableInstanceId: keepSelection,
    router,
    reopenable: reopen.reopenable,
    rememberReopenable: reopen.rememberReopenable,
    clearReopenable: reopen.clearReopenable,
  });

  const title =
    currentTimetableRoster?.instance.title ??
    selected?.title ??
    "Aktuelle Aufsicht";
  useSetBreadcrumb({ activeSupervisionName: selected ? title : undefined });

  const isLoading =
    (day.data === undefined && !day.error) ||
    (ownSessions.data === undefined && !ownSessions.error) ||
    roster.isWaitingForTimetableRoster;
  const presentCount =
    currentTimetableRoster?.rows.filter((row) => row.currentlyPresent).length ??
    0;
  const runState = currentTimetableRoster
    ? rosterRunState(currentTimetableRoster.instance.status)
    : null;

  // Wie auf der vollen Seite: am Desktop wechselt die Seitenleiste.
  const tabs =
    running.length >= 2 && !isDesktop && selected?.activeGroupId
      ? {
          value: `session:${selected.activeGroupId}`,
          onChange: (value: string) => {
            if (!value.startsWith("session:")) return;
            router.push(sessionHref(value.slice("session:".length)));
          },
          items: running.map((block) => ({
            value: `session:${block.activeGroupId}`,
            label: block.title,
          })),
          label: "Ihre laufenden Aufsichten",
        }
      : undefined;

  return (
    <TenantPage
      title={title}
      stats={
        selected ? formatChildCount(presentCount, null) : "Keine Aufsicht aktiv"
      }
      statsLoading={isLoading}
      loading={isLoading}
      loadingLabel="Aktuelle Aufsicht wird geladen…"
      actions={
        runState ? (
          // Eine Zeile, keine gestreckte Pille unter sm (wie die volle Seite).
          <span className="flex items-center gap-2">
            <StatusBadge label={runState.label} tone={runState.tone} />
          </span>
        ) : undefined
      }
      tabs={tabs}
      overlays={
        <CompleteInstanceModal
          isOpen={actions.showCompleteConfirmation}
          roster={currentTimetableRoster}
          isCompleting={actions.isCompletingInstance}
          error={actions.completeError}
          onClose={() => actions.setShowCompleteConfirmation(false)}
          onConfirm={() => void actions.confirmCompleteTimetableInstance()}
        />
      }
    >
      <LoadErrorAlert error={loadError} />
      {reopen.reopenable ? (
        <Alert
          type="success"
          message={reopenBannerMessage(reopen.reopenable.title)}
          action={
            <Button
              type="button"
              variant="outline"
              size="compact"
              isLoading={actions.isReopeningInstance}
              loadingText="Wird zurückgenommen…"
              onClick={() => void actions.handleReopenTimetableInstance()}
            >
              Rückgängig
            </Button>
          }
        />
      ) : null}
      <PlannedNowSection
        plannedNow={upcoming.data ?? []}
        hasActiveTimetableSession={currentTimetableRoster !== null}
        isStartingInstance={actions.isStartingInstance}
        onStart={(instance) =>
          void actions.handleStartPlannedInstance(instance)
        }
      />
      {currentTimetableRoster ? (
        <>
          {actions.moveNotice ? (
            <Alert type="info" message={actions.moveNotice} />
          ) : null}
          <TimetableRosterContent
            addStudentSearch={actions.addStudentSearch}
            addStudentError={actions.addStudentError}
            attendanceWebEnabled={attendanceWebEnabled}
            isAddingStudent={actions.isAddingStudent}
            isCompletingInstance={actions.isCompletingInstance}
            isConfirmingExpected={actions.isConfirmingExpected}
            roster={currentTimetableRoster}
            showTimetableCounts={showTimetableCounts}
            statusInPageHead
            // Die Auswahl der Kinder liest das Schülerverzeichnis
            // (users:read); ohne das Recht fehlt der Eintrag.
            canAddUnplanned={hasPermission(session, "users:read")}
            addUnplannedStudentModal={AddUnplannedStudentModal}
            onAddStudent={actions.handleAddUnplannedStudent}
            onComplete={actions.handleCompleteTimetableInstance}
            onConfirmExpected={actions.handleConfirmExpectedStudents}
            onRosterAction={actions.handleRosterAction}
            onSearchChange={actions.handleAddStudentSearchChange}
          />
        </>
      ) : selected === null &&
        day.data !== undefined &&
        ownSessions.data !== undefined ? (
        <SectionCard>
          <EmptyState
            icon={<MotoConceptIcon concept="rooms" size={48} />}
            title="Keine laufende Aufsicht"
            description="Starten Sie Ihren Termin unter „Als Nächstes“. Dann sehen Sie hier die Kinderliste."
          />
        </SectionCard>
      ) : null}
    </TenantPage>
  );
}
