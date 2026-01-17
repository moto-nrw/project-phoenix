"use client";

import { useState, useEffect } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
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
  shouldShowCheckoutSection,
  type ExtendedStudent,
} from "~/lib/hooks/use-student-data";
import type { SupervisorContact } from "~/lib/student-helpers";
import {
  StudentDetailHeader,
  SupervisorsCard,
  PersonalInfoReadOnly,
  FullAccessPersonalInfoReadOnly,
  StudentHistorySection,
} from "~/components/students/student-detail-components";
import { PersonalInfoEditForm } from "~/components/students/student-personal-info-form";
import {
  StudentCheckoutSection,
  StudentCheckinSection,
  getStudentActionType,
} from "~/components/students/student-checkout-section";
import { performImmediateCheckin } from "~/lib/checkin-api";
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";

// =============================================================================
// MAIN COMPONENT
// =============================================================================

export default function StudentDetailPage() {
  const router = useRouter();
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
    supervisors,
    myGroups,
    myGroupRooms,
    mySupervisedRooms,
    refreshData,
  } = useStudentData(studentId);

  // Edit mode states
  const [isEditingPersonal, setIsEditingPersonal] = useState(false);
  const [editedStudent, setEditedStudent] = useState<ExtendedStudent | null>(
    null,
  );

  // Checkout states
  const [showConfirmCheckout, setShowConfirmCheckout] = useState(false);
  const [checkingOut, setCheckingOut] = useState(false);

  // Check-in states
  const [showConfirmCheckin, setShowConfirmCheckin] = useState(false);
  const [checkingIn, setCheckingIn] = useState(false);
  const [selectedActiveGroupId, setSelectedActiveGroupId] =
    useState<string>("");
  const [activeGroups, setActiveGroups] = useState<ActiveGroup[]>([]);
  const [loadingActiveGroups, setLoadingActiveGroups] = useState(false);

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
        console.error("Failed to load active groups:", err);
        setActiveGroups([]);
      } finally {
        setLoadingActiveGroups(false);
      }
    };

    void loadActiveGroups();
  }, [showConfirmCheckin]);

  // Show loading state
  if (loading) {
    return (
      <ResponsiveLayout referrerPage={referrer} studentName="...">
        <Loading message="Laden..." fullPage={false} />
      </ResponsiveLayout>
    );
  }

  // Show error state
  if (error || !student) {
    return (
      <ResponsiveLayout referrerPage={referrer}>
        <div className="flex min-h-[80vh] flex-col items-center justify-center">
          <Alert type="error" message={error ?? "Schüler nicht gefunden"} />
          <button
            onClick={() => router.push(referrer)}
            className="mt-4 rounded bg-blue-100 px-4 py-2 text-blue-800 transition-colors hover:bg-blue-200"
          >
            Zurück
          </button>
        </div>
      </ResponsiveLayout>
    );
  }

  // =============================================================================
  // EVENT HANDLERS
  // =============================================================================

  const handleSavePersonal = async () => {
    if (!editedStudent) return;

    try {
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
        sick: editedStudent.sick ?? false,
      });

      refreshData();
      setIsEditingPersonal(false);
      toast.success("Persönliche Informationen erfolgreich aktualisiert");
    } catch (err) {
      console.error("Failed to save personal information:", err);
      toast.error("Fehler beim Speichern der persönlichen Informationen");
    }
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
      console.error("Failed to checkout student:", err);
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
      console.error("Failed to check in student:", err);
      toast.error("Fehler beim Anmelden des Kindes");
    } finally {
      setCheckingIn(false);
    }
  };

  const handleStartEditing = () => {
    setIsEditingPersonal(true);
    setEditedStudent(student);
  };

  const handleCancelEditing = () => {
    setIsEditingPersonal(false);
    setEditedStudent(student);
  };

  // =============================================================================
  // COMPUTED VALUES
  // =============================================================================

  const showCheckout = shouldShowCheckoutSection(
    student,
    myGroups,
    mySupervisedRooms,
  );

  // Determine if check-in should be shown (student is at home and user has access)
  const studentActionType = getStudentActionType(
    { group_id: student.group_id, current_location: student.current_location },
    myGroups,
    mySupervisedRooms,
  );
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
          Keine aktiven Räume verfügbar. Bitte starten Sie zuerst eine
          Gruppensitzung.
        </p>
      );
    }

    return (
      <select
        id="room-select"
        value={selectedActiveGroupId}
        onChange={(e) => setSelectedActiveGroupId(e.target.value)}
        className="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-blue-500 focus:outline-none"
      >
        <option value="">Bitte Raum auswählen...</option>
        {activeGroups.map((group) => (
          <option key={group.id} value={group.id}>
            {group.room?.name ?? "Unbekannter Raum"} (
            {group.actualGroup?.name ?? "Gruppe"})
          </option>
        ))}
      </select>
    );
  };

  // =============================================================================
  // RENDER
  // =============================================================================

  return (
    <ResponsiveLayout studentName={student.name} referrerPage={referrer}>
      <div className="mx-auto max-w-7xl">
        <BackButton referrer={referrer} />

        <StudentDetailHeader
          student={student}
          myGroups={myGroups}
          myGroupRooms={myGroupRooms}
          mySupervisedRooms={mySupervisedRooms}
        />

        {hasFullAccess ? (
          <FullAccessView
            student={student}
            studentId={studentId}
            editedStudent={editedStudent}
            isEditingPersonal={isEditingPersonal}
            showCheckout={showCheckout}
            showCheckin={showCheckin}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
            onCheckinClick={() => setShowConfirmCheckin(true)}
            onStartEditing={handleStartEditing}
            onCancelEditing={handleCancelEditing}
            onStudentChange={setEditedStudent}
            onSavePersonal={handleSavePersonal}
            onRefreshData={refreshData}
          />
        ) : (
          <LimitedAccessView
            student={student}
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
        confirmText={checkingOut ? "Wird abgemeldet..." : "Abmelden"}
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
    </ResponsiveLayout>
  );
}

// =============================================================================
// LIMITED ACCESS VIEW
// =============================================================================

interface LimitedAccessViewProps {
  student: ExtendedStudent;
  supervisors: SupervisorContact[];
  showCheckout: boolean;
  showCheckin: boolean;
  onCheckoutClick: () => void;
  onCheckinClick: () => void;
}

function LimitedAccessView({
  student,
  supervisors,
  showCheckout,
  showCheckin,
  onCheckoutClick,
  onCheckinClick,
}: Readonly<LimitedAccessViewProps>) {
  return (
    <div className="space-y-4 sm:space-y-6">
        {showCheckout && (
          <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
        )}
        {showCheckin && (
          <StudentCheckinSection onCheckinClick={onCheckinClick} />
        )}

      <SupervisorsCard supervisors={supervisors} studentName={student.name} />

      <PersonalInfoReadOnly student={student} />
    </div>
  );
}

// =============================================================================
// FULL ACCESS VIEW
// =============================================================================

interface FullAccessViewProps {
  student: ExtendedStudent;
  studentId: string;
  editedStudent: ExtendedStudent | null;
  isEditingPersonal: boolean;
  showCheckout: boolean;
  showCheckin: boolean;
  onCheckoutClick: () => void;
  onCheckinClick: () => void;
  onStartEditing: () => void;
  onCancelEditing: () => void;
  onStudentChange: (student: ExtendedStudent) => void;
  onSavePersonal: () => Promise<void>;
  onRefreshData: () => void;
}

function FullAccessView({
  student,
  studentId,
  editedStudent,
  isEditingPersonal,
  showCheckout,
  showCheckin,
  onCheckoutClick,
  onCheckinClick,
  onStartEditing,
  onCancelEditing,
  onStudentChange,
  onSavePersonal,
  onRefreshData,
}: Readonly<FullAccessViewProps>) {
  return (
    <>
      {showCheckout && (
        <StudentCheckoutSection onCheckoutClick={onCheckoutClick} />
      )}
      {showCheckin && <StudentCheckinSection onCheckinClick={onCheckinClick} />}

      <StudentHistorySection />

      <div className="mt-4 space-y-4 sm:mt-6 sm:space-y-6">
        {isEditingPersonal && editedStudent ? (
          <PersonalInfoEditForm
            editedStudent={editedStudent}
            onStudentChange={onStudentChange}
            onSave={onSavePersonal}
            onCancel={onCancelEditing}
          />
        ) : (
          <FullAccessPersonalInfoReadOnly
            student={student}
            onEditClick={onStartEditing}
          />
        )}

        <StudentGuardianManager
          studentId={studentId}
          readOnly={false}
          onUpdate={onRefreshData}
        />
      </div>
    </>
  );
}
