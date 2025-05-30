"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "@/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { useSession } from "next-auth/react";
import { studentService } from "~/lib/api";
import type { Student, SupervisorContact, StudentLocation } from "~/lib/student-helpers";


// Extended Student type for this page
interface ExtendedStudent extends Student {
    // Override optional location fields to be required
    wc: boolean;
    school_yard: boolean;
    bus: boolean;
    // Additional fields specific to the detail page
    current_room?: string;
    guardian_name: string;
    guardian_contact: string;
    guardian_phone?: string;
    birthday?: string;
    notes?: string;
    buskind?: boolean;
    attendance_rate?: number;
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

    // Fetch student data from API
    useEffect(() => {
        const fetchStudent = async () => {
            setLoading(true);
            setError(null);

            try {
                const response = await studentService.getStudent(studentId);
                
                // Debug logging (only in development)
                if (process.env.NODE_ENV !== "production") {
                    console.log("Student API Response:", response);
                }
                
                // Extract the actual student data from the wrapped response
                interface WrappedResponse {
                    data?: unknown;
                    success?: boolean;
                    message?: string;
                }
                const wrappedResponse = response as WrappedResponse;
                const fetchedStudent = wrappedResponse.data ?? response;
                
                // Check if the response has the detailed format with proper typing
                interface DetailedStudentResponse {
                    id?: number;
                    first_name?: string;
                    last_name?: string;
                    name?: string;
                    school_class?: string;
                    group_id?: number;
                    group_name?: string;
                    location?: string;
                    guardian_name?: string;
                    guardian_contact?: string;
                    guardian_phone?: string;
                    guardian_email?: string;
                    has_full_access?: boolean;
                    group_supervisors?: SupervisorContact[];
                }
                const detailedResponse = fetchedStudent as DetailedStudentResponse;
                const hasAccess = detailedResponse.has_full_access ?? true;
                const groupSupervisors = detailedResponse.group_supervisors ?? [];
                
                // Debug logging (only in development)
                if (process.env.NODE_ENV !== "production") {
                    console.log("Extracted Student Data:", fetchedStudent);
                    console.log("Has Full Access:", hasAccess);
                    console.log("Supervisors:", groupSupervisors);
                }
                
                // Map the API response to the expected format
                // Backend returns snake_case fields, frontend expects different names
                const mappedStudent: ExtendedStudent = {
                    id: String(detailedResponse.id ?? ""),
                    first_name: detailedResponse.first_name ?? "",
                    second_name: detailedResponse.last_name ?? "", // Backend uses last_name
                    name: detailedResponse.name ?? `${detailedResponse.first_name ?? ""} ${detailedResponse.last_name ?? ""}`,
                    school_class: detailedResponse.school_class ?? "",
                    group_id: String(detailedResponse.group_id ?? ""),
                    group_name: detailedResponse.group_name ?? "",
                    current_location: (detailedResponse.location as StudentLocation) ?? "Unknown",
                    in_house: detailedResponse.location === "In House",
                    wc: detailedResponse.location === "WC",
                    school_yard: detailedResponse.location === "School Yard",
                    bus: detailedResponse.location === "Bus",
                    current_room: undefined, // Not available from API yet
                    guardian_name: hasAccess ? (detailedResponse.guardian_name ?? "") : "",
                    guardian_contact: hasAccess ? (detailedResponse.guardian_contact ?? "") : "",
                    guardian_phone: hasAccess ? (detailedResponse.guardian_phone ?? "") : "",
                    birthday: undefined, // Not available from API yet
                    notes: undefined, // Not available from API yet
                    buskind: undefined, // Not available from API yet
                    attendance_rate: undefined // Not available from API yet
                };

                setStudent(mappedStudent);
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
    }, [studentId]);

    // Helper function to determine status label and color
    const getStatusDetails = () => {
        if (student?.in_house) {
            return { label: student.current_room ?? "Im Haus", bgColor: "bg-green-500", textColor: "text-green-800", bgLight: "bg-green-100" };
        } else if (student?.wc) {
            return { label: "Toilette", bgColor: "bg-blue-500", textColor: "text-blue-800", bgLight: "bg-blue-100" };
        } else if (student?.school_yard) {
            return { label: "Schulhof", bgColor: "bg-yellow-500", textColor: "text-yellow-800", bgLight: "bg-yellow-100" };
        } else if (student?.current_location === "Home" || (!student?.in_house && !student?.wc && !student?.school_yard)) {
            return { label: "Zuhause", bgColor: "bg-orange-500", textColor: "text-orange-800", bgLight: "bg-orange-100" };
        } else if (student?.current_location === "Bus") {
            return { label: "Unterwegs", bgColor: "bg-purple-500", textColor: "text-purple-800", bgLight: "bg-purple-100" };
        }
        return { label: "Unbekannt", bgColor: "bg-gray-500", textColor: "text-gray-800", bgLight: "bg-gray-100" };
    };

    // Get year from class
    const getYear = (schoolClass: string): number => {
        const yearMatch = /^(\d)/.exec(schoolClass);
        return yearMatch?.[1] ? parseInt(yearMatch[1], 10) : 0;
    };

    // Determine color for year indicator
    const getYearColor = (year: number): string => {
        switch (year) {
            case 1: return "bg-blue-500";
            case 2: return "bg-green-500";
            case 3: return "bg-yellow-500";
            case 4: return "bg-purple-500";
            default: return "bg-gray-400";
        }
    };

    const status = getStatusDetails();
    const year = student ? getYear(student.school_class) : 0;
    const yearColor = getYearColor(year);

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
                    <Alert
                        type="error"
                        message={error ?? "Schüler nicht gefunden"}
                    />
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
            <div className="max-w-7xl mx-auto">
                            {/* Back Button */}
                            <div className="mb-6">
                                <button
                                    onClick={() => router.push(referrer)}
                                    className="flex items-center text-gray-600 hover:text-blue-600 transition-colors"
                                >
                                    <svg
                                        xmlns="http://www.w3.org/2000/svg"
                                        className="h-5 w-5 mr-1"
                                        fill="none"
                                        viewBox="0 0 24 24"
                                        stroke="currentColor"
                                    >
                                        <path
                                            strokeLinecap="round"
                                            strokeLinejoin="round"
                                            strokeWidth={2}
                                            d="M10 19l-7-7m0 0l7-7m-7 7h18"
                                        />
                                    </svg>
                                    Zurück
                                </button>
                            </div>

                            {/* Check if user has limited access */}
                            {!hasFullAccess ? (
                                // Limited Access View
                                <>
                                    {/* Student Basic Info */}
                                    <div className="relative mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-gray-400 to-gray-600 p-6 text-white shadow-md">
                                        <div className="flex items-center">
                                            <div className="mr-6 flex h-24 w-24 items-center justify-center rounded-full bg-white/30 text-4xl font-bold">
                                                {student.first_name?.[0] ?? ''}{student.second_name?.[0] ?? ''}
                                            </div>
                                            <div>
                                                <h1 className="text-3xl font-bold">{student.name}</h1>
                                                <div className="flex items-center mt-1">
                                                    <span className="opacity-90">Klasse {student.school_class}</span>
                                                    <span className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`} title={`Jahrgang ${year}`}></span>
                                                    {student.group_name && (
                                                        <>
                                                            <span className="mx-2">•</span>
                                                            <span className="opacity-90">Gruppe: {student.group_name}</span>
                                                        </>
                                                    )}
                                                </div>
                                            </div>
                                        </div>
                                    </div>

                                    {/* Limited Access Notice */}
                                    <div className="mb-8 rounded-lg bg-yellow-50 border border-yellow-200 p-6">
                                        <div className="flex items-start">
                                            <svg className="h-6 w-6 text-yellow-600 mt-0.5 mr-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
                                            </svg>
                                            <div>
                                                <h3 className="text-lg font-medium text-yellow-800">Eingeschränkter Zugriff</h3>
                                                <p className="mt-2 text-yellow-700">
                                                    Sie haben keinen Zugriff auf die vollständigen Schülerdaten, da Sie nicht die Gruppe dieses Schülers betreuen.
                                                </p>
                                            </div>
                                        </div>
                                    </div>

                                    {/* Group Supervisors Contact */}
                                    {supervisors.length > 0 && (
                                        <div className="rounded-lg bg-white p-6 shadow-sm">
                                            <h2 className="mb-4 text-xl font-bold text-gray-800">
                                                Ansprechpartner für diesen Schüler
                                            </h2>
                                            <p className="mb-4 text-gray-600">
                                                Bitte kontaktieren Sie eine der folgenden Personen für weitere Informationen:
                                            </p>
                                            <div className="space-y-3">
                                                {supervisors.map((supervisor) => (
                                                    <div key={supervisor.id} className="border rounded-lg p-4 bg-gray-50">
                                                        <div className="flex items-center justify-between">
                                                            <div>
                                                                <p className="font-medium text-gray-900">
                                                                    {supervisor.first_name} {supervisor.last_name}
                                                                </p>
                                                                <p className="text-sm text-gray-500 capitalize">{supervisor.role}</p>
                                                                {supervisor.email && (
                                                                    <p className="text-sm text-gray-600 mt-1">{supervisor.email}</p>
                                                                )}
                                                            </div>
                                                            {supervisor.email && (
                                                                <Button
                                                                    variant="outline"
                                                                    size="sm"
                                                                    onClick={() => {
                                                                        window.location.href = `mailto:${supervisor.email}?subject=Anfrage zu ${student.name}`;
                                                                    }}
                                                                >
                                                                    E-Mail senden
                                                                </Button>
                                                            )}
                                                        </div>
                                                    </div>
                                                ))}
                                            </div>
                                        </div>
                                    )}
                                </>
                            ) : (
                                // Full Access View (existing content)
                                <>
                                    {/* Student Profile Header with Status */}
                                    <div className="relative mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-teal-500 to-blue-600 p-6 text-white shadow-md">
                                        <div className="flex items-center">
                                            <div className="mr-6 flex h-24 w-24 items-center justify-center rounded-full bg-white/30 text-4xl font-bold">
                                                {student.first_name?.[0] ?? ''}{student.second_name?.[0] ?? ''}
                                            </div>
                                            <div>
                                                <h1 className="text-3xl font-bold">{student.name}</h1>
                                                <div className="flex items-center mt-1">
                                                    <span className="opacity-90">Klasse {student.school_class}</span>
                                                    <span className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`} title={`Jahrgang ${year}`}></span>
                                                    <span className="mx-2">•</span>
                                                    <span className="opacity-90">Gruppe: {student.group_name}</span>
                                                </div>

                                                {/* Aktueller Standort - besser sichtbar - jetzt mit Raum */}
                                                <div className="mt-3 flex items-center">
                                                    <span className="text-white font-medium mr-2">Aktueller Standort:</span>
                                                    <div className={`rounded-full px-3 py-1 ${status.bgLight} ${status.textColor} font-medium flex items-center`}>
                                                        <span className={`mr-1.5 inline-block h-2.5 w-2.5 rounded-full ${status.bgColor}`}></span>
                                                        {status.label}
                                                    </div>
                                                </div>
                                            </div>
                                        </div>
                                    </div>

                                    {/* Navigation Tabs */}
                                    <div className="mb-8 grid grid-cols-3 gap-4">
                                        <button
                                            className="flex flex-col items-center justify-center rounded-lg bg-white p-4 shadow-sm transition-all hover:shadow-md border border-gray-100 hover:border-blue-200"
                                            onClick={() => router.push(`/students/${studentId}/room-history?from=${referrer}`)}
                                        >
                                            <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8 text-blue-500 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                            </svg>
                                            <span className="text-gray-800 font-medium">Raumverlauf</span>
                                        </button>

                                        <button
                                            className="flex flex-col items-center justify-center rounded-lg bg-white p-4 shadow-sm transition-all hover:shadow-md border border-gray-100 hover:border-blue-200"
                                            onClick={() => router.push(`/students/${studentId}/feedback-history?from=${referrer}`)}
                                        >
                                            <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8 text-green-500 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
                                            </svg>
                                            <span className="text-gray-800 font-medium">Feedbackhistorie</span>
                                        </button>

                                        <button
                                            className="flex flex-col items-center justify-center rounded-lg bg-white p-4 shadow-sm transition-all hover:shadow-md border border-gray-100 hover:border-blue-200"
                                            onClick={() => router.push(`/students/${studentId}/mensa-history?from=${referrer}`)}
                                        >
                                            {/* Gabel Icon für Mensa */}
                                            <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8 text-yellow-500 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13" />
                                            </svg>
                                            <span className="text-gray-800 font-medium">Mensaverlauf</span>
                                        </button>
                                    </div>

                                    {/* Student Information */}
                                    <div className="grid grid-cols-1 gap-6 md:grid-cols-2">
                                {/* Personal Information */}
                                <div className="rounded-lg bg-white p-6 shadow-sm">
                                    <h2 className="mb-4 border-b border-blue-200 pb-2 text-xl font-bold text-gray-800">
                                        Persönliche Informationen
                                    </h2>

                                    <div className="space-y-4">
                                        <div>
                                            <p className="text-sm text-gray-500">Vollständiger Name</p>
                                            <p className="font-medium">{student.name}</p>
                                        </div>

                                        <div>
                                            <p className="text-sm text-gray-500">Klasse</p>
                                            <p className="font-medium">{student.school_class}</p>
                                        </div>

                                        <div>
                                            <p className="text-sm text-gray-500">Gruppe</p>
                                            <p className="font-medium">{student.group_name}</p>
                                        </div>

                                        <div>
                                            <p className="text-sm text-gray-500">Geburtsdatum</p>
                                            <p className="font-medium">
                                                {student.birthday ? new Date(student.birthday).toLocaleDateString('de-DE') : 'Nicht angegeben'}
                                            </p>
                                        </div>

                                        {/* Buskind hinzugefügt */}
                                        <div>
                                            <p className="text-sm text-gray-500">Buskind</p>
                                            <p className="font-medium">
                                                {student.buskind ? 'Ja' : 'Nein'}
                                            </p>
                                        </div>

                                        {student.notes && (
                                            <div>
                                                <p className="text-sm text-gray-500">Notizen</p>
                                                <p className="font-medium">{student.notes}</p>
                                            </div>
                                        )}
                                    </div>
                                </div>

                                {/* Guardian Information */}
                                <div className="rounded-lg bg-white p-6 shadow-sm">
                                    <h2 className="mb-4 border-b border-purple-200 pb-2 text-xl font-bold text-gray-800">
                                        Erziehungsberechtigte
                                    </h2>

                                    <div className="space-y-4">
                                        <div>
                                            <p className="text-sm text-gray-500">Name</p>
                                            <p className="font-medium">{student.guardian_name}</p>
                                        </div>

                                        <div>
                                            <p className="text-sm text-gray-500">E-Mail</p>
                                            <p className="font-medium">{student.guardian_contact}</p>
                                        </div>

                                        {/* Telefonnummer hinzugefügt */}
                                        <div>
                                            <p className="text-sm text-gray-500">Telefonnummer</p>
                                            <p className="font-medium">{student.guardian_phone ?? 'Nicht angegeben'}</p>
                                        </div>

                                        <div className="border-t border-gray-200 pt-4">
                                            <h3 className="font-medium text-gray-800 mb-2">Kontaktoptionen:</h3>
                                            <div className="flex gap-2">
                                                <Button
                                                    variant="outline"
                                                    className="flex items-center gap-2"
                                                    onClick={() => {
                                                        if (student?.guardian_contact) {
                                                            window.location.href = `mailto:${student.guardian_contact}?subject=Betreff: ${student.name}`;
                                                        }
                                                    }}
                                                >
                                                    <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                                                    </svg>
                                                    E-Mail
                                                </Button>
                                                <Button
                                                    variant="outline"
                                                    className="flex items-center gap-2"
                                                    onClick={() => {
                                                        if (student?.guardian_phone) {
                                                            window.location.href = `tel:${student.guardian_phone.replace(/\s+/g, '')}`;
                                                        }
                                                    }}
                                                    disabled={!student?.guardian_phone}
                                                >
                                                    <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
                                                    </svg>
                                                    Anrufen
                                                </Button>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </div>
                                </>
                            )}
                        </div>
                    </ResponsiveLayout>
                );
            }