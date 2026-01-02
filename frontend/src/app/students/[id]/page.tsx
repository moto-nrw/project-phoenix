"use client";

import { useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { useSession } from "next-auth/react";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { ConfirmationModal } from "~/components/ui/modal";
import { BackButton } from "~/components/ui/back-button";
import { studentService } from "~/lib/api";
import { performImmediateCheckout } from "~/lib/scheduled-checkout-api";
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
import { StudentCheckoutSection } from "~/components/students/student-checkout-section";
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";

// =============================================================================
// ALERT MESSAGE TYPE
// =============================================================================

interface AlertMessage {
  type: "success" | "error";
  message: string;
}

// =============================================================================
// MAIN COMPONENT
// =============================================================================

export default function StudentDetailPage() {
  const router = useRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  const { data: session } = useSession();

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
  const [alertMessage, setAlertMessage] = useState<AlertMessage | null>(null);

  // Checkout states
  const [showConfirmCheckout, setShowConfirmCheckout] = useState(false);
  const [hasScheduledCheckout, setHasScheduledCheckout] = useState(false);
  const [checkingOut, setCheckingOut] = useState(false);

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

  const showTemporaryAlert = (alert: AlertMessage) => {
    setAlertMessage(alert);
    setTimeout(() => setAlertMessage(null), 3000);
  };

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
      showTemporaryAlert({
        type: "success",
        message: "Persönliche Informationen erfolgreich aktualisiert",
      });
    } catch (err) {
      console.error("Failed to save personal information:", err);
      showTemporaryAlert({
        type: "error",
        message: "Fehler beim Speichern der persönlichen Informationen",
      });
    }
  };

  const handleConfirmCheckout = async () => {
    if (!student) return;

    setCheckingOut(true);
    try {
      await performImmediateCheckout(
        Number.parseInt(studentId, 10),
        session?.user?.token,
      );
      refreshData();
      setShowConfirmCheckout(false);
      showTemporaryAlert({
        type: "success",
        message: `${student.name} wurde erfolgreich abgemeldet`,
      });
    } catch (err) {
      console.error("Failed to checkout student:", err);
      showTemporaryAlert({
        type: "error",
        message: "Fehler beim Abmelden des Kindes",
      });
    } finally {
      setCheckingOut(false);
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
            alertMessage={alertMessage}
            showCheckout={showCheckout}
            hasScheduledCheckout={hasScheduledCheckout}
            onRefreshData={refreshData}
            onScheduledCheckoutChange={setHasScheduledCheckout}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
            onStartEditing={handleStartEditing}
            onCancelEditing={handleCancelEditing}
            onStudentChange={setEditedStudent}
            onSavePersonal={handleSavePersonal}
          />
        ) : (
          <LimitedAccessView
            student={student}
            studentId={studentId}
            supervisors={supervisors}
            alertMessage={alertMessage}
            showCheckout={showCheckout}
            hasScheduledCheckout={hasScheduledCheckout}
            onRefreshData={refreshData}
            onScheduledCheckoutChange={setHasScheduledCheckout}
            onCheckoutClick={() => setShowConfirmCheckout(true)}
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
    </ResponsiveLayout>
  );
}

// =============================================================================
// LIMITED ACCESS VIEW
// =============================================================================

interface LimitedAccessViewProps {
  student: ExtendedStudent;
  studentId: string;
  supervisors: SupervisorContact[];
  alertMessage: AlertMessage | null;
  showCheckout: boolean;
  hasScheduledCheckout: boolean;
  onRefreshData: () => void;
  onScheduledCheckoutChange: (value: boolean) => void;
  onCheckoutClick: () => void;
}

function LimitedAccessView({
  student,
  studentId,
  supervisors,
  alertMessage,
  showCheckout,
  hasScheduledCheckout,
  onRefreshData,
  onScheduledCheckoutChange,
  onCheckoutClick,
}: Readonly<LimitedAccessViewProps>) {
  return (
    <>
      {alertMessage && (
        <div className="mb-6">
          <Alert type={alertMessage.type} message={alertMessage.message} />
        </div>
      )}
      <div className="space-y-4 sm:space-y-6">
        {showCheckout && (
          <StudentCheckoutSection
            studentId={studentId}
            hasScheduledCheckout={hasScheduledCheckout}
            onUpdate={onRefreshData}
            onScheduledCheckoutChange={onScheduledCheckoutChange}
            onCheckoutClick={onCheckoutClick}
          />
        )}

        <SupervisorsCard supervisors={supervisors} studentName={student.name} />

        <PersonalInfoReadOnly student={student} />

        <StudentGuardianManager
          studentId={studentId}
          readOnly={true}
          onUpdate={() => {
            // No-op for read-only mode
          }}
        />
      </div>
    </>
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
  alertMessage: AlertMessage | null;
  showCheckout: boolean;
  hasScheduledCheckout: boolean;
  onRefreshData: () => void;
  onScheduledCheckoutChange: (value: boolean) => void;
  onCheckoutClick: () => void;
  onStartEditing: () => void;
  onCancelEditing: () => void;
  onStudentChange: (student: ExtendedStudent) => void;
  onSavePersonal: () => Promise<void>;
}

function FullAccessView({
  student,
  studentId,
  editedStudent,
  isEditingPersonal,
  alertMessage,
  showCheckout,
  hasScheduledCheckout,
  onRefreshData,
  onScheduledCheckoutChange,
  onCheckoutClick,
  onStartEditing,
  onCancelEditing,
  onStudentChange,
  onSavePersonal,
}: Readonly<FullAccessViewProps>) {
  return (
    <>
      {showCheckout && (
        <StudentCheckoutSection
          studentId={studentId}
          hasScheduledCheckout={hasScheduledCheckout}
          onUpdate={onRefreshData}
          onScheduledCheckoutChange={onScheduledCheckoutChange}
          onCheckoutClick={onCheckoutClick}
        />
      )}

      {alertMessage && (
        <div className="mb-6">
          <Alert type={alertMessage.type} message={alertMessage.message} />
        </div>
      )}

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
