"use client";

import {
  useState,
  useEffect,
  useRef,
  Suspense,
  useMemo,
  useCallback,
} from "react";
// SSE is handled globally by TenantAuthWrapper - real-time updates work automatically
import { useSession } from "next-auth/react";
import { useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { studentService, groupService, roomService } from "~/lib/api";
import type { Student, Group, Room } from "~/lib/api";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { Loading } from "~/components/ui/loading";
import { StudentPresenceBadge } from "@/components/ui/student-presence-badge";
import {
  LOCATION_STATUSES,
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
import { SEARCH_ROOMS_LIST_CACHE_KEY } from "~/lib/swr/room-derived-caches";
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
const EMPTY_STRING_ARRAY: string[] = [];

type StatusFilter =
  | "all"
  | "anwesend"
  | "abwesend"
  | "unterwegs"
  | "schulhof"
  | "krank";
type SortMode = "name" | "arrival" | "pickup";
type GroupMode = "none" | "status" | "room" | "arrival" | "pickup";

const STATUS_FILTER_OPTIONS: Array<{ value: StatusFilter; label: string }> = [
  { value: "all", label: "Alle" },
  { value: "anwesend", label: "Anwesend" },
  { value: "abwesend", label: "Abwesend" },
  { value: "krank", label: "Krank" },
  { value: "unterwegs", label: "Unterwegs" },
  { value: "schulhof", label: "Schulhof" },
];

const SORT_OPTIONS: Array<{ value: SortMode; label: string }> = [
  { value: "name", label: "Name A-Z" },
  { value: "arrival", label: "Nächste Ankunft" },
  { value: "pickup", label: "Nächste Abholung" },
];

const GROUP_OPTIONS: Array<{ value: GroupMode; label: string }> = [
  { value: "none", label: "Liste" },
  { value: "status", label: "Nach Status" },
  { value: "room", label: "Nach Raum" },
  { value: "arrival", label: "Nach Ankunftszeit" },
  { value: "pickup", label: "Nach Abholzeit" },
];

const SCHOOL_YEAR_DROPDOWN_OPTIONS = SCHOOL_YEAR_FILTER_OPTIONS.map(
  (option) => ({
    value: option.value,
    label: option.value === "all" ? "Alle Stufen" : `Stufe ${option.label}`,
  }),
);

const STATUS_GROUP_ORDER = new Map([
  ["Anwesend", 0],
  ["Unterwegs", 1],
  ["Schulhof", 2],
  ["Krank", 3],
  ["Abwesend", 4],
]);

function compareByName(a: Student, b: Student) {
  const lastCmp = (a.second_name ?? "").localeCompare(
    b.second_name ?? "",
    "de",
  );
  if (lastCmp !== 0) return lastCmp;
  return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
}

function statusLabelForStudent(student: Student): string {
  if (student.sick) return "Krank";
  if (isSchoolyardLocation(student.current_location)) return "Schulhof";
  if (isTransitLocation(student.current_location)) return "Unterwegs";
  if (isHomeLocation(student.current_location)) return "Abwesend";
  return "Anwesend";
}

function roomLabelForStudent(student: Student): string {
  if (student.has_full_access === false) return "Nicht einsehbar";

  const location = student.current_location?.trim();
  if (!location || isHomeLocation(location)) return "Kein Raum";
  if (isTransitLocation(location)) return "Unterwegs";
  if (isSchoolyardLocation(location)) return "Schulhof";
  const separatorIndex = location.indexOf("-");
  if (separatorIndex >= 0) {
    const room = location.slice(separatorIndex + 1).trim();
    return room || "Kein Raum";
  }
  if (Object.values(LOCATION_STATUSES).some((status) => status === location)) {
    return "Kein Raum";
  }
  return location;
}

function pickupLabelForStudent(student: Student): string {
  if (student.has_full_access === false) return "Nicht einsehbar";
  return student.pickup_time ? `${student.pickup_time} Uhr` : "Keine Abholzeit";
}

function arrivalLabelForStudent(student: Student): string {
  if (student.has_full_access === false) return "Nicht einsehbar";
  return student.arrival_time
    ? `${student.arrival_time} Uhr`
    : "Keine Ankunftszeit";
}

function groupStudents(students: Student[], groupMode: GroupMode) {
  if (groupMode === "none") return [];

  const groups = new Map<string, Student[]>();
  for (const student of students) {
    const key =
      groupMode === "status"
        ? statusLabelForStudent(student)
        : groupMode === "room"
          ? roomLabelForStudent(student)
          : groupMode === "arrival"
            ? arrivalLabelForStudent(student)
            : pickupLabelForStudent(student);
    const bucket = groups.get(key);
    if (bucket) {
      bucket.push(student);
    } else {
      groups.set(key, [student]);
    }
  }

  return Array.from(groups.entries())
    .map(([label, items]) => ({
      label,
      items,
    }))
    .sort((a, b) => {
      if (groupMode === "status") {
        return (
          (STATUS_GROUP_ORDER.get(a.label) ?? 99) -
          (STATUS_GROUP_ORDER.get(b.label) ?? 99)
        );
      }
      if (groupMode === "pickup" || groupMode === "arrival") {
        const rank = (label: string) =>
          label === "Keine Abholzeit" || label === "Keine Ankunftszeit"
            ? "99"
            : label === "Nicht einsehbar"
              ? "zz"
              : label;
        return rank(a.label).localeCompare(rank(b.label), "de");
      }
      return a.label.localeCompare(b.label, "de");
    });
}

function orderedRooms(
  rooms: Room[],
  groupRoomNames: string[],
  supervisedRoomNames: string[],
) {
  const priorityNames = new Set(
    [...groupRoomNames, ...supervisedRoomNames].map((name) =>
      name.trim().toLowerCase(),
    ),
  );

  return [...rooms].sort((a, b) => {
    const aPriority = priorityNames.has(a.name.trim().toLowerCase());
    const bPriority = priorityNames.has(b.name.trim().toLowerCase());
    if (aPriority !== bPriority) return aPriority ? -1 : 1;
    return a.name.localeCompare(b.name, "de");
  });
}

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
  const validStatuses = STATUS_FILTER_OPTIONS.map((option) => option.value);
  const initialAttendanceFilter = validStatuses.includes(
    initialStatus as StatusFilter,
  )
    ? (initialStatus as StatusFilter)
    : "all";

  // Room filter — populated when the user lands here from the room detail
  // page's "In Kindersuche öffnen" link (#1323). room_name is purely a
  // display affordance for the chip; the backend filter only uses room_id.
  const initialRoomId = searchParams.get("room_id") ?? "";
  const initialRoomName = searchParams.get("room_name") ?? "";

  // Search and filter state
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearchTerm, setDebouncedSearchTerm] = useState(""); // Debounced version for SWR key
  const [selectedGroup, setSelectedGroup] = useState("");
  const [selectedYear, setSelectedYear] = useState("all");
  const [attendanceFilter, setAttendanceFilter] = useState<StatusFilter>(
    initialAttendanceFilter,
  );
  const [pickupTimeFilter, setPickupTimeFilter] = useState("all");
  const [arrivalTimeFilter, setArrivalTimeFilter] = useState("all");
  const [trackingFilter, setTrackingFilter] = useState<TrackingFilter>("all");
  const [sortMode, setSortMode] = useState<SortMode>("name");
  const [groupMode, setGroupMode] = useState<GroupMode>("none");
  const [selectedRoomId, setSelectedRoomId] = useState(initialRoomId);
  const [selectedRoomName, setSelectedRoomName] = useState(initialRoomName);

  // Current time for pickup urgency calculation (updates every minute)
  const now = useMinuteClock();

  // OGS group tracking via shared BFF endpoint with SWR caching
  // This eliminates 2 separate API calls with 2 auth() calls each
  const { userContext } = useUserContext();
  const myGroups = userContext?.educationalGroupIds ?? EMPTY_STRING_ARRAY;
  const myGroupRooms =
    userContext?.educationalGroupRoomNames ?? EMPTY_STRING_ARRAY;
  const mySupervisedRooms =
    userContext?.supervisedRoomNames ?? EMPTY_STRING_ARRAY;

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

  // Room names/options are mutable from the rooms admin page, so this must
  // revalidate stale cache entries when the search page remounts.
  const { data: rooms = [] } = useSWRAuth<Room[]>(
    SEARCH_ROOMS_LIST_CACHE_KEY,
    async () => {
      try {
        return await roomService.getRooms({ page: 1, pageSize: 1000 });
      } catch {
        logger.warn("could not load rooms for filter");
        return [];
      }
    },
  );

  // Generate SWR cache key for students (changes when filters change → SWR auto-cancels old requests)
  // Note: User context is only for badge styling, not for fetching students
  const studentsCacheKey = `search-students-${debouncedSearchTerm}-${selectedGroup}-${selectedRoomId}`;

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
        roomId: selectedRoomId || undefined,
        // When filtering by room, this page is the "see all" target the
        // room-detail modal links to (#1374). The modal itself caps at
        // 200 cards and shows a truncation notice; this page must show
        // every occupant so the overflow link actually delivers.
        // 1000 covers any realistic combined-group / assembly-room
        // session well above what backend ParsePagination would return
        // by default (50). General search keeps the default.
        pageSize: selectedRoomId ? 1000 : undefined,
        includePickupTimes: true,
        includeArrivalTimes: true,
      });
    },
    {
      // Keep previous data while fetching (prevents loading flash)
      keepPreviousData: true,
    },
  );

  // Keep room state and room_id/room_name query params in sync without a
  // Next.js navigation, which would discard SWR data and flash the loading
  // skeleton. Merge into the existing state object instead of replacing it —
  // App Router stashes routing metadata (scroll restoration, RSC cache keys)
  // on window.history.state, and clobbering it with `{}` can degrade browser
  // back/forward into a hard reload for this entry.
  const updateRoomFilter = useCallback((roomId: string, roomName: string) => {
    setSelectedRoomId(roomId);
    setSelectedRoomName(roomName);
    if (typeof window !== "undefined") {
      const url = new URL(window.location.href);
      if (roomId) {
        url.searchParams.set("room_id", roomId);
        if (roomName) {
          url.searchParams.set("room_name", roomName);
        } else {
          url.searchParams.delete("room_name");
        }
      } else {
        url.searchParams.delete("room_id");
        url.searchParams.delete("room_name");
      }
      window.history.replaceState(
        window.history.state ?? null,
        "",
        url.toString(),
      );
    }
  }, []);

  const clearRoomFilter = useCallback(() => {
    updateRoomFilter("", "");
  }, [updateRoomFilter]);

  const students = studentsData?.students ?? [];

  // Tracking indicators for student cards
  const trackingStudentIds = useMemo(
    () => (studentsData?.students ?? []).map((s) => s.id),
    [studentsData],
  );
  const { data: trackingData } = useSWRAuth<TrackingIndicatorsResponse>(
    trackingStudentIds.length > 0
      ? `tracking-indicators-${debouncedSearchTerm}-${selectedGroup}-${selectedRoomId}`
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
  const orderedRoomOptions = useMemo(
    () => orderedRooms(rooms, myGroupRooms, mySupervisedRooms),
    [rooms, myGroupRooms, mySupervisedRooms],
  );

  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "year",
        label: "Stufe",
        type: "dropdown",
        value: selectedYear,
        onChange: (value) => setSelectedYear(value as string),
        options: SCHOOL_YEAR_DROPDOWN_OPTIONS,
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
        id: "room",
        label: "Raum",
        type: "dropdown",
        value: selectedRoomId,
        onChange: (value: string | string[]) => {
          const v = Array.isArray(value) ? value[0] : value;
          if (!v) {
            clearRoomFilter();
            return;
          }
          const room = rooms.find((r) => r.id === v);
          updateRoomFilter(
            v,
            room?.name ?? (v === selectedRoomId ? selectedRoomName : ""),
          );
        },
        options: [
          { value: "", label: "Alle Räume" },
          ...orderedRoomOptions.map((room) => ({
            value: room.id,
            label: room.name,
          })),
          ...(selectedRoomId &&
          !orderedRoomOptions.some((room) => room.id === selectedRoomId)
            ? [
                {
                  value: selectedRoomId,
                  label: selectedRoomName || `Raum #${selectedRoomId}`,
                },
              ]
            : []),
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
      {
        id: "arrivalTime",
        label: "Ankunftszeit",
        type: "dropdown",
        value: arrivalTimeFilter,
        onChange: (value) => setArrivalTimeFilter(value as string),
        options: [
          { value: "all", label: "Alle Ankunftszeiten" },
          ...Array.from(
            new Set(
              (studentsData?.students ?? [])
                .map((s) => s.arrival_time)
                .filter((t): t is string => !!t),
            ),
          )
            .sort((a, b) => a.localeCompare(b))
            .map((time) => ({ value: time, label: `${time} Uhr` })),
          { value: "none", label: "Keine Ankunftszeit" },
        ],
      },
      {
        id: "attendance",
        label: "Status",
        type: "dropdown",
        value: attendanceFilter,
        onChange: (value) => setAttendanceFilter(value as StatusFilter),
        options: [
          { value: "all", label: "Alle Status" },
          { value: "anwesend", label: "Anwesend" },
          { value: "abwesend", label: "Abwesend" },
          { value: "krank", label: "Krank" },
          { value: "unterwegs", label: "Unterwegs" },
          { value: "schulhof", label: "Schulhof" },
        ],
      },
      {
        id: "sort",
        label: "Sortierung",
        type: "dropdown",
        value: sortMode,
        onChange: (value) => setSortMode(value as SortMode),
        options: SORT_OPTIONS,
      },
      {
        id: "groupMode",
        label: "Ansicht",
        type: "dropdown",
        value: groupMode,
        onChange: (value) => setGroupMode(value as GroupMode),
        options: GROUP_OPTIONS,
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
      pickupTimeFilter,
      arrivalTimeFilter,
      attendanceFilter,
      trackingFilter,
      trackingLabels,
      groups,
      rooms,
      studentsData,
      selectedRoomId,
      selectedRoomName,
      orderedRoomOptions,
      clearRoomFilter,
      updateRoomFilter,
      sortMode,
      groupMode,
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

    if (selectedRoomId) {
      // Fall back to "Raum #{id}" when no room_name was passed in the URL
      // (e.g. an old bookmark) — better than rendering an empty chip.
      const label = selectedRoomName
        ? `Raum: ${selectedRoomName}`
        : `Raum #${selectedRoomId}`;
      filters.push({
        id: "room",
        label,
        onRemove: clearRoomFilter,
      });
    }

    if (attendanceFilter !== "all") {
      const statusLabels: Record<Exclude<StatusFilter, "all">, string> = {
        anwesend: "Anwesend",
        abwesend: "Abwesend",
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

    if (arrivalTimeFilter !== "all") {
      filters.push({
        id: "arrivalTime",
        label:
          arrivalTimeFilter === "none"
            ? "Keine Ankunftszeit"
            : `Ankunftszeit ${arrivalTimeFilter} Uhr`,
        onRemove: () => setArrivalTimeFilter("all"),
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

    if (sortMode !== "name") {
      filters.push({
        id: "sort",
        label:
          SORT_OPTIONS.find((option) => option.value === sortMode)?.label ??
          "Sortierung",
        onRemove: () => setSortMode("name"),
      });
    }

    if (groupMode !== "none") {
      filters.push({
        id: "groupMode",
        label: `Ansicht: ${
          GROUP_OPTIONS.find((option) => option.value === groupMode)?.label ??
          "Ansicht"
        }`,
        onRemove: () => setGroupMode("none"),
      });
    }

    return filters;
  }, [
    searchTerm,
    selectedYear,
    selectedGroup,
    attendanceFilter,
    pickupTimeFilter,
    arrivalTimeFilter,
    trackingFilter,
    trackingLabels,
    groups,
    selectedRoomId,
    selectedRoomName,
    sortMode,
    groupMode,
    clearRoomFilter,
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

    if (arrivalTimeFilter !== "all") {
      if (student.has_full_access === false) return false;
      if (arrivalTimeFilter === "none") {
        if (student.arrival_time || student.arrival_is_exception) return false;
      } else if (student.arrival_time !== arrivalTimeFilter) {
        return false;
      }
    }

    if (!matchesTrackingFilter(student, trackingFilter, trackingData)) {
      return false;
    }

    return true;
  });

  // Apply sort mode (name = stable A-Z; arrival/pickup = daily operational order)
  const sortedStudents = useMemo(() => {
    if (sortMode === "name") return [...filteredStudents].sort(compareByName);

    return [...filteredStudents].sort((a, b) => {
      if (sortMode === "pickup") {
        const timeA = a.pickup_time;
        const timeB = b.pickup_time;
        if (timeA && !timeB) return -1;
        if (!timeA && timeB) return 1;
        if (timeA && timeB) {
          const timeCmp = timeA.localeCompare(timeB);
          if (timeCmp !== 0) return timeCmp;
        }
        return compareByName(a, b);
      }

      const aHome = isHomeLocation(a.current_location);
      const bHome = isHomeLocation(b.current_location);
      if (!aHome && bHome) return 1;
      if (aHome && !bHome) return -1;

      const statusA = getStudentTimeStatus({
        plannedTime: a.arrival_time,
        actualTime: a.actual_arrival_time,
        now,
      });
      const statusB = getStudentTimeStatus({
        plannedTime: b.arrival_time,
        actualTime: b.actual_arrival_time,
        now,
      });
      const rankA = getTimeStatusSortRank(statusA);
      const rankB = getTimeStatusSortRank(statusB);
      if (rankA !== rankB) return rankA - rankB;

      if (a.arrival_time && b.arrival_time) {
        const timeCmp = a.arrival_time.localeCompare(b.arrival_time);
        if (timeCmp !== 0) return timeCmp;
      }
      return compareByName(a, b);
    });
  }, [filteredStudents, sortMode, now]);

  const groupedStudents = useMemo(
    () => groupStudents(sortedStudents, groupMode),
    [sortedStudents, groupMode],
  );

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
          title="Kindersuche"
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
            setArrivalTimeFilter("all");
            setTrackingFilter("all");
            setSortMode("name");
            setGroupMode("none");
            clearRoomFilter();
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
          // Preserve the active room filter in the back-link so stepping
          // from a room → child → back returns to the same filtered list
          // (#1323). Plain `/students/search` would drop room_id/room_name.
          // Encoding is only needed when a query string is appended; the
          // bare path stays unencoded so existing call sites/tests keep
          // matching the established `from=/students/search` shape.
          //
          // Also forward `?status=` (attendanceFilter) — it is the only
          // URL-rehydrating refinement on this page beyond the room
          // filter, and the dashboard deep-links here with it. Without
          // forwarding it, drilling into a child after narrowing by
          // status silently broadens the result set on back. The other
          // local refinements (search term, group, year, pickup,
          // tracking, sort) are not URL-rehydrated on this page, so
          // forwarding them would not survive the remount anyway.
          const buildFromParam = (() => {
            const statusFilter =
              attendanceFilter !== "all" ? attendanceFilter : "";
            if (!selectedRoomId && !statusFilter) return "/students/search";
            const qs = new URLSearchParams();
            if (selectedRoomId) {
              qs.set("room_id", selectedRoomId);
              if (selectedRoomName) qs.set("room_name", selectedRoomName);
            }
            if (statusFilter) qs.set("status", statusFilter);
            return encodeURIComponent(`/students/search?${qs.toString()}`);
          })();
          const renderStudentCard = (student: Student) => {
            const checkinState = deriveCheckinState(student.current_location);
            const studentIdStr = student.id.toString();
            return (
              <StudentCard
                key={student.id}
                studentId={student.id}
                firstName={student.first_name}
                lastName={student.second_name}
                onClick={() =>
                  router.push(`/students/${student.id}?from=${buildFromParam}`)
                }
                checkinMode={isBinaryMode && schoolCheckin.isActive}
                checkinState={checkinState}
                isCheckinPending={schoolCheckin.pendingIds.has(studentIdStr)}
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
                          isException={student.arrival_is_exception ?? false}
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
                          isException={student.pickup_is_exception ?? false}
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
          };

          if (groupMode !== "none") {
            return (
              <div className="space-y-6">
                {groupedStudents.map((group) => (
                  <section key={group.label} data-testid="student-group">
                    <div className="mb-3 flex items-center gap-3">
                      <h2 className="text-sm font-semibold text-gray-900">
                        {group.label}
                      </h2>
                      <span className="rounded-full bg-gray-100 px-2 py-0.5 text-xs font-medium text-gray-600">
                        {group.items.length}
                      </span>
                      <div className="h-px min-w-8 flex-1 bg-gray-200" />
                    </div>
                    <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
                      {group.items.map(renderStudentCard)}
                    </div>
                  </section>
                ))}
              </div>
            );
          }

          return (
            <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
              {sortedStudents.map(renderStudentCard)}
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
