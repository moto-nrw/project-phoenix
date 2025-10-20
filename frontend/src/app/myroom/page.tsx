"use client";

import { useState, useEffect, Suspense, useMemo, useCallback, useRef } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { userContextService } from "~/lib/usercontext-api";
import { activeService } from "~/lib/active-api";
import { fetchStudent } from "~/lib/student-api";
import type { Student } from "~/lib/student-helpers";
import { UnclaimedRooms } from "~/components/active";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent } from "~/lib/sse-types";

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
  const router = useRouter();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

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
  const [refreshKey, setRefreshKey] = useState(0);
  const [sseNonce, setSseNonce] = useState(() => Date.now());

  // State for showing room selection (for 5+ rooms)
  const [showRoomSelection, setShowRoomSelection] = useState(true);

  // State for mobile detection
  const [isMobile, setIsMobile] = useState(false);

  // Get current selected room
  const currentRoom = allRooms[selectedRoomIndex] ?? null;

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
    };
    checkMobile();
    window.addEventListener('resize', checkMobile);
    return () => window.removeEventListener('resize', checkMobile);
  }, []);

  // Helper function to load visits for a specific room
  const loadRoomVisits = useCallback(
    async (roomId: string): Promise<StudentWithVisit[]> => {
      // Use bulk endpoint to fetch visits with display data for specific room
      const visits =
        await activeService.getActiveGroupVisitsWithDisplay(roomId);

      // Filter only active visits (students currently checked in)
      const currentlyCheckedIn = visits.filter((visit) => visit.isActive);

      // Fetch complete student data using student IDs from visits
      const studentPromises = currentlyCheckedIn.map(async (visit) => {
        try {
          // Fetch full student record using the student ID
          const studentData = await fetchStudent(visit.studentId);

          // Add visit-specific information to the student data
          return {
            ...studentData,
            activeGroupId: visit.activeGroupId,
            checkInTime: visit.checkInTime,
          };
        } catch (error) {
          console.error(`Error fetching student ${visit.studentId}:`, error);
          // Fallback to parsing student name if API call fails
          const nameParts = visit.studentName?.split(" ") ?? ["", ""];
          const firstName = nameParts[0] ?? "";
          const lastName = nameParts.slice(1).join(" ") ?? "";

          return {
            id: visit.studentId,
            name: visit.studentName ?? "",
            first_name: firstName,
            second_name: lastName,
            school_class: "",
            current_location: "Anwesend" as const,
            in_house: true,
            activeGroupId: visit.activeGroupId,
            checkInTime: visit.checkInTime,
          };
        }
      });

      return await Promise.all(studentPromises);
    },
    [],
  );

  const currentRoomRef = useRef<ActiveRoom | null>(null);
  const hasSupervisionRef = useRef(false);
  useEffect(() => {
    currentRoomRef.current = currentRoom;
  }, [currentRoom]);

  // SSE event handler - direct refetch for affected room only
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      console.log("SSE event received:", event.type, event.active_group_id);
      const activeRoom = currentRoomRef.current;
      if (activeRoom && event.active_group_id === activeRoom.id) {
        const targetRoomId = activeRoom.id;
        console.log("Event for current room - fetching updated data");
        void loadRoomVisits(targetRoomId)
          .then((studentsFromVisits) => {
            setStudents([...studentsFromVisits]);

            // Update room student count
            setAllRooms((prev) =>
              prev.map((existingRoom) =>
                existingRoom.id === targetRoomId
                  ? { ...existingRoom, student_count: studentsFromVisits.length }
                  : existingRoom,
              ),
            );
          })
          .catch((error) => {
            console.error("Error refetching room visits:", error);
          });
      }
    },
    [loadRoomVisits],
  );

  const sseEndpoint = useMemo(
    () => `/api/sse/events?nonce=${sseNonce}`,
    [sseNonce],
  );

  // Connect to SSE for real-time updates
  const { status: sseStatus, reconnectAttempts } = useSSE(sseEndpoint, {
    onMessage: handleSSEEvent,
  });

  // Check access and fetch active room data
  useEffect(() => {
    const checkAccessAndFetchData = async () => {
      try {
        setIsLoading(true);

        // Check if user has any active groups OR unclaimed groups available
        const [myActiveGroups, unclaimedGroups] = await Promise.all([
          userContextService.getMyActiveGroups(),
          activeService.getUnclaimedGroups(),
        ]);

        if (myActiveGroups.length === 0 && unclaimedGroups.length === 0) {
          // User has no active groups AND no unclaimed rooms to claim
          hasSupervisionRef.current = false;
          setHasAccess(false);
          router.push("/dashboard");
          return;
        }

        setHasAccess(true);

        const gainedSupervisions =
          !hasSupervisionRef.current && myActiveGroups.length > 0;
        if (gainedSupervisions) {
          setSseNonce((prev) => prev + 1);
        }
        hasSupervisionRef.current = myActiveGroups.length > 0;

        // Convert all active groups to ActiveRoom format
        const activeRooms: ActiveRoom[] = await Promise.all(
          myActiveGroups.map(async (activeGroup) => {
            // Get room information from the active group
            let roomName = activeGroup.room?.name;

            // If room name is not provided, fetch it separately using the room_id
            if (!roomName && activeGroup.room_id) {
              try {
                // Fetch room information from the rooms API
                const roomResponse = await fetch(
                  `/api/rooms/${activeGroup.room_id}`,
                  {
                    headers: {
                      Authorization: `Bearer ${session?.user?.token}`,
                      "Content-Type": "application/json",
                    },
                  },
                );

                if (roomResponse.ok) {
                  const roomData: { data?: { name?: string } } =
                    (await roomResponse.json()) as { data?: { name?: string } };
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
              student_count: undefined, // Will be loaded when room is viewed
              supervisor_name: undefined,
            };
          }),
        );

        setAllRooms(activeRooms);

        // Use the first active room
        const firstRoom = activeRooms[0];

        if (!firstRoom) {
          throw new Error("No active room found");
        }

        // Use bulk endpoint to fetch visits for this specific room
        const studentsFromVisits = await loadRoomVisits(firstRoom.id);

        // Set students state
        setStudents([...studentsFromVisits]);

        // Update room with actual student count
        setAllRooms((prev) =>
          prev.map((room, idx) =>
            idx === 0
              ? { ...room, student_count: studentsFromVisits.length }
              : room,
          ),
        );

        setError(null);
      } catch (err) {
        if (err instanceof Error && err.message.includes("403")) {
          setError(
            "Sie haben keine Berechtigung für den Zugriff auf Aktivitätsdaten.",
          );
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
  }, [session?.user?.token, refreshKey, loadRoomVisits, router]);

  // Callback when a room is claimed - triggers refresh
  const handleRoomClaimed = useCallback(() => {
    setSseNonce((prev) => prev + 1);
    setRefreshKey((prev) => prev + 1);
  }, []);

  // Function to switch between rooms
  const switchToRoom = async (roomIndex: number) => {
    if (roomIndex === selectedRoomIndex || !allRooms[roomIndex]) return;

    setIsLoading(true);
    setSelectedRoomIndex(roomIndex);
    setStudents([]); // Clear current students

    try {
      const selectedRoom = allRooms[roomIndex];

      if (!selectedRoom) {
        throw new Error("No active room found");
      }

      // Use bulk endpoint to fetch visits for selected room
      const studentsFromVisits = await loadRoomVisits(selectedRoom.id);

      // Set students state
      setStudents([...studentsFromVisits]);

      // Update room with actual student count
      setAllRooms((prev) =>
        prev.map((room, idx) =>
          idx === roomIndex
            ? { ...room, student_count: studentsFromVisits.length }
            : room,
        ),
      );

      setError(null);
    } catch (err) {
      setError("Fehler beim Laden der Raumdaten.");
      console.error("Error loading room data:", err);
    } finally {
      setIsLoading(false);
    }
  };

  // Apply filters to students (ensure students is an array)
  const filteredStudents = (Array.isArray(students) ? students : []).filter(
    (student) => {
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
    },
  );

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(() => {
    // Compute available groups inside useMemo to ensure proper updates
    const groups = Array.from(
      new Set(
        students
          .map((student) => student.group_name)
          .filter((name): name is string => Boolean(name)),
      ),
    ).sort();

    return [
      {
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        value: groupFilter,
        onChange: (value) => setGroupFilter(value as string),
        options: [
          { value: "all", label: "Alle Gruppen" },
          ...groups.map((groupName) => ({
            value: groupName,
            label: groupName,
          })),
        ],
      },
    ];
  }, [groupFilter, students]);

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (groupFilter !== "all") {
      filters.push({
        id: "group",
        label: `Gruppe: ${groupFilter}`,
        onRemove: () => setGroupFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, groupFilter]);

  // Helper function to get group status with enhanced design
  const getGroupStatus = (student: StudentWithVisit) => {
    const groupName = student.group_name ?? "Unbekannt";

    // Single color for all groups - clean and consistent
    const groupColor = {
      bg: "#5080D8",
      shadow: "0 8px 25px rgba(80, 128, 216, 0.4)",
    };

    return {
      label: groupName,
      badgeColor: "text-white backdrop-blur-sm",
      cardGradient: "from-blue-50/80 to-cyan-100/80",
      customBgColor: groupColor.bg,
      customShadow: groupColor.shadow,
    };
  };

  if (status === "loading" || isLoading || hasAccess === null) {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <div className="h-12 w-12 animate-spin rounded-full border-t-2 border-b-2 border-[#5080D8]"></div>
          <p className="text-gray-600">Daten werden geladen...</p>
        </div>
      </div>
    );
  }

  // If user doesn't have access, redirect to dashboard
  if (hasAccess === false) {
    router.push("/dashboard");
    return null;
  }

  // Show room selection screen for 5+ rooms
  if (allRooms.length >= 5 && showRoomSelection) {
    return (
      <ResponsiveLayout>
        <div className="mx-auto w-full max-w-6xl px-4">
          {/* Unclaimed Rooms Section - Also show in room selection view */}
          <UnclaimedRooms onClaimed={handleRoomClaimed} />

          <div className="mb-8">
            <h1 className="mb-2 text-3xl font-bold text-gray-900 md:text-4xl">
              Wählen Sie Ihren Raum
            </h1>
            <p className="text-lg text-gray-600">
              Sie haben {allRooms.length} aktive Räume
            </p>
          </div>

          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {allRooms.map((room, index) => (
              <button
                key={room.id}
                onClick={async () => {
                  await switchToRoom(index);
                  setShowRoomSelection(false);
                }}
                className="group rounded-2xl border-2 border-gray-200 bg-white p-6 text-left transition-all duration-200 hover:border-[#5080D8] hover:shadow-lg active:scale-95"
              >
                {/* Room Icon */}
                <div className="mb-4 flex h-16 w-16 items-center justify-center rounded-xl bg-gradient-to-br from-[#5080D8] to-[#83CD2D] transition-transform duration-200 group-hover:scale-110">
                  <svg
                    className="h-8 w-8 text-white"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
                    />
                  </svg>
                </div>

                {/* Room Name */}
                <h3 className="mb-2 text-xl font-bold text-gray-900 group-hover:text-[#5080D8]">
                  {room.room_name ?? room.name}
                </h3>

                {/* Activity Name */}
                <div className="mb-2 text-sm text-gray-600">
                  Aktivität: {room.name}
                </div>

                {/* Student Count */}
                <div className="flex items-center gap-2 text-gray-600">
                  <svg
                    className="h-5 w-5"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                    />
                  </svg>
                  <span className="font-medium">
                    {room.student_count ?? "..."} Schüler
                  </span>
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
        {/* Unclaimed Rooms Section - Shows rooms available for claiming */}
        <UnclaimedRooms onClaimed={handleRoomClaimed} />

        {/* SSE Connection Status Indicator */}
        <div className="mb-2 flex items-center gap-2 text-sm">
          <div
            className={`h-2 w-2 rounded-full ${
              sseStatus === "connected"
                ? "bg-green-500"
                : sseStatus === "reconnecting"
                  ? "bg-yellow-500"
                  : sseStatus === "failed"
                    ? "bg-red-500"
                    : "bg-gray-400"
            }`}
          />
          <span className="hidden md:inline text-gray-600">
            {sseStatus === "connected"
              ? "Live-Updates aktiv"
              : sseStatus === "reconnecting"
                ? `Verbindung wird wiederhergestellt... (Versuch ${reconnectAttempts}/5)`
                : sseStatus === "failed"
                  ? "Verbindung fehlgeschlagen"
                  : "Verbindung wird hergestellt..."}
          </span>
        </div>

        {/* Modern Header with PageHeaderWithSearch component */}
        <PageHeaderWithSearch
          title={isMobile ? (allRooms.length === 1 ? currentRoom?.name ?? "Mein Raum" : "Meine Räume") : currentRoom?.room_name ?? currentRoom?.name ?? "Mein Raum"}
          badge={{
            icon: (
              <svg
                className="h-5 w-5 text-gray-600"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
            ),
            count: currentRoom?.student_count ?? 0,
            label: "Schüler",
          }}
          tabs={
            allRooms.length > 1 && allRooms.length <= 4
              ? {
                  items: allRooms.map((room) => ({
                    id: room.id,
                    label: room.room_name ?? room.name,
                  })),
                  activeTab: currentRoom?.id ?? "",
                  onTabChange: (tabId) => {
                    const index = allRooms.findIndex((r) => r.id === tabId);
                    if (index !== -1) void switchToRoom(index);
                  },
                }
              : undefined
          }
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Name suchen...",
          }}
          filters={filterConfigs}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setGroupFilter("all");
          }}
        />

        {/* Room Change Button for 5+ Rooms */}
        {allRooms.length >= 5 && (
          <div className="mb-4">
            <button
              onClick={() => setShowRoomSelection(true)}
              className="flex items-center gap-2 rounded-xl bg-gray-100 px-4 py-2.5 text-sm font-medium text-gray-600 transition-all duration-200 hover:bg-gray-200 hover:text-gray-900"
            >
              <svg
                className="h-4 w-4"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
                />
              </svg>
              <span>Raum wechseln</span>
            </button>
          </div>
        )}

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
              <svg
                className="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
              <div>
                <h3 className="text-lg font-medium text-gray-900">
                  Keine Schüler in diesem Raum
                </h3>
                <p className="text-gray-600">
                  Es wurden noch keine Schüler zu dieser Aktivität eingecheckt.
                </p>
                <p className="mt-2 text-sm text-gray-500">
                  Gesamtzahl gefundener Schüler: {students.length}
                </p>
              </div>
            </div>
          </div>
        ) : filteredStudents.length > 0 ? (
          <div>
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
              {filteredStudents.map((student) => {
                const groupStatus = getGroupStatus(student);

                return (
                  <div
                    key={student.id}
                    onClick={() =>
                      router.push(`/students/${student.id}?from=/myroom`)
                    }
                    className={`group relative cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.97] md:hover:-translate-y-3 md:hover:scale-[1.03] md:hover:border-[#5080D8]/30 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]`}
                  >
                    {/* Modern gradient overlay */}
                    <div
                      className={`absolute inset-0 bg-gradient-to-br ${groupStatus.cardGradient} rounded-3xl opacity-[0.03]`}
                    ></div>
                    {/* Subtle inner glow */}
                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                    {/* Modern border highlight */}
                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60"></div>

                    <div className="relative p-6">
                      {/* Header with student name */}
                      <div className="mb-3 flex items-center justify-between">
                        {/* Student Name */}
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <h3 className="overflow-hidden text-lg font-bold text-ellipsis whitespace-nowrap text-gray-800 transition-colors duration-300 md:group-hover:text-blue-600">
                              {student.first_name}
                            </h3>
                            {/* Subtle integrated arrow */}
                            <svg
                              className="h-4 w-4 flex-shrink-0 text-gray-300 transition-all duration-300 md:group-hover:translate-x-1 md:group-hover:text-blue-500"
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
                          <p className="overflow-hidden text-base font-semibold text-ellipsis whitespace-nowrap text-gray-700 transition-colors duration-300 md:group-hover:text-blue-500">
                            {student.second_name}
                          </p>
                        </div>

                        {/* Group Badge */}
                        <span
                          className={`inline-flex items-center rounded-full px-3 py-1.5 text-xs font-bold ${groupStatus.badgeColor} ml-3`}
                          style={{
                            backgroundColor: groupStatus.customBgColor,
                            boxShadow: groupStatus.customShadow,
                          }}
                        >
                          <span className="mr-2 h-1.5 w-1.5 animate-pulse rounded-full bg-white/80"></span>
                          {groupStatus.label}
                        </span>
                      </div>

                      {/* Bottom row with click hint */}
                      <div className="flex justify-start">
                        <p className="text-xs text-gray-400 transition-colors duration-300 md:group-hover:text-blue-400">
                          Tippen für mehr Infos
                        </p>
                      </div>

                      {/* Decorative elements */}
                      <div className="absolute top-3 left-3 h-5 w-5 animate-ping rounded-full bg-white/20"></div>
                      <div className="absolute right-3 bottom-3 h-3 w-3 rounded-full bg-white/30"></div>
                    </div>

                    {/* Glowing border effect */}
                    <div className="absolute inset-0 rounded-3xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                  </div>
                );
              })}
            </div>
          </div>
        ) : (
          <div className="py-12 text-center">
            <div className="flex flex-col items-center gap-4">
              <svg
                className="h-12 w-12 text-gray-400"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
                />
              </svg>
              <div>
                <h3 className="text-lg font-medium text-gray-900">
                  Keine Schüler gefunden
                </h3>
                <p className="text-gray-600">
                  Versuche deine Suchkriterien anzupassen.
                </p>
                <p className="mt-2 text-sm text-gray-500">
                  {students.length} Schüler insgesamt, {filteredStudents.length}{" "}
                  nach Filtern
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
    <Suspense
      fallback={
        <div className="flex min-h-screen items-center justify-center">
          <div className="h-12 w-12 animate-spin rounded-full border-t-2 border-b-2 border-[#5080D8]"></div>
        </div>
      }
    >
      <MeinRaumPageContent />
    </Suspense>
  );
}
