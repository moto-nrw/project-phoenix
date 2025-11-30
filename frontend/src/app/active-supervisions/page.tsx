"use client";

import {
  useState,
  useEffect,
  Suspense,
  useMemo,
  useCallback,
  useRef,
} from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { Loading } from "~/components/ui/loading";
import { LocationBadge } from "@/components/ui/location-badge";
import { Modal } from "~/components/ui/modal";
import { userContextService } from "~/lib/usercontext-api";
import { activeService } from "~/lib/active-api";
import type { Student } from "~/lib/student-helpers";
import { UnclaimedRooms } from "~/components/active";
import { useSSE } from "~/lib/hooks/use-sse";
import { SSEErrorBoundary } from "~/components/sse/SSEErrorBoundary";
import type { SSEEvent } from "~/lib/sse-types";

/** Minimal active group interface - compatible with both helper types */
interface MinimalActiveGroup {
  id: string;
  room?: { name?: string };
}

// Schulhof room name - used for special release supervision feature
const SCHULHOF_ROOM_NAME = "Schulhof";

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

const GROUP_CARD_GRADIENT = "from-blue-50/80 to-cyan-100/80";

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

  // OGS group rooms for color detection
  const [myGroupRooms, setMyGroupRooms] = useState<string[]>([]);

  // OGS group IDs for permission checking
  const [myGroupIds, setMyGroupIds] = useState<string[]>([]);

  // Map from group name to group ID for enriching visit data
  const [groupNameToIdMap, setGroupNameToIdMap] = useState<Map<string, string>>(
    new Map(),
  );

  // State for Schulhof release supervision modal
  const [showReleaseModal, setShowReleaseModal] = useState(false);
  const [isReleasingSupervision, setIsReleasingSupervision] = useState(false);

  // Cached active groups for UnclaimedRooms (avoids duplicate API call)
  const [cachedActiveGroups, setCachedActiveGroups] = useState<
    MinimalActiveGroup[]
  >([]);
  const [currentStaffId, setCurrentStaffId] = useState<string | undefined>();

  // Get current selected room
  const currentRoom = allRooms[selectedRoomIndex] ?? null;

  // Check if current room is Schulhof (special release supervision feature)
  const isSchulhof = currentRoom?.room_name === SCHULHOF_ROOM_NAME;

  // Helper function to load visits for a specific room
  const loadRoomVisits = useCallback(
    async (
      roomId: string,
      roomName?: string,
      groupNameToId?: Map<string, string>,
    ): Promise<StudentWithVisit[]> => {
      try {
        // Use bulk endpoint to fetch visits with display data for specific room
        const visits =
          await activeService.getActiveGroupVisitsWithDisplay(roomId);

        // Filter only active visits (students currently checked in)
        const currentlyCheckedIn = visits.filter((visit) => visit.isActive);

        const enriched = currentlyCheckedIn.map((visit) => {
          // Build from visit display data only (cross-group)
          const nameParts = visit.studentName?.split(" ") ?? ["", ""];
          const firstName = nameParts[0] ?? "";
          const lastName = nameParts.slice(1).join(" ") ?? "";
          // Set location with room name for proper badge display
          const location = roomName ? `Anwesend - ${roomName}` : "Anwesend";

          // Look up group_id from group_name using the map
          const groupId =
            visit.groupName && groupNameToId
              ? groupNameToId.get(visit.groupName)
              : undefined;

          return {
            id: visit.studentId,
            name: visit.studentName ?? "",
            first_name: firstName,
            second_name: lastName,
            school_class: visit.schoolClass ?? "",
            current_location: location,
            group_name: visit.groupName,
            group_id: groupId, // Add group_id for permission checking
            activeGroupId: visit.activeGroupId,
            checkInTime: visit.checkInTime,
          } as StudentWithVisit;
        });

        return enriched;
      } catch (error) {
        // Handle 403 Forbidden gracefully - user might not have group access
        if (error instanceof Error && error.message.includes("403")) {
          console.warn(
            `No permission to view group ${roomId} - returning empty list`,
          );
          return []; // Return empty array instead of throwing
        }
        // Re-throw other errors
        throw error;
      }
    },
    [],
  );

  const currentRoomRef = useRef<ActiveRoom | null>(null);
  const hasSupervisionRef = useRef(false);
  const groupNameToIdMapRef = useRef<Map<string, string>>(new Map());

  useEffect(() => {
    currentRoomRef.current = currentRoom;
  }, [currentRoom]);

  useEffect(() => {
    groupNameToIdMapRef.current = groupNameToIdMap;
  }, [groupNameToIdMap]);

  // SSE event handler - direct refetch for affected room only
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      console.log("SSE event received:", event.type, event.active_group_id);
      const activeRoom = currentRoomRef.current;
      if (activeRoom && event.active_group_id === activeRoom.id) {
        const targetRoomId = activeRoom.id;
        const targetRoomName = activeRoom.room_name;
        console.log("Event for current room - fetching updated data");
        void loadRoomVisits(
          targetRoomId,
          targetRoomName,
          groupNameToIdMapRef.current,
        )
          .then((studentsFromVisits) => {
            setStudents([...studentsFromVisits]);

            // Update room student count
            setAllRooms((prev) =>
              prev.map((existingRoom) =>
                existingRoom.id === targetRoomId
                  ? {
                      ...existingRoom,
                      student_count: studentsFromVisits.length,
                    }
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
    enabled: true,
  });

  // Check access and fetch active room data
  useEffect(() => {
    const checkAccessAndFetchData = async () => {
      try {
        setIsLoading(true);

        // Check if user has any supervised groups OR unclaimed groups available
        // Changed from getMyActiveGroups() to getMySupervisedGroups()
        // This includes ALL supervisions (OGS groups + standalone activities)
        // Works even if user has NO OGS groups but supervises standalone activities
        const [myActiveGroups, unclaimedGroups, staffResult] =
          await Promise.all([
            userContextService.getMySupervisedGroups(),
            activeService.getUnclaimedGroups(),
            userContextService.getCurrentStaff().catch(() => null),
          ]);

        // Cache staff ID for UnclaimedRooms component
        if (staffResult) {
          setCurrentStaffId(staffResult.id);
        }

        // Cache active groups for UnclaimedRooms component
        // If user has supervisions, combine them with unclaimed groups
        // If user has NO supervisions, DON'T cache - let UnclaimedRooms fetch ALL active groups
        // This ensures Schulhof banner shows even when it already has supervisors
        if (myActiveGroups.length > 0) {
          const combinedGroups = [...myActiveGroups, ...unclaimedGroups];
          setCachedActiveGroups(combinedGroups);
        } else {
          // Don't cache - UnclaimedRooms will fetch all active groups including Schulhof
          setCachedActiveGroups([]);
        }

        if (myActiveGroups.length === 0 && unclaimedGroups.length === 0) {
          // User has no active groups AND no unclaimed rooms
          // But we still need to show the page so UnclaimedRooms can check for Schulhof
          hasSupervisionRef.current = false;
          setHasAccess(true); // Grant access so UnclaimedRooms banner can be shown
          setAllRooms([]);
          setIsLoading(false);
          return;
        }

        // User has access (either supervised groups or unclaimed groups to claim)
        setHasAccess(true);

        // If user has no supervised groups but there are unclaimed groups,
        // just show the unclaimed rooms banner without trying to load room content
        if (myActiveGroups.length === 0) {
          hasSupervisionRef.current = false;
          setAllRooms([]);
          setIsLoading(false);
          return;
        }

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
        const studentsFromVisits = await loadRoomVisits(
          firstRoom.id,
          firstRoom.room_name,
          groupNameToIdMapRef.current,
        );

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
          console.error("403 Forbidden - No access to room/group:", err);
          setError("Sie haben aktuell keinen aktiven Raum zur Supervision.");
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

  // Load OGS group rooms for color detection and group IDs for permissions
  useEffect(() => {
    const loadGroupRooms = async () => {
      if (!session?.user?.token) {
        setMyGroupRooms([]);
        setMyGroupIds([]);
        return;
      }

      try {
        const myOgsGroups = await userContextService.getMyEducationalGroups();
        const roomNames = myOgsGroups
          .map((group) => group.room?.name)
          .filter((name): name is string => Boolean(name));
        setMyGroupRooms(roomNames);

        // Store group IDs for permission checking
        const groupIds = myOgsGroups.map((group) => group.id);
        setMyGroupIds(groupIds);

        // Create map from group name to group ID
        const nameToIdMap = new Map<string, string>();
        myOgsGroups.forEach((group) => {
          if (group.name) {
            nameToIdMap.set(group.name, group.id);
          }
        });
        setGroupNameToIdMap(nameToIdMap);
      } catch (err) {
        console.error("Error loading OGS group rooms:", err);
        setMyGroupRooms([]);
        setMyGroupIds([]);
      }
    };

    void loadGroupRooms();
  }, [session?.user?.token]);

  // Callback when a room is claimed - triggers refresh
  const handleRoomClaimed = useCallback(() => {
    setSseNonce((prev) => prev + 1);
    setRefreshKey((prev) => prev + 1);
  }, []);

  // Handle releasing Schulhof supervision
  const handleReleaseSupervision = useCallback(async () => {
    if (!currentRoom) return;

    try {
      setIsReleasingSupervision(true);

      // Get current user's staff ID
      const currentStaff = await userContextService.getCurrentStaff();

      // Get all supervisors for this active group
      const supervisors = await activeService.getActiveGroupSupervisors(
        currentRoom.id,
      );

      // Find the supervisor record for the current user
      const mySupervision = supervisors.find(
        (sup) => sup.staffId === currentStaff.id && sup.isActive,
      );

      if (mySupervision) {
        await activeService.endSupervision(mySupervision.id);
      } else {
        console.warn("No active supervision found for current user");
      }

      setShowReleaseModal(false);

      // Refresh the page to show updated state
      setSseNonce((prev) => prev + 1);
      setRefreshKey((prev) => prev + 1);
    } catch (err) {
      console.error("Failed to release Schulhof supervision:", err);
      setError("Fehler beim Abgeben der Schulhof-Aufsicht.");
    } finally {
      setIsReleasingSupervision(false);
    }
  }, [currentRoom]);

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
      const studentsFromVisits = await loadRoomVisits(
        selectedRoom.id,
        selectedRoom.room_name,
        groupNameToIdMapRef.current,
      );

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
      // Handle 403 gracefully - show message but don't break the UI
      if (err instanceof Error && err.message.includes("403")) {
        setError(
          `Keine Berechtigung für "${allRooms[roomIndex]?.name}". Kontaktieren Sie einen Administrator.`,
        );
        setStudents([]); // Show empty list instead of crashing
      } else {
        setError("Fehler beim Laden der Raumdaten.");
        console.error("Error loading room data:", err);
      }
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

  if (status === "loading" || isLoading || hasAccess === null) {
    return (
      <ResponsiveLayout>
        <Loading fullPage={false} />
      </ResponsiveLayout>
    );
  }

  // Show empty state if no active supervision
  if (hasAccess === false) {
    return (
      <ResponsiveLayout pageTitle="Aktuelle Aufsicht">
        <div className="-mt-1.5 w-full">
          <PageHeaderWithSearch title="Aktuelle Aufsicht" />

          <div className="flex min-h-[60vh] items-center justify-center px-4">
            <div className="flex max-w-md flex-col items-center gap-6 text-center">
              <div className="flex h-20 w-20 items-center justify-center rounded-full bg-gray-100">
                <svg
                  className="h-10 w-10 text-gray-400"
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
              <div className="space-y-2">
                <h3 className="text-xl font-bold text-gray-900">
                  Keine aktive Raum-Aufsicht
                </h3>
                <p className="text-gray-600">
                  Du bist aktuell in keinem Raum als Live-Aktivität registriert.
                </p>
                <p className="mt-4 text-sm text-gray-500">
                  Starte eine Aktivität an einem Terminal, um Live-Raumdaten
                  einzusehen.
                </p>
              </div>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  // Show unclaimed rooms banner when user has no supervised groups but there are rooms to claim
  if (allRooms.length === 0 && hasAccess) {
    return (
      <ResponsiveLayout pageTitle="Aktuelle Aufsicht">
        <div className="w-full">
          {/* Show unclaimed rooms banner - full width */}
          <UnclaimedRooms
            onClaimed={handleRoomClaimed}
            activeGroups={
              cachedActiveGroups.length > 0 ? cachedActiveGroups : undefined
            }
            currentStaffId={currentStaffId}
          />

          {/* Search bar and filters - always visible */}
          <PageHeaderWithSearch
            title=""
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

          {/* Neutral info message */}
          <div className="mt-8 flex min-h-[30vh] items-center justify-center">
            <div className="flex max-w-md flex-col items-center gap-4 text-center">
              <div className="flex h-16 w-16 items-center justify-center rounded-full bg-gray-100">
                <svg
                  className="h-8 w-8 text-gray-400"
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
              <div className="space-y-1">
                <h3 className="text-lg font-semibold text-gray-900">
                  Keine aktive Raum-Aufsicht
                </h3>
                <p className="text-sm text-gray-500">
                  Du beaufsichtigst aktuell keinen Raum.
                </p>
              </div>
            </div>
          </div>
        </div>
      </ResponsiveLayout>
    );
  }

  // TODO: Remove room selection screen entirely - threshold raised to effectively disable
  // Show room selection screen for 99+ rooms (effectively disabled)
  if (allRooms.length >= 99 && showRoomSelection) {
    return (
      <ResponsiveLayout>
        <div className="mx-auto w-full max-w-6xl px-4">
          {/* Unclaimed Rooms Section - Also show in room selection view */}
          <UnclaimedRooms
            onClaimed={handleRoomClaimed}
            activeGroups={
              cachedActiveGroups.length > 0 ? cachedActiveGroups : undefined
            }
            currentStaffId={currentStaffId}
          />

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
    <ResponsiveLayout activeSupervisionName={currentRoom?.room_name}>
      <div className="w-full">
        {/* Unclaimed Rooms Section - Shows rooms available for claiming */}
        <UnclaimedRooms
          onClaimed={handleRoomClaimed}
          activeGroups={
            cachedActiveGroups.length > 0 ? cachedActiveGroups : undefined
          }
          currentStaffId={currentStaffId}
        />

        {/* Modern Header with PageHeaderWithSearch component */}
        {/* No title - breadcrumb menu handles page identification */}
        <PageHeaderWithSearch
          title=""
          statusIndicator={{
            color:
              sseStatus === "connected"
                ? "green"
                : sseStatus === "reconnecting"
                  ? "yellow"
                  : sseStatus === "failed"
                    ? "red"
                    : "gray",
            tooltip:
              sseStatus === "connected"
                ? "Live-Updates aktiv"
                : sseStatus === "reconnecting"
                  ? `Verbindung wird wiederhergestellt... (Versuch ${reconnectAttempts}/5)`
                  : sseStatus === "failed"
                    ? "Verbindung fehlgeschlagen"
                    : "Verbindung wird hergestellt...",
          }}
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
          actionButton={
            isSchulhof ? (
              <button
                onClick={() => setShowReleaseModal(true)}
                className="group relative flex h-10 items-center gap-2 rounded-full bg-gradient-to-br from-amber-400 to-yellow-500 px-4 text-white shadow-lg transition-all duration-300 hover:scale-105 hover:shadow-xl hover:shadow-amber-400/30 active:scale-95"
                aria-label="Aufsicht abgeben"
              >
                <div className="pointer-events-none absolute inset-[2px] rounded-full bg-gradient-to-br from-white/20 to-white/0 opacity-0 transition-opacity duration-300 group-hover:opacity-100"></div>
                <svg
                  className="relative h-5 w-5 transition-transform duration-300"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                  />
                </svg>
                <span className="relative text-sm font-semibold">
                  Aufsicht abgeben
                </span>
              </button>
            ) : undefined
          }
          mobileActionButton={
            isSchulhof ? (
              <button
                onClick={() => setShowReleaseModal(true)}
                className="flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-amber-400 to-yellow-500 text-white shadow-md transition-all duration-200 active:scale-90"
                aria-label="Aufsicht abgeben"
              >
                <svg
                  className="h-4 w-4"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                  />
                </svg>
              </button>
            ) : undefined
          }
        />

        {/* Schulhof Release Supervision Modal */}
        <Modal
          isOpen={showReleaseModal}
          onClose={() => setShowReleaseModal(false)}
          title="Schulhof-Aufsicht abgeben"
        >
          <div className="space-y-4 md:space-y-5">
            {/* Warning Box */}
            <div className="rounded-lg border border-amber-200 bg-amber-50/50 p-3 md:p-4">
              <div className="flex items-start gap-3">
                <svg
                  className="mt-0.5 h-5 w-5 flex-shrink-0 text-amber-600"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                  strokeWidth={2.5}
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                  />
                </svg>
                <div className="flex-1">
                  <p className="text-sm font-medium text-amber-900">
                    Du wirst nicht mehr als Aufsicht angezeigt.
                  </p>
                  <p className="mt-1 text-sm text-amber-800">
                    Der Schulhof wird dann als &quot;ohne Aufsicht&quot;
                    angezeigt, bis eine andere Lehrkraft die Aufsicht übernimmt.
                  </p>
                </div>
              </div>
            </div>

            {/* Current Room Info */}
            <div className="rounded-lg border border-gray-100 bg-gray-50 p-3 md:p-4">
              <p className="text-sm text-gray-600">
                <span className="font-medium text-gray-900">Raum:</span>{" "}
                {currentRoom?.room_name ?? "Schulhof"}
              </p>
            </div>

            {/* Action Buttons */}
            <div className="flex gap-3 pt-2 md:pt-4">
              <button
                type="button"
                onClick={() => setShowReleaseModal(false)}
                disabled={isReleasingSupervision}
                className="flex-1 rounded-lg border border-gray-300 px-4 py-2.5 text-sm font-medium text-gray-700 transition-all duration-200 hover:scale-105 hover:border-gray-400 hover:bg-gray-50 hover:shadow-md active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 md:hover:scale-105"
              >
                Abbrechen
              </button>

              <button
                type="button"
                onClick={() => void handleReleaseSupervision()}
                disabled={isReleasingSupervision}
                className="flex-1 rounded-lg bg-gradient-to-br from-amber-400 to-yellow-500 px-4 py-2.5 text-sm font-medium text-white shadow-md transition-all duration-200 hover:scale-105 hover:shadow-lg hover:shadow-amber-400/30 active:scale-100 disabled:cursor-not-allowed disabled:opacity-50 disabled:hover:scale-100 md:hover:scale-105"
              >
                {isReleasingSupervision ? (
                  <span className="flex items-center justify-center gap-2">
                    <svg
                      className="h-4 w-4 animate-spin text-white"
                      fill="none"
                      viewBox="0 0 24 24"
                    >
                      <circle
                        className="opacity-25"
                        cx="12"
                        cy="12"
                        r="10"
                        stroke="currentColor"
                        strokeWidth="4"
                      ></circle>
                      <path
                        className="opacity-75"
                        fill="currentColor"
                        d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"
                      ></path>
                    </svg>
                    Wird abgegeben...
                  </span>
                ) : (
                  "Aufsicht abgeben"
                )}
              </button>
            </div>
          </div>
        </Modal>

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
          <div className="py-8 text-center">
            <div className="flex flex-col items-center gap-3">
              <svg
                className="h-10 w-10 text-gray-300"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
                strokeWidth={1.5}
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
                />
              </svg>
              <div>
                <h3 className="text-sm font-medium text-gray-600">
                  Keine Schüler in diesem Raum
                </h3>
                <p className="mt-1 text-xs text-gray-500">
                  Es wurden noch keine Schüler eingecheckt
                </p>
              </div>
            </div>
          </div>
        ) : filteredStudents.length > 0 ? (
          <div>
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
              {filteredStudents.map((student) => {
                return (
                  <div
                    key={student.id}
                    onClick={() =>
                      router.push(`/students/${student.id}?from=/active-supervisions`)
                    }
                    className={`group relative cursor-pointer overflow-hidden rounded-3xl border border-gray-100/50 bg-white/90 shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.97] md:hover:-translate-y-3 md:hover:scale-[1.03] md:hover:border-[#5080D8]/30 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]`}
                  >
                    {/* Modern gradient overlay */}
                    <div
                      className={`absolute inset-0 bg-gradient-to-br ${GROUP_CARD_GRADIENT} rounded-3xl opacity-[0.03]`}
                    ></div>
                    {/* Subtle inner glow */}
                    <div className="absolute inset-px rounded-3xl bg-gradient-to-br from-white/80 to-white/20"></div>
                    {/* Modern border highlight */}
                    <div className="absolute inset-0 rounded-3xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60"></div>

                    <div className="relative p-6">
                      {/* Header with student name */}
                      <div className="mb-3 flex items-start justify-between gap-3">
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
                          {/* School Class */}
                          {student.school_class && (
                            <div className="mt-1 flex items-center gap-1.5">
                              <svg
                                className="h-3.5 w-3.5 text-gray-400"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  strokeLinecap="round"
                                  strokeLinejoin="round"
                                  strokeWidth={2}
                                  d="M12 6.253v13m0-13C10.832 5.477 9.246 5 7.5 5S4.168 5.477 3 6.253v13C4.168 18.477 5.754 18 7.5 18s3.332.477 4.5 1.253m0-13C13.168 5.477 14.754 5 16.5 5c1.747 0 3.332.477 4.5 1.253v13C19.832 18.477 18.247 18 16.5 18c-1.746 0-3.332.477-4.5 1.253"
                                />
                              </svg>
                              <span className="overflow-hidden text-xs font-medium text-ellipsis whitespace-nowrap text-gray-500">
                                Klasse {student.school_class}
                              </span>
                            </div>
                          )}
                          {/* OGS Group Label */}
                          {student.group_name && (
                            <div className="mt-1 flex items-center gap-1.5">
                              <svg
                                className="h-3.5 w-3.5 text-gray-400"
                                fill="none"
                                viewBox="0 0 24 24"
                                stroke="currentColor"
                              >
                                <path
                                  strokeLinecap="round"
                                  strokeLinejoin="round"
                                  strokeWidth={2}
                                  d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"
                                />
                              </svg>
                              <span className="overflow-hidden text-xs font-medium text-ellipsis whitespace-nowrap text-gray-500">
                                Gruppe: {student.group_name}
                              </span>
                            </div>
                          )}
                        </div>

                        {/* Location Badge */}
                        <LocationBadge
                          student={student}
                          displayMode="contextAware"
                          userGroups={myGroupIds}
                          groupRooms={myGroupRooms}
                          variant="modern"
                          size="md"
                        />
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
        <ResponsiveLayout>
          <Loading fullPage={false} />
        </ResponsiveLayout>
      }
    >
      <SSEErrorBoundary>
        <MeinRaumPageContent />
      </SSEErrorBoundary>
    </Suspense>
  );
}
