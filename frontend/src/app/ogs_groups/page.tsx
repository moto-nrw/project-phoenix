"use client";

import { useState, useEffect } from "react";
import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Input } from "~/components/ui";
import { Alert } from "~/components/ui/alert";
import { userContextService } from "~/lib/usercontext-api";

// Student type (should match the API response)
interface Student {
    id: string;
    name?: string;
    first_name?: string;
    second_name?: string;
    school_class?: string;
    in_house: boolean;
    wc: boolean;
    school_yard: boolean;
    bus: boolean;
}


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

export default function OGSGroupPage() {
    const { data: session, status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });

    // Check if user has access to OGS groups
    const [hasAccess, setHasAccess] = useState<boolean | null>(null);

    // State variables
    const [ogsGroup, setOGSGroup] = useState<OGSGroup | null>(null);
    const [students, setStudents] = useState<Student[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [selectedYear, setSelectedYear] = useState<string>("all");
    const [attendanceFilter, setAttendanceFilter] = useState<string>("all");
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    const [roomStatus, setRoomStatus] = useState<Record<string, { 
        in_group_room: boolean; 
        current_room_id?: number;
        first_name?: string;
        last_name?: string;
        reason?: string;
    }>>({})

    // Statistics
    const [stats, setStats] = useState({
        totalStudents: 0,
        presentStudents: 0,
        absentStudents: 0,
        schoolyard: 0,
        bathroom: 0,
        inHouse: 0,
    });

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

                // Fetch students for this group
                const response = await fetch(`/api/students?group_id=${educationalGroup.id}`, {
                    headers: {
                        'Authorization': `Bearer ${session?.user?.token}`,
                        'Content-Type': 'application/json'
                    }
                });

                if (!response.ok) {
                    throw new Error(`Failed to fetch students: ${response.status}`);
                }

                const responseData: { data?: Student[] } = await response.json() as { data?: Student[] };
                
                // Extract data from API response wrapper
                const studentsData: Student[] = responseData.data ?? responseData as Student[];
                
                // Ensure studentsData is an array
                if (Array.isArray(studentsData)) {
                    setStudents(studentsData);
                } else {
                    setStudents([]);
                }

                // Calculate statistics from real data (only if we have valid array data)
                const validStudents = Array.isArray(studentsData) ? studentsData : [];
                const inHouseCount = validStudents.filter(s => s.in_house).length;
                const schoolyardCount = validStudents.filter(s => s.school_yard).length;
                const bathroomCount = validStudents.filter(s => s.wc).length;
                // Note: "Im Gruppenraum" would require checking actual room visits - not implemented yet
                const presentInRoomCount = 0; // TODO: Implement room visit checking
                
                // Students are "at home" if no location flags are set
                const atHomeCount = validStudents.filter(s => !s.in_house && !s.school_yard && !s.wc).length;

                setStats({
                    totalStudents: validStudents.length,
                    presentStudents: presentInRoomCount,
                    absentStudents: atHomeCount,
                    schoolyard: schoolyardCount,
                    bathroom: bathroomCount,
                    inHouse: inHouseCount
                });

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
                            
                            // Update presentStudents count based on actual room status
                            const inRoomCount = Object.values(response.data.student_room_status).filter(s => s.in_group_room).length;
                            setStats(prev => ({ ...prev, presentStudents: inRoomCount }));
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
        // Apply search filter
        if (searchTerm && !student.name?.toLowerCase().includes(searchTerm.toLowerCase()) &&
            !student.school_class?.toLowerCase().includes(searchTerm.toLowerCase())) {
            return false;
        }

        // Apply attendance filter
        if (attendanceFilter === "in_house" && !student.in_house) return false;
        if (attendanceFilter === "wc" && !student.wc) return false;
        if (attendanceFilter === "school_yard" && !student.school_yard) return false;
        if (attendanceFilter === "at_home" && (student.in_house || student.wc || student.school_yard)) return false;
        
        // Check room status for "Im Gruppenraum" filter
        if (attendanceFilter === "in_room") {
            const studentRoomStatus = roomStatus[student.id.toString()];
            if (!studentRoomStatus?.in_group_room) {
                return false;
            }
        }

        // Apply year filter
        if (selectedYear !== "all") {
            const yearMatch = /^(\d)/.exec(student.school_class ?? '');
            const studentYear = yearMatch ? yearMatch[1] : null;
            if (studentYear !== selectedYear) {
                return false;
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

    // Common class for all dropdowns to ensure consistent height
    const dropdownClass = "mt-1 block w-full rounded-lg border-0 px-4 py-3 h-12 shadow-sm ring-1 ring-gray-200 transition-all duration-200 hover:bg-gray-50/50 hover:ring-gray-300 focus:ring-2 focus:ring-teal-500 focus:outline-none appearance-none pr-8";

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
        <ResponsiveLayout userName={session?.user?.name ?? "Root"}>
            <div className="max-w-7xl mx-auto">
                            {/* OGS Group Header with Gradient */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-gradient-to-r from-blue-600 to-teal-500 shadow-lg">
                                <div className="px-8 py-6 text-white">
                                    <div className="flex items-center justify-between">
                                        <div>
                                            <h1 className="text-3xl font-bold">OGS-Gruppe: {ogsGroup?.name}</h1>
                                            <p className="mt-1 text-white/80">
                                                {ogsGroup?.room_name && `Raum: ${ogsGroup.room_name}`}
                                                {ogsGroup?.supervisor_name && ` • Betreuer: ${ogsGroup.supervisor_name}`}
                                            </p>
                                        </div>
                                        <div className="rounded-full bg-white/20 px-4 py-2 text-center backdrop-blur-sm">
                                            <span className="text-xl font-bold">{ogsGroup?.student_count ?? 0}</span>
                                            <p className="text-xs font-medium">Schüler</p>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Stats Overview Cards */}
                            <div className="mb-8 grid grid-cols-1 gap-4 sm:grid-cols-3 lg:grid-cols-5">
                                {/* Present Students Card */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-green-100 p-3">
                                            <svg className="h-6 w-6 text-green-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Im Gruppenraum</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.presentStudents}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Schoolyard Card - Updated with playground icon */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-blue-100 p-3">
                                            <svg className="h-6 w-6 text-blue-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 5l-4-4-4 4M8 9v10M16 19V9M12 1v18M3 19h18" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Schulhof</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.schoolyard}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Bathroom Card - Updated with toilet icon */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-yellow-100 p-3">
                                            <span className="flex h-6 w-6 items-center justify-center font-bold text-yellow-600">
                                                WC
                                            </span>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Toilette</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.bathroom}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Im Haus Card - Students who are checked in */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-purple-100 p-3">
                                            <svg className="h-6 w-6 text-purple-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Im Haus</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.inHouse}</p>
                                        </div>
                                    </div>
                                </div>

                                {/* Home Card */}
                                <div className="overflow-hidden rounded-lg bg-white p-4 shadow-sm transition-all duration-200 hover:shadow-md">
                                    <div className="flex items-center">
                                        <div className="mr-4 rounded-full bg-red-100 p-3">
                                            <svg className="h-6 w-6 text-red-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
                                            </svg>
                                        </div>
                                        <div>
                                            <p className="text-sm font-medium text-gray-600">Zuhause</p>
                                            <p className="text-2xl font-bold text-gray-900">{stats.absentStudents}</p>
                                        </div>
                                    </div>
                                </div>
                            </div>

                            {/* Search Panel */}
                            <div className="mb-8 overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                <h2 className="mb-4 text-xl font-bold text-gray-800">Suchkriterien</h2>

                                <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
                                    {/* Name Search */}
                                    <Input
                                        label="Name"
                                        name="searchTerm"
                                        placeholder="Vor- oder Nachname"
                                        value={searchTerm}
                                        onChange={(e) => setSearchTerm(e.target.value)}
                                        className="h-12" // Add fixed height to the Input component
                                    />

                                    {/* School Year Filter */}
                                    <div className="relative">
                                        <label className="block text-sm font-medium text-gray-700">
                                            Jahrgangsstufe
                                        </label>
                                        <select
                                            value={selectedYear}
                                            onChange={(e) => setSelectedYear(e.target.value)}
                                            className={dropdownClass}
                                        >
                                            <option value="all">Alle Jahrgänge</option>
                                            <option value="1">Jahrgang 1</option>
                                            <option value="2">Jahrgang 2</option>
                                            <option value="3">Jahrgang 3</option>
                                            <option value="4">Jahrgang 4</option>
                                        </select>
                                        <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                            <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                                <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                            </svg>
                                        </div>
                                    </div>

                                    {/* Attendance Status */}
                                    <div className="relative">
                                        <label className="block text-sm font-medium text-gray-700">
                                            Anwesenheitsstatus
                                        </label>
                                        <select
                                            value={attendanceFilter}
                                            onChange={(e) => setAttendanceFilter(e.target.value)}
                                            className={dropdownClass}
                                        >
                                            <option value="all">Alle</option>
                                            <option value="in_house">Im Haus</option>
                                            <option value="in_room">Im Gruppenraum</option>
                                            <option value="wc">Toilette</option>
                                            <option value="school_yard">Schulhof</option>
                                            <option value="at_home">Zuhause</option>
                                        </select>
                                        <div className="pointer-events-none absolute inset-y-0 right-0 mt-6 flex items-center pr-3">
                                            <svg className="h-5 w-5 text-gray-400" xmlns="http://www.w3.org/2000/svg" viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
                                                <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                            </svg>
                                        </div>
                                    </div>
                                </div>

                                {/* Search Actions */}
                                <div className="mt-6 flex flex-wrap justify-end gap-3">
                                    <button
                                        onClick={() => {
                                            setSearchTerm("");
                                            setSelectedYear("all");
                                            setAttendanceFilter("all");
                                        }}
                                        className="rounded-lg border border-gray-300 bg-white px-4 py-2 text-sm font-medium text-gray-700 shadow-sm transition-colors hover:bg-gray-50"
                                    >
                                        Zurücksetzen
                                    </button>
                                </div>
                            </div>

                            {/* Results Section */}
                            <div className="overflow-hidden rounded-xl bg-white p-6 shadow-md">
                                <div className="flex justify-between items-center mb-6">
                                    <h2 className="text-xl font-bold text-gray-800">Schüler in dieser Gruppe</h2>
                                    <div className="flex items-center space-x-6">
                                        {/* Year legend */}
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
                                </div>

                                {error && (
                                    <div className="mb-6">
                                        <Alert type="error" message={error} />
                                    </div>
                                )}

                                <div className="space-y-2">
                                    {filteredStudents.length > 0 ? (
                                        filteredStudents.map((student) => {
                                            const year = getSchoolYear(student.school_class ?? '');
                                            const yearColor = getYearColor(year);

                                            return (
                                                <div
                                                    key={student.id}
                                                    onClick={() => {/* Navigate to student detail */}}
                                                    className="group cursor-pointer rounded-lg border border-gray-100 bg-white p-4 shadow-sm transition-all duration-200 hover:translate-y-[-1px] hover:border-blue-200 hover:shadow-md"
                                                >
                                                    <div className="flex items-center justify-between">
                                                        <div className="flex items-center space-x-3">
                                                            <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gradient-to-r from-blue-400 to-indigo-500 font-medium text-white">
                                                                {student.first_name?.charAt(0).toUpperCase() ?? student.name?.charAt(0).toUpperCase() ?? "S"}
                                                            </div>

                                                            <div className="flex flex-col">
                                                                <div className="flex items-center">
                                                                    <span className="font-medium text-gray-900 transition-colors group-hover:text-blue-600">
                                                                      {student.name ?? `${student.first_name ?? ''} ${student.second_name ?? ''}`.trim()}
                                                                    </span>
                                                                    {/* Year indicator */}
                                                                    <span className={`ml-2 inline-block h-3 w-3 rounded-full ${yearColor}`} title={`Jahrgang ${year}`}></span>
                                                                </div>
                                                                <span className="text-sm text-gray-500">
                                                                    Klasse: {student.school_class}
                                                                </span>
                                                            </div>
                                                        </div>

                                                        <div className="flex items-center space-x-4">
                                                            {/* Status indicators - Shows current location */}
                                                            <div className="flex space-x-2">
                                                                {(() => {
                                                                    const studentRoomStatus = roomStatus[student.id.toString()];
                                                                    const isInGroupRoom = studentRoomStatus?.in_group_room;
                                                                    
                                                                    // Show "Im Gruppenraum" if student is in their group's room
                                                                    if (isInGroupRoom) {
                                                                        return (
                                                                            <div className="flex h-7 items-center rounded-full bg-green-100 px-2 text-xs font-medium text-green-600" title="Im Gruppenraum">
                                                                                <span className="mr-1 h-2 w-2 rounded-full bg-green-600"></span>
                                                                                <span>Im Gruppenraum</span>
                                                                            </div>
                                                                        );
                                                                    }
                                                                    
                                                                    // Otherwise show location based on flags
                                                                    return (
                                                                        <>
                                                                            {student.in_house && (
                                                                                <div className="flex h-7 items-center rounded-full bg-purple-100 px-2 text-xs font-medium text-purple-600" title="Im Haus">
                                                                                    <span className="mr-1 h-2 w-2 rounded-full bg-purple-600"></span>
                                                                                    <span>Im Haus</span>
                                                                                </div>
                                                                            )}
                                                                            {student.wc && (
                                                                                <div className="flex h-7 items-center rounded-full bg-yellow-100 px-2 text-xs font-medium text-yellow-600" title="Toilette">
                                                                                    <span className="mr-1 h-2 w-2 rounded-full bg-yellow-600"></span>
                                                                                    <span>WC</span>
                                                                                </div>
                                                                            )}
                                                                            {student.school_yard && (
                                                                                <div className="flex h-7 items-center rounded-full bg-blue-100 px-2 text-xs font-medium text-blue-600" title="Schulhof">
                                                                                    <span className="mr-1 h-2 w-2 rounded-full bg-blue-600"></span>
                                                                                    <span>Schulhof</span>
                                                                                </div>
                                                                            )}
                                                                            {!student.in_house && !student.wc && !student.school_yard && (
                                                                                <div className="flex h-7 items-center rounded-full bg-red-100 px-2 text-xs font-medium text-red-600" title="Zuhause">
                                                                                    <span className="mr-1 h-2 w-2 rounded-full bg-red-600"></span>
                                                                                    <span>Zuhause</span>
                                                                                </div>
                                                                            )}
                                                                        </>
                                                                    );
                                                                })()}
                                                                {/* Transportation info - shown separately */}
                                                                {student.bus && (
                                                                    <div className="flex h-7 items-center rounded-full bg-gray-100 px-2 text-xs font-medium text-gray-600" title="Fährt mit Bus">
                                                                        <svg className="mr-1 h-3 w-3 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7v8a2 2 0 002 2h6M8 7V5a2 2 0 012-2h4.586a1 1 0 01.707.293l4.414 4.414a1 1 0 01.293.707V15a2 2 0 01-2 2h-2M8 7H6a2 2 0 00-2 2v10a2 2 0 002 2h8a2 2 0 002-2v-2" />
                                                                        </svg>
                                                                        <span>Bus</span>
                                                                    </div>
                                                                )}
                                                            </div>

                                                            <svg
                                                                xmlns="http://www.w3.org/2000/svg"
                                                                className="h-5 w-5 text-gray-400 transition-all duration-200 group-hover:translate-x-1 group-hover:text-blue-500"
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
                                                        </div>
                                                    </div>
                                                </div>
                                            );
                                        })
                                    ) : (
                                        <div className="py-8 text-center">
                                            <p className="text-gray-500">
                                                {searchTerm || selectedYear !== "all" || attendanceFilter !== "all"
                                                    ? "Keine Ergebnisse gefunden. Bitte passen Sie Ihre Suchkriterien an."
                                                    : "Keine Schüler in dieser Gruppe gefunden."}
                                            </p>
                                        </div>
                                    )}
                                </div>
                            </div>
            </div>
        </ResponsiveLayout>
    );
}