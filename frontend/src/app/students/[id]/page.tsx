"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { useSession } from "next-auth/react";
import { studentService } from "~/lib/api";
import type { Student, SupervisorContact } from "~/lib/student-helpers";
import { ModernContactActions } from "~/components/simple/student";
import { ScheduledCheckoutModal } from "~/components/scheduled-checkout/scheduled-checkout-modal";
import { ScheduledCheckoutInfo } from "~/components/scheduled-checkout/scheduled-checkout-info";
import { LocationBadge } from "~/components/simple/student/LocationBadge";

// Guardian type for multiple guardians
interface Guardian {
    id?: string;
    name: string;
    email: string;
    phone: string;
    relationship?: string;
}

// Extended Student type for this page
interface ExtendedStudent extends Student {
    wc: boolean;
    school_yard: boolean;
    bus: boolean;
    current_room?: string;
    guardian_name: string;
    guardian_contact: string;
    guardian_phone?: string;
    birthday?: string;
    buskind?: boolean;
    attendance_rate?: number;
    guardians?: Guardian[];
    extra_info?: string;
    supervisor_notes?: string;
    health_info?: string;
}

// Mobile-optimized info card component
function InfoCard({ title, children, icon }: { title: string; children: React.ReactNode; icon: React.ReactNode }) {
    return (
        <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-4 sm:p-6">
            <div className="flex items-center gap-3 mb-4">
                <div className="h-9 w-9 sm:h-10 sm:w-10 rounded-lg bg-gray-100 flex items-center justify-center text-gray-600 flex-shrink-0">
                    {icon}
                </div>
                <h2 className="text-base sm:text-lg font-semibold text-gray-900">{title}</h2>
            </div>
            <div className="space-y-3">{children}</div>
        </div>
    );
}

// Simplified info item component
function InfoItem({ label, value, icon }: { label: string; value: string | React.ReactNode; icon?: React.ReactNode }) {
    return (
        <div className="flex items-start gap-3">
            {icon && (
                <div className="flex-shrink-0 mt-0.5 text-gray-400">
                    <div className="h-4 w-4">{icon}</div>
                </div>
            )}
            <div className="flex-1 min-w-0">
                <p className="text-xs text-gray-500 mb-1">{label}</p>
                <div className="text-sm text-gray-900 font-medium">{value}</div>
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
    useSession();

    const [student, setStudent] = useState<ExtendedStudent | null>(null);
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [hasFullAccess, setHasFullAccess] = useState(true);
    const [supervisors, setSupervisors] = useState<SupervisorContact[]>([]);
    const [currentLocation, setCurrentLocation] = useState<{
        status: string;
        location: string;
        room: { name: string } | null;
    } | null>(null);
    const [showCheckoutModal, setShowCheckoutModal] = useState(false);
    const [checkoutUpdated, setCheckoutUpdated] = useState(0);
    const [hasScheduledCheckout, setHasScheduledCheckout] = useState(false);

    // Edit mode states
    const [isEditingPersonal, setIsEditingPersonal] = useState(false);
    const [isEditingGuardians, setIsEditingGuardians] = useState(false);
    const [editedStudent, setEditedStudent] = useState<ExtendedStudent | null>(null);
    const [editedGuardians, setEditedGuardians] = useState<Guardian[]>([]);
    const [alertMessage, setAlertMessage] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

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
                    guardian_name?: string;
                    guardian_contact?: string;
                    guardian_phone?: string;
                    guardian_email?: string;
                };

                const hasAccess = mappedStudent.has_full_access ?? true;
                const groupSupervisors = mappedStudent.group_supervisors ?? [];

                // Create guardians array from legacy fields
                const guardians: Guardian[] = [];
                if (hasAccess && mappedStudent.name_lg) {
                    guardians.push({
                        id: '1',
                        name: mappedStudent.name_lg,
                        email: mappedStudent.guardian_email ?? '',
                        phone: mappedStudent.contact_lg ?? '',
                        relationship: 'Erziehungsberechtigte/r'
                    });
                }

                const extendedStudent: ExtendedStudent = {
                    id: mappedStudent.id,
                    first_name: mappedStudent.first_name ?? "",
                    second_name: mappedStudent.second_name ?? "",
                    name: mappedStudent.name,
                    school_class: mappedStudent.school_class,
                    group_id: mappedStudent.group_id ?? "",
                    group_name: mappedStudent.group_name ?? "",
                    current_location: mappedStudent.current_location,
                    in_house: mappedStudent.in_house,
                    wc: mappedStudent.wc ?? false,
                    school_yard: mappedStudent.school_yard ?? false,
                    bus: mappedStudent.bus ?? false,
                    current_room: undefined,
                    guardian_name: hasAccess ? (mappedStudent.name_lg ?? "") : "",
                    guardian_contact: hasAccess ? (mappedStudent.guardian_email ?? "") : "",
                    guardian_phone: hasAccess ? (mappedStudent.contact_lg ?? "") : "",
                    birthday: mappedStudent.birthday ?? undefined,
                    buskind: mappedStudent.bus ?? false,
                    attendance_rate: undefined,
                    guardians,
                    extra_info: hasAccess ? (mappedStudent.extra_info ?? undefined) : undefined,
                    supervisor_notes: hasAccess ? (mappedStudent.supervisor_notes ?? undefined) : undefined,
                    health_info: hasAccess ? (mappedStudent.health_info ?? undefined) : undefined
                };

                setStudent(extendedStudent);
                setEditedStudent(extendedStudent);
                setEditedGuardians(guardians);
                setHasFullAccess(hasAccess);
                setSupervisors(groupSupervisors);

                // Fetch current location
                try {
                    const locationResponse = await fetch(`/api/students/${studentId}/current-location`);
                    if (locationResponse.ok) {
                        const response = await locationResponse.json() as unknown;
                        const locationData = (response && typeof response === 'object' && 'data' in response ? response.data : response) as {
                            status: string;
                            location: string;
                            room: { name: string } | null;
                        };
                        setCurrentLocation(locationData);
                    }
                } catch (locationErr) {
                    console.error("Error fetching student location:", locationErr);
                }

                setLoading(false);
            } catch (err) {
                console.error("Error fetching student:", err);
                setError("Fehler beim Laden der Schülerdaten.");
                setLoading(false);
            }
        };

        void fetchStudent();
    }, [studentId, checkoutUpdated]);

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
                extra_info: editedStudent.extra_info
            });

            setStudent(editedStudent);
            setIsEditingPersonal(false);
            setAlertMessage({ type: 'success', message: 'Persönliche Informationen erfolgreich aktualisiert' });
            setTimeout(() => setAlertMessage(null), 3000);
        } catch (error) {
            console.error('Failed to save personal information:', error);
            setAlertMessage({ type: 'error', message: 'Fehler beim Speichern der persönlichen Informationen' });
            setTimeout(() => setAlertMessage(null), 3000);
        }
    };

    // Handle save for guardians
    const handleSaveGuardians = async () => {
        if (!student) return;

        try {
            // For now, we'll save the first guardian to the legacy fields
            const primaryGuardian = editedGuardians[0];
            if (primaryGuardian) {
                await studentService.updateStudent(studentId, {
                    name_lg: primaryGuardian.name,
                    contact_lg: primaryGuardian.phone
                });
            }

            const updatedStudent = { ...student, guardians: editedGuardians };
            if (primaryGuardian) {
                updatedStudent.guardian_name = primaryGuardian.name;
                updatedStudent.guardian_phone = primaryGuardian.phone;
                updatedStudent.guardian_contact = primaryGuardian.email;
            }

            setStudent(updatedStudent);
            setIsEditingGuardians(false);
            setAlertMessage({ type: 'success', message: 'Erziehungsberechtigte erfolgreich aktualisiert' });
            setTimeout(() => setAlertMessage(null), 3000);
        } catch (error) {
            console.error('Failed to save guardians:', error);
            setAlertMessage({ type: 'error', message: 'Fehler beim Speichern der Erziehungsberechtigten' });
            setTimeout(() => setAlertMessage(null), 3000);
        }
    };

    // Add a new guardian
    const handleAddGuardian = () => {
        setEditedGuardians([...editedGuardians, { name: '', email: '', phone: '', relationship: 'Erziehungsberechtigte/r' }]);
    };

    // Remove a guardian
    const handleRemoveGuardian = (index: number) => {
        setEditedGuardians(editedGuardians.filter((_, i) => i !== index));
    };

    // Update guardian field
    const handleUpdateGuardian = (index: number, field: keyof Guardian, value: string) => {
        const updated = [...editedGuardians];
        const currentGuardian = updated[index] ?? { name: '', email: '', phone: '' };
        updated[index] = { ...currentGuardian, [field]: value };
        setEditedGuardians(updated);
    };

    if (loading) {
        return (
            <ResponsiveLayout>
                <div className="flex min-h-[80vh] items-center justify-center">
                    <div className="flex flex-col items-center gap-4">
                        <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                        <p className="text-gray-600">Daten werden geladen...</p>
                    </div>
                </div>
            </ResponsiveLayout>
        );
    }

    if (error || !student) {
        return (
            <ResponsiveLayout>
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

    return (
        <ResponsiveLayout>
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 pb-6">
                {/* Back button - Mobile optimized */}
                <button
                    onClick={() => router.push(referrer)}
                    className="flex items-center gap-2 mb-4 text-gray-600 hover:text-gray-900 transition-colors py-2 -ml-1 pl-1"
                >
                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                    </svg>
                    <span className="text-sm font-medium">Zurück</span>
                </button>

                {/* Student Header - Mobile optimized */}
                <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-4 sm:p-6 mb-6">
                    <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                        <div className="flex-1 min-w-0">
                            <h1 className="text-xl sm:text-2xl font-bold text-gray-900 truncate">
                                {student.first_name} {student.second_name}
                            </h1>
                            <div className="flex flex-wrap items-center gap-2 sm:gap-4 mt-2 text-sm text-gray-600">
                                <span>Klasse {student.school_class}</span>
                                {student.group_name && (
                                    <>
                                        <span className="hidden sm:inline">•</span>
                                        <span className="truncate">{student.group_name}</span>
                                    </>
                                )}
                            </div>
                        </div>
                        <div className="flex-shrink-0">
                            <LocationBadge locationStatus={student?.location_status ?? null} />
                        </div>
                    </div>
                </div>

                {!hasFullAccess ? (
                    // Limited Access View
                    <>
                        <div className="mb-6 rounded-lg bg-yellow-50 border border-yellow-200 p-4">
                            <div className="flex items-start">
                                <svg className="h-5 w-5 text-yellow-600 mt-0.5 mr-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                </svg>
                                <div>
                                    <h3 className="font-medium text-yellow-800">Eingeschränkter Zugriff</h3>
                                    <p className="mt-1 text-sm text-yellow-700">
                                        Sie haben keinen Zugriff auf die vollständigen Schülerdaten, da Sie nicht die Gruppe dieses Schülers betreuen.
                                    </p>
                                </div>
                            </div>
                        </div>

                        {supervisors.length > 0 && (
                            <InfoCard
                                title="Ansprechpartner"
                                icon={
                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                    </svg>
                                }
                            >
                                <div className="space-y-3">
                                    {supervisors.map((supervisor) => (
                                        <div key={supervisor.id} className="border border-gray-100 rounded-lg p-3 bg-gray-50">
                                            <div className="flex items-center justify-between">
                                                <div>
                                                    <p className="font-medium text-gray-900">
                                                        {supervisor.first_name} {supervisor.last_name}
                                                    </p>
                                                    <p className="text-sm text-gray-500">{supervisor.role}</p>
                                                    {supervisor.email && <p className="text-sm text-gray-600 mt-1">{supervisor.email}</p>}
                                                </div>
                                                {supervisor.email && (
                                                    <Button
                                                        variant="outline"
                                                        size="sm"
                                                        onClick={() => {
                                                            window.location.href = `mailto:${supervisor.email}?subject=Anfrage zu ${student.name}`;
                                                        }}
                                                    >
                                                        E-Mail
                                                    </Button>
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
                        {currentLocation?.location && currentLocation.location.startsWith("Anwesend") && (
                            <div className="mb-6 bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-4 sm:p-6">
                                <h3 className="text-base sm:text-lg font-semibold text-gray-900 mb-3 sm:mb-4">Checkout verwalten</h3>
                                <ScheduledCheckoutInfo
                                    studentId={studentId}
                                    onUpdate={() => setCheckoutUpdated(prev => prev + 1)}
                                    onScheduledCheckoutChange={setHasScheduledCheckout}
                                />
                                {!hasScheduledCheckout && (
                                    <div className="mt-4">
                                        <Button
                                            onClick={() => setShowCheckoutModal(true)}
                                            className="w-full bg-blue-500 hover:bg-blue-600 text-white py-3 sm:py-2"
                                        >
                                            <svg className="h-5 w-5 mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1" />
                                            </svg>
                                            Schüler ausloggen
                                        </Button>
                                    </div>
                                )}
                            </div>
                        )}

                        {alertMessage && (
                            <div className="mb-6">
                                <Alert type={alertMessage.type} message={alertMessage.message} />
                            </div>
                        )}

                        {/* History Section */}
                        <InfoCard
                            title="Historien"
                            icon={
                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                </svg>
                            }
                        >
                            <div className="grid grid-cols-1 gap-2">
                                {/* Room History - Blue */}
                                <button
                                    type="button"
                                    disabled
                                    className="flex items-center justify-between p-3 rounded-lg bg-gray-50 border border-gray-100 opacity-60 cursor-not-allowed"
                                >
                                    <div className="flex items-center gap-3">
                                        <div className="h-8 w-8 sm:h-9 sm:w-9 rounded-lg bg-[#5080D8] flex items-center justify-center flex-shrink-0">
                                            <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                            </svg>
                                        </div>
                                        <div className="text-left flex-1 min-w-0">
                                            <p className="font-medium text-gray-400 text-sm sm:text-base">Raumverlauf</p>
                                            <p className="text-xs text-gray-400">Verlauf der Raumbesuche</p>
                                        </div>
                                    </div>
                                    <svg className="h-4 w-4 text-gray-300 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                    </svg>
                                </button>

                                {/* Feedback History - Green */}
                                <button
                                    type="button"
                                    disabled
                                    className="flex items-center justify-between p-3 rounded-lg bg-gray-50 border border-gray-100 opacity-60 cursor-not-allowed"
                                >
                                    <div className="flex items-center gap-3">
                                        <div className="h-8 w-8 sm:h-9 sm:w-9 rounded-lg bg-[#83CD2D] flex items-center justify-center">
                                            <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
                                            </svg>
                                        </div>
                                        <div className="text-left">
                                            <p className="text-sm sm:text-base font-medium text-gray-400">Feedbackhistorie</p>
                                            <p className="text-xs text-gray-400">Feedback und Bewertungen</p>
                                        </div>
                                    </div>
                                    <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                    </svg>
                                </button>

                                {/* Mensa History - Orange */}
                                <button
                                    type="button"
                                    disabled
                                    className="flex items-center justify-between p-3 rounded-lg bg-gray-50 border border-gray-100 opacity-60 cursor-not-allowed"
                                >
                                    <div className="flex items-center gap-3">
                                        <div className="h-8 w-8 sm:h-9 sm:w-9 rounded-lg bg-[#F78C10] flex items-center justify-center">
                                            <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13" />
                                            </svg>
                                        </div>
                                        <div className="text-left">
                                            <p className="text-sm sm:text-base font-medium text-gray-400">Mensaverlauf</p>
                                            <p className="text-xs text-gray-400">Mahlzeiten und Bestellungen</p>
                                        </div>
                                    </div>
                                    <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                    </svg>
                                </button>
                            </div>
                        </InfoCard>

                        <div className="space-y-4 sm:space-y-6 mt-4 sm:mt-6">
                            {/* Personal Information - Mobile optimized */}
                            <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-4 sm:p-6">
                                <div className="flex items-center justify-between mb-4">
                                    <div className="flex items-center gap-3">
                                        <div className="h-9 w-9 sm:h-10 sm:w-10 rounded-lg bg-gray-100 flex items-center justify-center text-gray-600 flex-shrink-0">
                                            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                            </svg>
                                        </div>
                                        <h2 className="text-base sm:text-lg font-semibold text-gray-900">Persönliche Informationen</h2>
                                    </div>
                                    {!isEditingPersonal ? (
                                        <button
                                            onClick={() => {
                                                setIsEditingPersonal(true);
                                                setEditedStudent(student);
                                            }}
                                            className="p-2 text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
                                            title="Bearbeiten"
                                        >
                                            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                            </svg>
                                        </button>
                                    ) : (
                                        <div className="flex gap-2">
                                            <button
                                                onClick={() => {
                                                    setIsEditingPersonal(false);
                                                    setEditedStudent(student);
                                                }}
                                                className="p-2 text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
                                                title="Abbrechen"
                                            >
                                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                                </svg>
                                            </button>
                                            <button
                                                onClick={handleSavePersonal}
                                                className="p-2 text-white bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors"
                                                title="Speichern"
                                            >
                                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                                </svg>
                                            </button>
                                        </div>
                                    )}
                                </div>
                                <div className="space-y-3">
                                    {isEditingPersonal && editedStudent ? (
                                        <>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Vorname</label>
                                                <input
                                                    type="text"
                                                    value={editedStudent.first_name}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, first_name: e.target.value })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Nachname</label>
                                                <input
                                                    type="text"
                                                    value={editedStudent.second_name}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, second_name: e.target.value })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Klasse</label>
                                                <input
                                                    type="text"
                                                    value={editedStudent.school_class}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, school_class: e.target.value })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Geburtsdatum</label>
                                                <input
                                                    type="date"
                                                    value={editedStudent.birthday ? editedStudent.birthday.split('T')[0] : ''}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, birthday: e.target.value })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Buskind</label>
                                                <select
                                                    value={editedStudent.buskind ? 'true' : 'false'}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, buskind: e.target.value === 'true' })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 bg-white"
                                                >
                                                    <option value="false">Nein</option>
                                                    <option value="true">Ja</option>
                                                </select>
                                            </div>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Gesundheitsinformationen</label>
                                                <textarea
                                                    value={editedStudent.health_info ?? ''}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, health_info: e.target.value })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 min-h-[80px]"
                                                    rows={3}
                                                    placeholder="Allergien, Medikamente, wichtige medizinische Informationen"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Betreuernotizen</label>
                                                <textarea
                                                    value={editedStudent.supervisor_notes ?? ''}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, supervisor_notes: e.target.value })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 min-h-[80px]"
                                                    rows={3}
                                                    placeholder="Notizen für Betreuer"
                                                />
                                            </div>
                                            <div>
                                                <label className="text-xs text-gray-500 mb-1 block">Elternnotizen</label>
                                                <textarea
                                                    value={editedStudent.extra_info ?? ''}
                                                    onChange={(e) => setEditedStudent({ ...editedStudent, extra_info: e.target.value })}
                                                    className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 min-h-[60px]"
                                                    rows={2}
                                                    placeholder="Notizen der Eltern"
                                                />
                                            </div>
                                        </>
                                    ) : (
                                        <>
                                            <InfoItem label="Vollständiger Name" value={student.name} />
                                            <InfoItem label="Klasse" value={student.school_class} />
                                            <InfoItem label="Gruppe" value={student.group_name ?? 'Nicht zugewiesen'} />
                                            <InfoItem label="Geburtsdatum" value={student.birthday ? new Date(student.birthday).toLocaleDateString('de-DE') : 'Nicht angegeben'} />
                                            <InfoItem label="Buskind" value={student.buskind ? 'Ja' : 'Nein'} />
                                            {student.health_info && <InfoItem label="Gesundheitsinformationen" value={student.health_info} />}
                                            {student.supervisor_notes && <InfoItem label="Betreuernotizen" value={student.supervisor_notes} />}
                                            {student.extra_info && <InfoItem label="Elternnotizen" value={student.extra_info} />}
                                        </>
                                    )}
                                </div>
                            </div>

                            {/* Guardian Information - Mobile optimized */}
                            <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-4 sm:p-6">
                                <div className="flex items-center justify-between mb-4">
                                    <div className="flex items-center gap-3">
                                        <div className="h-9 w-9 sm:h-10 sm:w-10 rounded-lg bg-gray-100 flex items-center justify-center text-gray-600 flex-shrink-0">
                                            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                            </svg>
                                        </div>
                                        <h2 className="text-base sm:text-lg font-semibold text-gray-900">Erziehungsberechtigte</h2>
                                    </div>
                                    {!isEditingGuardians ? (
                                        <button
                                            onClick={() => {
                                                setIsEditingGuardians(true);
                                                setEditedGuardians(student.guardians ?? []);
                                            }}
                                            className="p-2 text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
                                            title="Bearbeiten"
                                        >
                                            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                            </svg>
                                        </button>
                                    ) : (
                                        <div className="flex gap-2">
                                            <button
                                                onClick={() => {
                                                    setIsEditingGuardians(false);
                                                    setEditedGuardians(student.guardians ?? []);
                                                }}
                                                className="p-2 text-gray-600 hover:bg-gray-100 rounded-lg transition-colors"
                                                title="Abbrechen"
                                            >
                                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                                </svg>
                                            </button>
                                            <button
                                                onClick={handleSaveGuardians}
                                                className="p-2 text-white bg-blue-500 hover:bg-blue-600 rounded-lg transition-colors"
                                                title="Speichern"
                                            >
                                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 13l4 4L19 7" />
                                                </svg>
                                            </button>
                                        </div>
                                    )}
                                </div>
                                <div className="space-y-4">
                                    {isEditingGuardians ? (
                                        <>
                                            {editedGuardians.map((guardian, index) => (
                                                <div key={index} className="p-4 border border-gray-200 rounded-lg space-y-3">
                                                    <div className="flex justify-between items-center mb-2">
                                                        <h3 className="text-sm font-semibold text-gray-700">Erziehungsberechtigte/r {index + 1}</h3>
                                                        {editedGuardians.length > 1 && (
                                                            <button
                                                                onClick={() => handleRemoveGuardian(index)}
                                                                className="text-red-500 hover:text-red-700 text-sm"
                                                            >
                                                                Entfernen
                                                            </button>
                                                        )}
                                                    </div>
                                                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                                                        <div className="sm:col-span-2">
                                                            <label className="text-xs text-gray-500 mb-1 block">Name</label>
                                                            <input
                                                                type="text"
                                                                value={guardian.name}
                                                                onChange={(e) => handleUpdateGuardian(index, 'name', e.target.value)}
                                                                className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                                placeholder="Max Mustermann"
                                                            />
                                                        </div>
                                                        <div>
                                                            <label className="text-xs text-gray-500 mb-1 block">Beziehung</label>
                                                            <input
                                                                type="text"
                                                                value={guardian.relationship ?? ''}
                                                                onChange={(e) => handleUpdateGuardian(index, 'relationship', e.target.value)}
                                                                className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                                placeholder="Mutter/Vater/etc."
                                                            />
                                                        </div>
                                                        <div>
                                                            <label className="text-xs text-gray-500 mb-1 block">Telefonnummer</label>
                                                            <input
                                                                type="tel"
                                                                value={guardian.phone}
                                                                onChange={(e) => handleUpdateGuardian(index, 'phone', e.target.value)}
                                                                className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                                placeholder="+49 123 456789"
                                                            />
                                                        </div>
                                                        <div className="sm:col-span-2">
                                                            <label className="text-xs text-gray-500 mb-1 block">E-Mail</label>
                                                            <input
                                                                type="email"
                                                                value={guardian.email}
                                                                onChange={(e) => handleUpdateGuardian(index, 'email', e.target.value)}
                                                                className="w-full px-3 py-2.5 border border-gray-300 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
                                                                placeholder="email@beispiel.de"
                                                            />
                                                        </div>
                                                    </div>
                                                </div>
                                            ))}
                                            <button
                                                onClick={handleAddGuardian}
                                                className="w-full py-2 border-2 border-dashed border-gray-300 rounded-lg text-gray-600 hover:border-gray-400 hover:text-gray-700 transition-colors"
                                            >
                                                + Weitere/n Erziehungsberechtigte/n hinzufügen
                                            </button>
                                        </>
                                    ) : (
                                        <>
                                            {student.guardians && student.guardians.length > 0 ? (
                                                student.guardians.map((guardian, index) => (
                                                    <div key={index} className="p-4 border border-gray-200 rounded-lg">
                                                        <div className="space-y-2">
                                                            <InfoItem label="Name" value={guardian.name} />
                                                            {guardian.relationship && <InfoItem label="Beziehung" value={guardian.relationship} />}
                                                            <InfoItem label="E-Mail" value={guardian.email || 'Nicht angegeben'} />
                                                            <InfoItem label="Telefonnummer" value={guardian.phone || 'Nicht angegeben'} />
                                                        </div>
                                                        <div className="mt-3">
                                                            <ModernContactActions email={guardian.email} phone={guardian.phone} studentName={student.name} />
                                                        </div>
                                                    </div>
                                                ))
                                            ) : (
                                                <>
                                                    <InfoItem label="Name" value={student.guardian_name || 'Nicht angegeben'} />
                                                    <InfoItem label="E-Mail" value={student.guardian_contact || 'Nicht angegeben'} />
                                                    <InfoItem label="Telefonnummer" value={student.guardian_phone ?? 'Nicht angegeben'} />
                                                    <ModernContactActions email={student.guardian_contact} phone={student.guardian_phone} studentName={student.name} />
                                                </>
                                            )}
                                        </>
                                    )}
                                </div>
                            </div>
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
                        setCheckoutUpdated(prev => prev + 1);
                        setShowCheckoutModal(false);
                    }}
                />
            )}
        </ResponsiveLayout>
    );
}