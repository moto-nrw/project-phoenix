"use client";

import { useState, useEffect } from "react";
import { useParams, useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { useToast } from "~/contexts/ToastContext";
import { Loading } from "~/components/ui/loading";
import { ConfirmationModal } from "~/components/ui/modal";
import { BackButton } from "~/components/ui/back-button";
import { studentService } from "~/lib/api";
import { activeService } from "~/lib/active-service";
import type { ActiveGroup } from "~/lib/active-helpers";
import {
  useStudentData,
  type ExtendedStudent,
} from "~/lib/hooks/use-student-data";
import type { SupervisorContact } from "~/lib/student-helpers";
import {
  StudentDetailHeader,
  SupervisorsCard,
  PersonalInfoReadOnly,
  StudentHistorySection,
} from "~/components/students/student-detail-components";
import { PersonalInfoFormModal } from "~/components/students/personal-info-form-modal";
import {
  StudentCheckoutSection,
  StudentCheckinSection,
  StudentSickReportSection,
  StudentExcusedReportSection,
  getStudentActionType,
} from "~/components/students/student-checkout-section";
import { performImmediateCheckin } from "~/lib/checkin-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentDetailPage" });
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";
import PickupScheduleManager from "~/components/students/pickup-schedule-manager";
import { ArrivalScheduleManager } from "~/components/students/arrival-schedule-manager";
import { fetchStudentPickupData } from "~/lib/pickup-schedule-api";
import { getDayData, formatPickupTime } from "~/lib/pickup-schedule-helpers";
import { fetchArrivalData } from "~/lib/student-arrival-api";
import {
  getDayData as getArrivalDayData,
  formatArrivalTime,
} from "~/lib/arrival-schedule-helpers";

// =============================================================================
// MAIN COMPONENT
// =============================================================================

export default function StudentDetailPage() {
  const router = useTenantRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  const toast = useToast();

  // Use custom hook for data fetching
  const {
    student,
    loading,
    error,
    hasFullAccess,
    hasWriteAccess,
    attendanceLogEnabled,
    feedbackEnabled,
    supervisors,
    myGroups,
    myGroupRooms,
    mySupervisedRooms,
    refreshData,
  } = useStudentData(studentId);

  // Set breadcrumb data — include group/room name for 3-level breadcrumb
  // when navigating from an accordion section (e.g. Meine Gruppe > 1a > Mia Fischer)
  const breadcrumbGroupName =
    referrer.startsWith("/ogs-groups") && globalThis.window !== undefined
      ? localStorage.getItem("sidebar-last-group-name")
      : undefined;
  const breadcrumbRoomName =
    referrer.startsWith("/active-supervisions") &&
    globalThis.window !== undefined
      ? localStorage.getItem("sidebar-last-room-name")
      : undefined;

  useSetBreadcrumb({
    studentName: student?.name,
    referrerPage: referrer,
    ogsGroupName: breadcrumbGroupName ?? undefined,
    activeSupervisionName: breadcrumbRoomName ?? undefined,
  });

  // Personal info modal state
  const [showPersonalInfoModal, setShowPersonalInfoModal] = useState(false);

  // Checkout states
  const [showConfirmCheckout, setShowConfirmCheckout] = useState(false);
  const [checkingOut, setCheckingOut] = useState(false);

  // Check-in states
  const [showConfirmCheckin, setShowConfirmCheckin] = useState(false);
  const [checkingIn, setCheckingIn] = useState(false);

  // Sick toggle state
  const [showConfirmSick, setShowConfirmSick] = useState(false);
  const [sickLoading, setSickLoading] = useState(false);
  const sickConfirmText = sickLoading
    ? "Wird gespeichert..."
    : student?.sick
      ? "Gesundmelden"
      : "Krankmelden";

  // Excused toggle state
  const [showConfirmExcused, setShowConfirmExcused] = useState(false);
  const [excusedLoading, setExcusedLoading] = useState(false);
  const excusedConfirmText = excusedLoading
    ? "Wird gespeichert..."
    : student?.excused
      ? "Entschuldigung aufheben"
      : "Entschuldigen";

  // Switch dialog: shown when the user clicks one flag but the other is set.
  // "sick" = we want to switch TO sick (excused is currently true).
  // "excused" = we want to switch TO excused (sick is currently true).
  const [switchTarget, setSwitchTarget] = useState<"sick" | "excused" | null>(
    null,
  );
  const [switchLoading, setSwitchLoading] = useState(false);
  const [selectedActiveGroupId, setSelectedActiveGroupId] =
    useState<string>("");
  const [activeGroups, setActiveGroups] = useState<ActiveGroup[]>([]);
  const [loadingActiveGroups, setLoadingActiveGroups] = useState(false);

  // Today's pickup info (for header display)
  const [todayPickup, setTodayPickup] = useState<{
    time?: string;
    note?: string;
    isException?: boolean;
  }>({});

  // Today's arrival info (for header display)
  const [todayArrival, setTodayArrival] = useState<{
    time?: string;
    note?: string;
    isException?: boolean;
    isAbsent?: boolean;
  }>({});

  // Load active groups when check-in modal opens
  useEffect(() => {
    if (!showConfirmCheckin) {
      // Reset state when modal closes
      setSelectedActiveGroupId("");
      return;
    }

    const loadActiveGroups = async () => {
      setLoadingActiveGroups(true);
      try {
        const groups = await activeService.getActiveGroups({ active: true });
        // Filter to only groups with rooms
        const groupsWithRooms = groups.filter((g) => g.room?.name);
        setActiveGroups(groupsWithRooms);
      } catch (err) {
        logger.error("failed to load active groups", {
          error: err instanceof Error ? err.message : String(err),
        });
        setActiveGroups([]);
      } finally {
        setLoadingActiveGroups(false);
      }
    };

    void loadActiveGroups();
  }, [showConfirmCheckin]);

  // Load today's pickup time for header (requires read access to student data)
  useEffect(() => {
    if (!hasFullAccess || !studentId) {
      setTodayPickup({});
      return;
    }

    const loadTodayPickup = async () => {
      try {
        const data = await fetchStudentPickupData(studentId);
        const today = new Date();

        const dayData = getDayData(
          today,
          data.schedules,
          data.exceptions,
          student?.sick ?? false,
          data.notes,
          student?.excused ?? false,
        );

        if (dayData.effectiveTime) {
          setTodayPickup({
            time: formatPickupTime(dayData.effectiveTime),
            note: dayData.effectiveNotes,
            isException: dayData.isException,
          });
        } else {
          setTodayPickup({});
        }
      } catch {
        // Silently handle errors (e.g., permission denied for non-full-access users)
        setTodayPickup({});
      }
    };

    void loadTodayPickup();
  }, [hasFullAccess, studentId, student?.sick, student?.excused]);

  // Load today's arrival time for header (requires read access to student data)
  useEffect(() => {
    if (!hasFullAccess || !studentId) {
      setTodayArrival({});
      return;
    }

    const loadTodayArrival = async () => {
      try {
        const data = await fetchArrivalData(studentId);
        const today = new Date();

        const dayData = getArrivalDayData(
          today,
          data.schedules,
          data.exceptions,
          data.notes,
          student?.sick ?? false,
          student?.excused ?? false,
        );

        if (dayData.isAbsent) {
          setTodayArrival({
            note: dayData.effectiveReason,
            isException: dayData.isException,
            isAbsent: true,
          });
        } else if (dayData.effectiveTime) {
          setTodayArrival({
            time: formatArrivalTime(dayData.effectiveTime),
            note: dayData.effectiveReason,
            isException: dayData.isException,
            isAbsent: false,
          });
        } else {
          setTodayArrival({});
        }
      } catch {
        setTodayArrival({});
      }
    };

    void loadTodayArrival();
  }, [hasFullAccess, studentId, student?.sick, student?.excused]);

  // Show loading state
  if (loading) {
    return <Loading message="Laden..." fullPage={false} />;
  }

  // Show error state
  if (error || !student) {
    return (
      <div className="flex min-h-[80vh] flex-col items-center justify-center">
        <Alert type="error" message={error ?? "Schüler nicht gefunden"} />
        <button
          onClick={() => router.push(referrer)}
          className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
        >
          Zurück
        </button>
      </div>
    );
  }

  // =============================================================================
  // EVENT HANDLERS
  // =============================================================================

  const handleSavePersonal = async (editedStudent: ExtendedStudent) => {
    await studentService.updateStudent(studentId, {
      first_name: editedStudent.first_name,
      second_name: editedStudent.second_name,
      school_class: editedStudent.school_class,
      birthday: editedStudent.birthday,
      bus: editedStudent.buskind ?? false,
      health_info: editedStudent.health_info,
      supervisor_notes: editedStudent.supervisor_notes,
      extra_info: editedStudent.extra_info,
      pickup_status: editedStudent.pickup_status,
    });

    refreshData();
    toast.success("Persönliche Informationen erfolgreich aktualisiert");
  };

  const handleConfirmCheckout = async () => {
    if (!student) return;

    setCheckingOut(true);
    try {
      // Use dedicated checkout endpoint which:
      // 1. Ends current visit (if any)
      // 2. Toggles attendance to checked_out (daily checkout)
      await activeService.checkoutStudent(studentId);
      refreshData();
      setShowConfirmCheckout(false);
      toast.success(`${student.name} wurde erfolgreich abgemeldet`);
    } catch (err) {
      logger.error("failed to checkout student", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Abmelden des Kindes");
    } finally {
      setCheckingOut(false);
    }
  };

  const handleConfirmCheckin = async () => {
    if (!student || !selectedActiveGroupId) return;

    setCheckingIn(true);
    try {
      await performImmediateCheckin(
        Number.parseInt(studentId, 10),
        Number.parseInt(selectedActiveGroupId, 10),
      );
      refreshData();
      setShowConfirmCheckin(false);
      toast.success(`${student.name} wurde erfolgreich angemeldet`);
    } catch (err) {
      logger.error("failed to check in student", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Anmelden des Kindes");
    } finally {
      setCheckingIn(false);
    }
  };

  const handleConfirmSickToggle = async () => {
    if (!student) return;

    setSickLoading(true);
    try {
      const newSickStatus = !(student.sick ?? false);
      await studentService.updateStudent(studentId, {
        sick: newSickStatus,
      });
      refreshData();
      setShowConfirmSick(false);
      toast.success(
        newSickStatus
          ? `${student.name} wurde krankgemeldet`
          : `Krankmeldung für ${student.name} wurde aufgehoben`,
      );
    } catch (err) {
      logger.error("sick_status_toggle_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Ändern des Krankheitsstatus");
    } finally {
      setSickLoading(false);
    }
  };

  const handleConfirmExcusedToggle = async () => {
    if (!student) return;

    setExcusedLoading(true);
    try {
      const newExcusedStatus = !(student.excused ?? false);
      await studentService.updateStudent(studentId, {
        excused: newExcusedStatus,
      });
      refreshData();
      setShowConfirmExcused(false);
      toast.success(
        newExcusedStatus
          ? `${student.name} wurde als entschuldigt markiert`
          : `Entschuldigung für ${student.name} wurde aufgehoben`,
      );
    } catch (err) {
      logger.error("excused_status_toggle_failed", {
        student_id: studentId,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Ändern des Entschuldigungsstatus");
    } finally {
      setExcusedLoading(false);
    }
  };

  // Click interceptor for the Krank button. If the student is currently
  // excused, we must first clear the excused flag — show the switch dialog
  // instead of the normal confirm modal.
  const handleSickClick = () => {
    if (student?.sick) {
      setShowConfirmSick(true);
      return;
    }
    if (student?.excused) {
      setSwitchTarget("sick");
      return;
    }
    setShowConfirmSick(true);
  };

  const handleExcusedClick = () => {
    if (student?.excused) {
      setShowConfirmExcused(true);
      return;
    }
    if (student?.sick) {
      setSwitchTarget("excused");
      return;
    }
    setShowConfirmExcused(true);
  };

  const handleConfirmSwitch = async () => {
    if (!student || !switchTarget) return;

    setSwitchLoading(true);
    try {
      // Send both flags in one request so the backend's mutual-exclusion guard
      // sees only the final state (one true, the other explicitly false).
      await studentService.updateStudent(studentId, {
        sick: switchTarget === "sick",
        excused: switchTarget === "excused",
      });
      refreshData();
      toast.success(
        switchTarget === "sick"
          ? `${student.name} wurde krankgemeldet (Entschuldigung aufgehoben)`
          : `${student.name} wurde entschuldigt (Krankmeldung aufgehoben)`,
      );
      setSwitchTarget(null);
    } catch (err) {
      logger.error("status_switch_failed", {
        student_id: studentId,
        target: switchTarget,
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Fehler beim Wechseln des Status");
    } finally {
      setSwitchLoading(false);
    }
  };

  // =============================================================================
  // COMPUTED VALUES
  // =============================================================================

  // Determine what action is available based on access (group membership / room supervision)
  const studentActionType = getStudentActionType(
    { group_id: student.group_id, current_location: student.current_location },
    myGroups,
    mySupervisedRooms,
  );
  const showCheckout = studentActionType === "checkout";
  const showCheckin = studentActionType === "checkin";

  // =============================================================================
  // RENDER HELPERS
  // =============================================================================

  const renderRoomSelector = () => {
    if (loadingActiveGroups) {
      return (
        <div className="flex items-center gap-2 text-sm text-gray-500">
          <svg className="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
            <circle
              className="opacity-25"
              cx="12"
              cy="12"
              r="10"
              stroke="currentColor"
              strokeWidth="4"
            />
            <path
              className="opacity-75"
              fill="currentColor"
              d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
            />
          </svg>
          Räume werden geladen...
        </div>
      );
    }

    if (activeGroups.length === 0) {
      return (
        <p className="text-sm text-amber-600">
          Keine aktiven Räume verfügbar. Bitte starten Sie zuerst eine aktive
          Aufsicht in einem Raum über ein NFC-Tablet.
        </p>
      );
    }

    return (
      <div className="relative">
        <select
          id="room-select"
          value={selectedActiveGroupId}
          onChange={(e) => setSelectedActiveGroupId(e.target.value)}
          className="w-full appearance-none rounded-lg border border-gray-300 bg-white px-3 py-2 pr-10 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
        >
          <option value="">Bitte Raum auswählen...</option>
          {activeGroups.map((group) => (
            <option key={group.id} value={group.id}>
              {group.room?.name ?? "Unbekannter Raum"} (
              {group.actualGroup?.name ?? "Gruppe"})
            </option>
          ))}
        </select>
        <svg
          className="pointer-events-none absolute top-1/2 right-3 h-4 w-4 -translate-y-1/2 text-gray-400"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </div>
    );
  };

  // =============================================================================
  // RENDER
  // =============================================================================

  return (
    <>
      <div className="mx-auto max-w-7xl">
        <BackButton referrer={referrer} />

        <StudentDetailHeader
          student={student}
          myGroups={myGroups}
          myGroupRooms={myGroupRooms}
          mySupervisedRooms={mySupervisedRooms}
          todayPickupTime={todayPickup.time}
          todayPickupNote={todayPickup.note}
          isPickupException={todayPickup.isException}
          todayArrivalTime={todayArrival.time}
          todayArrivalNote={todayArrival.note}
          isArrivalException={todayArrival.isException}
          isArrivalAbsent={todayArrival.isAbsent}
        />

        {hasFullAccess ? (
          <FullAccessView
            student={student}
            studentId={studentId}
            hasWriteAccess={hasWriteAccess}
            attendanceLogEnabled={attendanceLogEnabled}
            feedbackEnabled={feedbackEnabled}
            showCheckout={showCheckout}
            showCheckin={showCheckin}
            showPersonalInfoModal={showPersonalInfoModal}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
            onCheckinClick={() => setShowConfirmCheckin(true)}
            onOpenPersonalInfoModal={() => setShowPersonalInfoModal(true)}
            onClosePersonalInfoModal={() => setShowPersonalInfoModal(false)}
            onSavePersonal={handleSavePersonal}
            onRefreshData={refreshData}
            onSickClick={handleSickClick}
            sickLoading={sickLoading}
            onExcusedClick={handleExcusedClick}
            excusedLoading={excusedLoading}
          />
        ) : (
          <LimitedAccessView
            student={student}
            studentId={studentId}
            attendanceLogEnabled={attendanceLogEnabled}
            feedbackEnabled={feedbackEnabled}
            supervisors={supervisors}
            showCheckout={showCheckout}
            showCheckin={showCheckin}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
            onCheckinClick={() => setShowConfirmCheckin(true)}
          />
        )}
      </div>

      {/* Checkout Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmCheckout}
        onClose={() => setShowConfirmCheckout(false)}
        onConfirm={handleConfirmCheckout}
        title="Kind abmelden"
        confirmText={checkingOut ? "Wird abgemeldet..." : "Geht nach Hause"}
        cancelText="Abbrechen"
        isConfirmLoading={checkingOut}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          Möchten Sie <strong>{student.name}</strong> jetzt abmelden?
        </p>
      </ConfirmationModal>

      {/* Checkin Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmCheckin}
        onClose={() => setShowConfirmCheckin(false)}
        onConfirm={handleConfirmCheckin}
        title="Kind anmelden"
        confirmText={checkingIn ? "Wird angemeldet..." : "Anmelden"}
        cancelText="Abbrechen"
        isConfirmLoading={checkingIn}
        isConfirmDisabled={!selectedActiveGroupId}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <div className="space-y-4">
          <p>
            Möchten Sie <strong>{student.name}</strong> jetzt anmelden?
          </p>
          <div>
            <label
              htmlFor="room-select"
              className="mb-2 block text-sm font-medium text-gray-700"
            >
              Raum auswählen
            </label>
            {renderRoomSelector()}
          </div>
        </div>
      </ConfirmationModal>

      {/* Sick Report Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmSick}
        onClose={() => setShowConfirmSick(false)}
        onConfirm={handleConfirmSickToggle}
        title={student.sick ? "Krankmeldung aufheben" : "Kind krankmelden"}
        confirmText={sickConfirmText}
        cancelText="Abbrechen"
        isConfirmLoading={sickLoading}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          {student.sick ? (
            <>
              Möchten Sie die Krankmeldung für <strong>{student.name}</strong>{" "}
              aufheben?
            </>
          ) : (
            <>
              Möchten Sie <strong>{student.name}</strong> als krank melden?
            </>
          )}
        </p>
      </ConfirmationModal>

      {/* Excused Confirmation Modal */}
      <ConfirmationModal
        isOpen={showConfirmExcused}
        onClose={() => setShowConfirmExcused(false)}
        onConfirm={handleConfirmExcusedToggle}
        title={
          student.excused ? "Entschuldigung aufheben" : "Kind entschuldigen"
        }
        confirmText={excusedConfirmText}
        cancelText="Abbrechen"
        isConfirmLoading={excusedLoading}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          {student.excused ? (
            <>
              Möchten Sie die Entschuldigung für <strong>{student.name}</strong>{" "}
              aufheben?
            </>
          ) : (
            <>
              Möchten Sie <strong>{student.name}</strong> als entschuldigt
              markieren?
            </>
          )}
        </p>
      </ConfirmationModal>

      {/* Switch Dialog — shown when user clicks one flag but the other is set */}
      <ConfirmationModal
        isOpen={switchTarget !== null}
        onClose={() => setSwitchTarget(null)}
        onConfirm={handleConfirmSwitch}
        title={
          switchTarget === "sick"
            ? "Als krank melden?"
            : "Als entschuldigt markieren?"
        }
        confirmText={switchLoading ? "Wird gewechselt..." : "Status wechseln"}
        cancelText="Abbrechen"
        isConfirmLoading={switchLoading}
        confirmButtonClass="bg-gray-900 hover:bg-gray-700"
      >
        <p>
          {switchTarget === "sick" ? (
            <>
              <strong>{student.name}</strong> ist aktuell als entschuldigt
              markiert. Stattdessen als krank melden? Die Entschuldigung wird
              dabei aufgehoben.
            </>
          ) : (
            <>
              <strong>{student.name}</strong> ist aktuell als krank gemeldet.
              Stattdessen als entschuldigt markieren? Die Krankmeldung wird
              dabei aufgehoben.
            </>
          )}
        </p>
      </ConfirmationModal>
    </>
  );
}

// =============================================================================
// LIMITED ACCESS VIEW
// =============================================================================

interface LimitedAccessViewProps {
  student: ExtendedStudent;
  studentId: string;
  attendanceLogEnabled: boolean;
  feedbackEnabled: boolean;
  supervisors: SupervisorContact[];
  showCheckout: boolean;
  showCheckin: boolean;
  onCheckoutClick: () => void;
  onCheckinClick: () => void;
}

function LimitedAccessView({
  student,
  studentId,
  attendanceLogEnabled,
  feedbackEnabled,
  supervisors,
  showCheckout,
  showCheckin,
  onCheckoutClick,
  onCheckinClick,
}: Readonly<LimitedAccessViewProps>) {
  const historyRouter = useTenantRouter();
  return (
    <div className="space-y-4 sm:space-y-6">
      {(showCheckout || showCheckin) && (
        <div className="flex gap-3 sm:gap-4">
          {showCheckout && (
            <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
          )}
          {showCheckin && (
            <StudentCheckinSection onCheckinClick={onCheckinClick} />
          )}
        </div>
      )}

      <SupervisorsCard supervisors={supervisors} studentName={student.name} />

      <PersonalInfoReadOnly student={student} />

      <StudentGuardianManager studentId={student.id} readOnly={true} />

      <StudentHistorySection
        studentId={studentId}
        attendanceLogEnabled={attendanceLogEnabled}
        feedbackEnabled={feedbackEnabled}
        readOnly={true}
        onNavigate={(path) => historyRouter.push(path)}
      />
    </div>
  );
}

// =============================================================================
// FULL ACCESS VIEW
// =============================================================================

interface FullAccessViewProps {
  student: ExtendedStudent;
  studentId: string;
  hasWriteAccess: boolean;
  attendanceLogEnabled: boolean;
  feedbackEnabled: boolean;
  showCheckout: boolean;
  showCheckin: boolean;
  showPersonalInfoModal: boolean;
  onCheckoutClick: () => void;
  onCheckinClick: () => void;
  onOpenPersonalInfoModal: () => void;
  onClosePersonalInfoModal: () => void;
  onSavePersonal: (student: ExtendedStudent) => Promise<void>;
  onRefreshData: () => void;
  onSickClick: () => void;
  sickLoading: boolean;
  onExcusedClick: () => void;
  excusedLoading: boolean;
}

function FullAccessView({
  student,
  studentId,
  hasWriteAccess,
  attendanceLogEnabled,
  feedbackEnabled,
  showCheckout,
  showCheckin,
  showPersonalInfoModal,
  onCheckoutClick,
  onCheckinClick,
  onOpenPersonalInfoModal,
  onClosePersonalInfoModal,
  onSavePersonal,
  onRefreshData,
  onSickClick,
  sickLoading,
  onExcusedClick,
  excusedLoading,
}: Readonly<FullAccessViewProps>) {
  const historyRouter = useTenantRouter();
  return (
    <>
      {(showCheckout || showCheckin || hasWriteAccess) && (
        <div className="mb-4 flex gap-3 sm:mb-6 sm:gap-4">
          {showCheckout && (
            <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
          )}
          {showCheckin && (
            <StudentCheckinSection onCheckinClick={onCheckinClick} />
          )}
          {hasWriteAccess && (
            <StudentSickReportSection
              isSick={student.sick ?? false}
              sickSince={student.sick_since}
              onToggle={onSickClick}
              isLoading={sickLoading}
            />
          )}
          {hasWriteAccess && (
            <StudentExcusedReportSection
              isExcused={student.excused ?? false}
              excusedSince={student.excused_since}
              onToggle={onExcusedClick}
              isLoading={excusedLoading}
            />
          )}
        </div>
      )}

      <div className="space-y-4 sm:space-y-6">
        <ArrivalScheduleManager
          studentId={studentId}
          readOnly={!hasWriteAccess}
          onUpdate={hasWriteAccess ? onRefreshData : undefined}
        />

        <PickupScheduleManager
          studentId={studentId}
          readOnly={!hasWriteAccess}
          onUpdate={hasWriteAccess ? onRefreshData : undefined}
          isSick={student.sick}
          isExcused={student.excused}
        />

        <PersonalInfoReadOnly
          student={student}
          showEditButton={hasWriteAccess}
          onEditClick={hasWriteAccess ? onOpenPersonalInfoModal : undefined}
        />

        <StudentGuardianManager
          studentId={studentId}
          readOnly={!hasWriteAccess}
          onUpdate={hasWriteAccess ? onRefreshData : undefined}
        />

        <StudentHistorySection
          studentId={studentId}
          attendanceLogEnabled={attendanceLogEnabled}
          feedbackEnabled={feedbackEnabled}
          onNavigate={(path) => historyRouter.push(path)}
        />
      </div>

      {hasWriteAccess && (
        <PersonalInfoFormModal
          isOpen={showPersonalInfoModal}
          onClose={onClosePersonalInfoModal}
          student={student}
          onSave={onSavePersonal}
        />
      )}
    </>
  );
}
