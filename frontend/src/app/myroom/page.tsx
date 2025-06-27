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

                // Convert all active groups to ActiveRoom format
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
                    
                    return {
                        id: activeGroup.id,
                        name: activeGroup.name,
                        room_name: roomName,
                        room_id: activeGroup.room_id,
                        student_count: 0, // Will be calculated from actual visits
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
        students.map(student => student.group_name).filter((name): name is string => Boolean(name))
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
                {/* Modern Header with Clean Navigation */}
                <div className="mb-6">
                    {/* Title Section */}
                    <div className="mb-4">
                        <div className="flex items-center justify-between gap-4">
                            <h1 className="text-[1.625rem] md:text-3xl font-bold text-gray-900">
                                {currentRoom?.room_name ?? currentRoom?.name ?? "Mein Raum"}
                            </h1>
                            {/* Student Count Badge */}
                            <div className="flex items-center gap-2 px-3 py-1.5 bg-gray-100 rounded-full">
                                <svg className="h-5 w-5 text-gray-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                          d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z" />
                                </svg>
                                <span className="text-sm font-medium text-gray-700">
                                    {currentRoom?.student_count ?? 0}
                                </span>
                            </div>
                        </div>
                    </div>

                    {/* Room Navigation */}
                    {allRooms.length > 1 && (
                        <div className="mb-4">
                            <nav className="flex gap-2 overflow-x-auto pb-2 scrollbar-hide">
                                {/* Show room buttons for 2-4 rooms */}
                                {allRooms.length <= 4 && allRooms.map((room, index) => (
                                    <button
                                        key={room.id}
                                        onClick={() => switchToRoom(index)}
                                        className={`
                                            px-4 py-2.5 rounded-xl font-medium text-sm transition-all duration-200
                                            whitespace-nowrap flex-shrink-0
                                            ${index === selectedRoomIndex 
                                                ? 'bg-gray-900 text-white shadow-sm' 
                                                : 'bg-gray-100 text-gray-600 hover:bg-gray-200 hover:text-gray-900'
                                            }
                                        `}
                                    >
                                        {room.room_name ?? room.name}
                                    </button>
                                ))}
                                {/* Show More Button for 5+ Rooms */}
                                {allRooms.length >= 5 && (
                                    <button
                                        onClick={() => setShowRoomSelection(true)}
                                        className="flex items-center gap-2 px-4 py-2.5 rounded-xl
                                                 bg-gray-100 text-gray-600 hover:bg-gray-200 hover:text-gray-900
                                                 text-sm font-medium transition-all duration-200
                                                 whitespace-nowrap flex-shrink-0"
                                    >
                                        <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} 
                                                  d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
                                        </svg>
                                        <span>Raum wechseln</span>
                                    </button>
                                )}
                            </nav>
                        </div>
                    )}
                </div>

                {/* Mobile Search & Filters - Modern Minimal Design */}
                <div className="mb-6 md:hidden">
                    {/* Search Input with Integrated Filter Button */}
                    <div className="flex gap-2 mb-3">
                        <div className="relative flex-1">
                            <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                            </svg>
                            <input
                                type="text"
                                placeholder="Name suchen..."
                                value={searchTerm}
                                onChange={(e) => setSearchTerm(e.target.value)}
                                className="w-full pl-9 pr-3 py-2.5 bg-white border border-gray-200 rounded-2xl text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-all duration-200 text-sm"
                            />
                            {searchTerm && (
                                <button
                                    onClick={() => setSearchTerm("")}
                                    className="absolute right-2 top-1/2 transform -translate-y-1/2 p-1 hover:bg-gray-100 rounded-full transition-colors"
                                >
                                    <svg className="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                    </svg>
                                </button>
                            )}
                        </div>
                        <button
                            onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
                            className={`
                                p-2.5 rounded-2xl transition-all duration-200
                                ${isMobileFiltersOpen 
                                    ? 'bg-blue-500 text-white' 
                                    : 'bg-white border border-gray-200 text-gray-600 hover:bg-gray-50'
                                }
                                ${groupFilter !== "all" ? 'ring-2 ring-blue-500 ring-offset-1' : ''}
                            `}
                        >
                            <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
                            </svg>
                        </button>
                    </div>

                    {/* Active Filter Chips */}
                    {groupFilter !== "all" && (
                        <div className="flex gap-2 mb-3 flex-wrap">
                            <button
                                onClick={() => setGroupFilter("all")}
                                className="inline-flex items-center gap-1 px-2.5 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium"
                            >
                                Gruppe: {groupFilter}
                                <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                </svg>
                            </button>
                        </div>
                    )}

                    {/* Expandable Filter Panel */}
                    {isMobileFiltersOpen && (
                        <div className="bg-white rounded-2xl border border-gray-200 p-4 mb-3 shadow-sm">
                            <div className="space-y-3">
                                {/* Group Filter */}
                                <div>
                                    <label className="text-xs font-medium text-gray-600 mb-1.5 block">Gruppe filtern</label>
                                    <div className="grid grid-cols-2 gap-2">
                                        <button
                                            onClick={() => setGroupFilter("all")}
                                            className={`
                                                py-2 px-3 rounded-lg text-sm font-medium transition-all
                                                ${groupFilter === "all" 
                                                    ? 'bg-gray-900 text-white' 
                                                    : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                                                }
                                            `}
                                        >
                                            Alle Gruppen
                                        </button>
                                        {availableGroups.map((groupName) => (
                                            <button
                                                key={groupName}
                                                onClick={() => setGroupFilter(groupName)}
                                                className={`
                                                    py-2 px-3 rounded-lg text-sm font-medium transition-all
                                                    ${groupFilter === groupName 
                                                        ? 'bg-gray-900 text-white' 
                                                        : 'bg-gray-50 text-gray-600 hover:bg-gray-100'
                                                    }
                                                `}
                                            >
                                                {groupName}
                                            </button>
                                        ))}
                                    </div>
                                </div>
                            </div>

                            {/* Filter Actions */}
                            <div className="flex gap-2 mt-4 pt-3 border-t border-gray-100">
                                <button
                                    onClick={() => setGroupFilter("all")}
                                    className="flex-1 py-2 text-sm font-medium text-gray-600 hover:text-gray-900 transition-colors"
                                >
                                    Zurücksetzen
                                </button>
                                <button
                                    onClick={() => setIsMobileFiltersOpen(false)}
                                    className="flex-1 py-2 bg-gray-900 text-white rounded-lg text-sm font-medium hover:bg-gray-800 transition-colors"
                                >
                                    Fertig
                                </button>
                            </div>
                        </div>
                    )}
                </div>

                {/* Desktop Search & Filter - Modern Minimal Design */}
                <div className="hidden md:block mb-6">
                    <div className="flex items-center gap-3 mb-3">
                        {/* Search Input */}
                        <div className="flex-1 relative">
                            <svg className="absolute left-3 top-1/2 transform -translate-y-1/2 h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
                            </svg>
                            <input
                                type="text"
                                placeholder="Name suchen..."
                                value={searchTerm}
                                onChange={(e) => setSearchTerm(e.target.value)}
                                className="w-full pl-10 pr-10 py-2.5 bg-white border border-gray-200 rounded-2xl text-gray-900 placeholder-gray-400 focus:outline-none focus:ring-1 focus:ring-blue-500 focus:border-blue-500 transition-all duration-200"
                            />
                            {searchTerm && (
                                <button
                                    onClick={() => setSearchTerm("")}
                                    className="absolute right-3 top-1/2 transform -translate-y-1/2 p-1 hover:bg-gray-100 rounded-full transition-colors"
                                >
                                    <svg className="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                    </svg>
                                </button>
                            )}
                        </div>

                        {/* Group Filter Dropdown */}
                        <div className="relative">
                            <button
                                onClick={() => setIsMobileFiltersOpen(!isMobileFiltersOpen)}
                                className={`
                                    flex items-center gap-2 px-4 h-10 rounded-xl transition-all shadow-sm
                                    ${groupFilter !== "all" 
                                        ? 'bg-gray-900 text-white' 
                                        : 'bg-white text-gray-700 hover:bg-gray-50'
                                    }
                                `}
                            >
                                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z" />
                                </svg>
                                <span className="text-sm font-medium">
                                    {groupFilter === "all" ? "Alle Gruppen" : groupFilter}
                                </span>
                                <svg className="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
                                </svg>
                            </button>
                            
                            {/* Dropdown Menu */}
                            {isMobileFiltersOpen && (
                                <div className="absolute right-0 mt-2 w-48 bg-white rounded-xl shadow-lg border border-gray-200 py-1 z-10">
                                    <button
                                        onClick={() => {
                                            setGroupFilter("all");
                                            setIsMobileFiltersOpen(false);
                                        }}
                                        className={`
                                            w-full text-left px-4 py-2 text-sm transition-colors
                                            ${groupFilter === "all" 
                                                ? 'bg-gray-100 text-gray-900 font-medium' 
                                                : 'text-gray-700 hover:bg-gray-50'
                                            }
                                        `}
                                    >
                                        Alle Gruppen
                                    </button>
                                    {availableGroups.map(groupName => (
                                        <button
                                            key={groupName}
                                            onClick={() => {
                                                setGroupFilter(groupName);
                                                setIsMobileFiltersOpen(false);
                                            }}
                                            className={`
                                                w-full text-left px-4 py-2 text-sm transition-colors
                                                ${groupFilter === groupName 
                                                    ? 'bg-gray-100 text-gray-900 font-medium' 
                                                    : 'text-gray-700 hover:bg-gray-50'
                                                }
                                            `}
                                        >
                                            {groupName}
                                        </button>
                                    ))}
                                </div>
                            )}
                        </div>
                    </div>

                    {/* Active Filter Chips */}
                    {(searchTerm || groupFilter !== "all") && (
                        <div className="flex items-center justify-between">
                            <div className="flex gap-2 flex-wrap">
                                {searchTerm && (
                                    <span className="inline-flex items-center gap-1 px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium">
                                        &quot;{searchTerm}&quot;
                                        <button onClick={() => setSearchTerm("")} className="hover:text-blue-900">
                                            <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                            </svg>
                                        </button>
                                    </span>
                                )}
                                {groupFilter !== "all" && (
                                    <span className="inline-flex items-center gap-1 px-3 py-1 bg-blue-100 text-blue-700 rounded-full text-xs font-medium">
                                        Gruppe: {groupFilter}
                                        <button onClick={() => setGroupFilter("all")} className="hover:text-blue-900">
                                            <svg className="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                                                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                                            </svg>
                                        </button>
                                    </span>
                                )}
                            </div>
                            <button
                                onClick={() => {
                                    setSearchTerm("");
                                    setGroupFilter("all");
                                }}
                                className="text-sm text-gray-500 hover:text-gray-700 font-medium transition-colors"
                            >
                                Alle zurücksetzen
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