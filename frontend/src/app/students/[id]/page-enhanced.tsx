"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { useSession } from "next-auth/react";
import { studentService, updateStudent } from "~/lib/api";
import type { Student, SupervisorContact } from "~/lib/student-helpers";
import { ModernContactActions } from "~/components/simple/student";
import { ScheduledCheckoutModal } from "~/components/scheduled-checkout/scheduled-checkout-modal";
import { ScheduledCheckoutInfo } from "~/components/scheduled-checkout/scheduled-checkout-info";
import { SimpleAlert } from "~/components/simple/SimpleAlert";

// Guardian type
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
    guardians: Guardian[];
    birthday?: string;
    notes?: string;
    buskind?: boolean;
    attendance_rate?: number;
}

// Simplified status badge component
function StatusBadge({ location, roomName }: { location?: string; roomName?: string }) {
    const getStatusDetails = () => {
        if (location === "Anwesend" || location === "In House" || location?.startsWith("Anwesend")) {
            const label = roomName ?? (location?.startsWith("Anwesend - ") ? location.substring(11) : "Anwesend");
            return { label, bgColor: "#83CD2D", textColor: "text-white" };
        } else if (location === "Zuhause") {
            return { label: "Zuhause", bgColor: "#FF3130", textColor: "text-white" };
        } else if (location === "WC") {
            return { label: "WC", bgColor: "#5080D8", textColor: "text-white" };
        } else if (location === "School Yard") {
            return { label: "Schulhof", bgColor: "#F78C10", textColor: "text-white" };
        } else if (location === "Unterwegs" || location === "Bus") {
            return { label: location === "Bus" ? "Bus" : "Unterwegs", bgColor: "#D946EF", textColor: "text-white" };
        }
        return { label: "Unbekannt", bgColor: "#6B7280", textColor: "text-white" };
    };

    const status = getStatusDetails();

    return (
        <span
            className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-semibold ${status.textColor}`}
            style={{ backgroundColor: status.bgColor }}
        >
            <span className="w-2 h-2 bg-white/80 rounded-full mr-2 animate-pulse" />
            {status.label}
        </span>
    );
}

// Simplified info card component
function InfoCard({
    title,
    children,
    icon,
    onEdit,
    isEditing = false,
    onSave,
    onCancel
}: {
    title: string;
    children: React.ReactNode;
    icon: React.ReactNode;
    onEdit?: () => void;
    isEditing?: boolean;
    onSave?: () => void;
    onCancel?: () => void;
}) {
    return (
        <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-6">
            <div className="flex items-center justify-between mb-4">
                <div className="flex items-center gap-3">
                    <div className="h-10 w-10 rounded-lg bg-gray-100 flex items-center justify-center text-gray-600">
                        {icon}
                    </div>
                    <h2 className="text-lg font-semibold text-gray-900">{title}</h2>
                </div>
                {onEdit && !isEditing && (
                    <button
                        onClick={onEdit}
                        className="px-3 py-1.5 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 hover:shadow-md hover:scale-105 active:scale-100 transition-all duration-200"
                    >
                        <svg className="h-4 w-4 inline mr-1" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z" />
                        </svg>
                        Bearbeiten
                    </button>
                )}
                {isEditing && (
                    <div className="flex gap-2">
                        <button
                            onClick={onCancel}
                            className="px-3 py-1.5 rounded-lg border border-gray-300 text-sm font-medium text-gray-700 hover:bg-gray-50 hover:border-gray-400 transition-all duration-200"
                        >
                            Abbrechen
                        </button>
                        <button
                            onClick={onSave}
                            className="px-3 py-1.5 rounded-lg bg-gray-900 text-sm font-medium text-white hover:bg-gray-700 transition-all duration-200"
                        >
                            Speichern
                        </button>
                    </div>
                )}
            </div>
            <div className="space-y-3">{children}</div>
        </div>
    );
}

// Info item component (editable)
function InfoItem({
    label,
    value,
    icon,
    isEditing = false,
    onChange,
    type = "text"
}: {
    label: string;
    value: string | React.ReactNode;
    icon?: React.ReactNode;
    isEditing?: boolean;
    onChange?: (value: string) => void;
    type?: string;
}) {
    return (
        <div className="flex items-start gap-3">
            {icon && (
                <div className="flex-shrink-0 mt-0.5 text-gray-400">
                    <div className="h-4 w-4">{icon}</div>
                </div>
            )}
            <div className="flex-1 min-w-0">
                <p className="text-xs text-gray-500 mb-1">{label}</p>
                {isEditing && onChange ? (
                    <input
                        type={type}
                        value={value as string}
                        onChange={(e) => onChange(e.target.value)}
                        className="w-full px-3 py-1.5 text-sm border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
                    />
                ) : (
                    <div className="text-sm text-gray-900 font-medium">{value}</div>
                )}
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
    const [successMessage, setSuccessMessage] = useState("");
    const [errorMessage, setErrorMessage] = useState("");

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
                    name_lg?: string;
                    contact_lg?: string;
                };

                const hasAccess = mappedStudent.has_full_access ?? true;
                const groupSupervisors = mappedStudent.group_supervisors ?? [];

                // Parse guardians - for now, we create one from the existing data
                const guardians: Guardian[] = [];
                if (mappedStudent.name_lg || mappedStudent.guardian_name) {
                    guardians.push({
                        id: "1",
                        name: mappedStudent.name_lg ?? mappedStudent.guardian_name ?? "",
                        email: mappedStudent.guardian_email ?? mappedStudent.guardian_contact ?? "",
                        phone: mappedStudent.contact_lg ?? mappedStudent.guardian_phone ?? "",
                        relationship: "Erziehungsberechtigte/r"
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
                    guardians: hasAccess ? guardians : [],
                    birthday: undefined,
                    notes: undefined,
                    buskind: mappedStudent.bus,
                    attendance_rate: undefined
                };

                setStudent(extendedStudent);
                setEditedStudent(extendedStudent);
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

    const handleSavePersonal = async () => {
        if (!editedStudent) return;

        try {
            await updateStudent(studentId, {
                first_name: editedStudent.first_name ?? "",
                last_name: editedStudent.second_name ?? "",
                school_class: editedStudent.school_class,
            });

            setStudent(editedStudent);
            setIsEditingPersonal(false);
            setSuccessMessage("Persönliche Informationen erfolgreich gespeichert");
        } catch (err) {
            console.error("Error updating student:", err);
            setErrorMessage("Fehler beim Speichern der Änderungen");
        }
    };

    const handleSaveGuardians = async () => {
        if (!editedStudent || editedStudent.guardians.length === 0) return;

        try {
            // For now, save the first guardian
            const primaryGuardian = editedStudent.guardians[0];
            if (primaryGuardian) {
                await updateStudent(studentId, {
                    guardian_name: primaryGuardian.name,
                    guardian_email: primaryGuardian.email,
                    guardian_phone: primaryGuardian.phone,
                });
            }

            setStudent(editedStudent);
            setIsEditingGuardians(false);
            setSuccessMessage("Erziehungsberechtigte erfolgreich gespeichert");
        } catch (err) {
            console.error("Error updating guardians:", err);
            setErrorMessage("Fehler beim Speichern der Erziehungsberechtigten");
        }
    };

    const handleAddGuardian = () => {
        if (!editedStudent) return;

        const newGuardian: Guardian = {
            id: Date.now().toString(),
            name: "",
            email: "",
            phone: "",
            relationship: "Erziehungsberechtigte/r"
        };

        setEditedStudent({
            ...editedStudent,
            guardians: [...editedStudent.guardians, newGuardian]
        });
    };

    const handleRemoveGuardian = (guardianId: string) => {
        if (!editedStudent) return;

        setEditedStudent({
            ...editedStudent,
            guardians: editedStudent.guardians.filter(g => g.id !== guardianId)
        });
    };

    const handleGuardianChange = (guardianId: string, field: keyof Guardian, value: string) => {
        if (!editedStudent) return;

        setEditedStudent({
            ...editedStudent,
            guardians: editedStudent.guardians.map(g =>
                g.id === guardianId ? { ...g, [field]: value } : g
            )
        });
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

    if (error || !student || !editedStudent) {
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
            {successMessage && (
                <SimpleAlert
                    type="success"
                    message={successMessage}
                    onClose={() => setSuccessMessage("")}
                />
            )}
            {errorMessage && (
                <SimpleAlert
                    type="error"
                    message={errorMessage}
                    onClose={() => setErrorMessage("")}
                />
            )}

            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                {/* Back button */}
                <button
                    onClick={() => router.push(referrer)}
                    className="flex items-center gap-2 mb-4 text-gray-600 hover:text-gray-900 transition-colors"
                >
                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
                    </svg>
                    <span className="text-sm font-medium">Zurück</span>
                </button>

                {/* Student Header */}
                <div className="bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-6 mb-6">
                    <div className="flex items-center justify-between">
                        <div>
                            <h1 className="text-2xl font-bold text-gray-900">
                                {student.first_name} {student.second_name}
                            </h1>
                            <div className="flex items-center gap-4 mt-2 text-sm text-gray-600">
                                <span>Klasse {student.school_class}</span>
                                {student.group_name && (
                                    <>
                                        <span>•</span>
                                        <span>{student.group_name}</span>
                                    </>
                                )}
                            </div>
                        </div>
                        <StatusBadge location={currentLocation?.location ?? student.current_location} roomName={currentLocation?.room?.name} />
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
                        {/* Checkout Section */}
                        {currentLocation?.location && currentLocation.location.startsWith("Anwesend") && (
                            <div className="mb-6 bg-white/50 backdrop-blur-sm rounded-2xl border border-gray-100 p-6">
                                <h3 className="text-lg font-semibold text-gray-900 mb-4">Checkout verwalten</h3>
                                <ScheduledCheckoutInfo
                                    studentId={studentId}
                                    onUpdate={() => setCheckoutUpdated(prev => prev + 1)}
                                    onScheduledCheckoutChange={setHasScheduledCheckout}
                                />
                                {!hasScheduledCheckout && (
                                    <div className="mt-4">
                                        <Button
                                            onClick={() => setShowCheckoutModal(true)}
                                            className="w-full bg-blue-500 hover:bg-blue-600 text-white"
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
                                        <div className="h-9 w-9 rounded-lg bg-[#5080D8] flex items-center justify-center">
                                            <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                            </svg>
                                        </div>
                                        <div className="text-left">
                                            <p className="font-medium text-gray-400">Raumverlauf</p>
                                            <p className="text-xs text-gray-400">Verlauf der Raumbesuche</p>
                                        </div>
                                    </div>
                                    <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
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
                                        <div className="h-9 w-9 rounded-lg bg-[#83CD2D] flex items-center justify-center">
                                            <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
                                            </svg>
                                        </div>
                                        <div className="text-left">
                                            <p className="font-medium text-gray-400">Feedbackhistorie</p>
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
                                        <div className="h-9 w-9 rounded-lg bg-[#F78C10] flex items-center justify-center">
                                            <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13" />
                                            </svg>
                                        </div>
                                        <div className="text-left">
                                            <p className="font-medium text-gray-400">Mensaverlauf</p>
                                            <p className="text-xs text-gray-400">Mahlzeiten und Bestellungen</p>
                                        </div>
                                    </div>
                                    <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                    </svg>
                                </button>
                            </div>
                        </InfoCard>

                        <div className="space-y-6 mt-6">
                            {/* Personal Information */}
                            <InfoCard
                                title="Persönliche Informationen"
                                icon={
                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                    </svg>
                                }
                                onEdit={() => setIsEditingPersonal(true)}
                                isEditing={isEditingPersonal}
                                onSave={handleSavePersonal}
                                onCancel={() => {
                                    setIsEditingPersonal(false);
                                    setEditedStudent(student);
                                }}
                            >
                                <InfoItem
                                    label="Vorname"
                                    value={isEditingPersonal ? editedStudent.first_name ?? "" : student.first_name ?? ""}
                                    isEditing={isEditingPersonal}
                                    onChange={(value) => setEditedStudent({ ...editedStudent, first_name: value })}
                                />
                                <InfoItem
                                    label="Nachname"
                                    value={isEditingPersonal ? editedStudent.second_name ?? "" : student.second_name ?? ""}
                                    isEditing={isEditingPersonal}
                                    onChange={(value) => setEditedStudent({ ...editedStudent, second_name: value })}
                                />
                                <InfoItem
                                    label="Klasse"
                                    value={isEditingPersonal ? editedStudent.school_class : student.school_class}
                                    isEditing={isEditingPersonal}
                                    onChange={(value) => setEditedStudent({ ...editedStudent, school_class: value })}
                                />
                                <InfoItem label="Gruppe" value={student.group_name ?? 'Nicht zugewiesen'} />
                                <InfoItem
                                    label="Geburtsdatum"
                                    value={isEditingPersonal ? editedStudent.birthday ?? "" : student.birthday ? new Date(student.birthday).toLocaleDateString('de-DE') : 'Nicht angegeben'}
                                    isEditing={isEditingPersonal}
                                    type="date"
                                    onChange={(value) => setEditedStudent({ ...editedStudent, birthday: value })}
                                />
                                <InfoItem label="Buskind" value={student.buskind ? 'Ja' : 'Nein'} />
                                {student.notes && <InfoItem label="Notizen" value={student.notes} />}
                            </InfoCard>

                            {/* Guardians Information */}
                            <InfoCard
                                title="Erziehungsberechtigte"
                                icon={
                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                    </svg>
                                }
                                onEdit={() => setIsEditingGuardians(true)}
                                isEditing={isEditingGuardians}
                                onSave={handleSaveGuardians}
                                onCancel={() => {
                                    setIsEditingGuardians(false);
                                    setEditedStudent(student);
                                }}
                            >
                                {isEditingGuardians ? (
                                    <>
                                        {editedStudent.guardians.map((guardian, index) => (
                                            <div key={guardian.id} className="border border-gray-100 rounded-lg p-4 space-y-3">
                                                <div className="flex justify-between items-start">
                                                    <h4 className="text-sm font-medium text-gray-700">
                                                        Erziehungsberechtigte/r {index + 1}
                                                    </h4>
                                                    {editedStudent.guardians.length > 1 && (
                                                        <button
                                                            onClick={() => handleRemoveGuardian(guardian.id ?? "")}
                                                            className="text-red-500 hover:text-red-700"
                                                        >
                                                            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                                            </svg>
                                                        </button>
                                                    )}
                                                </div>
                                                <InfoItem
                                                    label="Name"
                                                    value={guardian.name}
                                                    isEditing={true}
                                                    onChange={(value) => handleGuardianChange(guardian.id ?? "", "name", value)}
                                                />
                                                <InfoItem
                                                    label="E-Mail"
                                                    value={guardian.email}
                                                    isEditing={true}
                                                    type="email"
                                                    onChange={(value) => handleGuardianChange(guardian.id ?? "", "email", value)}
                                                />
                                                <InfoItem
                                                    label="Telefon"
                                                    value={guardian.phone}
                                                    isEditing={true}
                                                    type="tel"
                                                    onChange={(value) => handleGuardianChange(guardian.id ?? "", "phone", value)}
                                                />
                                            </div>
                                        ))}
                                        <button
                                            onClick={handleAddGuardian}
                                            className="w-full p-3 border-2 border-dashed border-gray-300 rounded-lg text-gray-600 hover:border-gray-400 hover:text-gray-700 transition-all"
                                        >
                                            <svg className="h-5 w-5 inline mr-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6v6m0 0v6m0-6h6m-6 0H6" />
                                            </svg>
                                            Weitere/n Erziehungsberechtigte/n hinzufügen
                                        </button>
                                    </>
                                ) : (
                                    <>
                                        {student.guardians.map((guardian, index) => (
                                            <div key={guardian.id} className="space-y-3">
                                                {student.guardians.length > 1 && (
                                                    <h4 className="text-sm font-medium text-gray-700 border-t border-gray-100 pt-3">
                                                        Erziehungsberechtigte/r {index + 1}
                                                    </h4>
                                                )}
                                                <InfoItem label="Name" value={guardian.name} />
                                                <InfoItem label="E-Mail" value={guardian.email} />
                                                <InfoItem label="Telefonnummer" value={guardian.phone || 'Nicht angegeben'} />
                                                <ModernContactActions
                                                    email={guardian.email}
                                                    phone={guardian.phone}
                                                    studentName={student.name}
                                                />
                                            </div>
                                        ))}
                                    </>
                                )}
                            </InfoCard>
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