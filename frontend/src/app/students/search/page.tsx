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

function SearchPageContent() {
  const { data: session, status } = useSession();
  const router = useRouter();
  const searchParams = useSearchParams();
  const searchTimeoutRef = useRef<NodeJS.Timeout | null>(null);
  const abortControllerRef = useRef<AbortController | null>(null);
  const requestIdRef = useRef(0);
  const isInitialMountRef = useRef(true);

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
  const [selectedGroup, setSelectedGroup] = useState("");
  const [selectedYear, setSelectedYear] = useState("all");
  const [attendanceFilter, setAttendanceFilter] = useState(
    initialAttendanceFilter,
  );

  // Data state
  const [students, setStudents] = useState<Student[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [isSearching, setIsSearching] = useState(true); // Start with loading state
  const [error, setError] = useState<string | null>(null);

  // OGS group tracking
  const [myGroups, setMyGroups] = useState<string[]>([]);
  const [myGroupRooms, setMyGroupRooms] = useState<string[]>([]); // Räume meiner OGS-Gruppen
  const [mySupervisedRooms, setMySupervisedRooms] = useState<string[]>([]);
  const [groupsLoaded, setGroupsLoaded] = useState(false);

  // Refs to track current filter values without triggering re-renders
  const searchTermRef = useRef(searchTerm);
  const selectedGroupRef = useRef(selectedGroup);

  // Update refs when state changes
  useEffect(() => {
    searchTermRef.current = searchTerm;
  }, [searchTerm]);

  useEffect(() => {
    selectedGroupRef.current = selectedGroup;
  }, [selectedGroup]);

  // Silent refetch for SSE updates (no loading spinner)
  const silentRefetchStudents = useCallback(async () => {
    try {
      const fetchedStudents = await studentService.getStudents({
        search: searchTermRef.current,
        groupId: selectedGroupRef.current,
      });
      setStudents(fetchedStudents.students);
    } catch (err) {
      // Silently fail on background refresh - don't disrupt UI
      console.error("SSE background refresh failed:", err);
    }
  }, []);

  // SSE event handler - refresh when students check in/out
  // Always refresh on location events to handle:
  // 1. Students already in list whose location changed
  // 2. Students who should appear/disappear due to attendance filters
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      if (
        event.type === "student_checkin" ||
        event.type === "student_checkout"
      ) {
        silentRefetchStudents().catch(() => undefined);
      }
    },
    [silentRefetchStudents],
  );

  // SSE connection for real-time location updates
  // Backend enforces staff-only access via person/staff record check
  useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: groupsLoaded,
  });

  const fetchStudentsData = useCallback(
    async (filters?: { search?: string; groupId?: string }) => {
      // Cancel any previous request
      if (abortControllerRef.current) {
        abortControllerRef.current.abort();
      }

      // Create new abort controller for this request
      const abortController = new AbortController();
      abortControllerRef.current = abortController;

      // Track request ID to ignore stale responses
      const currentRequestId = ++requestIdRef.current;

      try {
        setIsSearching(true);
        setError(null);

        // Fetch students from API using refs for current values
        const fetchedStudents = await studentService.getStudents({
          search: filters?.search ?? searchTermRef.current,
          groupId: filters?.groupId ?? selectedGroupRef.current,
        });

        // Only update state if this is still the latest request
        if (currentRequestId === requestIdRef.current) {
          setStudents(fetchedStudents.students);
        }
      } catch (err) {
        // Ignore aborted requests
        if (err instanceof Error && err.name === "AbortError") {
          return;
        }

        // Only update state if this is still the latest request
        if (currentRequestId !== requestIdRef.current) {
          return;
        }

        // Error fetching students - handle gracefully with specific messages
        const errorMessage = err instanceof Error ? err.message : String(err);

        // Check for 403 Forbidden (missing permissions)
        if (errorMessage.includes("403")) {
          setError(
            "Du hast keine Berechtigung, Schülerdaten anzuzeigen. Bitte wende dich an einen Administrator.",
          );
        } else if (errorMessage.includes("401")) {
          setError("Deine Sitzung ist abgelaufen. Bitte melde dich erneut an.");
        } else {
          setError("Fehler beim Laden der Schülerdaten.");
        }
      } finally {
        // Only update loading state if this is still the latest request
        if (currentRequestId === requestIdRef.current) {
          setIsSearching(false);
        }
      }
    },
    [], // No dependencies - function is stable
  );

  // Load groups and user's OGS groups on mount
  useEffect(() => {
    const loadInitialData = async () => {
      // Load all groups for filter (non-fatal if user lacks permission)
      try {
        const fetchedGroups = await groupService.getGroups();
        setGroups(fetchedGroups);
      } catch (error) {
        // User might not have groups:read permission - continue with empty list
        console.warn("Could not load groups for filter:", error);
        setGroups([]);
      }

      // Load user's OGS groups and supervised rooms
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

    loadInitialData().catch(console.error);
  }, [session?.user?.token]);

  // Load initial students after groups are loaded
  useEffect(() => {
    if (groupsLoaded) {
      fetchStudentsData().catch(console.error);
      // Mark initial mount as complete after first successful fetch
      isInitialMountRef.current = false;
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groupsLoaded]);

  // Debounced search effect (skip initial mount - handled by groupsLoaded effect)
  useEffect(() => {
    if (isInitialMountRef.current) {
      return;
    }

    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      if (searchTerm.length >= 2 || searchTerm.length === 0) {
        fetchStudentsData().catch(console.error);
      }
    }, 300);

    return () => {
      if (searchTimeoutRef.current) {
        clearTimeout(searchTimeoutRef.current);
      }
    };
    // fetchStudentsData is stable (empty deps array), so no need to include it
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [searchTerm]);

  // Re-fetch when group filter changes (skip initial mount - handled by groupsLoaded effect)
  useEffect(() => {
    if (!isInitialMountRef.current) {
      fetchStudentsData().catch(console.error);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [selectedGroup]);

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
                {filteredStudents.map((student) => {
                  const handleClick = () =>
                    router.push(
                      `/students/${student.id}?from=/students/search`,
                    );
                  return (
                    <button
                      type="button"
                      key={student.id}
                      onClick={handleClick}
                      className="group relative w-full cursor-pointer overflow-hidden rounded-2xl border border-gray-100/50 bg-white/90 text-left shadow-[0_8px_30px_rgb(0,0,0,0.12)] backdrop-blur-md transition-all duration-500 active:scale-[0.97] md:hover:-translate-y-3 md:hover:scale-[1.03] md:hover:border-[#5080D8]/30 md:hover:bg-white md:hover:shadow-[0_20px_50px_rgb(0,0,0,0.15)]"
                    >
                      {/* Modern gradient overlay */}
                      <div className="absolute inset-0 rounded-2xl bg-gradient-to-br from-blue-50/80 to-cyan-100/80 opacity-[0.03]"></div>
                      {/* Subtle inner glow */}
                      <div className="absolute inset-px rounded-2xl bg-gradient-to-br from-white/80 to-white/20"></div>
                      {/* Modern border highlight */}
                      <div className="absolute inset-0 rounded-2xl ring-1 ring-white/20 transition-all duration-300 md:group-hover:ring-blue-200/60"></div>

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

                          {/* Status Badge */}
                          <LocationBadge
                            student={student}
                            displayMode="contextAware"
                            userGroups={myGroups}
                            groupRooms={myGroupRooms}
                            supervisedRooms={mySupervisedRooms}
                            variant="modern"
                            size="md"
                          />
                        </div>

                        {/* Additional Info */}
                        <div className="mb-3 space-y-1">
                          <div className="flex items-center text-sm text-gray-600">
                            <svg
                              className="mr-2 h-4 w-4 text-gray-400"
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
                            <span>Klasse {student.school_class}</span>
                          </div>
                          {student.group_name && (
                            <div className="flex items-center text-sm text-gray-600">
                              <svg
                                className="mr-2 h-4 w-4 text-gray-400"
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
                              Gruppe: {student.group_name}
                            </div>
                          )}
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
                      <div className="absolute inset-0 rounded-2xl bg-gradient-to-r from-transparent via-blue-100/30 to-transparent opacity-0 transition-opacity duration-300 md:group-hover:opacity-100"></div>
                    </button>
                  );
                })}
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
