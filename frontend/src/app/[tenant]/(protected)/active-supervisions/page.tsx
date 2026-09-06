"use client";

import { useState, useEffect, Suspense, useMemo, useCallback } from "react";
import { LogOut, UserPlus } from "lucide-react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { redirect } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  useAttendanceWebEnabled,
  useNFCEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { TenantPage } from "~/components/ui/tenant-page";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Button } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
import { ConfirmationModal } from "~/components/ui/modal";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { isCaregiver } from "~/lib/auth-utils";
import { UnclaimedRooms } from "~/components/active/unclaimed-rooms";
import { SSEErrorBoundary } from "~/components/sse/SSEErrorBoundary";
import {
  ActiveSupervisionLoadingView,
  EmptyRoomsView,
  ReleaseSupervisionModal,
  SchulhofNotSupervisingView,
} from "~/components/active-supervisions/states";
import { PastBlocksSection } from "~/components/active-supervisions/past-blocks-section";
import { PlannedNowSection } from "~/components/active-supervisions/planned-now-section";
import { SpontaneousActivityStart } from "~/components/active-supervisions/spontaneous-activity-start";
import { TransitStudentsSection } from "~/components/rooms/transit-students-section";
import {
  SCHULHOF_ROOM_NAME,
  SCHULHOF_TAB_ID,
  additionalSupervisionTarget,
  roomsOutsideSchulhofStatus,
  supervisionTabLabel,
} from "~/components/active-supervisions/view-model";
import { useSupervisionDashboard } from "~/components/active-supervisions/use-supervision-dashboard";
import { useTimetableRoster } from "~/components/active-supervisions/use-timetable-roster";
import { useStudentFilters } from "~/components/active-supervisions/use-student-filters";
import { useReopenBanner } from "~/components/active-supervisions/use-reopen-banner";
import { useTimetableActions } from "~/components/active-supervisions/use-timetable-actions";
import { useSchulhofActions } from "~/components/active-supervisions/use-schulhof-actions";
import { TimetableRosterContent } from "~/components/active-supervisions/timetable-roster";
import { SupervisionStudentGrid } from "~/components/active-supervisions/student-grid";
import { AddSupervisorModal } from "~/components/active-supervisions/add-supervisor-modal";

function MeinRaumPageContent() {
  const attendanceWebEnabled = useAttendanceWebEnabled();
  const nfcEnabled = useNFCEnabled();
  const showTimetableCounts = useShowTimetableCounts();
  const router = useTenantRouter();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  // Pre-select the session from the URL. `?session=<activeGroupId>` is the
  // precise key (parallel sessions can share one room, #2265); the legacy
  // `?room=<roomId>` entry point (sidebar, old links) still resolves but can
  // never switch between sessions inside the same room.
  const sessionParam = searchParams.get("session");
  const roomParam = searchParams.get("room");

  // Display clock for relative pickup information only. The dashboard's
  // school day and spontaneous-start window come exclusively from the backend.
  const now = useMinuteClock();

  // SSE is handled globally by TenantAuthWrapper - no page-level setup
  // needed. When relevant events occur, global SSE invalidates the
  // aggregated "active-supervision-dashboard-" cache, which triggers the
  // SWR refetch inside useSupervisionDashboard. Do NOT call useGlobalSSE()
  // here - it's already called in TenantAuthWrapper.
  const dashboard = useSupervisionDashboard({
    sessionToken: session?.user?.token,
    sessionParam,
    roomParam,
  });
  const {
    allRooms,
    plannedNow,
    currentStaffId,
    schulhofStatus,
    schulhofTabAvailable,
    currentRoom,
    isSchulhofTabSelected,
    selectedTimetableInstanceId,
    students,
    error,
    setError,
    mutateDashboard,
    refresh,
  } = dashboard;

  const roster = useTimetableRoster({
    selectedTimetableInstanceId,
    currentRoomId: currentRoom?.id,
  });
  const { currentTimetableRoster } = roster;

  const filters = useStudentFilters(students);
  const reopen = useReopenBanner();
  const [showAddSupervisor, setShowAddSupervisor] = useState(false);

  const actions = useTimetableActions({
    allRooms,
    currentStaffId,
    activeTimetableInstanceId: roster.activeTimetableInstanceId,
    currentTimetableRoster,
    mutateRoster: roster.mutateRoster,
    mutateDashboard,
    refresh,
    adoptSession: dashboard.adoptSession,
    setSelectedTimetableInstanceId: dashboard.setSelectedTimetableInstanceId,
    setError,
    router,
    reopenableInstanceId: reopen.reopenableInstanceId,
    rememberReopenable: reopen.rememberReopenable,
    clearReopenable: reopen.clearReopenable,
  });

  const spontaneousStartBlockedReason =
    dashboard.spontaneousStartAvailability?.blockedReason === "weekend"
      ? "Spontane Aktivitäten sind nur montags bis freitags möglich."
      : undefined;

  const schulhof = useSchulhofActions({
    schulhofStatus,
    currentStaffId,
    currentRoom,
    spontaneousStartBlockedReason: schulhofStatus?.activeGroupId
      ? undefined
      : spontaneousStartBlockedReason,
    refresh,
    setError,
  });

  // Desktop detection — sidebar handles room switching at lg+
  const [isDesktop, setIsDesktop] = useState(false);
  useEffect(() => {
    const check = () => setIsDesktop(window.innerWidth >= 1024);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  // Ref to always have latest schulhofStatus (prevents stale closure in callbacks)
  const schulhofStatusRef = useLatest(schulhofStatus);

  const occupiedRoomIds = useMemo(() => {
    const ids = allRooms
      .map((room) => room.room_id)
      .filter((roomId): roomId is string => Boolean(roomId));
    // Schulhof is tracked separately from allRooms, so keep its live room id
    // in the occupancy set. The spontaneous modal deliberately treats that
    // one destination as navigation to the dedicated supervision instead of
    // disabling it; every normal occupied room remains unavailable to start.
    if (schulhofStatus?.activeGroupId && schulhofStatus.roomId)
      ids.push(schulhofStatus.roomId);
    return ids;
  }, [allRooms, schulhofStatus?.activeGroupId, schulhofStatus?.roomId]);

  // Set breadcrumb so the header names the session (not just the room —
  // parallel sessions can share one room, #2265)
  useSetBreadcrumb({
    activeSupervisionName: isSchulhofTabSelected
      ? SCHULHOF_ROOM_NAME
      : (currentRoom?.name ?? currentRoom?.room_name),
  });

  const handleOpenSchulhofSupervision = useCallback(() => {
    if (!schulhofStatusRef.current?.exists) {
      setError(
        "Die Schulhof-Aufsicht ist gerade nicht verfügbar. Bitte laden Sie die Seite neu.",
      );
      return;
    }
    dashboard.selectSchulhof({ clearTimetableInstance: true });
    router.push(`/active-supervisions?session=${SCHULHOF_TAB_ID}`);
    localStorage.setItem("supervision-last-session", SCHULHOF_TAB_ID);
    localStorage.setItem("sidebar-last-room", SCHULHOF_TAB_ID);
    localStorage.setItem("sidebar-last-room-name", SCHULHOF_ROOM_NAME);
    // The selection reconciliation in useSupervisionDashboard re-runs the
    // aggregate for the Schulhof session and is the single owner of loading
    // its visits. A second manual request can land later and overwrite a
    // fresher SSE revalidation.
  }, [dashboard, router, schulhofStatusRef, setError]);

  const handleTabChange = (tabId: string) => {
    if (tabId === SCHULHOF_TAB_ID) {
      handleOpenSchulhofSupervision();
      return;
    }
    // Switch to the chosen session (keyed by active group, not by room —
    // parallel sessions can share one room, #2265)
    dashboard.deselectSchulhof();
    const room = allRooms.find((r) => r.id === tabId);
    if (room) {
      router.push(`/active-supervisions?session=${tabId}`);
      localStorage.setItem("supervision-last-session", tabId);
      if (room.room_id) {
        localStorage.setItem("sidebar-last-room", room.room_id);
      }
      if (room.room_name) {
        localStorage.setItem("sidebar-last-room-name", room.room_name);
      }
      void dashboard.switchToRoom(tabId);
    }
  };

  // Laden, fehlender Zugriff und Fehler laufen ueber das Geruest
  // (`loading`/`empty`/`error`) und nicht mehr ueber vollstaendige
  // Alternativ-Rueckgaben: sonst verliert die Seite genau in diesen
  // Zustaenden Kopf, Titel und Orientierung.
  const isPageLoading =
    status === "loading" ||
    dashboard.isInitialLoading ||
    dashboard.isSwitchingSession ||
    dashboard.hasAccess === null;
  const hasNoAccess = !isPageLoading && !dashboard.hasAccess;

  // Statuszeile unter dem Seitentitel: welche Aufsicht gerade offen ist und
  // wie viele Kinder in ihr geführt werden. Beides steht schon im geladenen
  // Dashboard-Zustand.
  const supervisionName = isSchulhofTabSelected
    ? SCHULHOF_ROOM_NAME
    : (currentRoom?.name ?? currentRoom?.room_name ?? null);
  // Die Zahl kommt aus derselben Quelle wie früher der Zähler im Kopf: der
  // gemeldeten Belegung der Aufsicht, ersatzweise den geladenen Kindern.
  const supervisionCount = isSchulhofTabSelected
    ? (schulhofStatus?.studentCount ?? students.length)
    : (currentRoom?.student_count ?? students.length);
  const supervisionSummary = supervisionName
    ? `${supervisionName} · ${supervisionCount} ${supervisionCount === 1 ? "Kind" : "Kinder"}`
    : "Keine Aufsicht aktiv";

  // Reiterleiste der einzelnen Aufsichten. Am Desktop wechselt die
  // Seitenleiste die Aufsicht, deshalb stehen die Reiter nur darunter.
  const roomsOutsideStatus = roomsOutsideSchulhofStatus(allRooms, {
    schulhofTabEnabled: dashboard.schulhofTabEnabled,
    statusActiveGroupId: schulhofStatus?.activeGroupId,
  });
  const totalSupervisions =
    roomsOutsideStatus.length + (schulhofTabAvailable ? 1 : 0);
  const allSupervisionTabItems = [
    // Reguläre Aufsichten, einschließlich einer parallelen
    // Schulhof-Gruppe, die der feste Reiter nicht abbildet.
    ...roomsOutsideStatus.map((room) => ({
      value: room.id,
      label: supervisionTabLabel(
        room,
        dashboard.sessionInfoByActiveGroup.get(room.id) ?? null,
      ),
    })),
    // Fester Schulhof-Reiter, nur mit spontaner Aufsicht (#2161).
    ...(schulhofTabAvailable
      ? [{ value: SCHULHOF_TAB_ID, label: SCHULHOF_ROOM_NAME }]
      : []),
  ];
  const supervisionTabItems = allSupervisionTabItems;
  const supervisionTabs =
    totalSupervisions >= 2 && !isDesktop
      ? {
          value: isSchulhofTabSelected
            ? SCHULHOF_TAB_ID
            : (currentRoom?.id ?? ""),
          onChange: handleTabChange,
          items: supervisionTabItems,
          label: "Meine Aufsichten",
        }
      : undefined;

  // Die Aufsicht abgeben kann nur, wer den Schulhof gerade beaufsichtigt; im
  // Leerzustand steht dort stattdessen „Beaufsichtigen".
  const releaseAction =
    isSchulhofTabSelected && schulhofStatus?.isUserSupervising ? (
      <Button
        type="button"
        variant="outline_danger"
        size="md"
        onClick={() => schulhof.setShowReleaseModal(true)}
        className="gap-2"
      >
        <LogOut className="h-4 w-4" aria-hidden="true" />
        Aufsicht abgeben
      </Button>
    ) : undefined;

  const spontaneousStartBanner = dashboard.webSpontaneousActivitiesEnabled ? (
    <SpontaneousActivityStart
      currentStaffId={currentStaffId}
      defaultRoomId={currentRoom?.room_id}
      disabled={dashboard.spontaneousStartAvailability?.available === false}
      disabledReason={spontaneousStartBlockedReason}
      isStarting={actions.isStartingSpontaneous}
      occupiedRoomIds={occupiedRoomIds}
      onStart={(payload) =>
        void actions.handleStartSpontaneousActivity(payload)
      }
    />
  ) : null;
  const reopenBanner = reopen.reopenableInstanceId ? (
    <div>
      <Alert
        type="success"
        message="Aktivität wurde beendet. Die Rücknahme ist fünf Minuten lang möglich."
        action={
          <Button
            type="button"
            variant="outline"
            size="compact"
            onClick={() => void actions.handleReopenTimetableInstance()}
          >
            Rückgängig
          </Button>
        }
      />
    </div>
  ) : null;

  // Zusätzliche Betreuer (#2806): auf der eigenen aktiven Aufsicht steht die
  // Aktion in der Kopfkarte; das Gerüst regelt die mobile Darstellung, eine
  // eigene Icon-Variante braucht es nicht mehr.
  const additionalSupervisionActiveGroupId = additionalSupervisionTarget({
    currentRoom,
    isSchulhofTabSelected,
    schulhofStatus,
  });
  const isCurrentSupervisionOwn = isSchulhofTabSelected
    ? (schulhofStatus?.isUserSupervising ?? false)
    : (currentRoom?.isCurrentUserSupervising ?? false);

  const addSupervisorButton = additionalSupervisionActiveGroupId ? (
    <>
      {isCurrentSupervisionOwn ? (
        <StatusBadge label="Eigene Aufsicht" tone="green" />
      ) : null}
      <Button
        type="button"
        variant="outline"
        size="md"
        onClick={() => setShowAddSupervisor(true)}
      >
        <UserPlus className="h-4 w-4" aria-hidden="true" />
        Betreuer hinzufügen
      </Button>
    </>
  ) : null;

  // Keine eigene Aufsicht, kein Schulhof, nichts geplant: dieselbe Kopfkarte,
  // nur ein anderer Inhalt — keine zweite Seite.
  const showUnclaimedOnly =
    !isPageLoading &&
    !hasNoAccess &&
    allRooms.length === 0 &&
    !schulhofTabAvailable &&
    plannedNow.length === 0;

  // Render helper for student grid content
  const renderStudentContent = () => {
    if (
      dashboard.isWaitingForUrlRoomSelection ||
      roster.isWaitingForTimetableRoster
    ) {
      return <ActiveSupervisionLoadingView withHeader={false} />;
    }

    if (currentTimetableRoster) {
      return (
        <>
          {actions.moveNotice && (
            <div className="mb-4">
              <Alert type="info" message={actions.moveNotice} />
            </div>
          )}
          <TimetableRosterContent
            addStudentResults={actions.addStudentResults}
            addStudentSearch={actions.addStudentSearch}
            attendanceWebEnabled={attendanceWebEnabled}
            isAddingStudent={actions.isAddingStudent}
            isCompletingInstance={actions.isCompletingInstance}
            isConfirmingExpected={actions.isConfirmingExpected}
            roster={currentTimetableRoster}
            showTimetableCounts={showTimetableCounts}
            onAddStudent={actions.handleAddUnplannedStudent}
            onComplete={actions.handleCompleteTimetableInstance}
            onConfirmExpected={actions.handleConfirmExpectedStudents}
            onRosterAction={actions.handleRosterAction}
            onSearchChange={actions.handleAddStudentSearchChange}
          />
        </>
      );
    }

    return (
      <SupervisionStudentGrid
        students={students}
        filteredStudents={filters.filteredStudents}
        pickupTimesData={dashboard.pickupTimesData}
        arrivalTimesData={dashboard.arrivalTimesData}
        trackingData={dashboard.trackingData}
        myGroupIds={dashboard.myGroupIds}
        myGroupRooms={dashboard.myGroupRooms}
        now={now}
        onOpenStudent={(studentId) =>
          router.push(`/students/${studentId}?from=/active-supervisions`)
        }
      />
    );
  };

  return (
    <TenantPage
      title="Aktuelle Aufsicht"
      stats={supervisionSummary}
      actions={
        (addSupervisorButton ?? releaseAction) ? (
          <>
            {addSupervisorButton}
            {releaseAction}
          </>
        ) : undefined
      }
      search={{
        value: filters.searchTerm,
        onChange: filters.setSearchTerm,
        placeholder: "Name suchen…",
      }}
      filters={filters.filterConfigs}
      activeFilters={filters.activeFilters}
      onClearAllFilters={filters.clearAllFilters}
      tabs={supervisionTabs}
      statsLoading={isPageLoading}
      loading={isPageLoading}
      loadingLabel="Aktuelle Aufsicht wird geladen…"
      empty={
        hasNoAccess
          ? {
              icon: <MotoConceptIcon concept="rooms" size={48} />,
              title: "Keine aktive Raum-Aufsicht",
              description: `Sie sind aktuell in keinem Raum als Live-Aktivität registriert. Starten Sie eine Aktivität ${
                nfcEnabled ? "an einem Terminal" : "in der Web-App"
              }, um Live-Raumdaten einzusehen.`,
            }
          : null
      }
      overlays={
        <>
          <ConfirmationModal
            isOpen={actions.showCompleteConfirmation}
            onClose={() => actions.setShowCompleteConfirmation(false)}
            onConfirm={() => void actions.confirmCompleteTimetableInstance()}
            title="Aktivität wirklich beenden?"
            confirmText="Aktivität beenden"
            isConfirmLoading={actions.isCompletingInstance}
            isDismissDisabled={actions.isCompletingInstance}
          >
            <div className="space-y-3 text-sm text-gray-700">
              <p>
                <strong>{currentTimetableRoster?.instance.title}</strong> endet
                laut Plan um {currentTimetableRoster?.instance.endTime} Uhr.
              </p>
              <p>
                Aktuell anwesend:{" "}
                {currentTimetableRoster?.rows.filter(
                  (row) => row.currentlyPresent,
                ).length ?? 0}
              </p>
              {(currentTimetableRoster?.rows.filter(
                (row) => row.currentlyPresent,
              ).length ?? 0) > 0 ? (
                <ul className="list-disc space-y-1 pl-5">
                  {currentTimetableRoster?.rows
                    .filter((row) => row.currentlyPresent)
                    .map((row) => (
                      <li key={row.studentId}>{row.studentName}</li>
                    ))}
                </ul>
              ) : null}
            </div>
          </ConfirmationModal>
          {/* Schulhof Release Supervision Modal */}
          <ReleaseSupervisionModal
            isOpen={schulhof.showReleaseModal}
            onClose={() => schulhof.setShowReleaseModal(false)}
            onConfirm={() =>
              schulhof.handleReleaseSupervision().catch(() => undefined)
            }
            isConfirmLoading={schulhof.isReleasingSupervision}
          />
          {showAddSupervisor ? (
            <AddSupervisorModal
              activeGroupId={additionalSupervisionActiveGroupId}
              isOpen
              onClose={() => setShowAddSupervisor(false)}
              onAdded={mutateDashboard}
            />
          ) : null}
        </>
      }
    >
      {/* Fehler der Seite stehen als Alert oben im Inhalt und nicht im
          `error`-Zustand des Geruests: hier meldet auch eine misslungene
          Einzelaktion (Kind hinzufuegen, Aufsicht wechseln), und die Flaeche
          darunter muss bedienbar bleiben, damit man es erneut versuchen kann. */}
      {error && !hasNoAccess ? <Alert type="error" message={error} /> : null}
      {showUnclaimedOnly ? (
        <>
          {reopenBanner}
          {spontaneousStartBanner}
          <EmptyRoomsView
            onClaimed={refresh}
            cachedActiveGroups={dashboard.cachedActiveGroups}
            currentStaffId={currentStaffId}
          />
          {/* The day review must survive the empty state: after the last block
              ends, supervisors land exactly here (#2335). */}
          <PastBlocksSection />
        </>
      ) : (
        <>
          {reopenBanner}
          {/* Unclaimed Rooms Section - Shows rooms available for claiming */}
          <UnclaimedRooms
            onClaimed={refresh}
            activeGroups={
              dashboard.cachedActiveGroups.length > 0
                ? dashboard.cachedActiveGroups
                : undefined
            }
            currentStaffId={currentStaffId}
          />

          <PlannedNowSection
            plannedNow={plannedNow}
            hasActiveTimetableSession={currentTimetableRoster !== null}
            isStartingInstance={actions.isStartingInstance}
            onStart={(instance) =>
              void actions.handleStartPlannedInstance(instance)
            }
          />

          {spontaneousStartBanner}

          {/* Schulhof Not Supervising View - matches suggestions page empty state style */}
          {isSchulhofTabSelected &&
            schulhofStatus &&
            !schulhofStatus.isUserSupervising && (
              <SchulhofNotSupervisingView
                supervisorCount={schulhofStatus.supervisorCount}
                supervisorNames={schulhofStatus.supervisors.map((s) => s.name)}
                isToggling={schulhof.isTogglingSchulhof}
                startDisabled={
                  !schulhofStatus.activeGroupId &&
                  dashboard.spontaneousStartAvailability?.available === false
                }
                startDisabledReason={
                  schulhofStatus.activeGroupId
                    ? undefined
                    : spontaneousStartBlockedReason
                }
                onToggle={() =>
                  schulhof.handleToggleSchulhof().catch(() => undefined)
                }
              />
            )}

          {currentRoom &&
          (!isSchulhofTabSelected || schulhofStatus?.isUserSupervising) ? (
            <Suspense fallback={null}>
              <TransitStudentsSection
                fromReferrer="/active-supervisions"
                collapsible
              />
            </Suspense>
          ) : null}

          {/* Student Grid - Mobile Optimized */}
          {(!isSchulhofTabSelected || schulhofStatus?.isUserSupervising) &&
            renderStudentContent()}

          {/* Read-only end-of-day review of finished and expired blocks (#2335) */}
          <PastBlocksSection />
        </>
      )}
    </TenantPage>
  );
}

// Gate component: allows caregivers always, everyone else only when the
// server confirmed the school-wide overview covers them (#2380).
function ActiveSupervisionGate({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/");
    },
  });
  const { overviewEnabled, isLoadingSupervision } = useOptionalSupervision();

  if (status === "loading" || isLoadingSupervision) {
    return (
      <TenantPage
        title="Aktuelle Aufsicht"
        statsLoading
        loading
        loadingLabel="Aktuelle Aufsicht wird geladen…"
      />
    );
  }

  // Caregivers (user/teacher role) always have access
  if (isCaregiver(session)) {
    return <>{children}</>;
  }

  // The overview endpoint confirms the backend granted this caller access.
  // That includes effective admins and verified staff under all_staff.
  // Checking supervisedRooms.length would incorrectly let callers through
  // when the scope is "own" but a synthetic Schulhof entry is present.
  if (overviewEnabled) {
    return <>{children}</>;
  }

  // Fehlendes Recht ist ein Zustand der Seite, kein Fehler: dieselbe
  // Kopfkarte mit demselben Titel, darunter der ruhige Leerzustand.
  return <ForbiddenPage title="Aktuelle Aufsicht" />;
}

// Main component with Suspense wrapper. BinaryModeGuard runs first so
// binary-mode tenants get a 404 before the supervision gate tries to load
// data that depends on detailed-mode room visits.
export default function MeinRaumPage() {
  return (
    <BinaryModeGuard title="Aktuelle Aufsicht">
      <Suspense
        fallback={
          <TenantPage
            title="Aktuelle Aufsicht"
            statsLoading
            loading
            loadingLabel="Aktuelle Aufsicht wird geladen…"
          />
        }
      >
        <ActiveSupervisionGate>
          <SSEErrorBoundary>
            <MeinRaumPageContent />
          </SSEErrorBoundary>
        </ActiveSupervisionGate>
      </Suspense>
    </BinaryModeGuard>
  );
}
