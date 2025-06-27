"use client";

import { useState, useEffect, Suspense } from "react";
import { useSession } from "next-auth/react";
import { redirect, useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { userContextService } from "~/lib/usercontext-api";
import { activeService } from "~/lib/active-api";
import { fetchStudent } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";

// Extended student interface that includes visit information
interface StudentWithVisit extends Student {
    activeGroupId: string;
    checkInTime: Date;
}

// Define ActiveRoom type based on ActiveGroup with additional fields
interface ActiveRoom {
    id: string;
    name: string;
    room_name?: string;
    room_id?: string;
    student_count?: number;
    supervisor_name?: string;
    students?: StudentWithVisit[];
}

function MeinRaumPageContent() {
    const { data: session, status } = useSession({
        required: true,
        onUnauthenticated() {
            redirect("/");
        },
    });
    const router = useRouter();

    // Check if user has access to active rooms
    const [hasAccess, setHasAccess] = useState<boolean | null>(null);

    // State variables for multiple rooms
    const [allRooms, setAllRooms] = useState<ActiveRoom[]>([]);
    const [selectedRoomIndex, setSelectedRoomIndex] = useState(0);
    const [students, setStudents] = useState<StudentWithVisit[]>([]);
    const [searchTerm, setSearchTerm] = useState("");
    const [groupFilter, setGroupFilter] = useState("all");
    const [isLoading, setIsLoading] = useState(true);
    const [error, setError] = useState<string | null>(null);
    
    // Mobile-specific state
    const [isMobileFiltersOpen, setIsMobileFiltersOpen] = useState(false);
    
    // State for showing room selection (for 5+ rooms)
    const [showRoomSelection, setShowRoomSelection] = useState(true);
    
    // Get current selected room
    const currentRoom = allRooms[selectedRoomIndex] ?? null;

    // Check access and fetch active room data
    useEffect(() => {
        const checkAccessAndFetchData = async () => {
            try {
                setIsLoading(true);

                // First check if user has any active groups (active activities)
                const myActiveGroups = await userContextService.getMyActiveGroups();
                
                if (myActiveGroups.length === 0) {
                    // User has no active groups, redirect to dashboard
                    setHasAccess(false);
                    redirect("/dashboard");
                    return;
                }

                setHasAccess(true);

                // Fetch all active visits once for efficiency
                const prefetchedActiveVisits = await activeService.getVisits({ active: true });
                
                // Convert all active groups to ActiveRoom format and load student counts
                const activeRooms: ActiveRoom[] = await Promise.all(myActiveGroups.map(async (activeGroup) => {
                    // Get room information from the active group
                    let roomName = activeGroup.room?.name;
                    
                    // If room name is not provided, fetch it separately using the room_id
                    if (!roomName && activeGroup.room_id) {
                        try {
                            // Fetch room information from the rooms API
                            const roomResponse = await fetch(`/api/rooms/${activeGroup.room_id}`, {
                                headers: {
                                    Authorization: `Bearer ${session?.user?.token}`,
                                    "Content-Type": "application/json",
                                },
                            });
                            
                            if (roomResponse.ok) {
                                const roomData: { data?: { name?: string } } = await roomResponse.json() as { data?: { name?: string } };
                                roomName = roomData.data?.name;
                            }
                        } catch (error) {
                            console.error("Error fetching room name:", error);
                        }
                    }
                    
                    // Pre-load student count for this room using the pre-fetched visits
                    const activeVisits = prefetchedActiveVisits.filter(visit => visit.activeGroupId === activeGroup.id);
                    const currentlyCheckedIn = activeVisits.filter(visit => visit.isActive);
                    const studentCount = currentlyCheckedIn.length;
                    
                    return {
                        id: activeGroup.id,
                        name: activeGroup.name,
                        room_name: roomName,
                        room_id: activeGroup.room_id,
                        student_count: studentCount, // Pre-loaded actual student count
                        supervisor_name: undefined // Will be fetched separately if needed
                    };
                }));


                setAllRooms(activeRooms);

                // Use the first active room
                const firstRoom = activeRooms[0];
                
                if (!firstRoom) {
                    throw new Error('No active room found');
                }

                // Fetch current visits for this active group to get students actually checked in
                // Use getVisits with active filter instead of getActiveGroupVisits (which doesn't exist)
                const allActiveVisits = await activeService.getVisits({ active: true });
                
                // Filter visits for this specific active group
                const activeVisits = allActiveVisits.filter(visit => visit.activeGroupId === firstRoom.id);
                
                // Filter only active visits (students currently checked in)
                const currentlyCheckedIn = activeVisits.filter(visit => visit.isActive);
                
                // Fetch complete student data using student IDs from visits
                const studentPromises = currentlyCheckedIn.map(async (visit) => {
                    try {
                        // Fetch full student record using the student ID
                        const studentData = await fetchStudent(visit.studentId);
                        
                        // Add visit-specific information to the student data
                        // Keep the student's actual OGS group info (group_name, group_id)
                        return {
                            ...studentData,
                            activeGroupId: visit.activeGroupId,
                            checkInTime: visit.checkInTime
                            // Don't override group_name and group_id - keep student's OGS group
                        };
                    } catch (error) {
                        console.error(`Error fetching student ${visit.studentId}:`, error);
                        // Fallback to parsing student name if API call fails
                        const nameParts = visit.studentName?.split(' ') ?? ['', ''];
                        const firstName = nameParts[0] ?? '';
                        const lastName = nameParts.slice(1).join(' ') ?? '';
                        
                        return {
                            id: visit.studentId,
                            name: visit.studentName ?? '',
                            first_name: firstName,
                            second_name: lastName,
                            school_class: '',
                            current_location: 'Anwesend' as const,
                            in_house: true,
                            // No OGS group info available in fallback
                            activeGroupId: visit.activeGroupId,
                            checkInTime: visit.checkInTime
                        };
                    }
                });

                const studentsFromVisits = await Promise.all(studentPromises);
                
                // Debug: Log the first student to check group info
                if (studentsFromVisits.length > 0) {
                    const firstStudent = studentsFromVisits[0];
                    if (firstStudent) {
                        console.log('Student OGS group info:', {
                            name: firstStudent.name,
                            group_name: firstStudent.group_name,
                            group_id: firstStudent.group_id
                        });
                    }
                }
                
                setStudents(studentsFromVisits);

                // Update room with actual student count
                setAllRooms(prev => prev.map((room, idx) => 
                    idx === 0 ? { ...room, student_count: studentsFromVisits.length } : room
                ));

                setError(null);
            } catch (err) {
                if (err instanceof Error && err.message.includes("403")) {
                    setError("Sie haben keine Berechtigung für den Zugriff auf Aktivitätsdaten.");
                    setHasAccess(false);
                } else {
                    setError("Fehler beim Laden der Aktivitätsdaten.");
                    console.error("Error loading room data:", err);
                }
            } finally {
                setIsLoading(false);
            }
        };

        if (session?.user?.token) {
            void checkAccessAndFetchData();
        }
    }, [session?.user?.token]);

    // Function to switch between rooms
    const switchToRoom = async (roomIndex: number) => {
        if (roomIndex === selectedRoomIndex || !allRooms[roomIndex]) return;
        
        setIsLoading(true);
        setSelectedRoomIndex(roomIndex);
        setStudents([]); // Clear current students
        
        try {
            const selectedRoom = allRooms[roomIndex];
            
            if (!selectedRoom) {
                throw new Error('No active room found');
            }
            
            // Fetch current visits for the selected room
            const allActiveVisits = await activeService.getVisits({ active: true });
            
            // Filter visits for this specific active room
            const activeVisits = allActiveVisits.filter(visit => visit.activeGroupId === selectedRoom.id);
            
            // Filter only active visits (students currently checked in)
            const currentlyCheckedIn = activeVisits.filter(visit => visit.isActive);
            
            // Fetch complete student data using student IDs from visits
            const studentPromises = currentlyCheckedIn.map(async (visit) => {
                try {
                    // Fetch full student record using the student ID
                    const studentData = await fetchStudent(visit.studentId);
                    
                    // Add visit-specific information to the student data
                    return {
                        ...studentData,
                        activeGroupId: visit.activeGroupId,
                        checkInTime: visit.checkInTime
                    };
                } catch (error) {
                    console.error(`Error fetching student ${visit.studentId}:`, error);
                    // Fallback to parsing student name if API call fails
                    const nameParts = visit.studentName?.split(' ') ?? ['', ''];
                    const firstName = nameParts[0] ?? '';
                    const lastName = nameParts.slice(1).join(' ') ?? '';
                    
                    return {
                        id: visit.studentId,
                        name: visit.studentName ?? '',
                        first_name: firstName,
                        second_name: lastName,
                        school_class: '',
                        current_location: 'Anwesend' as const,
                        in_house: true,
                        activeGroupId: visit.activeGroupId,
                        checkInTime: visit.checkInTime
                    };
                }
            });

            const studentsFromVisits = await Promise.all(studentPromises);
            
            setStudents(studentsFromVisits);

            // Update room with actual student count
            setAllRooms(prev => prev.map((room, idx) => 
                idx === roomIndex ? { ...room, student_count: studentsFromVisits.length } : room
            ));

            setError(null);
        } catch (err) {
            setError("Fehler beim Laden der Raumdaten.");
            console.error("Error loading room data:", err);
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
                (student.second_name?.toLowerCase().includes(searchLower) ?? false);
            
            if (!matchesSearch) return false;
        }

        // Apply year filter (skip since we don't have school_class in visits)
        // Year filtering would require additional student data lookup

        // Apply group filter
        if (groupFilter !== "all") {
            const studentGroupName = student.group_name ?? "Unbekannt";
            
            if (studentGroupName !== groupFilter) {
                return false;
            }
        }

        return true;
    });

    // Get unique group names for filter dropdown
    const availableGroups = Array.from(new Set(
        students.map(student => student.group_name).filter(Boolean)
    )).sort();

    // Helper function to get group status with enhanced design
    const getGroupStatus = (student: StudentWithVisit) => {
        const groupName = student.group_name ?? "Unbekannt";
        
        // Single color for all groups - clean and consistent
        const groupColor = { 
            bg: "#5080D8", 
            shadow: "0 8px 25px rgba(80, 128, 216, 0.4)" 
        };
        
        return { 
            label: groupName, 
            badgeColor: "text-white backdrop-blur-sm",
            cardGradient: "from-blue-50/80 to-cyan-100/80",
            glowColor: "ring-blue-200/50 shadow-blue-100/50",
            customBgColor: groupColor.bg,
            customShadow: groupColor.shadow
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

    // Show room selection screen for 5+ rooms
    if (allRooms.length >= 5 && showRoomSelection) {
        return (
            <ResponsiveLayout>
                <div className="w-full max-w-6xl mx-auto px-4">
                    <div className="mb-8">
                        <h1 className="text-3xl md:text-4xl font-bold text-gray-900 mb-2">Wählen Sie Ihren Raum</h1>
                        <p className="text-lg text-gray-600">Sie haben {allRooms.length} aktive Räume</p>
                    </div>
                    
                    <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
                        {allRooms.map((room, index) => (
                            <button
                                key={room.id}
                                onClick={async () => {
                                    await switchToRoom(index);
                                    setShowRoomSelection(false);
                                }}
                                className="group bg-white rounded-2xl border-2 border-gray-200 p-6 
                                         hover:border-[#5080D8] hover:shadow-lg transition-all duration-200
                                         active:scale-95 text-left"
                            >
                                {/* Room Icon */}
                                <div className="w-16 h-16 bg-gradient-to-br from-[#5080D8] to-[#83CD2D] 
                                              rounded-xl mb-4 flex items-center justify-center
                                              group-hover:scale-110 transition-transform duration-200">
                                    <svg className="h-8 w-8 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                              d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4" />
                                    </svg>
                                </div>
                                
                                {/* Room Name */}
                                <h3 className="text-xl font-bold text-gray-900 mb-2 group-hover:text-[#5080D8]">
                                    {room.room_name ?? room.name}
                                </h3>
                                
                                {/* Activity Name */}
                                <div className="text-sm text-gray-600 mb-2">
                                    Aktivität: {room.name}
                                </div>
                                
                                {/* Student Count */}
                                <div className="flex items-center gap-2 text-gray-600">
                                    <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                              d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                    </svg>
                                    <span className="font-medium">{room.student_count ?? '...'} Schüler</span>
                                </div>
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
                {/* Header with Tab Navigation for Multiple Rooms */}
                <div className="mb-6 md:mb-8">
                    {/* Show tabs only if there are 2-4 rooms */}
                    {allRooms.length >= 2 && allRooms.length <= 4 ? (
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
                                    {allRooms.map((room, index) => (
                                        <button
                                            key={room.id}
                                            onClick={() => switchToRoom(index)}
                                            className={`
                                                flex items-center gap-3 rounded-2xl sm:rounded-2xl
                                                font-semibold transition-all duration-200 
                                                whitespace-nowrap flex-shrink-0 w-full sm:w-auto justify-between sm:justify-center
                                                ${index === selectedRoomIndex
                                                    ? 'px-4 sm:px-7 py-3 sm:py-4.5 text-lg sm:text-2xl bg-white text-gray-900 border-2 border-gray-300 shadow-sm sm:drop-shadow-sm sm:transform sm:scale-110 min-h-[52px] sm:min-h-[68px]'
                                                    : 'px-4 py-2.5 sm:py-2 text-base bg-gray-50 border sm:border-2 border-gray-200 text-gray-600 sm:text-gray-500 hover:bg-gray-100 hover:border-gray-300 hover:text-gray-700 min-h-[44px] sm:min-h-[40px]'
                                                }
                                            `}
                                        >
                                            <span>{room.room_name ?? room.name}</span>
                                            <div className={`
                                                flex items-center gap-1.5 sm:gap-2 rounded-full
                                                ${index === selectedRoomIndex
                                                    ? 'px-3 sm:px-3.5 py-1 sm:py-1.5 bg-gray-100'
                                                    : 'px-2.5 py-1 bg-gray-100 sm:bg-white'
                                                }
                                            `}>
                                                <svg className={`${index === selectedRoomIndex ? 'h-4 w-4 sm:h-5 sm:w-5' : 'h-3.5 w-3.5'} text-gray-600`} 
                                                     fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                                          d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                                </svg>
                                                <span className={`font-bold ${index === selectedRoomIndex ? 'text-sm sm:text-base' : 'text-xs'} text-gray-700`}>
                                                    {room.student_count ?? 0}
                                                </span>
                                            </div>
                                        </button>
                                    ))}
                                </nav>
                            </div>
                        </>
                    ) : (
                        /* Single room or 5+ rooms - show simple header */
                        <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
                            <h1 className="text-3xl md:text-4xl font-bold text-gray-900">
                                {currentRoom?.room_name ?? currentRoom?.name ?? "Mein Raum"}
                            </h1>
                            <div className="flex items-center gap-3">
                                {allRooms.length >= 5 && (
                                    <button
                                        onClick={() => setShowRoomSelection(true)}
                                        className="px-4 py-2 bg-white border border-gray-300 rounded-lg 
                                                 hover:bg-gray-50 hover:border-gray-400 transition-all duration-200
                                                 flex items-center gap-2 text-gray-700"
                                    >
                                        <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                                  d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                                        </svg>
                                        <span className="font-medium">Raum wechseln</span>
                                    </button>
                                )}
                                <div className="flex items-center gap-3 px-4 py-3 bg-gray-100 rounded-full">
                                    <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                    </svg>
                                    <span className="text-sm font-medium text-gray-700">{currentRoom?.student_count ?? 0}</span>
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
                            className="w-full pl-10 pr-4 py-3 bg-white border border-gray-200 rounded-lg text-gray-900 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors text-base"
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
                        {groupFilter !== "all" && (
                            <button
                                onClick={() => {
                                    setGroupFilter("all");
                                }}
                                className="text-sm text-gray-500"
                            >
                                Alle löschen
                            </button>
                        )}
                    </div>

                    {/* Collapsible Filter Dropdowns */}
                    {isMobileFiltersOpen && (
                        <div className="mb-3">
                            <div className="relative">
                                <select
                                    value={groupFilter}
                                    onChange={(e) => setGroupFilter(e.target.value)}
                                    className="w-full pl-3 pr-8 py-2.5 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-base appearance-none"
                                >
                                    <option value="all">Alle Gruppen</option>
                                    {availableGroups.map(groupName => (
                                        <option key={groupName} value={groupName}>{groupName}</option>
                                    ))}
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
                    {(searchTerm || groupFilter !== "all") && (
                        <div className="flex items-center justify-between text-sm text-gray-600">
                            <span>
                                {(() => {
                                    const activeFilters = [];
                                    if (searchTerm) activeFilters.push(`Suche: "${searchTerm}"`);
                                    if (groupFilter !== "all") activeFilters.push(`Gruppe: ${groupFilter}`);
                                    
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
                                    setGroupFilter("all");
                                }}
                                className="text-blue-600 hover:text-blue-700 font-medium"
                            >
                                Zurücksetzen
                            </button>
                        </div>
                    )}
                </div>

                {/* Desktop Search & Filter - Minimalistic */}
                <div className="hidden md:block mb-4">
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
                                    className="w-full pl-10 pr-4 py-2 bg-white border border-gray-200 rounded-lg text-gray-900 placeholder-gray-500 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-colors"
                                />
                            </div>
                        </div>

                        {/* Filter Dropdown */}
                        <div className="relative">
                            <select
                                value={groupFilter}
                                onChange={(e) => setGroupFilter(e.target.value)}
                                className="pl-3 pr-8 py-2 bg-white border border-gray-200 rounded-lg text-gray-900 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 text-sm min-w-[140px] appearance-none"
                            >
                                <option value="all">Alle Gruppen</option>
                                {availableGroups.map(groupName => (
                                    <option key={groupName} value={groupName}>{groupName}</option>
                                ))}
                            </select>
                            <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-2">
                                <svg className="h-4 w-4 text-gray-400" fill="currentColor" viewBox="0 0 20 20">
                                    <path fillRule="evenodd" d="M5.293 7.293a1 1 0 011.414 0L10 10.586l3.293-3.293a1 1 0 111.414 1.414l-4 4a1 1 0 01-1.414 0l-4-4a1 1 0 010-1.414z" clipRule="evenodd" />
                                </svg>
                            </div>
                        </div>
                    </div>

                    {/* Active Filters Bar */}
                    {(searchTerm || groupFilter !== "all") && (
                        <div className="mt-3 flex items-center justify-between text-sm text-gray-600">
                            <div className="flex items-center gap-2">
                                <span>
                                    {(() => {
                                        const activeFilters = [];
                                        if (searchTerm) activeFilters.push(`Suche: "${searchTerm}"`);
                                            if (groupFilter !== "all") activeFilters.push(`Gruppe: ${groupFilter}`);
                                        
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
                                    setGroupFilter("all");
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
                {students.length === 0 ? (
                    <div className="py-12 text-center">
                        <div className="flex flex-col items-center gap-4">
                            <svg className="h-12 w-12 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                            </svg>
                            <div>
                                <h3 className="text-lg font-medium text-gray-900">Keine Schüler in diesem Raum</h3>
                                <p className="text-gray-600">
                                    Es wurden noch keine Schüler zu dieser Aktivität eingecheckt.
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
                        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3 gap-6">
                        {filteredStudents.map((student, index) => {
                            const groupStatus = getGroupStatus(student);

                            return (
                                <div
                                    key={student.id}
                                    onClick={() => router.push(`/students/${student.id}?from=/myroom`)}
                                    className={`group cursor-pointer relative overflow-hidden rounded-2xl bg-white/90 backdrop-blur-md border border-gray-100/50 shadow-[0_8px_30px_rgb(0,0,0,0.12)] transition-all duration-500 md:hover:scale-[1.03] md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)] md:hover:bg-white md:hover:-translate-y-3 active:scale-[0.97] md:hover:border-blue-200/50`}
                                    style={{
                                        transform: `rotate(${(index % 3 - 1) * 0.8}deg)`,
                                        animation: `float 8s ease-in-out infinite ${index * 0.7}s`
                                    }}
                                >
                                    {/* Modern gradient overlay */}
                                    <div className={`absolute inset-0 bg-gradient-to-br ${groupStatus.cardGradient} opacity-[0.03] rounded-2xl`}></div>
                                    {/* Subtle inner glow */}
                                    <div className="absolute inset-px rounded-2xl bg-gradient-to-br from-white/80 to-white/20"></div>
                                    {/* Modern border highlight */}
                                    <div className="absolute inset-0 rounded-2xl ring-1 ring-white/20 md:group-hover:ring-blue-200/60 transition-all duration-300"></div>
                                    

                                    <div className="relative p-6">
                                        {/* Header with student name */}
                                        <div className="flex items-center justify-between mb-3">
                                            {/* Student Name */}
                                            <div className="flex-1 min-w-0">
                                                <div className="flex items-center gap-2">
                                                    <h3 className="text-lg font-bold text-gray-800 md:group-hover:text-blue-600 transition-colors duration-300 whitespace-nowrap overflow-hidden text-ellipsis">
                                                        {student.first_name}
                                                    </h3>
                                                    {/* Subtle integrated arrow */}
                                                    <svg className="w-4 h-4 text-gray-300 md:group-hover:text-blue-500 md:group-hover:translate-x-1 transition-all duration-300 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
                                                    </svg>
                                                </div>
                                                <p className="text-base font-semibold text-gray-700 md:group-hover:text-blue-500 transition-colors duration-300 whitespace-nowrap overflow-hidden text-ellipsis">
                                                    {student.second_name}
                                                </p>
                                            </div>
                                            
                                            {/* Group Badge */}
                                            <span 
                                                className={`inline-flex items-center px-3 py-1.5 rounded-full text-xs font-bold ${groupStatus.badgeColor} ml-3`}
                                                style={{ 
                                                    backgroundColor: groupStatus.customBgColor,
                                                    boxShadow: groupStatus.customShadow
                                                }}
                                            >
                                                <span className="w-1.5 h-1.5 bg-white/80 rounded-full mr-2 animate-pulse"></span>
                                                {groupStatus.label}
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
                                    <div className="absolute inset-0 rounded-2xl opacity-0 md:group-hover:opacity-100 transition-opacity duration-300 bg-gradient-to-r from-transparent via-blue-100/30 to-transparent"></div>
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
export default function MeinRaumPage() {
    return (
        <Suspense fallback={
            <div className="flex min-h-screen items-center justify-center">
                <div className="h-12 w-12 animate-spin rounded-full border-b-2 border-t-2 border-blue-500"></div>
            </div>
        }>
            <MeinRaumPageContent />
        </Suspense>
    );
}