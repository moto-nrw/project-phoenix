"use client";

import { useEffect, useState } from "react";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { useSession } from "next-auth/react";
import { studentService } from "~/lib/api";
import type { Student, SupervisorContact } from "~/lib/student-helpers";
import { 
  ModernStudentProfile,
  ModernInfoCard, 
  ModernInfoItem, 
  ModernContactActions
} from "~/components/simple/student";


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

    // Fetch student data from API
    useEffect(() => {
        const fetchStudent = async () => {
            setLoading(true);
            setError(null);

            try {
                const response = await studentService.getStudent(studentId);
                
                // The API route handler already maps the response correctly
                // Extract the student data directly without re-mapping
                interface WrappedResponse {
                    data?: unknown;
                    success?: boolean;
                    message?: string;
                }
                const wrappedResponse = response as WrappedResponse;
                const studentData = wrappedResponse.data ?? response;
                
                // Cast to Student type since API route already mapped correctly
                const mappedStudent = studentData as Student & {
                    has_full_access?: boolean;
                    group_supervisors?: SupervisorContact[];
                    guardian_name?: string;
                    guardian_contact?: string;
                    guardian_phone?: string;
                };
                
                const hasAccess = mappedStudent.has_full_access ?? true;
                const groupSupervisors = mappedStudent.group_supervisors ?? [];
                
                console.log('Student access check:', {
                    studentId,
                    hasFullAccess: mappedStudent.has_full_access,
                    hasAccess,
                    willFetchLocation: hasAccess
                });
                
                // Create ExtendedStudent with the properly mapped data
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
                    current_room: undefined, // Not available from API yet
                    guardian_name: hasAccess ? (mappedStudent.guardian_name ?? "") : "",
                    guardian_contact: hasAccess ? (mappedStudent.guardian_contact ?? "") : "",
                    guardian_phone: hasAccess ? (mappedStudent.guardian_phone ?? "") : "",
                    birthday: undefined, // Not available from API yet
                    notes: undefined, // Not available from API yet
                    buskind: mappedStudent.bus, // Use bus field for buskind
                    attendance_rate: undefined // Not available from API yet
                };

                setStudent(extendedStudent);
                setHasFullAccess(hasAccess);
                setSupervisors(groupSupervisors);
                
                // If user has full access (is a supervisor), fetch current location
                if (hasAccess) {
                    console.log('Fetching location for student:', studentId);
                    try {
                        const locationResponse = await fetch(`/api/students/${studentId}/current-location`);
                        if (locationResponse.ok) {
                            const response = await locationResponse.json();
                            console.log('Location response received:', response);
                            
                            // Unwrap the response
                            const locationData = response.data || response;
                            console.log('Location data:', locationData);
                            
                            setCurrentLocation(locationData);
                        } else {
                            console.error("Location response not ok:", locationResponse.status);
                        }
                    } catch (locationErr) {
                        console.error("Error fetching student location:", locationErr);
                    }
                } else {
                    console.log('Not fetching location - no access for student:', studentId);
                }
                
                setLoading(false);
            } catch (err) {
                console.error("Error fetching student:", err);
                setError("Fehler beim Laden der Schülerdaten.");
                setLoading(false);
            }
        };

        void fetchStudent();
    }, [studentId]);

    // Helper functions moved to individual components for better separation of concerns

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
            {/* Desktop Back Button (visible only on larger screens) */}
            <div className="hidden sm:block mb-2 sm:mb-3 px-4 sm:px-6 lg:px-8 py-1">
                <button
                    onClick={() => router.push(referrer)}
                    className="group flex items-center min-h-[36px] px-3 py-1 rounded-lg text-gray-600 hover:text-gray-900 hover:bg-white/60 transition-all duration-200"
                >
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="h-5 w-5 mr-2 group-hover:-translate-x-0.5 transition-transform duration-200 flex-shrink-0"
                        fill="none"
                        viewBox="0 0 24 24"
                        stroke="currentColor"
                    >
                        <path
                            strokeLinecap="round"
                            strokeLinejoin="round"
                            strokeWidth={1.5}
                            d="M15 19l-7-7 7-7"
                        />
                    </svg>
                    <span className="font-medium text-sm">Zurück</span>
                </button>
            </div>

            {/* Mobile Back Button - centered between header and student profile */}
            <div className="block sm:hidden sticky top-[67px] z-40 mb-2">
                <button
                    onClick={() => router.push(referrer)}
                    className="flex items-center gap-1 pl-1 pr-2 py-1 transition-all duration-200 hover:opacity-70"
                >
                    <svg
                        xmlns="http://www.w3.org/2000/svg"
                        className="h-5 w-5 text-gray-600"
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
                    <span className="text-sm font-medium text-gray-700">Meine Gruppe</span>
                </button>
            </div>

            {/* Main content container */}
            <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
                
                {/* Mobile Sticky Container for Student Profile */}
                <div className="block sm:hidden sticky top-[95px] z-40 -mx-4">
                    <div className="px-4 pb-2">

                    {/* Check if user has limited access */}
                    {!hasFullAccess ? (
                        // Limited Access View
                        <>
                            {/* Mobile-Optimized Student Profile */}
                            <ModernStudentProfile 
                                student={{
                                    first_name: student.first_name ?? '',
                                    second_name: student.second_name ?? '',
                                    name: student.name ?? '',
                                    school_class: student.school_class ?? '',
                                    group_name: student.group_name,
                                    current_location: currentLocation?.location || student.current_location,
                                    current_room: currentLocation?.room?.name
                                }}
                                index={0}
                            />
                        </>
                    ) : (
                        // Full Access View
                        <>
                            {/* Mobile-Optimized Student Profile */}
                            <ModernStudentProfile 
                                student={{
                                    first_name: student.first_name ?? '',
                                    second_name: student.second_name ?? '',
                                    name: student.name ?? '',
                                    school_class: student.school_class ?? '',
                                    group_name: student.group_name,
                                    current_location: currentLocation?.location || student.current_location,
                                    current_room: currentLocation?.room?.name
                                }}
                                index={0}
                            />
                        </>
                    )}
                    </div>
                </div>

                {/* Content wrapper with margin to prevent overlap with sticky header on mobile */}
                <div className="sm:mt-0">
                    {/* Content that appears on both mobile and desktop */}
                    {!hasFullAccess ? (
                            // Limited Access View
                            <>
                                {/* Mobile-Optimized Limited Access Notice */}
                                <div className="mb-6 rounded-lg bg-yellow-50 border border-yellow-200 p-4">
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

                                {/* Mobile-Optimized Group Supervisors Contact */}
                                {supervisors.length > 0 && (
                                    <div className="rounded-lg bg-white p-4 shadow-sm">
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
                            // Full Access View
                            <>
                                {/* Rest of mobile content... */}
                            </>
                        )}

                {/* Desktop only - Back button and Student Profile */}
                <div className="hidden sm:block">
                    {/* Check if user has limited access */}
                    {!hasFullAccess ? (
                        // Limited Access View
                        <>
                            {/* Student Profile */}
                            <div className="mb-6 sm:mb-8">
                                <ModernStudentProfile 
                                    student={{
                                        first_name: student.first_name ?? '',
                                        second_name: student.second_name ?? '',
                                        name: student.name ?? '',
                                        school_class: student.school_class ?? '',
                                        group_name: student.group_name,
                                        current_location: currentLocation?.location || student.current_location,
                                        current_room: currentLocation?.room?.name
                                    }}
                                    index={0}
                                />
                            </div>

                                    {/* Mobile-Optimized Limited Access Notice */}
                                    <div className="mb-6 sm:mb-8 rounded-lg bg-yellow-50 border border-yellow-200 p-4 sm:p-6">
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

                                    {/* Mobile-Optimized Group Supervisors Contact */}
                                    {supervisors.length > 0 && (
                                        <div className="rounded-lg bg-white p-4 sm:p-6 shadow-sm">
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
                        // Full Access View
                        <>
                            {/* Student Profile */}
                            <div className="mb-6 sm:mb-8">
                                <ModernStudentProfile 
                                    student={{
                                        first_name: student.first_name ?? '',
                                        second_name: student.second_name ?? '',
                                        name: student.name ?? '',
                                        school_class: student.school_class ?? '',
                                        group_name: student.group_name,
                                        current_location: currentLocation?.location || student.current_location,
                                        current_room: currentLocation?.room?.name
                                    }}
                                    index={0}
                                />
                            </div>

                                    {/* History Navigation Container */}
                                    <div className="mb-6 sm:mb-8">
                                        <ModernInfoCard 
                                            title="Historien" 
                                            icon={
                                                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                </svg>
                                            }
                                            iconColor="text-white"
                                            iconBg="bg-[#6366F1]"
                                            index={2}
                                            disableHover={true}
                                        >
                                            {/* Modern Navigation Grid */}
                                            <div className="grid grid-cols-1 gap-3">
                                                {/* Room History Button - Disabled */}
                                                <button
                                                    type="button"
                                                    disabled
                                                    className="group/btn cursor-not-allowed flex items-center justify-between p-4 rounded-xl bg-gray-50/50 border border-gray-100/30 opacity-50 z-10 relative"
                                                >
                                                    <div className="flex items-center gap-3">
                                                        <div className="h-10 w-10 rounded-lg bg-[#5080D8] flex items-center justify-center shadow-sm group-hover/btn:shadow-[#5080D8]/30 group-hover/btn:shadow-lg transition-all duration-300">
                                                            <svg className="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                                            </svg>
                                                        </div>
                                                        <div className="text-left">
                                                            <p className="font-semibold text-gray-900 group-hover/btn:text-[#5080D8] transition-colors duration-300">Raumverlauf</p>
                                                            <p className="text-xs text-gray-500 group-hover/btn:text-gray-600">Verlauf der Raumbesuche</p>
                                                        </div>
                                                    </div>
                                                    <svg className="h-5 w-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                    </svg>
                                                </button>

                                                {/* Feedback History Button - Disabled */}
                                                <button
                                                    type="button"
                                                    disabled
                                                    className="group/btn cursor-not-allowed flex items-center justify-between p-4 rounded-xl bg-gray-50/50 border border-gray-100/30 opacity-50 z-10 relative"
                                                >
                                                    <div className="flex items-center gap-3">
                                                        <div className="h-10 w-10 rounded-lg bg-[#83CD2D] flex items-center justify-center shadow-sm group-hover/btn:shadow-[#83CD2D]/30 group-hover/btn:shadow-lg transition-all duration-300">
                                                            <svg className="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
                                                            </svg>
                                                        </div>
                                                        <div className="text-left">
                                                            <p className="font-semibold text-gray-900 group-hover/btn:text-[#83CD2D] transition-colors duration-300">Feedbackhistorie</p>
                                                            <p className="text-xs text-gray-500 group-hover/btn:text-gray-600">Feedback und Bewertungen</p>
                                                        </div>
                                                    </div>
                                                    <svg className="h-5 w-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                    </svg>
                                                </button>

                                                {/* Mensa History Button - Disabled */}
                                                <button
                                                    type="button"
                                                    disabled
                                                    className="group/btn cursor-not-allowed flex items-center justify-between p-4 rounded-xl bg-gray-50/50 border border-gray-100/30 opacity-50 z-10 relative"
                                                >
                                                    <div className="flex items-center gap-3">
                                                        <div className="h-10 w-10 rounded-lg bg-[#F78C10] flex items-center justify-center shadow-sm group-hover/btn:shadow-[#F78C10]/30 group-hover/btn:shadow-lg transition-all duration-300">
                                                            <svg className="h-5 w-5 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13" />
                                                            </svg>
                                                        </div>
                                                        <div className="text-left">
                                                            <p className="font-semibold text-gray-900 group-hover/btn:text-[#F78C10] transition-colors duration-300">Mensaverlauf</p>
                                                            <p className="text-xs text-gray-500 group-hover/btn:text-gray-600">Mahlzeiten und Bestellungen</p>
                                                        </div>
                                                    </div>
                                                    <svg className="h-5 w-5 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                    </svg>
                                                </button>
                                            </div>
                                        </ModernInfoCard>
                                    </div>

                                    {/* Mobile-Optimized Student Information */}
                                    <div className="space-y-6 sm:space-y-8">
                                        {/* Personal Information */}
                                        <ModernInfoCard 
                                            title="Persönliche Informationen" 
                                            icon={
                                                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                                </svg>
                                            }
                                            iconColor="text-white"
                                            iconBg="bg-[#5080D8]"
                                            index={0}
                                            disableHover={true}
                                        >
                                            <ModernInfoItem 
                                                label="Vollständiger Name" 
                                                value={student.name}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.121 17.804A13.937 13.937 0 0112 16c2.5 0 4.847.655 6.879 1.804M15 10a3 3 0 11-6 0 3 3 0 016 0zm6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                    </svg>
                                                }
                                            />
                                            <ModernInfoItem 
                                                label="Klasse" 
                                                value={student.school_class}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                                    </svg>
                                                }
                                            />
                                            <ModernInfoItem 
                                                label="Gruppe" 
                                                value={student.group_name ?? 'Nicht zugewiesen'}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                                    </svg>
                                                }
                                            />
                                            <ModernInfoItem 
                                                label="Geburtsdatum" 
                                                value={student.birthday ? new Date(student.birthday).toLocaleDateString('de-DE') : 'Nicht angegeben'}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                                                    </svg>
                                                }
                                            />
                                            <ModernInfoItem 
                                                label="Buskind" 
                                                value={student.buskind ? 'Ja' : 'Nein'}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                                    </svg>
                                                }
                                            />
                                            {student.notes && (
                                                <ModernInfoItem 
                                                    label="Notizen" 
                                                    value={student.notes}
                                                    icon={
                                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                                        </svg>
                                                    }
                                                />
                                            )}
                                        </ModernInfoCard>

                                        {/* Guardian Information */}
                                        <ModernInfoCard 
                                            title="Erziehungsberechtigte" 
                                            icon={
                                                <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                                </svg>
                                            }
                                            iconColor="text-white"
                                            iconBg="bg-[#D946EF]"
                                            index={1}
                                            disableHover={true}
                                        >
                                            <ModernInfoItem 
                                                label="Name" 
                                                value={student.guardian_name}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                                    </svg>
                                                }
                                            />
                                            <ModernInfoItem 
                                                label="E-Mail" 
                                                value={student.guardian_contact}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                                                    </svg>
                                                }
                                            />
                                            <ModernInfoItem 
                                                label="Telefonnummer" 
                                                value={student.guardian_phone ?? 'Nicht angegeben'}
                                                icon={
                                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
                                                    </svg>
                                                }
                                            />
                                            
                                            <ModernContactActions 
                                                email={student.guardian_contact} 
                                                phone={student.guardian_phone}
                                                studentName={student.name}
                                            />
                                        </ModernInfoCard>
                                    </div>
                        </>
                    )}
                </div>
                
                {/* Mobile specific content */}
                <div className="sm:hidden">
                    {!hasFullAccess ? (
                    // Limited Access Content
                    <>
                        {/* Mobile-Optimized Limited Access Notice */}
                        <div className="mb-6 sm:mb-8 rounded-lg bg-yellow-50 border border-yellow-200 p-4 sm:p-6">
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

                        {/* Mobile-Optimized Group Supervisors Contact */}
                        {supervisors.length > 0 && (
                            <div className="rounded-lg bg-white p-4 sm:p-6 shadow-sm">
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
                    // Full Access Content
                    <>
                        {/* History Navigation Container */}
                        <div className="mb-6 sm:mb-8">
                            <ModernInfoCard 
                                title="Historien" 
                                icon={
                                    <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
                                    </svg>
                                }
                                iconColor="text-white"
                                iconBg="bg-[#6366F1]"
                                index={2}
                                disableHover={true}
                            >
                                {/* Modern Navigation Grid */}
                                <div className="grid grid-cols-1 gap-3">
                                    {/* Room History Button - Disabled */}
                                    <button
                                        type="button"
                                        disabled
                                        className="group/btn cursor-not-allowed flex items-center justify-between p-4 rounded-xl bg-gray-50/50 border border-gray-100/30 opacity-50 z-10 relative"
                                    >
                                        <div className="flex items-center gap-3">
                                            <div className="h-8 w-8 rounded-lg bg-[#5080D8] flex items-center justify-center shadow-sm group-hover/btn:shadow-[#5080D8]/30 group-hover/btn:shadow-lg transition-all duration-300">
                                                <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                                </svg>
                                            </div>
                                            <div className="text-left">
                                                <p className="font-semibold text-gray-900 group-hover/btn:text-[#5080D8] transition-colors duration-300">Raumverlauf</p>
                                                <p className="text-xs text-gray-500 group-hover/btn:text-gray-600">Verlauf der Raumbesuche</p>
                                            </div>
                                        </div>
                                        <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                        </svg>
                                    </button>

                                    {/* Feedback History Button - Disabled */}
                                    <button
                                        type="button"
                                        disabled
                                        className="group/btn cursor-not-allowed flex items-center justify-between p-4 rounded-xl bg-gray-50/50 border border-gray-100/30 opacity-50 z-10 relative"
                                    >
                                        <div className="flex items-center gap-3">
                                            <div className="h-8 w-8 rounded-lg bg-[#83CD2D] flex items-center justify-center shadow-sm group-hover/btn:shadow-[#83CD2D]/30 group-hover/btn:shadow-lg transition-all duration-300">
                                                <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 8h10M7 12h4m1 8l-4-4H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-3l-4 4z" />
                                                </svg>
                                            </div>
                                            <div className="text-left">
                                                <p className="font-semibold text-gray-900 group-hover/btn:text-[#83CD2D] transition-colors duration-300">Feedbackhistorie</p>
                                                <p className="text-xs text-gray-500 group-hover/btn:text-gray-600">Feedback und Bewertungen</p>
                                            </div>
                                        </div>
                                        <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                        </svg>
                                    </button>

                                    {/* Mensa History Button - Disabled */}
                                    <button
                                        type="button"
                                        disabled
                                        className="group/btn cursor-not-allowed flex items-center justify-between p-4 rounded-xl bg-gray-50/50 border border-gray-100/30 opacity-50 z-10 relative"
                                    >
                                        <div className="flex items-center gap-3">
                                            <div className="h-8 w-8 rounded-lg bg-[#F78C10] flex items-center justify-center shadow-sm group-hover/btn:shadow-[#F78C10]/30 group-hover/btn:shadow-lg transition-all duration-300">
                                                <svg className="h-4 w-4 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.5 3v18M7 3v3.5M10 3v3.5M7 10h3M15.5 3v3c0 1-2 2-2 2v13" />
                                                </svg>
                                            </div>
                                            <div className="text-left">
                                                <p className="font-semibold text-gray-900 group-hover/btn:text-[#F78C10] transition-colors duration-300">Mensaverlauf</p>
                                                <p className="text-xs text-gray-500 group-hover/btn:text-gray-600">Mahlzeiten und Bestellungen</p>
                                            </div>
                                        </div>
                                        <svg className="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                        </svg>
                                    </button>
                                </div>
                            </ModernInfoCard>
                        </div>

                        {/* Mobile-Optimized Student Information */}
                        <div className="space-y-6 sm:space-y-8">
                            {/* Personal Information */}
                            <ModernInfoCard 
                                title="Persönliche Informationen" 
                                icon={
                                    <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                    </svg>
                                }
                                iconColor="text-white"
                                iconBg="bg-[#5080D8]"
                                index={0}
                                disableHover={true}
                            >
                                <ModernInfoItem 
                                    label="Vollständiger Name" 
                                    value={student.name}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5.121 17.804A13.937 13.937 0 0112 16c2.5 0 4.847.655 6.879 1.804M15 10a3 3 0 11-6 0 3 3 0 016 0zm6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                        </svg>
                                    }
                                />
                                <ModernInfoItem 
                                    label="Klasse" 
                                    value={student.school_class}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                        </svg>
                                    }
                                />
                                <ModernInfoItem 
                                    label="Gruppe" 
                                    value={student.group_name ?? 'Nicht zugewiesen'}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                        </svg>
                                    }
                                />
                                <ModernInfoItem 
                                    label="Geburtsdatum" 
                                    value={student.birthday ? new Date(student.birthday).toLocaleDateString('de-DE') : 'Nicht angegeben'}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
                                        </svg>
                                    }
                                />
                                <ModernInfoItem 
                                    label="Buskind" 
                                    value={student.buskind ? 'Ja' : 'Nein'}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
                                        </svg>
                                    }
                                />
                                {student.notes && (
                                    <ModernInfoItem 
                                        label="Notizen" 
                                        value={student.notes}
                                        icon={
                                            <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
                                            </svg>
                                        }
                                    />
                                )}
                            </ModernInfoCard>

                            {/* Guardian Information */}
                            <ModernInfoCard 
                                title="Erziehungsberechtigte" 
                                icon={
                                    <svg className="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                    </svg>
                                }
                                iconColor="text-white"
                                iconBg="bg-[#D946EF]"
                                index={1}
                                disableHover={true}
                            >
                                <ModernInfoItem 
                                    label="Name" 
                                    value={student.guardian_name}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                                        </svg>
                                    }
                                />
                                <ModernInfoItem 
                                    label="E-Mail" 
                                    value={student.guardian_contact}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 8l7.89 5.26a2 2 0 002.22 0L21 8M5 19h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                                        </svg>
                                    }
                                />
                                <ModernInfoItem 
                                    label="Telefonnummer" 
                                    value={student.guardian_phone ?? 'Nicht angegeben'}
                                    icon={
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 5a2 2 0 012-2h3.28a1 1 0 01.948.684l1.498 4.493a1 1 0 01-.502 1.21l-2.257 1.13a11.042 11.042 0 005.516 5.516l1.13-2.257a1 1 0 011.21-.502l4.493 1.498a1 1 0 01.684.949V19a2 2 0 01-2 2h-1C9.716 21 3 14.284 3 6V5z" />
                                        </svg>
                                    }
                                />
                                
                                <ModernContactActions 
                                    email={student.guardian_contact} 
                                    phone={student.guardian_phone}
                                    studentName={student.name}
                                />
                            </ModernInfoCard>
                        </div>
                    </>
                )}
                </div>
                </div>
            </div>
        </ResponsiveLayout>
    );
}