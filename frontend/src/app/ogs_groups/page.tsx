"use client";

import { useState, useEffect, Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";
import { userContextService } from "~/lib/usercontext-api";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";

// Define OGSGroup type based on EducationalGroup with additional fields
interface OGSGroup {
    id: string;
    name: string;
    room_name?: string;
    room_id?: string;
    student_count?: number;
    supervisor_name?: string;
    students?: Student[];
}

function OGSGroupPageContent() {
    const { data: session, status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });
    const router = useRouter();

    // Check if user has access to OGS groups
    const [hasAccess, setHasAccess] = useState<boolean | null>(null);

    // State variables
    const [ogsGroup, setOGSGroup] = useState<OGSGroup | null>(null);
    const [students, setStudents] = useState<Student[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [selectedYear, setSelectedYear] = useState("all");
    const [attendanceFilter, setAttendanceFilter] = useState("all");
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [roomStatus, setRoomStatus] = useState<Record<string, { 
        in_group_room: boolean; 
        current_room_id?: number;
        first_name?: string;
        last_name?: string;
        reason?: string;
    }>>({});
    
    // Mobile-specific state
    const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);

    // Check access and fetch OGS group data
    useEffect(() => {
        const checkAccessAndFetchData = async () => {
            try {
                setIsLoading(true);

                // First check if user has any educational groups (OGS groups)
                const myGroups = await userContextService.getMyEducationalGroups();
                
                if (myGroups.length === 0) {
                    // User has no OGS groups, redirect to dashboard
                    setHasAccess(false);
                    redirect("/dashboard");
                    return;
                }

                setHasAccess(true);

                // Use the first group as the OGS group (assuming user has access to one OGS group)
                const educationalGroup = myGroups[0];
                
                if (!educationalGroup) {
                    throw new Error('No educational group found');
                }
                
                const ogsGroupData: OGSGroup = {
                    id: educationalGroup.id,
                    name: educationalGroup.name,
                    room_name: educationalGroup.room?.name,
                    room_id: educationalGroup.room_id,
                    student_count: 0, // Will be calculated from actual students
                    supervisor_name: undefined // Will be fetched separately if needed
                };

                setOGSGroup(ogsGroupData);

                // Fetch students for this group using the same service as students/search
                const studentsResponse = await studentService.getStudents({
                    groupId: educationalGroup.id
                });
                
                const studentsData = studentsResponse.students || [];
                setStudents(studentsData);

                // Calculate statistics from real data (only if we have valid array data)
                const validStudents = Array.isArray(studentsData) ? studentsData : [];

                // Update group with actual student count
                setOGSGroup(prev => prev ? { ...prev, student_count: validStudents.length } : null);

                // Fetch room status for all students in the group
                try {
                    const roomStatusResponse = await fetch(`/api/groups/${educationalGroup.id}/students/room-status`, {
                        headers: {
                            'Authorization': `Bearer ${session?.user?.token}`,
                            'Content-Type': 'application/json'
                        }
                    });

                    if (roomStatusResponse.ok) {
                        const response = await roomStatusResponse.json() as {
                            success: boolean;
                            message: string;
                            data: {
                                group_has_room: boolean;
                                group_room_id?: number;
                                student_room_status: Record<string, { 
                                    in_group_room: boolean; 
                                    current_room_id?: number;
                                    first_name?: string;
                                    last_name?: string;
                                    reason?: string;
                                }>;
                            };
                        };
                        
                        if (response.data?.student_room_status) {
                            setRoomStatus(response.data.student_room_status);
                        }
                    }
                } catch (roomStatusErr) {
                    console.error("Failed to fetch room status:", roomStatusErr);
                    // Don't fail the whole page if room status fails
                }

                setError(null);
            } catch (err) {
                if (err instanceof Error && err.message.includes("403")) {
                    setError("Sie haben keine Berechtigung für den Zugriff auf OGS-Gruppendaten.");
                    setHasAccess(false);
                } else {
                    setError("Fehler beim Laden der OGS-Gruppendaten.");
                }
            } finally {
                setIsLoading(false);
            }
        };

        if (session?.user?.token) {
            void checkAccessAndFetchData();
        }
    }, [session?.user?.token]);

    // Apply filters to students (ensure students is an array)
    const filteredStudents = (Array.isArray(students) ? students : []).filter((student) => {
        // Apply search filter - search in multiple fields
        if (searchTerm) {
            const searchLower = searchTerm.toLowerCase();
            const matchesSearch = 
                student.name?.toLowerCase().includes(searchLower) ||
                student.first_name?.toLowerCase().includes(searchLower) ||
                student.second_name?.toLowerCase().includes(searchLower) ||
                student.school_class?.toLowerCase().includes(searchLower);
            
            if (!matchesSearch) return false;
        }

        // Apply year filter
        if (selectedYear !== "all") {
            const yearMatch = /^(\d)/.exec(student.school_class ?? '');
            const studentYear = yearMatch ? yearMatch[1] : null;
            if (studentYear !== selectedYear) {
                return false;
            }
        }

        // Apply attendance filter
        if (attendanceFilter !== "all") {
            const studentRoomStatus = roomStatus[student.id.toString()];
            
            switch (attendanceFilter) {
                case "in_room":
                    if (!studentRoomStatus?.in_group_room) return false;
                    break;
                case "in_house":
                    // Check both the in_house flag and current_location
                    if (!student.in_house && student.current_location !== "In House") return false;
                    break;
                case "wc":
                    if (!student.wc && student.current_location !== "WC") return false;
                    break;
                case "school_yard":
                    if (!student.school_yard && student.current_location !== "School Yard") return false;
                    break;
                case "at_home":
                    // Student is at home if no location flags are set OR current_location is "Home"
                    const isAtHome = (!student.in_house && !student.wc && !student.school_yard && !studentRoomStatus?.in_group_room) ||
                                    student.current_location === "Home";
                    if (!isAtHome) return false;
                    break;
            }
        }

        return true;
    });

    // Helper function to determine the school year
    const getSchoolYear = (schoolClass: string): number => {
        const yearMatch = /^(\d)/.exec(schoolClass);
        return yearMatch?.[1] ? parseInt(yearMatch[1], 10) : 0;
    };

    // Determine color for year dot
    const getYearColor = (year: number): string => {
        switch (year) {
            case 1: return "bg-blue-500";
            case 2: return "bg-green-500";
            case 3: return "bg-yellow-500";
            case 4: return "bg-purple-500";
            default: return "bg-gray-400";
        }
    };

    // Helper function to get location status
    const getLocationStatus = (student: Student) => {
        if (student.in_house === true) return { label: "Im Haus", color: "bg-green-500 text-green-50" };
        if (student.wc === true) return { label: "Toilette", color: "bg-blue-500 text-blue-50" };
        if (student.school_yard === true) return { label: "Schulhof", color: "bg-yellow-500 text-yellow-50" };
        // Student is at home when current_location is "Home" or all location flags are false
        if (student.current_location === "Home" || (!student.in_house && !student.wc && !student.school_yard)) {
            return { label: "Zuhause", color: "bg-orange-500 text-orange-50" };
        }
        if (student.current_location === "Bus") return { label: "Unterwegs", color: "bg-purple-500 text-purple-50" };
        return { label: "Unbekannt", color: "bg-gray-500 text-gray-50" };
    };

    if (status === "loading" || isLoading || hasAccess === null) {
        return (
            <div className="flex min-h-screen items-center justify-center">
                <div className="flex flex-col items-center gap-4">
                    <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
                    <p className="text-gray-600">Daten werden geladen...</p>
                </div>
            </div>
        );
    }

    // If user doesn't have access, redirect to dashboard
    if (hasAccess === false) {
        redirect("/dashboard");
        return null;
    }

    return (
        <ResponsiveLayout>
            <div className="max-w-7xl mx-auto">
                {/* Header */}
                <div className="mb-4 md:mb-8">
                    <div className="flex items-center justify-between">
                        <h1 className="text-3xl md:text-4xl font-bold text-gray-900">{ogsGroup?.name}</h1>
                        <div className="flex items-center gap-3 px-4 py-3 bg-gray-100 rounded-full">
                            <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                            </svg>
                            <span className="text-sm font-medium text-gray-700">{ogsGroup?.student_count ?? 0}</span>
                        </div>
                    </div>
                </div>

                {/* Mobile Search Bar - Always Visible */}
                <div className="mb-4 md:hidden">
                    <div className="relative">
                        <Input
                            label="Schnellsuche"
                            name="searchTerm"
                            placeholder="Schüler suchen..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="text-base pr-10" // Prevent iOS zoom, add padding for clear button
                        />
                        {searchTerm && (
                            <button
                                onClick={() => setSearchTerm("")}
                                className="absolute right-2 top-[38px] p-1 text-gray-400 hover:text-gray-600 transition-colors"
                                aria-label="Suche löschen"
                            >
                                <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        )}
                    </div>
                </div>

                {/* Mobile Filter Toggle */}
                <div className="mb-4 md:hidden">
                    <button
                        onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
                        className="flex w-full items-center justify-between rounded-lg bg-white px-4 py-3 shadow-sm ring-1 ring-gray-200 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none"
                    >
                        <span className="text-sm font-medium text-gray-700">
                            Filter & Erweiterte Suche
                        </span>
                        <svg 
                            className={`h-5 w-5 text-gray-400 transition-transform ${isMobileFiltersOpen ? 'rotate-180' : ''}`} 
                            fill="none" 
                            viewBox="0 0 24 24" 
                            stroke="currentColor"
                        >
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                        </svg>
                    </button>
                </div>

                {/* Search Panel - Desktop always visible, Mobile collapsible */}
                <div className={`mb-6 overflow-hidden rounded-xl bg-white shadow-md transition-all duration-300 ${
                    isMobileFiltersOpen ? 'block' : 'hidden md:block'
                }`}>
                    <div className="p-4 md:p-6">
                        <h2 className="mb-4 text-lg md:text-xl font-bold text-gray-800">Suchkriterien</h2>

                        <div className="grid grid-cols-1 gap-4 md:gap-6 md:grid-cols-2 lg:grid-cols-3">
                            {/* Name Search - Desktop only (mobile has quick search above) */}
                            <div className="hidden md:block">
                                <label className="block text-sm font-medium text-gray-700 mb-1">
                                    Name
                                </label>
                                <div className="relative">
                                    <input
                                        type="text"
                                        name="searchTerm"
                                        placeholder="Vor- oder Nachname"
                                        value={searchTerm}
                                        onChange={(e) => setSearchTerm(e.target.value)}
                                        className="block w-full rounded-lg border-0 px-4 py-3 h-12 text-base text-gray-900 bg-white shadow-sm ring-1 ring-inset ring-gray-200 placeholder:text-gray-400 focus:ring-2 focus:ring-inset focus:ring-blue-500 transition-all duration-200 pr-10"
                                    />
                                    {searchTerm && (
                                        <button
                                            onClick={() => setSearchTerm("")}
                                            className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-gray-400 hover:text-gray-600 transition-colors"
                                            aria-label="Suche löschen"
                                        >
                                            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                            </svg>
                                        </button>
                                    )}
                                </div>
                            </div>

                            {/* School Year Filter */}
                            <div className="relative">
                                <label className="block text-sm font-medium text-gray-700 mb-1">
                                    Jahrgangsstufe
                                </label>
                                <select
                                    value={selectedYear}
                                    onChange={(e) => setSelectedYear(e.target.value)}
                                    className="mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 text-base shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none appearance-none pr-8"
                                >
                                    <option value="all">Alle Jahrgänge</option>
                                    <option value="1">Jahrgang 1</option>
                                    <option value="2">Jahrgang 2</option>
                                    <option value="3">Jahrgang 3</option>
                                    <option value="4">Jahrgang 4</option>
                                </select>
                                <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                    <svg className="h-5 w-5 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                        <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                    </svg>
                                </div>
                            </div>

                            {/* Attendance Status */}
                            <div className="relative">
                                <label className="block text-sm font-medium text-gray-700 mb-1">
                                    Anwesenheitsstatus
                                </label>
                                <select
                                    value={attendanceFilter}
                                    onChange={(e) => setAttendanceFilter(e.target.value)}
                                    className="mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 text-base shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-blue-500 focus:outline-none appearance-none pr-8"
                                >
                                    <option value="all">Alle</option>
                                    <option value="in_room">Im Gruppenraum</option>
                                    <option value="in_house">Im Raumwechsel</option>
                                    <option value="wc">Toilette</option>
                                    <option value="school_yard">Schulhof</option>
                                    <option value="at_home">Zuhause</option>
                                </select>
                                <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                    <svg className="h-5 w-5 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                        <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                    </svg>
                                </div>
                            </div>
                        </div>

                        {/* Active filters indicator and clear option */}
                        {(selectedYear !== "all" || attendanceFilter !== "all") && (
                            <div className="mt-4 flex items-center justify-between">
                                <p className="text-sm text-gray-600">
                                    {(() => {
                                        let activeFilters = 0;
                                        if (selectedYear !== "all") activeFilters++;
                                        if (attendanceFilter !== "all") activeFilters++;
                                        return `${activeFilters} ${activeFilters === 1 ? 'Filter aktiv' : 'Filter aktiv'}`;
                                    })()}
                                </p>
                                <button
                                    onClick={() => {
                                        setSearchTerm("");
                                        setSelectedYear("all");
                                        setAttendanceFilter("all");
                                        setIsMobileFiltersOpen(false);
                                    }}
                                    className="text-sm text-blue-600 hover:text-blue-800 hover:underline transition-colors"
                                >
                                    Alle Filter löschen
                                </button>
                            </div>
                        )}
                    </div>
                </div>

                {/* Results Section */}
                <div className="rounded-xl bg-white shadow-md overflow-hidden">
                    <div className="p-4 md:p-6">
                        {/* Results Header - Mobile Optimized */}
                        <div className="flex flex-col md:flex-row md:justify-between md:items-center mb-4 md:mb-6 gap-4">
                            <div>
                                <h2 className="text-lg md:text-xl font-bold text-gray-800">
                                    Schüler in dieser Gruppe
                                </h2>
                                <p className="text-sm text-gray-600 mt-1">
                                    {filteredStudents.length} {filteredStudents.length === 1 ? 'Schüler' : 'Schüler'}
                                </p>
                            </div>
                            
                            {/* Year Legend - Hidden on mobile, shown on tablet+ */}
                            <div className="hidden md:flex items-center space-x-4">
                                <div className="flex items-center">
                                    <span className="inline-block h-3 w-3 rounded-full bg-blue-500 mr-1"></span>
                                    <span className="text-xs text-gray-600">Jahr 1</span>
                                </div>
                                <div className="flex items-center">
                                    <span className="inline-block h-3 w-3 rounded-full bg-green-500 mr-1"></span>
                                    <span className="text-xs text-gray-600">Jahr 2</span>
                                </div>
                                <div className="flex items-center">
                                    <span className="inline-block h-3 w-3 rounded-full bg-yellow-500 mr-1"></span>
                                    <span className="text-xs text-gray-600">Jahr 3</span>
                                </div>
                                <div className="flex items-center">
                                    <span className="inline-block h-3 w-3 rounded-full bg-purple-500 mr-1"></span>
                                    <span className="text-xs text-gray-600">Jahr 4</span>
                                </div>
                            </div>
                        </div>

                        {error && (
                            <div className="mb-6">
                                <Alert type="error" message={error} />
                            </div>
                        )}

                        {/* Student Grid - Mobile Optimized */}
                        {students.length === 0 ? (
                            <div className="py-12 text-center">
                                <div className="flex flex-col items-center gap-4">
                                    <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                    </svg>
                                    <div>
                                        <h3 className="text-lg font-medium text-gray-900">Keine Schüler in dieser Gruppe</h3>
                                        <p className="text-gray-600">
                                            Es wurden noch keine Schüler zu dieser OGS-Gruppe hinzugefügt.
                                        </p>
                                        <p className="text-sm text-gray-500 mt-2">
                                            Gesamtzahl gefundener Schüler: {students.length}
                                        </p>
                                    </div>
                                </div>
                            </div>
                        ) : filteredStudents.length > 0 ? (
                            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                                {filteredStudents.map((student) => {
                                    const year = getSchoolYear(student.school_class ?? '');
                                    const yearColor = getYearColor(year);
                                    const locationStatus = getLocationStatus(student);

                                    return (
                                        <div
                                            key={student.id}
                                            onClick={() => router.push(`/students/${student.id}?from=/ogs_groups`)}
                                            className="group cursor-pointer rounded-lg border border-gray-200 bg-white p-4 shadow-sm transition-all duration-200 hover:border-blue-300 hover:shadow-md active:scale-[0.98]"
                                        >
                                            <div className="flex items-start justify-between mb-3">
                                                <div className="flex-1 min-w-0">
                                                    <h3 className="font-semibold text-gray-900 truncate group-hover:text-blue-600 transition-colors">
                                                        {student.first_name} {student.second_name}
                                                    </h3>
                                                    <div className="flex items-center mt-1 gap-2">
                                                        <span className="text-sm text-gray-500">
                                                            Klasse {student.school_class}
                                                        </span>
                                                        <span className={`inline-block h-2 w-2 rounded-full ${yearColor}`} />
                                                    </div>
                                                </div>
                                                <svg className="h-5 w-5 text-gray-400 group-hover:text-blue-500 transition-colors flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                </svg>
                                            </div>

                                            <div className="space-y-2">
                                                {student.group_name && (
                                                    <div className="flex items-center text-sm text-gray-600">
                                                        <svg className="h-4 w-4 mr-2 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                                        </svg>
                                                        Gruppe: {student.group_name}
                                                    </div>
                                                )}

                                                <div className="flex items-center justify-between">
                                                    <span className="text-sm text-gray-500">Status:</span>
                                                    <span className={`inline-flex items-center px-2 py-1 rounded-full text-xs font-medium ${locationStatus.color}`}>
                                                        {locationStatus.label}
                                                    </span>
                                                </div>
                                            </div>
                                        </div>
                                    );
                                })}
                            </div>
                        ) : (
                            <div className="py-12 text-center">
                                <div className="flex flex-col items-center gap-4">
                                    <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                    </svg>
                                    <div>
                                        <h3 className="text-lg font-medium text-gray-900">Keine Schüler gefunden</h3>
                                        <p className="text-gray-600">
                                            Versuche deine Suchkriterien anzupassen.
                                        </p>
                                        <p className="text-sm text-gray-500 mt-2">
                                            {students.length} Schüler insgesamt, {filteredStudents.length} nach Filtern
                                        </p>
                                    </div>
                                </div>
                            </div>
                        )}
                    </div>
                </div>
            </div>
        </ResponsiveLayout>
    );
}

// Main component with Suspense wrapper
export default function OGSGroupPage() {
    return (
        <Suspense fallback={
            <div className="flex min-h-screen items-center justify-center">
                <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
            </div>
        }>
            <OGSGroupPageContent />
        </Suspense>
    );
}