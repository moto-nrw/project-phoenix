"use client";

import { useState, useEffect, Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
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
    
    // Mobile-specific state
    const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);
    
    // State for showing group selection (for 5+ groups)
    const [showGroupSelection, setShowGroupSelection] = useState(true);
    
    // Get current selected group
    const currentGroup = allGroups[selectedGroupIndex] ?? null;

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
            return { 
                label: "Gruppenraum", 
                badgeColor: "text-white backdrop-blur-sm",
                cardGradient: "from-emerald-50/80 to-green-100/80",
                glowColor: "ring-emerald-200/50 shadow-emerald-100/50",
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
        if (student.in_house === true || student.current_location === LOCATIONS.BUS) {
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
            <div className="w-full">
                {/* Header with Tab Navigation for Multiple Groups */}
                <div className="mb-6 md:mb-8">
                    {/* Show tabs only if there are 2-4 groups */}
                    {allGroups.length >= 2 && allGroups.length <= 4 ? (
                        <>
                            {/* Tab Navigation - Responsive Design */}
                            <div className="mb-6">
                                <style jsx>{`
                                    .scrollbar-hide {
                                        -ms-overflow-style: none;
                                        scrollbar-width: none;
                                    }
                                    .scrollbar-hide::-webkit-scrollbar {
                                        display: none;
                                    }
                                `}</style>
                                <nav className="flex flex-col sm:flex-row gap-2 sm:gap-7 overflow-x-auto scrollbar-hide sm:px-4 sm:py-1">
                                    {allGroups.map((group, index) => (
                                        <button
                                            key={group.id}
                                            onClick={() => switchToGroup(index)}
                                            className={`
                                                flex items-center gap-3 rounded-2xl sm:rounded-2xl
                                                font-semibold transition-all duration-200 
                                                whitespace-nowrap flex-shrink-0 w-full sm:w-auto justify-between sm:justify-center
                                                ${index === selectedGroupIndex
                                                    ? 'px-4 sm:px-7 py-3 sm:py-4.5 text-lg sm:text-2xl bg-white text-gray-900 border-2 border-gray-300 shadow-sm sm:drop-shadow-sm sm:transform sm:scale-110 min-h-[52px] sm:min-h-[68px]'
                                                    : 'px-4 py-2.5 sm:py-2 text-base bg-gray-50 border sm:border-2 border-gray-200 text-gray-600 sm:text-gray-500 hover:bg-gray-100 hover:border-gray-300 hover:text-gray-700 min-h-[44px] sm:min-h-[40px]'
                                                }
                                            `}
                                        >
                                            <span>{group.name}</span>
                                            <div className={`
                                                flex items-center gap-1.5 sm:gap-2 rounded-full
                                                ${index === selectedGroupIndex
                                                    ? 'px-3 sm:px-3.5 py-1 sm:py-1.5 bg-gray-100'
                                                    : 'px-2.5 py-1 bg-gray-100 sm:bg-white'
                                                }
                                            `}>
                                                <svg className={`${index === selectedGroupIndex ? 'h-4 w-4 sm:h-5 sm:w-5' : 'h-3.5 w-3.5'} text-gray-600`} 
                                                     fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                                          d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                                </svg>
                                                <span className={`font-bold ${index === selectedGroupIndex ? 'text-sm sm:text-base' : 'text-xs'} text-gray-700`}>
                                                    {group.student_count ?? 0}
                                                </span>
                                            </div>
                                        </button>
                                    ))}
                                </nav>
                            </div>
                        </>
                    ) : (
                        /* Single group or 5+ groups - show simple header */
                        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                            <h1 className="text-3xl md:text-4xl font-bold text-gray-900">{currentGroup?.name}</h1>
                            <div className="flex items-center gap-3">
                                {allGroups.length >= 5 && (
                                    <button
                                        onClick={() => setShowGroupSelection(true)}
                                        className="px-4 py-2 bg-white border border-gray-300 rounded-lg 
                                                 hover:bg-gray-50 hover:border-gray-400 transition-all duration-200
                                                 flex items-center gap-2 text-gray-700"
                                    >
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                                  d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                                        </svg>
                                        <span className="font-medium">Gruppe wechseln</span>
                                    </button>
                                )}
                                <div className="flex items-center gap-3 px-4 py-3 bg-gray-100 rounded-full">
                                    <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                    </svg>
                                    <span className="text-sm font-medium text-gray-700">{currentGroup?.student_count ?? 0}</span>
                                </div>
                            </div>
                        </div>
                    )}
                </div>

                {/* Mobile Search - Minimalistic Design */}
                <div className="mb-4 md:hidden">
                    {/* Search Input */}
                    <div className="relative mb-3">
                        <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                        </svg>
                        <input
                            type="text"
                            placeholder="Schüler suchen..."
                            value={searchTerm}
                            onChange={(e) => setSearchTerm(e.target.value)}
                            className="w-full pl-10 pr-4 py-3 bg-white border border-gray-200 rounded-2xl text-gray-900 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors text-base"
                        />
                    </div>

                    {/* Filter Toggle Button */}
                    <div className="flex justify-between items-center mb-3">
                        <button
                            onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
                            className="flex items-center gap-2 text-base text-blue-600 font-medium py-2"
                        >
                            <svg className={`h-5 w-5 transition-transform duration-200 ${isMobileFiltersOpen ? 'rotate-180' : ''}`} fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
                            </svg>
                            {isMobileFiltersOpen ? 'Filter ausblenden' : 'Filter anzeigen'}
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

                    {/* Collapsible Filter Dropdowns */}
                    {isMobileFiltersOpen && (
                        <div className="grid grid-cols-2 gap-3 mb-3">
                            <div className="relative">
                                <select
                                    value={selectedYear}
                                    onChange={(e) => setSelectedYear(e.target.value)}
                                    className="w-full pl-3 pr-8 py-2.5 bg-white border border-gray-200 rounded-2xl text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-base appearance-none"
                                >
                                    <option value="all">Alle Jahre</option>
                                    <option value="1">Jahr 1</option>
                                    <option value="2">Jahr 2</option>
                                    <option value="3">Jahr 3</option>
                                    <option value="4">Jahr 4</option>
                                </select>
                                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                                    <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                        <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                    </svg>
                                </div>
                            </div>

                            <div className="relative">
                                <select
                                    value={attendanceFilter}
                                    onChange={(e) => setAttendanceFilter(e.target.value)}
                                    className="w-full pl-3 pr-8 py-2.5 bg-white border border-gray-200 rounded-2xl text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-base appearance-none"
                                >
                                    <option value="all">Alle Orte</option>
                                    <option value="in_room">Gruppenraum</option>
                                    <option value="in_house">Unterwegs</option>
                                    <option value="school_yard">Schulhof</option>
                                    <option value="at_home">Zuhause</option>
                                </select>
                                <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                                    <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                        <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                    </svg>
                                </div>
                            </div>
                        </div>
                    )}

                    {/* Active Filters Bar */}
                    {(searchTerm || selectedYear !== "all" || attendanceFilter !== "all") && (
                        <div className="flex items-center justify-between text-sm text-gray-600">
                            <span>
                                {(() => {
                                    const activeFilters = [];
                                    if (searchTerm) activeFilters.push(`Suche: "${searchTerm}"`);
                                    if (selectedYear !== "all") activeFilters.push(`Jahr ${selectedYear}`);
                                    if (attendanceFilter !== "all") {
                                        const statusLabels: Record<string, string> = {
                                            "in_room": "Gruppenraum",
                                            "in_house": "Unterwegs", 
                                            "school_yard": "Schulhof",
                                            "at_home": "Zuhause"
                                        };
                                        activeFilters.push(statusLabels[attendanceFilter] ?? attendanceFilter);
                                    }
                                    
                                    if (activeFilters.length === 1) {
                                        return `1 Filter aktiv: ${activeFilters[0]}`;
                                    } else {
                                        return `${activeFilters.length} Filter aktiv: ${activeFilters.join(", ")}`;
                                    }
                                })()}
                            </span>
                            <button
                                onClick={() => {
                                    setSearchTerm("");
                                    setSelectedYear("all");
                                    setAttendanceFilter("all");
                                }}
                                className="text-blue-600 hover:text-blue-700 font-medium"
                            >
                                Zurücksetzen
                            </button>
                        </div>
                    )}
                </div>

                {/* Desktop Search & Filter - Minimalistic */}
                <div className="hidden md:block mb-6">
                    <div className="flex items-center gap-3">
                        {/* Search Input */}
                        <div className="flex-1">
                            <div className="relative">
                                <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                                </svg>
                                <input
                                    type="text"
                                    placeholder="Schüler suchen..."
                                    value={searchTerm}
                                    onChange={(e) => setSearchTerm(e.target.value)}
                                    className="w-full pl-10 pr-4 py-2 bg-white border border-gray-200 rounded-2xl text-gray-900 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                                />
                            </div>
                        </div>

                        {/* Filter Dropdowns */}
                        <div className="relative">
                            <select
                                value={selectedYear}
                                onChange={(e) => setSelectedYear(e.target.value)}
                                className="pl-3 pr-8 py-2 bg-white border border-gray-200 rounded-2xl text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-sm min-w-[120px] appearance-none"
                            >
                                <option value="all">Alle Jahre</option>
                                <option value="1">Jahr 1</option>
                                <option value="2">Jahr 2</option>
                                <option value="3">Jahr 3</option>
                                <option value="4">Jahr 4</option>
                            </select>
                            <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                                <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                </svg>
                            </div>
                        </div>

                        <div className="relative">
                            <select
                                value={attendanceFilter}
                                onChange={(e) => setAttendanceFilter(e.target.value)}
                                className="pl-3 pr-8 py-2 bg-white border border-gray-200 rounded-2xl text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-sm min-w-[140px] appearance-none"
                            >
                                <option value="all">Alle Orte</option>
                                <option value="in_room">Gruppenraum</option>
                                <option value="in_house">Unterwegs</option>
                                <option value="school_yard">Schulhof</option>
                                <option value="at_home">Zuhause</option>
                            </select>
                            <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                                <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                </svg>
                            </div>
                        </div>
                    </div>

                    {/* Active Filters Bar */}
                    {(searchTerm || selectedYear !== "all" || attendanceFilter !== "all") && (
                        <div className="mt-3 flex items-center justify-between text-sm text-gray-600">
                            <div className="flex items-center gap-2">
                                <span>
                                    {(() => {
                                        const activeFilters = [];
                                        if (searchTerm) activeFilters.push(`Suche: "${searchTerm}"`);
                                        if (selectedYear !== "all") activeFilters.push(`Jahr ${selectedYear}`);
                                        if (attendanceFilter !== "all") {
                                            const statusLabels: Record<string, string> = {
                                                "in_room": "Gruppenraum",
                                                "in_house": "Unterwegs", 
                                                "school_yard": "Schulhof",
                                                "at_home": "Zuhause"
                                            };
                                            activeFilters.push(statusLabels[attendanceFilter] ?? attendanceFilter);
                                        }
                                        
                                        if (activeFilters.length === 1) {
                                            return `1 Filter aktiv: ${activeFilters[0]}`;
                                        } else {
                                            return `${activeFilters.length} Filter aktiv: ${activeFilters.join(", ")}`;
                                        }
                                    })()}
                                </span>
                            </div>
                            <button
                                onClick={() => {
                                    setSearchTerm("");
                                    setSelectedYear("all");
                                    setAttendanceFilter("all");
                                }}
                                className="text-blue-600 hover:text-blue-700 font-medium"
                            >
                                Filter zurücksetzen
                            </button>
                        </div>
                    )}
                </div>


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
                        {/* Add floating animation keyframes */}
                        <style jsx>{`
                            @keyframes float {
                                0%, 100% { transform: translateY(0px) rotate(var(--rotation)); }
                                50% { transform: translateY(-4px) rotate(var(--rotation)); }
                            }
                        `}</style>
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3 gap-6">
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