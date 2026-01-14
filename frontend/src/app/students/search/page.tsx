"use client";

import {
  useState,
  useEffect,
  useCallback,
  useRef,
  Suspense,
  useMemo,
} from "react";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent } from "~/lib/sse-types";
import { useSession } from "next-auth/react";
import { useRouter, useSearchParams } from "next/navigation";
import { ResponsiveLayout } from "~/components/dashboard";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { studentService, groupService } from "~/lib/api";
import type { Student, Group } from "~/lib/api";
import { userContextService } from "~/lib/usercontext-api";
import { Loading } from "~/components/ui/loading";
import { LocationBadge } from "@/components/ui/location-badge";
import {
  isHomeLocation,
  isPresentLocation,
  isSchoolyardLocation,
  isTransitLocation,
} from "~/lib/location-helper";
import { SCHOOL_YEAR_FILTER_OPTIONS } from "~/lib/student-helpers";
import {
  StudentCard,
  SchoolClassIcon,
  GroupIcon,
  StudentInfoRow,
} from "~/components/students/student-card";
import { useSWRAuth, useImmutableSWR, mutate } from "~/lib/swr";

function SearchPageContent() {
  const { data: session, status } = useSession();
  const router = useRouter();
  const searchParams = useSearchParams();
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Read initial filter from URL params (supports deep-linking from dashboard)
  const initialStatus = searchParams.get("status") ?? "all";
  const validStatuses = [
    "all",
    "anwesend",
    "abwesend",
    "unterwegs",
    "schulhof",
  ];
  const initialAttendanceFilter = validStatuses.includes(initialStatus)
    ? initialStatus
    : "all";

  // Search and filter state
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState(""); // Debounced version for SWR key
  const [selectedGroup, setSelectedGroup] = useState("");
  const [selectedYear, setSelectedYear] = useState("all");
  const [attendanceFilter, setAttendanceFilter] = useState(
    initialAttendanceFilter,
  );

  // OGS group tracking
  const [myGroups, setMyGroups] = useState<string[]>([]);
  const [myGroupRooms, setMyGroupRooms] = useState<string[]>([]); // Räume meiner OGS-Gruppen
  const [mySupervisedRooms, setMySupervisedRooms] = useState<string[]>([]);
  const [groupsLoaded, setGroupsLoaded] = useState(false);

  // Debounce search term for SWR key (prevents excessive API calls while typing)
  useEffect(() => {
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      if (searchTerm.length >= 2 || searchTerm.length === 0) {
        setDebouncedSearchTerm(searchTerm);
      }
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
  }, [searchTerm]);

  // Fetch groups with SWR (immutable - only fetched once)
  const { data: groups = [] } = useImmutableSWR<Group[]>(
    "search-groups-list",
    async () => {
      try {
        return await groupService.getGroups();
      } catch {
        // User might not have groups:read permission - continue with empty list
        console.warn("Could not load groups for filter");
        return [];
      }
    },
  );

  // Generate SWR cache key for students (changes when filters change → SWR auto-cancels old requests)
  const studentsCacheKey = groupsLoaded
    ? `search-students-${debouncedSearchTerm}-${selectedGroup}`
    : null;

  // Fetch students with SWR (automatic deduplication, cancellation, and revalidation)
  const {
    data: studentsData,
    isLoading: isSearching,
    error: studentsError,
  } = useSWRAuth<{ students: Student[] }>(
    studentsCacheKey,
    async () => {
      return await studentService.getStudents({
        search: debouncedSearchTerm,
        groupId: selectedGroup,
      });
    },
    {
      // Keep previous data while fetching (prevents loading flash)
      keepPreviousData: true,
    },
  );

  const students = studentsData?.students ?? [];

  // Parse error messages for user-friendly display
  const error = studentsError
    ? (() => {
        const errorMessage =
          studentsError instanceof Error
            ? studentsError.message
            : String(studentsError);
        if (errorMessage.includes("403")) {
          return "Du hast keine Berechtigung, Schülerdaten anzuzeigen. Bitte wende dich an einen Administrator.";
        } else if (errorMessage.includes("401")) {
          return "Deine Sitzung ist abgelaufen. Bitte melde dich erneut an.";
        }
        return "Fehler beim Laden der Schülerdaten.";
      })()
    : null;

  // SSE event handler - revalidate SWR cache when students check in/out
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      if (
        event.type === "student_checkin" ||
        event.type === "student_checkout"
      ) {
        // Trigger SWR revalidation silently (no loading state change due to keepPreviousData)
        void mutate(studentsCacheKey);
      }
    },
    [studentsCacheKey],
  );

  // SSE connection for real-time location updates
  // Backend enforces staff-only access via person/staff record check
  useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: groupsLoaded,
  });

  // Load user's OGS groups and supervised rooms on mount
  useEffect(() => {
    const loadUserContext = async () => {
      if (session?.user?.token) {
        try {
          const myOgsGroups = await userContextService.getMyEducationalGroups();
          setMyGroups(myOgsGroups.map((g) => g.id));

          // Extract room names from OGS groups (for green color detection)
          const ogsGroupRoomNames = myOgsGroups
            .map((group) => group.room?.name)
            .filter((name): name is string => !!name);
          setMyGroupRooms(ogsGroupRoomNames);

          // Load supervised rooms (active sessions) for room-based access
          const supervisedGroups =
            await userContextService.getMySupervisedGroups();
          const roomNames = supervisedGroups
            .map((group) => group.room?.name)
            .filter((name): name is string => !!name);
          setMySupervisedRooms(roomNames);
        } catch (ogsError) {
          console.error("Error loading OGS groups:", ogsError);
          // User might not have OGS groups, which is fine
        }
      }

      // Always mark groups as loaded so student search can proceed
      setGroupsLoaded(true);
    };

    loadUserContext().catch(console.error);
  }, [session?.user?.token]);

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "year",
        label: "Klassenstufe",
        type: "buttons",
        value: selectedYear,
        onChange: (value) => setSelectedYear(value as string),
        options: [...SCHOOL_YEAR_FILTER_OPTIONS],
      },
      {
        id: "group",
        label: "Gruppe",
        type: "dropdown",
        value: selectedGroup,
        onChange: (value) => setSelectedGroup(value as string),
        options: [
          { value: "", label: "Alle Gruppen" },
          ...groups.map((group) => ({ value: group.id, label: group.name })),
        ],
      },
      {
        id: "attendance",
        label: "Anwesenheit",
        type: "dropdown",
        value: attendanceFilter,
        onChange: (value) => setAttendanceFilter(value as string),
        options: [
          { value: "all", label: "Alle Status" },
          { value: "anwesend", label: "Anwesend" },
          { value: "abwesend", label: "Zuhause" },
          { value: "unterwegs", label: "Unterwegs" },
          { value: "schulhof", label: "Schulhof" },
        ],
      },
    ],
    [selectedYear, selectedGroup, attendanceFilter, groups],
  );

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

    if (selectedYear !== "all") {
      filters.push({
        id: "year",
        label: `Jahr ${selectedYear}`,
        onRemove: () => setSelectedYear("all"),
      });
    }

    if (selectedGroup) {
      const groupName =
        groups.find((g) => g.id === selectedGroup)?.name ?? "Gruppe";
      filters.push({
        id: "group",
        label: groupName,
        onRemove: () => setSelectedGroup(""),
      });
    }

    if (attendanceFilter !== "all") {
      const statusLabels: Record<string, string> = {
        anwesend: "Anwesend",
        abwesend: "Zuhause",
        unterwegs: "Unterwegs",
        schulhof: "Schulhof",
      };
      filters.push({
        id: "attendance",
        label: statusLabels[attendanceFilter] ?? attendanceFilter,
        onRemove: () => setAttendanceFilter("all"),
      });
    }

    return filters;
  }, [searchTerm, selectedYear, selectedGroup, attendanceFilter, groups]);

  // Apply additional client-side filtering for attendance statuses and year
  const filteredStudents = students.filter((student) => {
    // Apply attendance filter
    if (attendanceFilter !== "all") {
      const isOnSite =
        isPresentLocation(student.current_location) ||
        isTransitLocation(student.current_location) ||
        isSchoolyardLocation(student.current_location);

      if (attendanceFilter === "anwesend" && !isOnSite) {
        return false;
      }

      if (
        attendanceFilter === "abwesend" &&
        !isHomeLocation(student.current_location)
      ) {
        return false;
      }

      // Filter for "Unterwegs" status specifically
      if (
        attendanceFilter === "unterwegs" &&
        !isTransitLocation(student.current_location)
      ) {
        return false;
      }

      // Filter for "Schulhof" status specifically
      if (
        attendanceFilter === "schulhof" &&
        !isSchoolyardLocation(student.current_location)
      ) {
        return false;
      }
    }

    // Apply year filter - extract year from school_class (e.g., "1a" → year 1)
    if (selectedYear !== "all") {
      const yearMatch = /^(\d)/.exec(student.school_class);
      const studentYear = yearMatch ? yearMatch[1] : null;
      if (studentYear !== selectedYear) {
        return false;
      }
    }

    return true;
  });

  if (status === "loading") {
    return <Loading />;
  }

  return (
    <ResponsiveLayout>
      <div className="-mt-1.5 w-full">
        {/* PageHeaderWithSearch - With Suche title */}
        <PageHeaderWithSearch
          title="Suche"
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
            count: filteredStudents.length,
          }}
          search={{
            value: searchTerm,
            onChange: setSearchTerm,
            placeholder: "Name suchen...",
          }}
          filters={filterConfigs}
          activeFilters={activeFilters}
          onClearAllFilters={() => {
            setSearchTerm("");
            setSelectedGroup("");
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

        {/* Student Grid - Mobile Optimized with Playful Design */}
        {(() => {
          if (isSearching) {
            return <Loading fullPage={false} />;
          }
          if (error) {
            return (
              <div className="py-12 text-center">
                <div className="flex flex-col items-center gap-4">
                  <svg
                    className="h-12 w-12 text-red-400"
                    fill="none"
                    viewBox="0 0 24 24"
                    stroke="currentColor"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth={2}
                      d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
                    />
                  </svg>
                  <div>
                    <h3 className="text-lg font-medium text-gray-900">
                      {error.includes("403") ? "Keine Berechtigung" : "Fehler"}
                    </h3>
                    <p className="text-gray-600">{error}</p>
                  </div>
                </div>
              </div>
            );
          }
          if (filteredStudents.length === 0) {
            return (
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
                  </div>
                </div>
              </div>
            );
          }
          return (
            <div>
              <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
                {filteredStudents.map((student) => (
                  <StudentCard
                    key={student.id}
                    studentId={student.id}
                    firstName={student.first_name}
                    lastName={student.second_name}
                    onClick={() =>
                      router.push(
                        `/students/${student.id}?from=/students/search`,
                      )
                    }
                    locationBadge={
                      <LocationBadge
                        student={student}
                        displayMode="contextAware"
                        userGroups={myGroups}
                        groupRooms={myGroupRooms}
                        supervisedRooms={mySupervisedRooms}
                        variant="modern"
                        size="md"
                      />
                    }
                    extraContent={
                      <>
                        <StudentInfoRow icon={<SchoolClassIcon />}>
                          Klasse {student.school_class}
                        </StudentInfoRow>
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
        })()}
      </div>
    </ResponsiveLayout>
  );
}

// Main component with Suspense wrapper
export default function StudentSearchPage() {
  return (
    <Suspense
      fallback={
        <ResponsiveLayout>
          <Loading fullPage={false} />
        </ResponsiveLayout>
      }
    >
      <SearchPageContent />
    </Suspense>
  );
}
