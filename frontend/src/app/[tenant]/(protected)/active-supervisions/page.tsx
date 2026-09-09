"use client";

import { useState, useEffect, Suspense, useMemo, useCallback } from "react";
import { LogOut, UserPlus } from "lucide-react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { redirect } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
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
  OpenRoomNotice,
  ReleaseSupervisionModal,
  SchulhofSuperviseButton,
} from "~/components/active-supervisions/states";
import { PastBlocksSection } from "~/components/active-supervisions/past-blocks-section";
import { PlannedNowSection } from "~/components/active-supervisions/planned-now-section";
import { SpontaneousActivityStart } from "~/components/active-supervisions/spontaneous-activity-start";
import { TransitStudentsSection } from "~/components/rooms/transit-students-section";
import {
  additionalSupervisionTarget,
  sessionsOutsideOpenRooms,
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
    openRooms,
    currentRoom,
    currentOpenRoom,
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
    supervisedActiveGroupId:
      schulhofStatus?.isUserSupervising && schulhofStatus.activeGroupId
        ? schulhofStatus.activeGroupId
        : null,
    spontaneousStartBlockedReason: schulhofStatus?.activeGroupId
      ? undefined
      : spontaneousStartBlockedReason,
    refresh,
    setError,
  });

  // The Schulhof's own supervision offer (#2161) belongs to the Schulhof room,
  // matched by room id — never by the name of the room on screen.
  const isSchulhofOpenRoom =
    !!currentOpenRoom &&
    !!schulhofStatus?.roomId &&
    schulhofStatus.roomId === currentOpenRoom.roomId;

  // Desktop detection — sidebar handles room switching at lg+
  const [isDesktop, setIsDesktop] = useState(false);
  useEffect(() => {
    const check = () => setIsDesktop(window.innerWidth >= 1024);
    check();
    window.addEventListener("resize", check);
    return () => window.removeEventListener("resize", check);
  }, []);

  const openRoomIds = useMemo(
    () => new Set(openRooms.map((room) => room.roomId)),
    [openRooms],
  );

  const occupiedRoomIds = useMemo(() => {
    const ids = allRooms
      .map((room) => room.room_id)
      .filter((roomId): roomId is string => Boolean(roomId));
    // A released room that currently holds a session is occupied too, even
    // though it is not one of the caller's own tabs. The spontaneous modal
    // treats such a destination as navigation to the running supervision
    // instead of disabling it; every normal occupied room stays unavailable.
    for (const room of openRooms) {
      if (room.activeGroupIds.length > 0) ids.push(room.roomId);
    }
    return ids;
  }, [allRooms, openRooms]);

  // Set breadcrumb so the header names what is open — a shared room by its
  // room name, an own supervision by its session (parallel sessions can share
  // one room, #2265).
  useSetBreadcrumb({
    activeSupervisionName:
      currentOpenRoom?.name ?? currentRoom?.name ?? currentRoom?.room_name,
  });

  // Opening a shared room is a pure selection: its occupancy already arrived
  // with the dashboard, so there is nothing to load and nothing to claim.
  const handleOpenRoom = useCallback(
    (roomId: string) => {
      const room = openRooms.find((candidate) => candidate.roomId === roomId);
      if (!room) return;
      dashboard.selectOpenRoom(roomId, { clearTimetableInstance: true });
      router.push(`/active-supervisions?room=${roomId}`);
      localStorage.removeItem("supervision-last-session");
      localStorage.setItem("sidebar-last-room", roomId);
      localStorage.setItem("sidebar-last-room-name", room.name);
    },
    [dashboard, openRooms, router],
  );

  const handleTabChange = (tabValue: string) => {
    if (tabValue.startsWith("room:")) {
      handleOpenRoom(tabValue.slice("room:".length));
      return;
    }
    // Switch to the chosen session (keyed by active group, not by room —
    // parallel sessions can share one room, #2265)
    if (!tabValue.startsWith("session:")) return;
    const sessionId = tabValue.slice("session:".length);
    dashboard.clearOpenRoom();
    const room = allRooms.find((r) => r.id === sessionId);
    if (room) {
      router.push(`/active-supervisions?session=${sessionId}`);
      localStorage.setItem("supervision-last-session", sessionId);
      if (room.room_id) {
        localStorage.setItem("sidebar-last-room", room.room_id);
      }
      if (room.room_name) {
        localStorage.setItem("sidebar-last-room-name", room.room_name);
      }
      void dashboard.switchToRoom(sessionId);
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
    dashboard.isWaitingForUrlRoomSelection ||
    dashboard.hasAccess === null;
  const hasNoAccess = !isPageLoading && !dashboard.hasAccess;

  // Statuszeile unter dem Seitentitel: welche Aufsicht gerade offen ist und
  // wie viele Kinder in ihr geführt werden. Beides steht schon im geladenen
  // Dashboard-Zustand.
  const supervisionName =
    currentOpenRoom?.name ??
    currentRoom?.name ??
    currentRoom?.room_name ??
    null;
  // Die Zahl kommt aus derselben Quelle wie früher der Zähler im Kopf: der
  // gemeldeten Belegung, ersatzweise den geladenen Kindern.
  const supervisionCount =
    currentOpenRoom?.studentCount ??
    currentRoom?.student_count ??
    students.length;
  const supervisionSummary = supervisionName
    ? `${supervisionName} · ${supervisionCount} ${supervisionCount === 1 ? "Kind" : "Kinder"}`
    : "Keine Aufsicht aktiv";

  // Reiterleiste. Am Desktop wechselt die Seitenleiste, deshalb stehen die
  // Reiter nur darunter. Offene Räume stehen hinter den eigenen Aufsichten:
  // sie sind gemeinsam, nicht die eigene Aufsicht der Person (#3065).
  const ownSessions = sessionsOutsideOpenRooms(allRooms, openRoomIds);
  const totalSupervisions = ownSessions.length + openRooms.length;
  const supervisionTabItems = [
    ...ownSessions.map((room) => ({
      value: `session:${room.id}`,
      label: supervisionTabLabel(
        room,
        dashboard.sessionInfoByActiveGroup.get(room.id) ?? null,
      ),
    })),
    ...openRooms.map((room) => ({
      value: `room:${room.roomId}`,
      label: room.name,
    })),
  ];
  const supervisionTabs =
    totalSupervisions >= 2 && !isDesktop
      ? {
          value: currentOpenRoom
            ? `room:${currentOpenRoom.roomId}`
            : currentRoom
              ? `session:${currentRoom.id}`
              : "",
          onChange: handleTabChange,
          items: supervisionTabItems,
          label: "Aufsichten und offene Räume",
        }
      : undefined;

  // Die Aufsicht abgeben kann nur, wer den Schulhof gerade beaufsichtigt; im
  // Leerzustand steht dort stattdessen „Beaufsichtigen".
  const releaseAction =
    isSchulhofOpenRoom && schulhofStatus?.isUserSupervising ? (
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
      defaultRoomId={currentRoom?.room_id ?? currentOpenRoom?.roomId}
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
    currentOpenRoom,
  });
  const isCurrentSupervisionOwn = currentOpenRoom
    ? currentOpenRoom.isUserSupervising
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

  // Keine eigene Aufsicht, kein offener Raum, nichts geplant: dieselbe
  // Kopfkarte, nur ein anderer Inhalt — keine zweite Seite.
  const showUnclaimedOnly =
    !isPageLoading &&
    !hasNoAccess &&
    allRooms.length === 0 &&
    openRooms.length === 0 &&
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

          {/* Ein offener Raum sagt in einer Zeile, was er ist und ob die
              Person hier die Aufsicht hat. Die Kinder bleiben in jedem Fall
              sichtbar — genau das unterscheidet ihn von einer Aufsicht. */}
          {currentOpenRoom ? (
            <OpenRoomNotice
              isUserSupervising={currentOpenRoom.isUserSupervising}
              supervisorNames={
                isSchulhofOpenRoom
                  ? schulhofStatus?.supervisors.map((s) => s.name)
                  : undefined
              }
              hint={
                isSchulhofOpenRoom &&
                !currentOpenRoom.isUserSupervising &&
                !schulhofStatus?.activeGroupId
                  ? spontaneousStartBlockedReason
                  : undefined
              }
              action={
                isSchulhofOpenRoom && !currentOpenRoom.isUserSupervising ? (
                  <SchulhofSuperviseButton
                    isToggling={schulhof.isTogglingSchulhof}
                    disabled={
                      !schulhofStatus?.activeGroupId &&
                      dashboard.spontaneousStartAvailability?.available ===
                        false
                    }
                    onToggle={() =>
                      schulhof.handleToggleSchulhof().catch(() => undefined)
                    }
                  />
                ) : undefined
              }
            />
          ) : null}

          {/* Kinder in einen Raum setzen ist eine Verwaltungsaktion. Einen
              offenen Raum zu sehen verleiht sie nicht (#3065); wer dort die
              Aufsicht hat, behält sie. */}
          {(currentRoom ?? currentOpenRoom?.isUserSupervising) ? (
            <Suspense fallback={null}>
              <TransitStudentsSection
                fromReferrer="/active-supervisions"
                collapsible
              />
            </Suspense>
          ) : null}

          {/* Student Grid - Mobile Optimized */}
          {renderStudentContent()}

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
  // when the scope is "own" but a released room is present: that room is
  // reachable for everyone and grants no supervision (#3065).
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
