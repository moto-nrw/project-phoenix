"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { useSession } from "next-auth/react";
import { studentService } from "~/lib/api";
import type { Student, SupervisorContact } from "~/lib/student-helpers";
import { ModernContactActions } from "~/components/simple/student";
import { ScheduledCheckoutModal } from "~/components/scheduled-checkout/scheduled-checkout-modal";
import { ScheduledCheckoutInfo } from "~/components/scheduled-checkout/scheduled-checkout-info";
import { userContextService } from "~/lib/usercontext-api";
import { LocationBadge } from "@/components/ui/location-badge";
import StudentGuardianManager from "~/components/guardians/student-guardian-manager";

// Extended Student type for this page
interface ExtendedStudent extends Student {
  bus: boolean;
  current_room?: string;
  birthday?: string;
  buskind?: boolean;
  attendance_rate?: number;
  extra_info?: string;
  supervisor_notes?: string;
  health_info?: string;
}

// Mobile-optimized info card component
function InfoCard({
  title,
  children,
  icon,
}: {
  title: string;
  children: React.ReactNode;
  icon: React.ReactNode;
}) {
  return (
    <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
          {icon}
        </div>
        <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
          {title}
        </h2>
      </div>
      <div className="space-y-3">{children}</div>
    </div>
  );
}

// Simplified info item component
function InfoItem({
  label,
  value,
  icon,
}: {
  label: string;
  value: string | React.ReactNode;
  icon?: React.ReactNode;
}) {
  return (
    <div className="flex items-start gap-3">
      {icon && (
        <div className="mt-0.5 flex-shrink-0 text-gray-400">
          <div className="h-4 w-4">{icon}</div>
        </div>
      )}
      <div className="min-w-0 flex-1">
        <p className="mb-1 text-xs text-gray-500">{label}</p>
        <div className="text-sm font-medium text-gray-900">{value}</div>
      </div>
    </div>
  );
}

export default function StudentDetailPage() {
  const router = useRouter();
  const params = useParams();
  const searchParams = useSearchParams();
  const studentId = params.id as string;
  const referrer = searchParams.get("from") ?? "/students/search";
  const { data: session } = useSession();

  const [student, setStudent] = useState<ExtendedStudent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasFullAccess, setHasFullAccess] = useState(true);
  const [supervisors, setSupervisors] = useState<SupervisorContact[]>([]);
  const [showCheckoutModal, setShowCheckoutModal] = useState(false);
  const [checkoutUpdated, setCheckoutUpdated] = useState(0);
  const [hasScheduledCheckout, setHasScheduledCheckout] = useState(false);
  const [myGroups, setMyGroups] = useState<string[]>([]);

  // Edit mode states
  const [isEditingPersonal, setIsEditingPersonal] = useState(false);
  const [editedStudent, setEditedStudent] = useState<ExtendedStudent | null>(
    null,
  );
  const [alertMessage, setAlertMessage] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  // Fetch student data
  useEffect(() => {
    const fetchStudent = async () => {
      setLoading(true);
      setError(null);

      try {
        const response = await studentService.getStudent(studentId);

        interface WrappedResponse {
          data?: unknown;
          success?: boolean;
          message?: string;
        }
        const wrappedResponse = response as WrappedResponse;
        const studentData = wrappedResponse.data ?? response;

        const mappedStudent = studentData as Student & {
          has_full_access?: boolean;
          group_supervisors?: SupervisorContact[];
        };

        const hasAccess = mappedStudent.has_full_access ?? true;
        const groupSupervisors = mappedStudent.group_supervisors ?? [];

        const extendedStudent: ExtendedStudent = {
          id: mappedStudent.id,
          first_name: mappedStudent.first_name ?? "",
          second_name: mappedStudent.second_name ?? "",
          name: mappedStudent.name,
          school_class: mappedStudent.school_class,
          group_id: mappedStudent.group_id ?? "",
          group_name: mappedStudent.group_name ?? "",
          current_location: mappedStudent.current_location,
          bus: mappedStudent.bus ?? false,
          current_room: undefined,
          birthday: mappedStudent.birthday ?? undefined,
          buskind: mappedStudent.bus ?? false,
          attendance_rate: undefined,
          extra_info: hasAccess
            ? (mappedStudent.extra_info ?? undefined)
            : undefined,
          supervisor_notes: hasAccess
            ? (mappedStudent.supervisor_notes ?? undefined)
            : undefined,
          health_info: hasAccess
            ? (mappedStudent.health_info ?? undefined)
            : undefined,
        };

        setStudent(extendedStudent);
        setEditedStudent(extendedStudent);
        setHasFullAccess(hasAccess);
        setSupervisors(groupSupervisors);

        setLoading(false);
      } catch (err) {
        console.error("Error fetching student:", err);
        setError("Fehler beim Laden der Schülerdaten.");
        setLoading(false);
      }
    };

    void fetchStudent();
  }, [studentId, checkoutUpdated]);

  useEffect(() => {
    const loadMyGroups = async () => {
      if (!session?.user?.token) {
        setMyGroups([]);
        return;
      }

      try {
        const groups = await userContextService.getMyEducationalGroups();
        setMyGroups(groups.map((group) => group.id));
      } catch (err) {
        console.error("Error loading supervisor groups:", err);
      }
    };

    void loadMyGroups();
  }, [session?.user?.token]);

  // Handle save for personal information
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
      });

      setStudent(editedStudent);
      setIsEditingPersonal(false);
      setAlertMessage({
        type: "success",
        message: "Persönliche Informationen erfolgreich aktualisiert",
      });
      setTimeout(() => setAlertMessage(null), 3000);
    } catch (error) {
      console.error("Failed to save personal information:", error);
      setAlertMessage({
        type: "error",
        message: "Fehler beim Speichern der persönlichen Informationen",
      });
      setTimeout(() => setAlertMessage(null), 3000);
    }
  };


  if (loading) {
    return (
      <ResponsiveLayout referrerPage={referrer} studentName="...">
        <Loading message="Laden..." fullPage={false} />
      </ResponsiveLayout>
    );
  }

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

  const badgeStudent = {
    current_location: student.current_location,
    group_id: student.group_id,
    group_name: student.group_name,
  };

  return (
    <ResponsiveLayout studentName={student.name} referrerPage={referrer}>
      <div className="mx-auto max-w-7xl px-4 pb-6 sm:px-6 lg:px-8">
        {/* Back button - Mobile optimized */}
        <button
          onClick={() => router.push(referrer)}
          className="mb-4 -ml-1 flex items-center gap-2 py-2 pl-1 text-gray-600 transition-colors hover:text-gray-900"
        >
          <svg
            className="h-5 w-5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M15 19l-7-7 7-7"
            />
          </svg>
          <span className="text-sm font-medium">Zurück</span>
        </button>

        {/* Student Header - Mobile optimized */}
        <div className="mb-6">
          <div className="flex items-end justify-between gap-4">
            {/* Title */}
            <div className="ml-6 flex-1">
              <h1 className="text-2xl font-bold text-gray-900 md:text-3xl">
                {student.first_name} {student.second_name}
              </h1>
              <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-gray-600 sm:gap-4">
                <span>Klasse {student.school_class}</span>
                {student.group_name &&
                  student.group_name !== student.school_class && (
                    <>
                      <span className="hidden sm:inline">•</span>
                      <span className="truncate">{student.group_name}</span>
                    </>
                  )}
              </div>
            </div>

            {/* Status Badge */}
            <div className="mr-4 flex-shrink-0 pb-3">
              <LocationBadge
                student={badgeStudent}
                displayMode="contextAware"
                userGroups={myGroups}
                variant="modern"
                size="md"
              />
            </div>
          </div>
        </div>

        {!hasFullAccess ? (
          // Limited Access View
          <>
            <div className="mb-6 rounded-lg border border-yellow-200 bg-yellow-50 p-4">
              <div className="flex items-start">
                <svg
                  className="mt-0.5 mr-3 h-5 w-5 text-yellow-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <div>
                  <h3 className="font-medium text-yellow-800">
                    Eingeschränkter Zugriff
                  </h3>
                  <p className="mt-1 text-sm text-yellow-700">
                    Sie haben keinen Zugriff auf die vollständigen Schülerdaten,
                    da Sie nicht die Gruppe dieses Schülers betreuen.
                  </p>
                </div>
              </div>
            </div>

            {supervisors.length > 0 && (
              <InfoCard
                title="Ansprechpartner"
                icon={
                  <svg
                    className="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                    />
                  </svg>
                }
              >
                <div className="space-y-3">
                  {supervisors.map((supervisor) => (
                    <div
                      key={supervisor.id}
                      className="rounded-lg border border-gray-100 bg-gray-50 p-3"
                    >
                      <div className="flex items-center justify-between">
                        <div>
                          <p className="font-medium text-gray-900">
                            {supervisor.first_name} {supervisor.last_name}
                          </p>
                          <p className="text-sm text-gray-500">
                            {supervisor.role}
                          </p>
                          {supervisor.email && (
                            <p className="mt-1 text-sm text-gray-600">
                              {supervisor.email}
                            </p>
                          )}
                        </div>
                        {supervisor.email && (
                          <button
                            onClick={() => {
                              window.location.href = `mailto:${supervisor.email}?subject=Anfrage zu ${student.name}`;
                            }}
                            className="rounded-lg border border-gray-300 bg-white px-3 py-1.5 text-xs font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-sm active:scale-100"
                          >
                            E-Mail
                          </button>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </InfoCard>
            )}
          </>
        ) : (
          // Full Access View
          <>
            {/* Checkout Section - Mobile optimized */}
            {student.current_location?.startsWith("Anwesend") && (
              <div className="mb-6 rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
                <div className="mb-4 flex items-center gap-3">
                  <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                      />
                    </svg>
                  </div>
                  <h3 className="text-base font-semibold text-gray-900 sm:text-lg">
                    Checkout verwalten
                  </h3>
                </div>
                <ScheduledCheckoutInfo
                  studentId={studentId}
                  onUpdate={() => setCheckoutUpdated((prev) => prev + 1)}
                  onScheduledCheckoutChange={setHasScheduledCheckout}
                />
                {!hasScheduledCheckout && (
                  <button
                    onClick={() => setShowCheckoutModal(true)}
                    className="mt-4 flex w-full items-center justify-center gap-2 rounded-lg bg-gray-900 px-4 py-3 text-sm font-medium text-white transition-all duration-200 hover:scale-[1.01] hover:bg-gray-700 hover:shadow-lg active:scale-[0.99] sm:py-2.5"
                  >
                    <svg
                      className="h-5 w-5"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                      />
                    </svg>
                    Schüler ausloggen
                  </button>
                )}
              </div>
            )}

            {alertMessage && (
              <div className="mb-6">
                <Alert
                  type={alertMessage.type}
                  message={alertMessage.message}
                />
              </div>
            )}

            {/* History Section */}
            <InfoCard
              title="Historien"
              icon={
                <svg
                  className="h-5 w-5"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              }
            >
              <div className="grid grid-cols-1 gap-2">
                {/* Room History - Blue */}
                <button
                  type="button"
                  disabled
                  className="flex cursor-not-allowed items-center justify-between rounded-lg border border-gray-100 bg-gray-50 p-3 opacity-60"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg bg-[#5080D8] sm:h-9 sm:w-9">
                      <svg
                        className="h-4 w-4 text-white"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                        />
                      </svg>
                    </div>
                    <div className="min-w-0 flex-1 text-left">
                      <p className="text-sm font-medium text-gray-400 sm:text-base">
                        Raumverlauf
                      </p>
                      <p className="text-xs text-gray-400">
                        Verlauf der Raumbesuche
                      </p>
                    </div>
                  </div>
                  <svg
                    className="h-4 w-4 flex-shrink-0 text-gray-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 5l7 7-7 7"
                    />
                  </svg>
                </button>

                {/* Feedback History - Green */}
                <button
                  type="button"
                  disabled
                  className="flex cursor-not-allowed items-center justify-between rounded-lg border border-gray-100 bg-gray-50 p-3 opacity-60"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[#83CD2D] sm:h-9 sm:w-9">
                      <svg
                        className="h-4 w-4 text-white"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z"
                        />
                      </svg>
                    </div>
                    <div className="text-left">
                      <p className="text-sm font-medium text-gray-400 sm:text-base">
                        Feedbackhistorie
                      </p>
                      <p className="text-xs text-gray-400">
                        Feedback und Bewertungen
                      </p>
                    </div>
                  </div>
                  <svg
                    className="h-4 w-4 text-gray-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 5l7 7-7 7"
                    />
                  </svg>
                </button>

                {/* Mensa History - Orange */}
                <button
                  type="button"
                  disabled
                  className="flex cursor-not-allowed items-center justify-between rounded-lg border border-gray-100 bg-gray-50 p-3 opacity-60"
                >
                  <div className="flex items-center gap-3">
                    <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-[#F78C10] sm:h-9 sm:w-9">
                      <svg
                        className="h-4 w-4 text-white"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13"
                        />
                      </svg>
                    </div>
                    <div className="text-left">
                      <p className="text-sm font-medium text-gray-400 sm:text-base">
                        Mensaverlauf
                      </p>
                      <p className="text-xs text-gray-400">
                        Mahlzeiten und Bestellungen
                      </p>
                    </div>
                  </div>
                  <svg
                    className="h-4 w-4 text-gray-300"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M9 5l7 7-7 7"
                    />
                  </svg>
                </button>
              </div>
            </InfoCard>

            <div className="mt-4 space-y-4 sm:mt-6 sm:space-y-6">
              {/* Personal Information - Mobile optimized */}
              <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm sm:p-6">
                <div className="mb-4 flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg bg-gray-100 text-gray-600 sm:h-10 sm:w-10">
                      <svg
                        className="h-5 w-5"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"
                        />
                      </svg>
                    </div>
                    <h2 className="text-base font-semibold text-gray-900 sm:text-lg">
                      Persönliche Informationen
                    </h2>
                  </div>
                  {!isEditingPersonal ? (
                    <button
                      onClick={() => {
                        setIsEditingPersonal(true);
                        setEditedStudent(student);
                      }}
                      className="rounded-lg p-2 text-gray-600 transition-colors hover:bg-gray-100"
                      title="Bearbeiten"
                    >
                      <svg
                        className="h-5 w-5"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                      >
                        <path
                          strokeLinecap="round"
                          strokeLinejoin="round"
                          strokeWidth={2}
                          d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"
                        />
                      </svg>
                    </button>
                  ) : (
                    <div className="flex gap-2">
                      <button
                        onClick={() => {
                          setIsEditingPersonal(false);
                          setEditedStudent(student);
                        }}
                        className="rounded-lg p-2 text-gray-600 transition-colors hover:bg-gray-100"
                        title="Abbrechen"
                      >
                        <svg
                          className="h-5 w-5"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M6 18L18 6M6 6l12 12"
                          />
                        </svg>
                      </button>
                      <button
                        onClick={handleSavePersonal}
                        className="rounded-lg bg-blue-500 p-2 text-white transition-colors hover:bg-blue-600"
                        title="Speichern"
                      >
                        <svg
                          className="h-5 w-5"
                          fill="none"
                          viewBox="0 0 24 24"
                          stroke="currentColor"
                        >
                          <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={2}
                            d="M5 13l4 4L19 7"
                          />
                        </svg>
                      </button>
                    </div>
                  )}
                </div>
                <div className="space-y-3">
                  {isEditingPersonal && editedStudent ? (
                    <>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Vorname
                        </label>
                        <input
                          type="text"
                          value={editedStudent.first_name}
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              first_name: e.target.value,
                            })
                          }
                          className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Nachname
                        </label>
                        <input
                          type="text"
                          value={editedStudent.second_name}
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              second_name: e.target.value,
                            })
                          }
                          className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Klasse
                        </label>
                        <input
                          type="text"
                          value={editedStudent.school_class}
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              school_class: e.target.value,
                            })
                          }
                          className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Geburtsdatum
                        </label>
                        <input
                          type="date"
                          value={
                            editedStudent.birthday
                              ? editedStudent.birthday.split("T")[0]
                              : ""
                          }
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              birthday: e.target.value,
                            })
                          }
                          className="w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Buskind
                        </label>
                        <select
                          value={editedStudent.buskind ? "true" : "false"}
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              buskind: e.target.value === "true",
                            })
                          }
                          className="w-full rounded-lg border border-gray-300 bg-white px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                        >
                          <option value="false">Nein</option>
                          <option value="true">Ja</option>
                        </select>
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Gesundheitsinformationen
                        </label>
                        <textarea
                          value={editedStudent.health_info ?? ""}
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              health_info: e.target.value,
                            })
                          }
                          className="min-h-[80px] w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                          rows={3}
                          placeholder="Allergien, Medikamente, wichtige medizinische Informationen"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Betreuernotizen
                        </label>
                        <textarea
                          value={editedStudent.supervisor_notes ?? ""}
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              supervisor_notes: e.target.value,
                            })
                          }
                          className="min-h-[80px] w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                          rows={3}
                          placeholder="Notizen für Betreuer"
                        />
                      </div>
                      <div>
                        <label className="mb-1 block text-xs text-gray-500">
                          Elternnotizen
                        </label>
                        <textarea
                          value={editedStudent.extra_info ?? ""}
                          onChange={(e) =>
                            setEditedStudent({
                              ...editedStudent,
                              extra_info: e.target.value,
                            })
                          }
                          className="min-h-[60px] w-full rounded-lg border border-gray-300 px-3 py-2.5 text-sm focus:ring-2 focus:ring-blue-500 focus:outline-none"
                          rows={2}
                          placeholder="Notizen der Eltern"
                        />
                      </div>
                    </>
                  ) : (
                    <>
                      <InfoItem
                        label="Vollständiger Name"
                        value={student.name}
                      />
                      <InfoItem label="Klasse" value={student.school_class} />
                      <InfoItem
                        label="Gruppe"
                        value={student.group_name ?? "Nicht zugewiesen"}
                      />
                      <InfoItem
                        label="Geburtsdatum"
                        value={
                          student.birthday
                            ? new Date(student.birthday).toLocaleDateString(
                                "de-DE",
                              )
                            : "Nicht angegeben"
                        }
                      />
                      <InfoItem
                        label="Buskind"
                        value={student.buskind ? "Ja" : "Nein"}
                      />
                      {student.health_info && (
                        <InfoItem
                          label="Gesundheitsinformationen"
                          value={student.health_info}
                        />
                      )}
                      {student.supervisor_notes && (
                        <InfoItem
                          label="Betreuernotizen"
                          value={student.supervisor_notes}
                        />
                      )}
                      {student.extra_info && (
                        <InfoItem
                          label="Elternnotizen"
                          value={student.extra_info}
                        />
                      )}
                    </>
                  )}
                </div>
              </div>

              {/* Guardian Information */}
              <StudentGuardianManager
                studentId={studentId}
                readOnly={!hasFullAccess}
                onUpdate={() => setCheckoutUpdated((prev) => prev + 1)}
              />
            </div>
          </>
        )}
      </div>

      {/* Scheduled Checkout Modal */}
      {student && (
        <ScheduledCheckoutModal
          isOpen={showCheckoutModal}
          onClose={() => setShowCheckoutModal(false)}
          studentId={studentId}
          studentName={student.name}
          onCheckoutScheduled={() => {
            setCheckoutUpdated((prev) => prev + 1);
            setShowCheckoutModal(false);
          }}
        />
      )}
    </ResponsiveLayout>
  );
}
