"use client";

import { useState, useEffect, Suspense, useMemo } from "react";
import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { userContextService } from "~/lib/usercontext-api";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import type { StudentLocation } from "~/lib/student-helpers";
import { fetchRooms } from "~/lib/rooms-api";
import { createRoomIdToNameMap } from "~/lib/rooms-helpers";

// Location constants to ensure type safety
const LOCATIONS = {
    HOME: "Home" as StudentLocation,
    IN_HOUSE: "In House" as StudentLocation,
    WC: "WC" as StudentLocation,
    SCHOOL_YARD: "School Yard" as StudentLocation,
    BUS: "Bus" as StudentLocation,
    UNKNOWN: "Unknown" as StudentLocation,
} as const;

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

    // State variables for multiple groups
    const [allGroups, setAllGroups] = useState<OGSGroup[]>([]);
    const [selectedGroupIndex, setSelectedGroupIndex] = useState(0);
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
    const [roomIdToNameMap, setRoomIdToNameMap] = useState<Record<string, string>>({});
    
    // State for showing group selection (for 5+ groups)
    const [showGroupSelection, setShowGroupSelection] = useState(true);

    // State for mobile detection
    const [isMobile, setIsMobile] = useState(false);

    // Get current selected group
    const currentGroup = allGroups[selectedGroupIndex] ?? null;

    // Handle mobile detection
    useEffect(() => {
        const checkMobile = () => {
            setIsMobile(window.innerWidth < 768);
        };
        checkMobile();
        window.addEventListener('resize', checkMobile);
        return () => window.removeEventListener('resize', checkMobile);
    }, []);

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

                // Convert all groups to OGSGroup format and pre-load student counts
                const ogsGroups: OGSGroup[] = await Promise.all(myGroups.map(async (group) => {
                    // Pre-load student count for this group
                    let studentCount = 0;
                    try {
                        const studentsResponse = await studentService.getStudents({
                            groupId: group.id
                        });
                        studentCount = studentsResponse.students?.length || 0;
                    } catch (error) {
                        console.error("Error fetching student count for group:", error);
                    }
                    
                    return {
                        id: group.id,
                        name: group.name,
                        room_name: group.room?.name,
                        room_id: group.room_id,
                        student_count: studentCount, // Pre-loaded actual student count
                        supervisor_name: undefined // Will be fetched separately if needed
                    };
                }));


                setAllGroups(ogsGroups);

                // Use the first group by default
                const firstGroup = ogsGroups[0];
                
                if (!firstGroup) {
                    throw new Error('No educational group found');
                }

                // Fetch students for the first group
                const studentsResponse = await studentService.getStudents({
                    groupId: firstGroup.id
                });
                const studentsData = studentsResponse.students || [];
                
                setStudents(studentsData);

                // Calculate statistics from real data (only if we have valid array data)
                const validStudents = Array.isArray(studentsData) ? studentsData : [];
                setStudents(validStudents);

                // Update group with actual student count
                setAllGroups(prev => prev.map((group, idx) => 
                    idx === 0 ? { ...group, student_count: validStudents.length } : group
                ));

                // Fetch room status for all students in the group
                try {
                    const roomStatusResponse = await fetch(`/api/groups/${firstGroup.id}/students/room-status`, {
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

                // Fetch all rooms to get their names
                try {
                    const rooms = await fetchRooms(session?.user?.token);
                    const roomMap = createRoomIdToNameMap(rooms);
                    setRoomIdToNameMap(roomMap);
                } catch (roomErr) {
                    console.error("Failed to fetch rooms:", roomErr);
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

    // Function to switch between groups
    const switchToGroup = async (groupIndex: number) => {
        if (groupIndex === selectedGroupIndex || !allGroups[groupIndex]) return;
        
        setIsLoading(true);
        setSelectedGroupIndex(groupIndex);
        setStudents([]); // Clear current students
        setRoomStatus({}); // Clear room status
        
        try {
            const selectedGroup = allGroups[groupIndex];
            
            // Fetch students for the selected group
            const studentsResponse = await studentService.getStudents({
                groupId: selectedGroup.id
            });
            const studentsData = studentsResponse.students || [];
            
            setStudents(studentsData);
            
            // Update group with actual student count
            setAllGroups(prev => prev.map((group, idx) => 
                idx === groupIndex ? { ...group, student_count: studentsData.length } : group
            ));
            
            // Fetch room status for the selected group
            try {
                const roomStatusResponse = await fetch(`/api/groups/${selectedGroup.id}/students/room-status`, {
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
        } catch {
            setError("Fehler beim Laden der Gruppendaten.");
        } finally {
            setIsLoading(false);
        }
    };

    // Apply filters to students (ensure students is an array)
    const filteredStudents = (Array.isArray(students) ? students : []).filter((student) => {
        // Apply search filter - search in multiple fields
        if (searchTerm) {
            const searchLower = searchTerm.toLowerCase();
            const matchesSearch = 
                (student.name?.toLowerCase().includes(searchLower) ?? false) ||
                (student.first_name?.toLowerCase().includes(searchLower) ?? false) ||
                (student.second_name?.toLowerCase().includes(searchLower) ?? false) ||
                (student.school_class?.toLowerCase().includes(searchLower) ?? false);
            
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
                case "foreign_room":
                    // Student is in a room but NOT their group room
                    // They have a current_room_id but in_group_room is false
                    if (!studentRoomStatus?.current_room_id || studentRoomStatus?.in_group_room !== false) return false;
                    break;
                case "in_house":
                    // Check both the in_house flag and current_location
                    if (!student.in_house && student.current_location !== LOCATIONS.IN_HOUSE) return false;
                    break;
                case "school_yard":
                    if (!student.school_yard && student.current_location !== LOCATIONS.SCHOOL_YARD) return false;
                    break;
                case "at_home":
                    // Student is at home if no location flags are set OR current_location is "Home"
                    const isAtHome = (!student.in_house && !student.wc && !student.school_yard && !studentRoomStatus?.in_group_room) ||
                                    student.current_location === LOCATIONS.HOME;
                    if (!isAtHome) return false;
                    break;
            }
        }

        return true;
    });



    // Helper function to get location status with enhanced design
    const getLocationStatus = (student: Student) => {
        const studentRoomStatus = roomStatus[student.id.toString()];
        
        // Check if student is in group room
        if (studentRoomStatus?.in_group_room) {
            // Use the actual room name instead of generic "Gruppenraum"
            const roomLabel = currentGroup?.room_name ?? "Gruppenraum";
            return { 
                label: roomLabel, 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-emerald-50/80 to-green-100/80",
                customBgColor: "#83CD2D",
                customShadow: "0 8px 25px rgba(131, 205, 45, 0.4)"
            };
        }
        
        // Check if student is in a specific room (not group room)
        if (studentRoomStatus?.current_room_id && !studentRoomStatus.in_group_room) {
            const roomName = roomIdToNameMap[studentRoomStatus.current_room_id.toString()] ?? `Raum ${studentRoomStatus.current_room_id}`;
            return { 
                label: roomName, 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-blue-50/80 to-cyan-100/80",
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
                customBgColor: "#F78C10",
                customShadow: "0 8px 25px rgba(247, 140, 16, 0.4)"
            };
        }
        
        // Check for in transit/movement
        if (student.in_house === true || student.current_location === LOCATIONS.BUS) {
            return { 
                label: "Unterwegs", 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-fuchsia-50/80 to-pink-100/80",
                customBgColor: "#D946EF",
                customShadow: "0 8px 25px rgba(217, 70, 239, 0.4)"
            };
        }
        
        // Default to at home
        return { 
            label: "Zuhause", 
            badgeColor: "text-white backdrop-blur-sm",
            cardGradient: "from-red-50/80 to-rose-100/80",
            customBgColor: "#FF3130",
            customShadow: "0 8px 25px rgba(255, 49, 48, 0.4)"
        };
    };

    // Prepare filter configurations for PageHeaderWithSearch
    const filterConfigs: FilterConfig[] = useMemo(() => [
        {
            id: 'year',
            label: 'Klassenstufe',
            type: 'buttons',
            value: selectedYear,
            onChange: (value) => setSelectedYear(value as string),
            options: [
                { value: 'all', label: 'Alle' },
                { value: '1', label: '1' },
                { value: '2', label: '2' },
                { value: '3', label: '3' },
                { value: '4', label: '4' }
            ]
        },
        {
            id: 'location',
            label: 'Aufenthaltsort',
            type: 'grid',
            value: attendanceFilter,
            onChange: (value) => setAttendanceFilter(value as string),
            options: [
                { value: "all", label: "Alle Orte", icon: "M4 6h16M4 12h16M4 18h16" },
                { value: "in_room", label: "Gruppenraum", icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" },
                { value: "foreign_room", label: "Fremder Raum", icon: "M8 14v3m4-3v3m4-3v3M3 21h18M3 10h18M3 7l9-4 9 4M4 10h16v11H4V10z" },
                { value: "in_house", label: "Unterwegs", icon: "M13 10V3L4 14h7v7l9-11h-7z" },
                { value: "school_yard", label: "Schulhof", icon: "M21 12a9 9 0 11-18 0 9 9 0 0118 0zM12 12a8 8 0 008 4M7.5 13.5a12 12 0 008.5 6.5M12 12a8 8 0 00-7.464 4.928M12.951 7.353a12 12 0 00-9.88 4.111M12 12a8 8 0 00-.536-8.928M15.549 15.147a12 12 0 001.38-10.611" },
                { value: "at_home", label: "Zuhause", icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" }
            ]
        }
    ], [selectedYear, attendanceFilter]);

    // Prepare active filters for display
    const activeFilters: ActiveFilter[] = useMemo(() => {
        const filters: ActiveFilter[] = [];
        
        if (searchTerm) {
            filters.push({
                id: 'search',
                label: `"${searchTerm}"`,
                onRemove: () => setSearchTerm("")
            });
        }
        
        if (selectedYear !== "all") {
            filters.push({
                id: 'year',
                label: `Jahr ${selectedYear}`,
                onRemove: () => setSelectedYear("all")
            });
        }
        
        if (attendanceFilter !== "all") {
            const locationLabels: Record<string, string> = {
                "in_room": "Gruppenraum",
                "in_house": "Unterwegs", 
                "school_yard": "Schulhof",
                "at_home": "Zuhause"
            };
            filters.push({
                id: 'location',
                label: locationLabels[attendanceFilter] ?? attendanceFilter,
                onRemove: () => setAttendanceFilter("all")
            });
        }
        
        return filters;
    }, [searchTerm, selectedYear, attendanceFilter]);

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

    // Show group selection screen for 5+ groups
    if (allGroups.length >= 5 && showGroupSelection) {
        return (
            <ResponsiveLayout>
                <div className="w-full max-w-6xl mx-auto px-4">
                    <div className="mb-8">
                        <h1 className="text-3xl md:text-4xl font-bold text-gray-900 mb-2">Wählen Sie Ihre Gruppe</h1>
                        <p className="text-lg text-gray-600">Sie haben Zugriff auf {allGroups.length} Gruppen</p>
                    </div>
                    
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {allGroups.map((group, index) => (
                            <button
                                key={group.id}
                                onClick={async () => {
                                    await switchToGroup(index);
                                    setShowGroupSelection(false);
                                }}
                                className="group bg-white rounded-2xl border-2 border-gray-200 p-6 
                                         hover:border-[#5080D8] hover:shadow-lg transition-all duration-200
                                         active:scale-95 text-left"
                            >
                                {/* Group Icon */}
                                <div className="w-16 h-16 bg-gradient-to-br from-[#5080D8] to-[#83CD2D] 
                                              rounded-xl mb-4 flex items-center justify-center
                                              group-hover:scale-110 transition-transform duration-200">
                                    <span className="text-2xl font-bold text-white">
                                        {group.name.charAt(0)}
                                    </span>
                                </div>
                                
                                {/* Group Name */}
                                <h3 className="text-xl font-bold text-gray-900 mb-2 group-hover:text-[#5080D8]">
                                    {group.name}
                                </h3>
                                
                                {/* Student Count */}
                                <div className="flex items-center gap-2 text-gray-600">
                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                              d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                    </svg>
                                    <span className="font-medium">{group.student_count ?? '...'} Schüler</span>
                                </div>
                                
                                {/* Room Info if available */}
                                {group.room_name && (
                                    <div className="mt-2 text-sm text-gray-500">
                                        Raum: {group.room_name}
                                    </div>
                                )}
                            </button>
                        ))}
                    </div>
                </div>
            </ResponsiveLayout>
        );
    }

    return (
        <ResponsiveLayout>
            <div className="w-full -mt-1.5">
                {/* PageHeaderWithSearch - Title only on mobile */}
                <PageHeaderWithSearch
                    title={isMobile ? (allGroups.length === 1 ? currentGroup?.name ?? "Meine Gruppe" : "Meine Gruppen") : ""}
                    badge={{
                        icon: (
                            <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2}
                                      d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                            </svg>
                        ),
                        count: allGroups.length === 1
                            ? currentGroup?.student_count ?? 0
                            : allGroups.reduce((sum, group) => sum + (group.student_count ?? 0), 0)
                    }}
                    tabs={allGroups.length > 1 ? {
                        items: allGroups.map((group) => ({
                            id: group.id,
                            label: group.name,
                            count: group.student_count
                        })),
                        activeTab: currentGroup?.id ?? '',
                        onTabChange: (tabId) => {
                            const index = allGroups.findIndex(g => g.id === tabId);
                            if (index !== -1) void switchToGroup(index);
                        }
                    } : undefined}
                    search={{
                        value: searchTerm,
                        onChange: setSearchTerm,
                        placeholder: "Name suchen..."
                    }}
                    filters={filterConfigs}
                    activeFilters={activeFilters}
                    onClearAllFilters={() => {
                        setSearchTerm("");
                        setSelectedYear("all");
                        setAttendanceFilter("all");
                    }}
                />


                {/* Mobile Error Display */}
                {error && (
                    <div className="mb-4 md:hidden">
                        <Alert type="error" message={error} />
                    </div>
                )}

                {/* Student Grid - Mobile Optimized */}
                {isLoading && selectedGroupIndex > 0 ? (
                    <div className="py-12 text-center">
                        <div className="flex flex-col items-center gap-4">
                            <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-[#5080D8]"></div>
                            <p className="text-gray-600">Gruppe wird geladen...</p>
                        </div>
                    </div>
                ) : students.length === 0 ? (
                    <div className="py-12 text-center">
                        <div className="flex flex-col items-center gap-4">
                            <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                            </svg>
                            <div>
                                <h3 className="text-lg font-medium text-gray-900">Keine Schüler in {currentGroup?.name ?? 'dieser Gruppe'}</h3>
                                <p className="text-gray-600">
                                    Es wurden noch keine Schüler zu dieser OGS-Gruppe hinzugefügt.
                                </p>
                                {allGroups.length > 1 && (
                                    <p className="text-sm text-gray-500 mt-2">
                                        Versuchen Sie eine andere Gruppe auszuwählen.
                                    </p>
                                )}
                            </div>
                        </div>
                    </div>
                ) : filteredStudents.length > 0 ? (
                    <div>
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3 gap-6">
                        {filteredStudents.map((student) => {
                            const locationStatus = getLocationStatus(student);

                            return (
                                <div
                                    key={student.id}
                                    onClick={() => router.push(`/students/${student.id}?from=/ogs_groups`)}
                                    className={`group cursor-pointer relative overflow-hidden rounded-3xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.03] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-3 active:scale-[0.97] md:hover:border-blue-200/50`}
                                >
                                    {/* Modern gradient overlay */}
                                    <div className={`absolute inset-0 bg-gradient-to-br ${locationStatus.cardGradient} opacity-[0.03] rounded-3xl`}></div>
                                    {/* Subtle inner glow */}
                                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                                    {/* Modern border highlight */}
                                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
                                    

                                    <div className="relative p-6">
                                        {/* Header with student name */}
                                        <div className="flex items-center justify-between mb-2">
                                            {/* Student Name */}
                                            <div className="flex-1 min-w-0">
                                                <div className="flex items-center gap-2">
                                                    <h3 className="text-lg font-bold text-gray-800 whitespace-nowrap overflow-hidden text-ellipsis md:group-hover:text-blue-600 transition-colors duration-300">
                                                        {student.first_name}
                                                    </h3>
                                                    {/* Subtle integrated arrow */}
                                                    <svg className="w-4 h-4 text-gray-300 md:group-hover:text-blue-500 md:group-hover:translate-x-1 transition-all duration-300 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                    </svg>
                                                </div>
                                                <p className="text-base font-semibold text-gray-700 whitespace-nowrap overflow-hidden text-ellipsis md:group-hover:text-blue-500 transition-colors duration-300">
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