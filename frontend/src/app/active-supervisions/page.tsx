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
import { EmptyStudentResults } from "~/components/ui/empty-student-results";
import {
  StudentCard,
  StudentInfoRow,
  SchoolClassIcon,
  GroupIcon,
} from "~/components/students/student-card";
import { activeService } from "~/lib/active-api";
import type { Student } from "~/lib/student-helpers";
import { UnclaimedRooms } from "~/components/active";
import { SSEErrorBoundary } from "~/components/sse/SSEErrorBoundary";
import { useSWRAuth } from "~/lib/swr";

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

// BFF response type for consolidated dashboard data
interface BFFDashboardResponse {
  supervisedGroups: Array<{
    id: string;
    name: string;
    room_id?: string;
    room?: { id: string; name: string };
  }>;
  unclaimedGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  currentStaff: { id: string } | null;
  educationalGroups: Array<{
    id: string;
    name: string;
    room?: { name: string };
  }>;
  firstRoomVisits: Array<{
    studentId: string;
    studentName: string;
    schoolClass: string;
    groupName: string;
    activeGroupId: string;
    checkInTime: string;
    isActive: boolean;
  }>;
  firstRoomId: string | null;
}

const GROUP_CARD_GRADIENT = "from-blue-50/80 to-cyan-100/80";

/** Loading state view */
function LoadingView() {
  return (
    <ResponsiveLayout>
      <Loading fullPage={false} />
    </ResponsiveLayout>
  );
}

/** No access empty state view */
function NoAccessView() {
  return (
    <ResponsiveLayout pageTitle="Aktuelle Aufsicht">
      <div className="-mt-1.5 w-full">
        <PageHeaderWithSearch title="Aktuelle Aufsicht" />

        <div className="flex min-h-[60vh] items-center justify-center px-4">
          <div className="flex max-w-md flex-col items-center gap-6 text-center">
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
                d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
              />
            </svg>
            <div className="space-y-2">
              <h3 className="text-lg font-medium text-gray-900">
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

/** Props for EmptyRoomsView */
interface EmptyRoomsViewProps {
  onClaimed: () => void;
  cachedActiveGroups: MinimalActiveGroup[];
  currentStaffId: string | undefined;
  searchTerm: string;
  setSearchTerm: (term: string) => void;
  setGroupFilter: (filter: string) => void;
  filterConfigs: FilterConfig[];
  activeFilters: ActiveFilter[];
}

/** View when user has access but no supervised rooms */
function EmptyRoomsView({
  onClaimed,
  cachedActiveGroups,
  currentStaffId,
  searchTerm,
  setSearchTerm,
  setGroupFilter,
  filterConfigs,
  activeFilters,
}: Readonly<EmptyRoomsViewProps>) {
  return (
    <ResponsiveLayout pageTitle="Aktuelle Aufsicht">
      <div className="w-full">
        {/* Show unclaimed rooms banner - full width */}
        <UnclaimedRooms
          onClaimed={onClaimed}
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
                d="M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4"
              />
            </svg>
            <div className="space-y-1">
              <h3 className="text-lg font-medium text-gray-900">
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

  // Helper to update room student count - extracted to reduce nesting depth
  const updateRoomStudentCount = useCallback(
    (roomId: string, studentCount: number) => {
      setAllRooms((prev) =>
        prev.map((room) =>
          room.id === roomId ? { ...room, student_count: studentCount } : room,
        ),
      );
    },
    [],
  );

  // SSE is handled globally by AuthWrapper - no page-level setup needed.
  // When student_checkin/checkout events occur, global SSE invalidates "visit*" caches,
  // which triggers SWR refetch for supervision-visits-* keys automatically.
  // NOTE: Do NOT call useGlobalSSE() here - it's already called in AuthWrapper.
  // Calling it again would create a duplicate SSE connection.

  // SWR-based BFF data fetching with caching
  // Cache key "active-supervision-dashboard" will be invalidated by global SSE on relevant events
  const {
    data: dashboardData,
    isLoading: isDashboardLoading,
    error: dashboardError,
  } = useSWRAuth<BFFDashboardResponse>(
    session?.user?.token ? `active-supervision-dashboard-${refreshKey}` : null,
    async () => {
      console.log("⏱️ [Page] SWR fetching BFF data...");
      const start = performance.now();

      const response = await fetch("/api/active-supervision-dashboard", {
        headers: {
          Authorization: `Bearer ${session?.user?.token}`,
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) {
        throw new Error(`BFF request failed: ${response.status}`);
      }

      const bffData = (await response.json()) as {
        data: BFFDashboardResponse;
      };

      console.log(
        `⏱️ [Page] SWR fetch complete: ${(performance.now() - start).toFixed(0)}ms`,
      );
      return bffData.data;
    },
    {
      keepPreviousData: true,
      revalidateOnFocus: false,
    },
  );

  // Sync SWR dashboard data with local state
  useEffect(() => {
    if (!dashboardData) return;

    const data = dashboardData;

    // Set staff ID for UnclaimedRooms component
    if (data.currentStaff) {
      setCurrentStaffId(data.currentStaff.id);
    }

    // Set educational groups data (for OGS group permissions)
    const roomNames = data.educationalGroups
      .map((group) => group.room?.name)
      .filter((name): name is string => !!name);
    setMyGroupRooms(roomNames);

    const groupIds = data.educationalGroups.map((group) => group.id);
    setMyGroupIds(groupIds);

    // Create map from group name to group ID
    const nameToIdMap = new Map<string, string>();
    data.educationalGroups.forEach((group) => {
      if (group.name) {
        nameToIdMap.set(group.name, group.id);
      }
    });
    setGroupNameToIdMap(nameToIdMap);
    groupNameToIdMapRef.current = nameToIdMap;

    // Cache active groups for UnclaimedRooms component
    if (data.supervisedGroups.length > 0) {
      const combinedGroups = [
        ...data.supervisedGroups.map((g) => ({
          id: g.id,
          room: g.room ? { name: g.room.name } : undefined,
        })),
        ...data.unclaimedGroups.map((g) => ({
          id: g.id,
          room: g.room,
        })),
      ];
      setCachedActiveGroups(combinedGroups);
    } else {
      setCachedActiveGroups([]);
    }

    // Check access
    if (
      data.supervisedGroups.length === 0 &&
      data.unclaimedGroups.length === 0
    ) {
      hasSupervisionRef.current = false;
      setHasAccess(true);
      setAllRooms([]);
      setIsLoading(false);
      return;
    }

    setHasAccess(true);

    // If no supervised groups but unclaimed groups exist
    if (data.supervisedGroups.length === 0) {
      hasSupervisionRef.current = false;
      setAllRooms([]);
      setIsLoading(false);
      return;
    }

    // Track if supervision was gained
    hasSupervisionRef.current = data.supervisedGroups.length > 0;

    // Convert supervised groups to ActiveRoom format
    const activeRooms: ActiveRoom[] = data.supervisedGroups.map((group) => ({
      id: group.id,
      name: group.name,
      room_name: group.room?.name,
      room_id: group.room_id,
      student_count: undefined,
      supervisor_name: undefined,
    }));

    setAllRooms(activeRooms);

    // Use pre-loaded visits from BFF for the first room
    // IMPORTANT: Only apply first room visits when the first room is selected.
    // When SSE triggers revalidation while user views another room, we must NOT
    // overwrite their current view with the first room's data.
    const firstRoom = activeRooms[0];
    if (selectedRoomIndex === 0) {
      if (firstRoom && data.firstRoomVisits.length > 0) {
        const studentsFromVisits: StudentWithVisit[] = data.firstRoomVisits.map(
          (visit) => {
            const nameParts = visit.studentName?.split(" ") ?? ["", ""];
            const firstName = nameParts[0] ?? "";
            const lastName = nameParts.slice(1).join(" ") ?? "";
            const location = firstRoom.room_name
              ? `Anwesend - ${firstRoom.room_name}`
              : "Anwesend";

            const groupId = visit.groupName
              ? nameToIdMap.get(visit.groupName)
              : undefined;

            return {
              id: visit.studentId,
              name: visit.studentName ?? "",
              first_name: firstName,
              second_name: lastName,
              school_class: visit.schoolClass ?? "",
              current_location: location,
              group_name: visit.groupName,
              group_id: groupId,
              activeGroupId: visit.activeGroupId,
              checkInTime: new Date(visit.checkInTime),
            } as StudentWithVisit;
          },
        );

        setStudents(studentsFromVisits);
        updateRoomStudentCount(firstRoom.id, studentsFromVisits.length);
      } else if (firstRoom) {
        setStudents([]);
        updateRoomStudentCount(firstRoom.id, 0);
      }
    }

    setError(null);
    setIsLoading(false);
  }, [dashboardData, updateRoomStudentCount, selectedRoomIndex]);

  // Handle dashboard error
  useEffect(() => {
    if (dashboardError) {
      if (dashboardError.message.includes("403")) {
        setError("Sie haben aktuell keinen aktiven Raum zur Supervision.");
        setHasAccess(false);
      } else {
        setError("Fehler beim Laden der Aktivitätsdaten.");
      }
      setIsLoading(false);
    }
  }, [dashboardError]);

  // Derive loading state from SWR
  useEffect(() => {
    if (isDashboardLoading && !dashboardData) {
      setIsLoading(true);
    }
  }, [isDashboardLoading, dashboardData]);

  // Callback when a room is claimed - triggers refresh
  const handleRoomClaimed = useCallback(() => {
    setRefreshKey((prev) => prev + 1);
  }, []);

  // Handle releasing Schulhof supervision
  const handleReleaseSupervision = useCallback(async () => {
    if (!currentRoom || !currentStaffId) return;

    try {
      setIsReleasingSupervision(true);

      // Get all supervisors for this active group
      const supervisors = await activeService.getActiveGroupSupervisors(
        currentRoom.id,
      );

      // Find the supervisor record for the current user (using cached staff ID)
      const mySupervision = supervisors.find(
        (sup) => sup.staffId === currentStaffId && sup.isActive,
      );

      if (mySupervision) {
        await activeService.endSupervision(mySupervision.id);
      } else {
        console.warn("No active supervision found for current user");
      }

      setShowReleaseModal(false);

      // Refresh the page to show updated state
      setRefreshKey((prev) => prev + 1);
    } catch (err) {
      console.error("Failed to release Schulhof supervision:", err);
      setError("Fehler beim Abgeben der Schulhof-Aufsicht.");
    } finally {
      setIsReleasingSupervision(false);
    }
  }, [currentRoom, currentStaffId]);

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
          .filter((name): name is string => !!name),
      ),
    ).sort((a, b) => a.localeCompare(b, "de"));

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
    return <LoadingView />;
  }

  // Show empty state if no active supervision
  if (!hasAccess) {
    return <NoAccessView />;
  }

  // Show unclaimed rooms banner when user has no supervised groups but there are rooms to claim
  if (allRooms.length === 0 && hasAccess) {
    return (
      <EmptyRoomsView
        onClaimed={handleRoomClaimed}
        cachedActiveGroups={cachedActiveGroups}
        currentStaffId={currentStaffId}
        searchTerm={searchTerm}
        setSearchTerm={setSearchTerm}
        setGroupFilter={setGroupFilter}
        filterConfigs={filterConfigs}
        activeFilters={activeFilters}
      />
    );
  }

  // Render helper for student grid content
  const renderStudentContent = () => {
    if (students.length === 0) {
      return (
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
      );
    }

    if (filteredStudents.length > 0) {
      return (
        <div>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
            {filteredStudents.map((student) => (
              <StudentCard
                key={student.id}
                studentId={student.id}
                firstName={student.first_name}
                lastName={student.second_name}
                gradient={GROUP_CARD_GRADIENT}
                onClick={() =>
                  router.push(
                    `/students/${student.id}?from=/active-supervisions`,
                  )
                }
                locationBadge={
                  <LocationBadge
                    student={student}
                    displayMode="contextAware"
                    userGroups={myGroupIds}
                    groupRooms={myGroupRooms}
                    variant="modern"
                    size="md"
                  />
                }
                extraContent={
                  <>
                    {student.school_class && (
                      <StudentInfoRow icon={<SchoolClassIcon />}>
                        Klasse {student.school_class}
                      </StudentInfoRow>
                    )}
                    {student.group_name && (
                      <StudentInfoRow icon={<GroupIcon />}>
                        Gruppe: {student.group_name}
                      </StudentInfoRow>
                    )}
                  </>
                }
              />
            ))}
          </div>
        </div>
      );
    }

    return (
      <EmptyStudentResults
        totalCount={students.length}
        filteredCount={filteredStudents.length}
      />
    );
  };

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
                onClick={() =>
                  handleReleaseSupervision().catch(() => undefined)
                }
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

        {/* Mobile Error Display */}
        {error && (
          <div className="mb-4 md:hidden">
            <Alert type="error" message={error} />
          </div>
        )}

        {/* Student Grid - Mobile Optimized */}
        {renderStudentContent()}
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
