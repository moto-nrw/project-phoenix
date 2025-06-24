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
                setStudents(validStudents);

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


    // Helper function to get location status with enhanced design
    const getLocationStatus = (student: Student) => {
        const studentRoomStatus = roomStatus[student.id.toString()];
        
        // Check if student is in group room
        if (studentRoomStatus?.in_group_room) {
            return { 
                label: "Im Gruppenraum", 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-emerald-50/80 to-green-100/80",
                glowColor: "ring-emerald-200/50 shadow-emerald-100/50",
                customBgColor: "#83CD2D",
                customShadow: "0 8px 25px rgba(131, 205, 45, 0.4)"
            };
        }
        
        // Check if student is in a specific room (not group room)
        if (studentRoomStatus?.current_room_id && !studentRoomStatus.in_group_room) {
            return { 
                label: `Raum ${studentRoomStatus.current_room_id}`, 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-blue-50/80 to-cyan-100/80",
                glowColor: "ring-blue-200/50 shadow-blue-100/50",
                customBgColor: "#5080D8",
                customShadow: "0 8px 25px rgba(80, 128, 216, 0.4)"
            };
        }
        
        // Check for schoolyard
        if (student.school_yard === true) {
            return { 
                label: "Schulhof", 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-amber-50/80 to-yellow-100/80",
                glowColor: "ring-amber-200/50 shadow-amber-100/50",
                customBgColor: "#F78C10",
                customShadow: "0 8px 25px rgba(247, 140, 16, 0.4)"
            };
        }
        
        // Check for in transit/movement
        if (student.in_house === true || student.current_location === "Bus") {
            return { 
                label: "Unterwegs", 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-fuchsia-50/80 to-pink-100/80",
                glowColor: "ring-fuchsia-200/50 shadow-fuchsia-100/50",
                customBgColor: "#D946EF",
                customShadow: "0 8px 25px rgba(217, 70, 239, 0.4)"
            };
        }
        
        // Default to at home
        return { 
            label: "Zuhause", 
            badgeColor: "text-white backdrop-blur-sm",
            cardGradient: "from-red-50/80 to-rose-100/80",
            glowColor: "ring-red-200/50 shadow-red-100/50",
            customBgColor: "#FF3130",
            customShadow: "0 8px 25px rgba(255, 49, 48, 0.4)"
        };
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

                {/* Mobile Search - Minimalist Design */}
                <div className="mb-6 md:hidden">
                    <div className="relative">
                        <div className="absolute inset-y-0 left-0 flex items-center pl-4">
                            <svg className="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                            </svg>
                        </div>
                        <input
                            type="text"
                            placeholder="Schüler suchen..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="w-full rounded-lg border-0 bg-white py-4 pl-12 pr-12 text-base text-gray-900 placeholder-gray-400 shadow-sm ring-1 ring-gray-200 focus:ring-2 focus:ring-blue-500 focus:outline-none transition-all"
                        />
                        {searchTerm && (
                            <button
                                onClick={() => setSearchTerm("")}
                                className="absolute inset-y-0 right-0 flex items-center pr-4"
                                aria-label="Suche löschen"
                            >
                                <svg className="h-5 w-5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        )}
                    </div>
                    
                    {/* Simple Filter Pills - Only show if needed */}
                    {(selectedYear !== "all" || attendanceFilter !== "all") && (
                        <div className="flex gap-2 mt-3">
                            {selectedYear !== "all" && (
                                <button
                                    onClick={() => setSelectedYear("all")}
                                    className="inline-flex items-center gap-1 rounded-full bg-blue-100 px-3 py-1 text-sm text-blue-700"
                                >
                                    Jahr {selectedYear}
                                    <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                    </svg>
                                </button>
                            )}
                            {attendanceFilter !== "all" && (
                                <button
                                    onClick={() => setAttendanceFilter("all")}
                                    className="inline-flex items-center gap-1 rounded-full bg-green-100 px-3 py-1 text-sm text-green-700"
                                >
                                    {attendanceFilter === "in_room" && "Im Raum"}
                                    {attendanceFilter === "in_house" && "Im Haus"}
                                    {attendanceFilter === "school_yard" && "Schulhof"}
                                    {attendanceFilter === "at_home" && "Zuhause"}
                                    <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                    </svg>
                                </button>
                            )}
                        </div>
                    )}
                    
                    {/* Minimalist Filter Access */}
                    <div className="flex justify-between items-center mt-2">
                        <button
                            onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
                            className="flex items-center gap-2 text-base text-blue-600 font-medium py-2"
                        >
                            <svg className={`h-5 w-5 transition-transform duration-200 ${isMobileFiltersOpen ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
                            </svg>
                            {isMobileFiltersOpen ? 'Filter ausblenden' : 'Filter'}
                        </button>
                        {(selectedYear !== "all" || attendanceFilter !== "all") && (
                            <button
                                onClick={() => {
                                    setSelectedYear("all");
                                    setAttendanceFilter("all");
                                }}
                                className="text-sm text-gray-500"
                            >
                                Alle löschen
                            </button>
                        )}
                    </div>
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
                                    <option value="in_house">Unterwegs</option>
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

                {/* Desktop Results Header - Hidden on mobile */}
                <div className="hidden md:block mb-6 rounded-xl bg-white shadow-md overflow-hidden">
                    <div className="p-6">
                        <div className="flex justify-between items-center mb-6">
                            <div>
                                <h2 className="text-xl font-bold text-gray-800">
                                    Schüler in dieser Gruppe
                                </h2>
                                <p className="text-sm text-gray-600 mt-1">
                                    {filteredStudents.length} {filteredStudents.length === 1 ? 'Schüler' : 'Schüler'}
                                </p>
                            </div>
                            
                            {/* Year Legend */}
                            <div className="flex items-center space-x-4">
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
                    </div>
                </div>

                {/* Mobile Error Display */}
                {error && (
                    <div className="mb-4 md:hidden">
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
                    <div>
                        {/* Add floating animation keyframes */}
                        <style jsx>{`
                            @keyframes float {
                                0%, 100% { transform: translateY(0px) rotate(var(--rotation)); }
                                50% { transform: translateY(-4px) rotate(var(--rotation)); }
                            }
                        `}</style>
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {filteredStudents.map((student, index) => {
                            const locationStatus = getLocationStatus(student);

                            return (
                                <div
                                    key={student.id}
                                    onClick={() => router.push(`/students/${student.id}?from=/ogs_groups`)}
                                    className={`group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.03] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-3 active:scale-[0.97] md:hover:border-blue-200/50`}
                                    style={{
                                        transform: `rotate(${(index % 3 - 1) * 0.8}deg)`,
                                        animation: `float 8s ease-in-out infinite ${index * 0.7}s`
                                    }}
                                >
                                    {/* Modern gradient overlay */}
                                    <div className={`absolute inset-0 bg-gradient-to-br ${locationStatus.cardGradient} opacity-[0.03] rounded-3xl`}></div>
                                    {/* Subtle inner glow */}
                                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                                    {/* Modern border highlight */}
                                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
                                    

                                    <div className="relative p-5">
                                        {/* Header with student name */}
                                        <div className="flex items-center justify-between mb-2">
                                            {/* Student Name */}
                                            <div className="flex-1 min-w-0">
                                                <div className="flex items-center gap-2">
                                                    <h3 className="text-lg font-bold text-gray-800 truncate md:group-hover:text-blue-600 transition-colors duration-300">
                                                        {student.first_name}
                                                    </h3>
                                                    {/* Subtle integrated arrow */}
                                                    <svg className="w-4 h-4 text-gray-300 md:group-hover:text-blue-500 md:group-hover:translate-x-1 transition-all duration-300 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                    </svg>
                                                </div>
                                                <p className="text-sm font-semibold text-gray-700 truncate md:group-hover:text-blue-500 transition-colors duration-300">
                                                    {student.second_name}
                                                </p>
                                            </div>
                                            
                                            {/* Status Badge */}
                                            <span 
                                                className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-bold ${locationStatus.badgeColor} ml-3`}
                                                style={{ 
                                                    backgroundColor: locationStatus.customBgColor,
                                                    boxShadow: locationStatus.customShadow
                                                }}
                                            >
                                                <span className="w-1.5 h-1.5 bg-white/80 rounded-full mr-2 animate-pulse"></span>
                                                {locationStatus.label}
                                            </span>
                                        </div>

                                        {/* Bottom row with click hint */}
                                        <div className="flex justify-start">
                                            <p className="text-xs text-gray-400 md:group-hover:text-blue-400 transition-colors duration-300">
                                                Tippen für mehr Infos
                                            </p>
                                        </div>

                                        {/* Decorative elements */}
                                        <div className="absolute top-3 left-3 w-5 h-5 bg-white/20 rounded-full animate-ping"></div>
                                        <div className="absolute bottom-3 right-3 w-3 h-3 bg-white/30 rounded-full"></div>
                                    </div>

                                    {/* Glowing border effect */}
                                    <div className="absolute inset-0 rounded-3xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
                                </div>
                            );
                        })}
                        </div>
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