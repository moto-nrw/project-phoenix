"use client";

import { useState, useEffect, useRef, Suspense, useMemo } from "react";
// SSE is handled globally by TenantAuthWrapper - real-time updates work automatically
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { studentService, groupService } from "~/lib/api";
import type { Student, Group } from "~/lib/api";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { Loading } from "~/components/ui/loading";
import { StudentPresenceBadge } from "@/components/ui/student-presence-badge";
import {
  isHomeLocation,
  isPresentLocation,
  isSchoolyardLocation,
  isTransitLocation,
} from "~/lib/location-helper";
import {
  SCHOOL_YEAR_FILTER_OPTIONS,
  getSchoolYear,
} from "~/lib/student-helpers";
import { useMinuteClock } from "~/lib/pickup-helpers";
import {
  StudentCard,
  SchoolClassIcon,
  GroupIcon,
  StudentInfoRow,
  PickupTimeRow,
  ArrivalTimeRow,
} from "~/components/students/student-card";
import { SchoolCheckinFab } from "~/components/students/school-checkin-fab";
import { SchoolCheckinModeMobile } from "~/components/students/school-checkin-mode-mobile";
import {
  deriveCheckinState,
  useSchoolCheckinMode,
} from "~/lib/hooks/use-school-checkin-mode";
import { usePresenceMode } from "~/components/tenant/tenant-provider";
import { useSWRAuth, useImmutableSWR } from "~/lib/swr";
import { activeService } from "~/lib/active-api";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import { TrackingIndicators } from "~/components/students/tracking-indicators";
import { createLogger } from "~/lib/logger";
import {
  getStudentTimeStatus,
  getTimeStatusSortRank,
} from "~/lib/student-time-status";
import {
  matchesTrackingFilter,
  resolveTrackingFilterAfterLabelChange,
  trackingFilterChipLabel,
  type TrackingFilter,
} from "./tracking-filter";

const logger = createLogger({ component: "StudentSearchPage" });

function SearchPageContent() {
  const router = useTenantRouter();
  // Use required: true to auto-redirect unauthenticated users (same pattern as /active-supervisions)
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });
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
    "krank",
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
  const [pickupTimeFilter, setPickupTimeFilter] = useState("all");
  const [trackingFilter, setTrackingFilter] = useState<TrackingFilter>("all");
  const [sortMode, setSortMode] = useState<"default" | "arrival">("default");

  // Current time for pickup urgency calculation (updates every minute)
  const now = useMinuteClock();

  // OGS group tracking via shared BFF endpoint with SWR caching
  // This eliminates 2 separate API calls with 2 auth() calls each
  const { userContext } = useUserContext();
  const myGroups = userContext?.educationalGroupIds ?? [];
  const myGroupRooms = userContext?.educationalGroupRoomNames ?? [];
  const mySupervisedRooms = userContext?.supervisedRoomNames ?? [];

  // Page-level school check-in/out mode. When active, clicking a card toggles
  // the student's attendance instead of navigating to the detail page.
  //
  // Only exposed in binary-mode tenants — detailed-mode schools check
  // students in via the RFID kiosk and a parallel web button would create
  // confusing divergent state.
  const presenceMode = usePresenceMode();
  const isBinaryMode = presenceMode === "binary";
  const schoolCheckin = useSchoolCheckinMode();

  // Debounce search term for SWR key (prevents excessive API calls while typing)
  useEffect(() => {
    if (searchTimeoutRef.current) {
      clearTimeout(searchTimeoutRef.current);
    }

    searchTimeoutRef.current = setTimeout(() => {
      setDebouncedSearchTerm(searchTerm);
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
        logger.warn("could not load groups for filter");
        return [];
      }
    },
  );

  // Generate SWR cache key for students (changes when filters change → SWR auto-cancels old requests)
  // Note: User context is only for badge styling, not for fetching students
  const studentsCacheKey = `search-students-${debouncedSearchTerm}-${selectedGroup}`;

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
        includePickupTimes: true,
        includeArrivalTimes: true,
      });
    },
    {
      // Keep previous data while fetching (prevents loading flash)
      keepPreviousData: true,
    },
  );

  const students = studentsData?.students ?? [];

  // Tracking indicators for student cards
  const trackingStudentIds = useMemo(
    () => (studentsData?.students ?? []).map((s) => s.id),
    [studentsData],
  );
  const { data: trackingData } = useSWRAuth<TrackingIndicatorsResponse>(
    trackingStudentIds.length > 0
      ? `tracking-indicators-${debouncedSearchTerm}-${selectedGroup}`
      : null,
    async () => activeService.getTrackingIndicators(trackingStudentIds),
    { keepPreviousData: true, revalidateOnFocus: false },
  );

  // Reset tracking filter if labels vanish or the selected label index becomes stale.
  // Without this, the active-filter chip (guarded by trackingData.labels) can hide
  // while trackingFilter remains set, leaving an invisible filter applied.
  const trackingLabels = trackingData?.labels;
  useEffect(() => {
    const next = resolveTrackingFilterAfterLabelChange(
      trackingFilter,
      trackingData,
    );
    if (next !== trackingFilter) setTrackingFilter(next);
  }, [trackingData, trackingFilter]);

  // Error type for proper heading display (Fix P3: substring matching on transformed string)
  type ErrorType = "permission" | "session" | "generic" | null;

  // Parse error messages for user-friendly display, returning both type and message
  const [errorType, errorMessage]: [ErrorType, string | null] = useMemo(() => {
    if (!studentsError) return [null, null];

    const rawMessage =
      studentsError instanceof Error
        ? studentsError.message
        : String(studentsError);

    if (rawMessage.includes("403")) {
      return [
        "permission",
        "Du hast keine Berechtigung, Schülerdaten anzuzeigen. Bitte wende dich an einen Administrator.",
      ];
    }
    if (rawMessage.includes("401")) {
      return [
        "session",
        "Deine Sitzung ist abgelaufen. Bitte melde dich erneut an.",
      ];
    }
    return ["generic", "Fehler beim Laden der Schülerdaten."];
  }, [studentsError]);

  // Fix P1: Detect when auth prevents fetching (user can't fetch but no error from SWR)
  const canFetch = status === "authenticated" && !!session?.user?.token;
  const isAuthError = !canFetch && !studentsError && status !== "loading";

  // Fix P2: Track initialization state to prevent empty state flash
  // Only wait for session - user context loads in parallel (for badge styling only)
  const isInitializing = status === "loading";
  const hasFetchedOnce =
    studentsData !== undefined || studentsError !== undefined;

  // SSE is handled globally by TenantAuthWrapper - no page-level setup needed.
  // When student_checkin/checkout events occur, global SSE invalidates "student*" caches,
  // which triggers SWR refetch for search-students-* keys automatically.

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
        id: "sort",
        label: "Sortierung",
        type: "buttons",
        value: sortMode,
        onChange: (value) => setSortMode(value as "default" | "arrival"),
        options: [
          { value: "default", label: "Alphabetisch" },
          { value: "arrival", label: "Nächste Ankunft" },
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
          { value: "krank", label: "Krank" },
        ],
      },
      {
        id: "pickupTime",
        label: "Abholzeit",
        type: "dropdown",
        value: pickupTimeFilter,
        onChange: (value) => setPickupTimeFilter(value as string),
        options: [
          { value: "all", label: "Alle Abholzeiten" },
          ...Array.from(
            new Set(
              (studentsData?.students ?? [])
                .map((s) => s.pickup_time)
                .filter((t): t is string => !!t),
            ),
          )
            .sort((a, b) => a.localeCompare(b))
            .map((time) => ({ value: time, label: `${time} Uhr` })),
          { value: "none", label: "Keine Abholzeit" },
        ],
      },
      ...(trackingLabels && trackingLabels.length > 0
        ? [
            {
              id: "tracking",
              label: "Aktivitäten heute",
              type: "dropdown" as const,
              value: trackingFilter,
              onChange: (value: string | string[]) =>
                setTrackingFilter(value as TrackingFilter),
              options: [
                { value: "all", label: "Alle Aktivitäten heute" },
                ...trackingLabels.map((lbl, idx) => ({
                  value: `missing:${idx}`,
                  label: `Noch nicht in ${lbl}`,
                })),
                { value: "none_visited", label: "Noch nichts erledigt" },
              ],
            },
          ]
        : []),
    ],
    [
      selectedYear,
      selectedGroup,
      attendanceFilter,
      pickupTimeFilter,
      trackingFilter,
      trackingLabels,
      groups,
      studentsData,
      sortMode,
    ],
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
        krank: "Krank",
      };
      filters.push({
        id: "attendance",
        label: statusLabels[attendanceFilter] ?? attendanceFilter,
        onRemove: () => setAttendanceFilter("all"),
      });
    }

    if (pickupTimeFilter !== "all") {
      filters.push({
        id: "pickupTime",
        label:
          pickupTimeFilter === "none"
            ? "Keine Abholzeit"
            : `Abholzeit ${pickupTimeFilter} Uhr`,
        onRemove: () => setPickupTimeFilter("all"),
      });
    }

    if (trackingLabels) {
      const chipLabel = trackingFilterChipLabel(trackingFilter, trackingLabels);
      if (chipLabel !== null) {
        filters.push({
          id: "tracking",
          label: chipLabel,
          onRemove: () => setTrackingFilter("all"),
        });
      }
    }

    return filters;
  }, [
    searchTerm,
    selectedYear,
    selectedGroup,
    attendanceFilter,
    pickupTimeFilter,
    trackingFilter,
    trackingLabels,
    groups,
  ]);

  // Apply additional client-side filtering for attendance statuses and year
  const filteredStudents: Student[] = students.filter((student) => {
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

      // Filter for "Krank" status specifically — independent of location
      if (attendanceFilter === "krank" && !student.sick) {
        return false;
      }
    }

    // Apply year filter - extract year from school_class (e.g., "Klasse 3a" → year 3)
    if (selectedYear !== "all") {
      const studentYear = getSchoolYear(student.school_class);
      if (studentYear !== selectedYear) {
        return false;
      }
    }

    // Apply pickup time filter (skip redacted students — missing pickup_time
    // due to has_full_access=false is not the same as "no schedule")
    if (pickupTimeFilter !== "all") {
      if (student.has_full_access === false) return false;
      if (pickupTimeFilter === "none") {
        if (student.pickup_time || student.pickup_is_exception) return false;
      } else if (student.pickup_time !== pickupTimeFilter) {
        return false;
      }
    }

    if (!matchesTrackingFilter(student, trackingFilter, trackingData)) {
      return false;
    }

    return true;
  });

  // Apply sort mode (default = backend order; arrival = by arrival urgency + time)
  const sortedStudents = useMemo(() => {
    if (sortMode !== "arrival") return filteredStudents;

    const compareByName = (a: Student, b: Student) => {
      const lastCmp = (a.second_name ?? "").localeCompare(
        b.second_name ?? "",
        "de",
      );
      if (lastCmp !== 0) return lastCmp;
      return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
    };

    return [...filteredStudents].sort((a, b) => {
      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);

      if (!aHome && bHome) return 1;
      if (aHome && !bHome) return -1;

      const timeA = a.arrival_time;
      const timeB = b.arrival_time;
      const statusA = getStudentTimeStatus({
        plannedTime: timeA,
        actualTime: a.actual_arrival_time,
        now,
      });
      const statusB = getStudentTimeStatus({
        plannedTime: timeB,
        actualTime: b.actual_arrival_time,
        now,
      });
      const rankA = getTimeStatusSortRank(statusA);
      const rankB = getTimeStatusSortRank(statusB);

      if (rankA !== rankB) return rankA - rankB;

      if (timeA && timeB) {
        const timeCmp = timeA.localeCompare(timeB);
        if (timeCmp !== 0) return timeCmp;
      }

      return compareByName(a, b);
    });
  }, [filteredStudents, sortMode, now]);

  // Fix P2: Show loading during initialization (prevents empty state flash)
  // Note: With required: true, unauthenticated users are auto-redirected to login
  if (isInitializing || isAuthError) {
    return <Loading />;
  }

  return (
    <div className="-mt-1.5 w-full">
      {/* Page header — scrolls with the rest of the page (no sticky).
          Active filters surface as a count badge on the filter pill. The
          check-in/out trigger lives in a floating FAB rendered at the
          bottom of this component on mobile/tablet, or inline in the
          header on desktop via primaryAction. */}
      <div className="-mx-1 px-1 pb-2 sm:mx-0 sm:px-0">
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
          primaryAction={
            isBinaryMode ? (
              <SchoolCheckinFab
                variant="inline"
                isActive={schoolCheckin.isActive}
                onToggle={schoolCheckin.toggleActive}
                successCount={schoolCheckin.successCount}
                pendingCount={schoolCheckin.pendingIds.size}
              />
            ) : undefined
          }
          // 6 filters overflow the inline desktop row at iPad-class
          // viewports — switch to the mobile sheet pattern up to xl
          // (1280px). Matches Stripe / Airbnb / Slack pattern for
          // filter-heavy pages.
          desktopFiltersFrom="xl"
          activeFilterDisplay="count"
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
            setPickupTimeFilter("all");
            setTrackingFilter("all");
          }}
        />
      </div>

      {/* Mobile Error Display — outside the sticky stack so it doesn't
          push everything down on small screens. */}
      {errorMessage && (
        <div className="mb-4 md:hidden">
          <Alert type="error" message={errorMessage} />
        </div>
      )}

      {/* Mobile (<md) check-in mode trigger — inline pill / sticky bar. */}
      {isBinaryMode && (
        <div className="mb-3 md:hidden">
          <SchoolCheckinModeMobile
            isActive={schoolCheckin.isActive}
            onToggle={schoolCheckin.toggleActive}
            successCount={schoolCheckin.successCount}
            pendingCount={schoolCheckin.pendingIds.size}
          />
        </div>
      )}

      {/* Student Grid. Bottom padding reserves room for the mobile sticky
          bar / tablet floating FAB; desktop has neither. */}
      <div className={isBinaryMode ? "pb-24 lg:pb-0" : undefined}>
        {(() => {
          // Fix P2: Show loading while first fetch is in progress (not yet hasFetchedOnce)
          if (isSearching && !hasFetchedOnce) {
            return <Loading fullPage={false} />;
          }
          if (errorMessage) {
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
                    {/* Fix P3: Use errorType instead of substring matching */}
                    <h3 className="text-lg font-medium text-gray-900">
                      {errorType === "permission"
                        ? "Keine Berechtigung"
                        : "Fehler"}
                    </h3>
                    <p className="text-gray-600">{errorMessage}</p>
                  </div>
                </div>
              </div>
            );
          }
          // Fix P2: Only show empty state if we've fetched at least once
          if (filteredStudents.length === 0 && hasFetchedOnce) {
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
                {sortedStudents.map((student) => {
                  const checkinState = deriveCheckinState(
                    student.current_location,
                  );
                  const studentIdStr = student.id.toString();
                  return (
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
                      checkinMode={isBinaryMode && schoolCheckin.isActive}
                      checkinState={checkinState}
                      isCheckinPending={schoolCheckin.pendingIds.has(
                        studentIdStr,
                      )}
                      onCheckinClick={() =>
                        void schoolCheckin.toggle(studentIdStr, checkinState)
                      }
                      locationBadge={
                        <StudentPresenceBadge
                          student={{
                            ...student,
                            not_arrival_today:
                              (student.arrival_is_exception ?? false) &&
                              !student.arrival_time,
                            not_arrival_reason: student.arrival_notes ?? null,
                          }}
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
                            {student.school_class}
                          </StudentInfoRow>
                          {student.group_name && (
                            <StudentInfoRow icon={<GroupIcon />}>
                              Gruppe: {student.group_name}
                            </StudentInfoRow>
                          )}
                          {student.has_full_access !== false && (
                            <>
                              <ArrivalTimeRow
                                arrivalTime={student.arrival_time}
                                actualTime={student.actual_arrival_time}
                                isException={
                                  student.arrival_is_exception ?? false
                                }
                                isAbsent={
                                  (student.arrival_is_exception ?? false) &&
                                  !student.arrival_time
                                }
                                notes={student.arrival_notes}
                                now={now}
                              />
                              <PickupTimeRow
                                pickupTime={student.pickup_time ?? undefined}
                                actualTime={student.actual_pickup_time}
                                isException={
                                  student.pickup_is_exception ?? false
                                }
                                notes={student.pickup_notes}
                                now={now}
                              />
                            </>
                          )}
                        </>
                      }
                      trackingIndicators={
                        trackingData?.labels?.length &&
                        student.has_full_access !== false ? (
                          <TrackingIndicators
                            labels={trackingData.labels}
                            results={trackingData.results[student.id] ?? []}
                          />
                        ) : undefined
                      }
                    />
                  );
                })}
              </div>
            </div>
          );
        })()}
      </div>

      {/* Tablet (md..xl) check-in mode trigger — floating FAB. Tablet
          range is bumped to `xl` here to stay aligned with the header's
          desktopFiltersFrom="xl" — both the filter sheet and the FAB
          live under the same boundary so iPad Air gets the consistent
          tablet UX. */}
      {isBinaryMode && (
        <div className="hidden md:block xl:hidden">
          <SchoolCheckinFab
            variant="floating"
            isActive={schoolCheckin.isActive}
            onToggle={schoolCheckin.toggleActive}
            successCount={schoolCheckin.successCount}
            pendingCount={schoolCheckin.pendingIds.size}
          />
        </div>
      )}
    </div>
  );
}

// Main component with Suspense wrapper
export default function StudentSearchPage() {
  return (
    <Suspense fallback={<Loading fullPage={false} />}>
      <SearchPageContent />
    </Suspense>
  );
}
