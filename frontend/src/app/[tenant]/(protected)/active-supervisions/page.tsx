"use client";

import { useState, useEffect, Suspense, useMemo, useCallback } from "react";
import { LogOut } from "lucide-react";
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { redirect } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useLatest } from "~/lib/hooks/use-latest";
import {
  useAttendanceWebEnabled,
  useShowTimetableCounts,
} from "~/lib/tenant-context";
import { useOptionalSupervision } from "~/lib/supervision-context";
import { ForbiddenPage } from "~/components/ui/forbidden-page";
import { BinaryModeGuard } from "~/components/tenant/binary-mode-guard";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  CardGridSkeleton,
  PageHeaderSkeleton,
  SkeletonRegion,
} from "~/components/ui/page-skeletons";
import { useMinuteClock } from "~/lib/pickup-helpers";
import { isCaregiver } from "~/lib/auth-utils";
import { UnclaimedRooms } from "~/components/active/unclaimed-rooms";
import { SSEErrorBoundary } from "~/components/sse/SSEErrorBoundary";
import {
  ActiveSupervisionLoadingView,
  EmptyRoomsView,
  NoActiveSupervisionAccessView,
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
} from "~/components/active-supervisions/view-model";
import { useSupervisionDashboard } from "~/components/active-supervisions/use-supervision-dashboard";
import { useTimetableRoster } from "~/components/active-supervisions/use-timetable-roster";
import { useStudentFilters } from "~/components/active-supervisions/use-student-filters";
import { useReopenBanner } from "~/components/active-supervisions/use-reopen-banner";
import { useTimetableActions } from "~/components/active-supervisions/use-timetable-actions";
import { useSchulhofActions } from "~/components/active-supervisions/use-schulhof-actions";
import { TimetableRosterContent } from "~/components/active-supervisions/timetable-roster";
import { SupervisionStudentGrid } from "~/components/active-supervisions/student-grid";
import { SupervisionHeader } from "~/components/active-supervisions/supervision-header";

// Re-exported for its unit tests; the implementation lives with the other
// active-supervisions helpers.
export { spontaneousActivityWindow } from "~/components/active-supervisions/spontaneous-window";

function MeinRaumPageContent() {
  const attendanceWebEnabled = useAttendanceWebEnabled();
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

  const now = useMinuteClock();

  // SSE is handled globally by TenantAuthWrapper - no page-level setup
  // needed. When relevant events occur, global SSE invalidates the
  // aggregated "active-supervision-dashboard-" cache, which triggers the
  // SWR refetch inside useSupervisionDashboard. Do NOT call useGlobalSSE()
  // here - it's already called in TenantAuthWrapper.
  const dashboard = useSupervisionDashboard({
    sessionToken: session?.user?.token,
    now,
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

  const schulhof = useSchulhofActions({
    schulhofStatus,
    currentStaffId,
    currentRoom,
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

  if (
    status === "loading" ||
    dashboard.isInitialLoading ||
    dashboard.isSwitchingSession ||
    dashboard.hasAccess === null
  ) {
    return <ActiveSupervisionLoadingView />;
  }

  // Show empty state if no active supervision
  if (!dashboard.hasAccess) {
    return <NoActiveSupervisionAccessView />;
  }

  const spontaneousStartBanner = dashboard.webSpontaneousActivitiesEnabled ? (
    <SpontaneousActivityStart
      currentStaffId={currentStaffId}
      defaultRoomId={currentRoom?.room_id}
      isStarting={actions.isStartingSpontaneous}
      occupiedRoomIds={occupiedRoomIds}
      onStart={(payload) =>
        void actions.handleStartSpontaneousActivity(payload)
      }
    />
  ) : null;
  const reopenBanner = reopen.reopenableInstanceId ? (
    <div className="mb-4">
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

  // Show unclaimed rooms banner when user has no supervised groups and no Schulhof
  // If the Schulhof tab is available, we'll show the main view with just that tab
  if (
    allRooms.length === 0 &&
    !schulhofTabAvailable &&
    plannedNow.length === 0
  ) {
    return (
      <div className="w-full">
        {reopenBanner}
        {spontaneousStartBanner}
        <EmptyRoomsView
          onClaimed={refresh}
          cachedActiveGroups={dashboard.cachedActiveGroups}
          currentStaffId={currentStaffId}
          searchTerm={filters.searchTerm}
          setSearchTerm={filters.setSearchTerm}
          setGroupFilter={filters.setGroupFilter}
          setSelectedYear={filters.setSelectedYear}
          filterConfigs={filters.filterConfigs}
          activeFilters={filters.activeFilters}
        />
        {/* The day review must survive the empty state: after the last block
            ends, supervisors land exactly here (#2335). */}
        <PastBlocksSection />
      </div>
    );
  }

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
    <div className="w-full">
      {reopenBanner}
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
            <strong>{currentTimetableRoster?.instance.title}</strong> endet laut
            Plan um {currentTimetableRoster?.instance.endTime} Uhr.
          </p>
          <p>
            Aktuell anwesend:{" "}
            {currentTimetableRoster?.rows.filter((row) => row.currentlyPresent)
              .length ?? 0}
          </p>
          {(currentTimetableRoster?.rows.filter((row) => row.currentlyPresent)
            .length ?? 0) > 0 ? (
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

      <SupervisionHeader
        isDesktop={isDesktop}
        allRooms={allRooms}
        currentRoom={currentRoom}
        isSchulhofTabSelected={isSchulhofTabSelected}
        schulhofTabEnabled={dashboard.schulhofTabEnabled}
        schulhofTabAvailable={schulhofTabAvailable}
        schulhofStatus={schulhofStatus}
        sessionInfoByActiveGroup={dashboard.sessionInfoByActiveGroup}
        searchTerm={filters.searchTerm}
        onSearchChange={filters.setSearchTerm}
        filterConfigs={filters.filterConfigs}
        activeFilters={filters.activeFilters}
        onClearAllFilters={filters.clearAllFilters}
        onTabChange={handleTabChange}
        actionButton={
          // Only show release button when user IS supervising Schulhof
          // "Beaufsichtigen" button is shown in the empty state instead (no duplicate)
          isSchulhofTabSelected && schulhofStatus?.isUserSupervising ? (
            <button
              type="button"
              onClick={() => schulhof.setShowReleaseModal(true)}
              className="flex h-10 items-center gap-2 rounded-full border border-red-200 bg-red-50 px-4 text-red-600 transition-colors hover:bg-red-100"
              aria-label="Aufsicht abgeben"
            >
              <LogOut className="h-5 w-5" aria-hidden="true" />
              <span className="text-sm font-medium">Aufsicht abgeben</span>
            </button>
          ) : undefined
        }
        mobileActionButton={
          // Only show release button when user IS supervising Schulhof
          isSchulhofTabSelected && schulhofStatus?.isUserSupervising ? (
            <button
              type="button"
              onClick={() => schulhof.setShowReleaseModal(true)}
              className="flex h-8 w-8 items-center justify-center rounded-full border border-red-200 bg-red-50 text-red-600 transition-colors hover:bg-red-100"
              aria-label="Aufsicht abgeben"
            >
              <LogOut className="h-4 w-4" aria-hidden="true" />
            </button>
          ) : undefined
        }
      />

      {/* Schulhof Release Supervision Modal */}
      <ReleaseSupervisionModal
        isOpen={schulhof.showReleaseModal}
        onClose={() => schulhof.setShowReleaseModal(false)}
        onConfirm={() =>
          schulhof.handleReleaseSupervision().catch(() => undefined)
        }
        isConfirmLoading={schulhof.isReleasingSupervision}
      />

      {/* Mobile Error Display */}
      {error && (
        <div className="mb-4 md:hidden">
          <Alert type="error" message={error} />
        </div>
      )}

      {/* Schulhof Not Supervising View - matches suggestions page empty state style */}
      {isSchulhofTabSelected &&
        schulhofStatus &&
        !schulhofStatus.isUserSupervising && (
          <SchulhofNotSupervisingView
            supervisorCount={schulhofStatus.supervisorCount}
            supervisorNames={schulhofStatus.supervisors.map((s) => s.name)}
            isToggling={schulhof.isTogglingSchulhof}
            onToggle={() =>
              schulhof.handleToggleSchulhof().catch(() => undefined)
            }
          />
        )}

      {currentRoom &&
      (!isSchulhofTabSelected || schulhofStatus?.isUserSupervising) ? (
        <div className="mb-4">
          <Suspense fallback={null}>
            <TransitStudentsSection
              fromReferrer="/active-supervisions"
              collapsible
            />
          </Suspense>
        </div>
      ) : null}

      {/* Student Grid - Mobile Optimized */}
      {(!isSchulhofTabSelected || schulhofStatus?.isUserSupervising) &&
        renderStudentContent()}

      {/* Read-only end-of-day review of finished and expired blocks (#2335) */}
      <PastBlocksSection />
    </div>
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
    return <ActiveSupervisionLoadingView />;
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

  return <ForbiddenPage />;
}

// Main component with Suspense wrapper. BinaryModeGuard runs first so
// binary-mode tenants get a 404 before the supervision gate tries to load
// data that depends on detailed-mode room visits.
export default function MeinRaumPage() {
  return (
    <BinaryModeGuard>
      <Suspense
        fallback={
          <SkeletonRegion label="Mein Raum wird geladen">
            <PageHeaderSkeleton actions={1} />
            <CardGridSkeleton
              cards={6}
              rowsPerCard={2}
              className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3"
            />
          </SkeletonRegion>
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
