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
import { useSearchParams } from "next/navigation";
import { useTenantRouter } from "~/lib/tenant-router";
import { RoleGuard } from "~/components/auth/role-guard";
import { OpenCareModeGuard } from "~/components/tenant/open-care-mode-guard";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { PageHeaderWithSearch } from "~/components/ui/page-header/PageHeaderWithSearch";
import type {
  FilterConfig,
  ActiveFilter,
} from "~/components/ui/page-header/types";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import { getPresenceCardGradient, isHomeLocation } from "~/lib/location-helper";
import {
  countCheckedInStudents,
  formatGroupLabelWithAttendance,
  isStudentInGroupRoom,
  matchesSearchFilter,
  matchesAttendanceFilter,
} from "./ogs-group-helpers";
import { useMinuteClock } from "~/lib/pickup-helpers";
import type { OGSGroup } from "./ogs-group-helpers";
import { SSEErrorBoundary } from "~/components/sse/SSEErrorBoundary";
import { GroupTransferModal } from "~/components/groups/group-transfer-modal";
import { groupTransferService } from "~/lib/group-transfer-api";
import type { StaffWithRole, GroupTransfer } from "~/lib/group-transfer-api";
import { useToast } from "~/contexts/ToastContext";
import { useSWRAuth } from "~/lib/swr";
import { useUserContext } from "~/lib/hooks/use-user-context";
import { useGroupAttendanceCounts } from "~/lib/group-attendance-count-context";

import { StudentPresenceBadge } from "@/components/ui/student-presence-badge";
import { EmptyStudentResults } from "~/components/ui/empty-student-results";
import {
  StudentCard,
  PickupTimeRow,
  ArrivalTimeRow,
  StudentAbsenceRow,
  StudentPendingExcusedRow,
} from "~/components/students/student-card";
import { StudentCardGridSkeleton } from "~/components/students/student-card-skeleton";
import { SchoolCheckinFab } from "~/components/students/school-checkin-fab";
import { SchoolCheckinModeMobile } from "~/components/students/school-checkin-mode-mobile";
import {
  deriveCheckinState,
  useSchoolCheckinMode,
} from "~/lib/hooks/use-school-checkin-mode";
import { buildGroupOverflowItems } from "./components/group-overflow-items";
import { usePresenceMode } from "~/lib/tenant-context";
import { useStudentPhotosEnabled } from "~/lib/hooks/use-student-photos-enabled";
import { fetchBulkPickupTimes } from "~/lib/pickup-schedule-api";
import type { BulkPickupTime } from "~/lib/pickup-schedule-api";
import { activeService } from "~/lib/active-api";
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import { TrackingIndicators } from "~/components/students/tracking-indicators";
import {
  combineTimeNotes,
  getStudentAbsence,
  getStudentTimeStatus,
  getTimeStatusSortRank,
} from "~/lib/student-time-status";
import {
  getDayPlanningNotComingLabel,
  getStudentPresenceBadgePlanning,
} from "~/lib/day-planning-helper";

import { createLogger } from "~/lib/logger";
import { OgsGroupsPageSkeleton } from "./page-skeleton";

const logger = createLogger({ component: "OgsGroupsPage" });

// Backend pickup time response (from BFF)
interface BackendPickupTime {
  student_id: number;
  date: string;
  weekday_name: string;
  pickup_time?: string;
  is_exception: boolean;
  day_notes?: Array<{ id: number; content: string }>;
  notes?: string;
}

// Backend student response (raw from Go backend via BFF)
// Note: Backend uses "last_name", frontend uses "second_name"
interface BackendStudentFromBFF {
  id: number;
  first_name: string;
  last_name: string; // Backend field name
  name?: string;
  school_class?: string;
  current_location?: string;
  current_room_color?: string | null;
  sick?: boolean;
  sick_since?: string;
  excused?: boolean;
  excused_since?: string;
  class_trip?: boolean;
  class_trip_since?: string;
  location_since?: string;
  group_id?: number;
  group_name?: string;
  arrival_time?: string;
  arrival_is_exception?: boolean;
  arrival_notes?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  // Parent's note for a still-pending "entschuldigt" request covering today.
  // Informational only — the child stays "expected".
  pending_excused_note?: string;
  actual_arrival_time?: string;
  actual_pickup_time?: string;
  // Authenticated photo URL (already rewritten by the backend response
  // mapper from the raw /uploads path to /api/students/{id}/photo/...).
  photo_url?: string;
}

// BFF response type for dashboard data
interface OGSDashboardBFFResponse {
  groups: Array<{
    id: number;
    name: string;
    room_id?: number;
    room?: { id: number; name: string };
    via_substitution?: boolean;
  }>;
  students: BackendStudentFromBFF[]; // Raw backend format
  roomStatus: {
    group_has_room: boolean;
    group_room_id?: number;
    student_room_status: Record<
      string,
      {
        in_group_room: boolean;
        current_room_id?: number;
        first_name?: string;
        last_name?: string;
        reason?: string;
      }
    >;
  } | null;
  substitutions: Array<{
    id: number;
    group_id: number;
    regular_staff_id: number | null;
    substitute_staff_id: number;
    substitute_staff?: {
      person?: { first_name: string; last_name: string };
    };
    start_date: string;
    end_date: string;
  }>;
  pickupTimes: BackendPickupTime[];
  firstGroupId: string | null;
}

function mapStudentForOgsPage(
  student: Student | BackendStudentFromBFF,
): Student {
  if ("last_name" in student) {
    return {
      id: student.id.toString(),
      name: `${student.first_name} ${student.last_name}`.trim(),
      first_name: student.first_name,
      second_name: student.last_name,
      school_class: student.school_class ?? "",
      current_location: student.current_location ?? "",
      current_room_color: student.current_room_color ?? null,
      sick: student.sick ?? false,
      sick_since: student.sick_since,
      excused: student.excused ?? false,
      excused_since: student.excused_since,
      class_trip: student.class_trip ?? false,
      class_trip_since: student.class_trip_since,
      location_since: student.location_since,
      group_id: student.group_id?.toString(),
      group_name: student.group_name,
      arrival_time: student.arrival_time,
      arrival_is_exception: student.arrival_is_exception,
      arrival_notes: student.arrival_notes,
      day_planning_status: student.day_planning_status,
      day_planning_reason: student.day_planning_reason,
      day_planning_label: student.day_planning_label,
      pending_excused_note: student.pending_excused_note,
      actual_arrival_time: student.actual_arrival_time,
      actual_pickup_time: student.actual_pickup_time,
      // Photo URL is forwarded as-is. Backend has already rewritten it
      // to the authenticated /api/students/{id}/photo/{filename} proxy.
      photo_url: student.photo_url,
    };
  }

  return student;
}

function GroupAbsenceOverview({
  totalStudents,
  sickCount,
  excusedCount,
}: Readonly<{
  totalStudents: number;
  sickCount: number;
  excusedCount: number;
}>) {
  return (
    <section
      className="mb-3 flex flex-wrap items-center gap-2 text-sm"
      aria-label="Abwesenheiten heute"
    >
      <span className="border-moto-red/20 bg-moto-red/10 rounded-full border px-3 py-1 font-medium text-gray-900">
        {sickCount}/{totalStudents} krank
      </span>
      <span className="border-moto-purple/20 bg-moto-purple/10 rounded-full border px-3 py-1 font-medium text-gray-900">
        {excusedCount}/{totalStudents} entschuldigt
      </span>
    </section>
  );
}

function OGSGroupPageContent() {
  const router = useTenantRouter();
  const searchParams = useSearchParams();
  const { data: session } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  const { success: showSuccessToast } = useToast();
  const { userContext } = useUserContext();
  const { setGroupAttendanceCount } = useGroupAttendanceCounts();

  // Only binary-mode tenants expose the web check-in toggle; in detailed
  // mode the kiosk owns check-in/out and a parallel web button would
  // confuse users. We gate both the header toggle and the card's
  // check-in mode on this flag — when false the page behaves exactly as
  // it did before this feature landed.
  const presenceMode = usePresenceMode();
  const isBinaryMode = presenceMode === "binary";

  // Photo feature flag — only the OGS-groups view has compact StudentCards
  // (no Klasse / Gruppe rows in extraContent because the user is already
  // inside a specific group). When the photo feature is on AND tracking
  // indicators are configured, the right column (locationBadge +
  // indicators) becomes taller than the left column (name + lastname +
  // arrival + pickup), so the absolute Avatar at bottom-3 right-3 in
  // StudentCard would visually overlap the indicator stack. We patch this
  // ONLY here — Kindersuche and Aktuelle Aufsicht render Klasse + Gruppe
  // rows that already provide enough natural left-column height. The patch
  // is a hidden in-flow spacer added to extraContent below; StudentCard
  // itself stays untouched.
  const { enabled: photosEnabled } = useStudentPhotosEnabled();

  // Page-level school check-in/out mode. Toggle lives in the header; when
  // isActive, clicking a card toggles that student's attendance instead of
  // navigating to the detail page.
  const schoolCheckin = useSchoolCheckinMode();

  // Check if user has access to OGS groups
  const [hasAccess, setHasAccess] = useState<boolean | null>(null);

  // State variables for multiple groups
  const [allGroups, setAllGroups] = useState<OGSGroup[]>([]);
  const [selectedGroupId, setSelectedGroupId] = useState<string | null>(null);

  // Pre-select group from URL param (?group=<id>)
  const groupParam = searchParams.get("group");
  const [students, setStudents] = useState<Student[]>([]);
  const [searchTerm, setSearchTerm] = useState("");
  const [attendanceFilter, setAttendanceFilter] = useState("all");
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [roomStatus, setRoomStatus] = useState<
    Record<
      string,
      {
        in_group_room: boolean;
        current_room_id?: number;
        first_name?: string;
        last_name?: string;
        reason?: string;
      }
    >
  >({});

  // State for mobile/desktop detection
  const [isMobile, setIsMobile] = useState(false);
  const [isDesktop, setIsDesktop] = useState(false);

  // State for pickup times (bulk fetched for all students)
  const [pickupTimes, setPickupTimes] = useState<Map<string, BulkPickupTime>>(
    new Map(),
  );

  // Sort mode for student list
  const [sortMode, setSortMode] = useState<"default" | "pickup" | "arrival">(
    "default",
  );

  // Current time for urgency calculation (updates every minute)
  const now = useMinuteClock();

  // State for group transfer modal
  const [showTransferModal, setShowTransferModal] = useState(false);
  const [availableUsers, setAvailableUsers] = useState<StaffWithRole[]>([]);
  const [activeTransfers, setActiveTransfers] = useState<GroupTransfer[]>([]);

  // SWR-based dashboard data fetching with caching
  // Cache key "ogs-dashboard" will be invalidated by global SSE on relevant events
  const {
    data: dashboardData,
    isLoading: isDashboardLoading,
    error: dashboardError,
  } = useSWRAuth<OGSDashboardBFFResponse>(
    session?.user?.token ? "ogs-dashboard" : null,
    async () => {
      logger.debug("SWR fetching dashboard via BFF");
      const start = performance.now();

      const response = await fetch("/api/ogs-dashboard", {
        credentials: "include",
        headers: {
          Authorization: `Bearer ${session?.user?.token}`,
          "Content-Type": "application/json",
        },
      });

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const json = (await response.json()) as {
        success: boolean;
        data: OGSDashboardBFFResponse;
      };

      logger.debug("SWR fetch complete", {
        duration_ms: (performance.now() - start).toFixed(0),
      });
      return json.data;
    },
    {
      keepPreviousData: true, // Show cached data while revalidating
      revalidateOnFocus: false, // Handled by global SSE
    },
  );

  // Sync SWR dashboard data with local state
  useEffect(() => {
    if (!dashboardData) return;

    const {
      groups,
      students: studentsData,
      roomStatus: rs,
      substitutions,
      pickupTimes: pickupTimesData,
    } = dashboardData;

    if (groups.length === 0) {
      setHasAccess(false);
      setIsLoading(false);
      return;
    }

    setHasAccess(true);

    // Convert groups to OGSGroup format, sorted alphabetically by name
    const ogsGroups: OGSGroup[] = groups
      .map((group) => ({
        id: group.id.toString(),
        name: group.name,
        room_name: group.room?.name,
        room_id: group.room_id?.toString(),
        student_count: undefined,
        supervisor_name: undefined,
        viaSubstitution: group.via_substitution,
      }))
      .sort((a, b) => a.name.localeCompare(b.name, "de"));

    // Update student count on the first sorted group (BFF pre-loads data for it)
    if (ogsGroups[0]) {
      ogsGroups[0].student_count = studentsData.length;
      ogsGroups[0].present_count = countCheckedInStudents(studentsData);
      setGroupAttendanceCount(ogsGroups[0].id, {
        present: ogsGroups[0].present_count,
        total: ogsGroups[0].student_count,
      });
    }

    setAllGroups(ogsGroups);

    // IMPORTANT: Only apply first group's students/roomStatus when first group is selected.
    // The BFF sorts groups alphabetically, so groups[0] matches ogsGroups[0].
    // When SSE triggers revalidation while user views another group, we must NOT
    // overwrite their current view with the first group's data.
    const firstGroupId = ogsGroups[0]?.id;

    // If the previously selected group no longer exists in the refreshed list
    // (e.g., access revoked, group removed), reset to the first group so
    // the student data stays in sync with what the UI displays.
    if (selectedGroupId && !ogsGroups.some((g) => g.id === selectedGroupId)) {
      setSelectedGroupId(firstGroupId ?? null);
    }

    if (!selectedGroupId || selectedGroupId === firstGroupId) {
      // When no group is explicitly selected yet, lock in the first group's ID
      // so the URL-sync effect won't try to "switch" to it via localStorage.
      if (!selectedGroupId && firstGroupId) {
        setSelectedGroupId(firstGroupId);
      }

      // Map backend students to frontend format (last_name → second_name)
      const mappedStudents = studentsData.map(mapStudentForOgsPage);
      setStudents(mappedStudents);

      // Set pickup times from BFF response (prevents loading flash)
      // Convert backend format to Map for O(1) lookup
      const pickupMap = new Map<string, BulkPickupTime>();
      for (const pt of pickupTimesData ?? []) {
        pickupMap.set(pt.student_id.toString(), {
          studentId: pt.student_id.toString(),
          date: pt.date,
          weekdayName: pt.weekday_name,
          pickupTime: pt.pickup_time,
          isException: pt.is_exception,
          dayNotes: (pt.day_notes ?? []).map((n) => ({
            id: n.id.toString(),
            content: n.content,
          })),
          notes: pt.notes,
        });
      }
      setPickupTimes(pickupMap);

      if (rs?.student_room_status) {
        setRoomStatus(rs.student_room_status);
      }
    }

    // Convert substitutions to GroupTransfer format
    const transfers = substitutions
      .filter((sub) => !sub.regular_staff_id)
      .map((transfer) => {
        const targetName = transfer.substitute_staff?.person
          ? `${transfer.substitute_staff.person.first_name} ${transfer.substitute_staff.person.last_name}`
          : "Unbekannt";
        return {
          substitutionId: transfer.id.toString(),
          groupId: transfer.group_id.toString(),
          targetStaffId: transfer.substitute_staff_id.toString(),
          targetName,
          validUntil: transfer.end_date,
        };
      });
    setActiveTransfers(transfers);
    setError(null);
    setIsLoading(false);
  }, [dashboardData, selectedGroupId, setGroupAttendanceCount]);

  // Sync selected group with URL param.
  // The sidebar navigates with the correct ?group= param at click-time,
  // so this effect only needs to react to URL changes.
  // When no param is present (e.g. fresh login), persist the default (first group)
  // so localStorage stays in sync and the sidebar picks it up on next click.
  useEffect(() => {
    if (allGroups.length === 0) return;

    if (groupParam) {
      if (
        groupParam !== selectedGroupId &&
        allGroups.some((g) => g.id === groupParam)
      ) {
        void switchToGroup(groupParam);
      }
    } else {
      // No ?group= param (e.g. after login or browser back) — restore from
      // localStorage so the user returns to their previously selected group.
      const savedGroupId = localStorage.getItem("sidebar-last-group");
      const savedGroup = savedGroupId
        ? allGroups.find((g) => g.id === savedGroupId)
        : undefined;
      if (savedGroup && savedGroup.id !== selectedGroupId) {
        void switchToGroup(savedGroup.id);
      } else if (!savedGroup) {
        // Nothing saved or saved group no longer exists — persist first group
        const firstGroup = allGroups[0];
        if (firstGroup) {
          localStorage.setItem("sidebar-last-group", firstGroup.id);
        }
      }
      // When savedGroup.id === selectedGroupId, do nothing — already in sync
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [allGroups, groupParam]);

  // Handle dashboard error
  useEffect(() => {
    if (dashboardError) {
      if (dashboardError.message.includes("403")) {
        setError(
          "Sie haben keine Berechtigung für den Zugriff auf OGS-Gruppendaten.",
        );
        setHasAccess(false);
      } else {
        setError("Fehler beim Laden der OGS-Gruppendaten.");
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

  // Get current selected group — derived from ID, stable across re-sorts
  const currentGroup = useMemo(
    () =>
      allGroups.find((g) => g.id === selectedGroupId) ?? allGroups[0] ?? null,
    [allGroups, selectedGroupId],
  );
  const currentGroupId = currentGroup?.id;

  // Set breadcrumb data
  useSetBreadcrumb({
    ogsGroupName: currentGroup?.name,
    pageTitle: "Meine Gruppe",
  });

  // SWR-based student data subscription for real-time updates.
  // When global SSE invalidates "student*" caches, this triggers a refetch.
  // Only fetches when hasAccess is confirmed and we have a group ID.
  // Includes room status and pickup times to prevent "loading flash" on student cards.
  const { data: swrStudentsData } = useSWRAuth<{
    groupId: string;
    students: Student[];
    roomStatus?: Record<
      string,
      {
        in_group_room: boolean;
        current_room_id?: number;
        first_name?: string;
        last_name?: string;
        reason?: string;
      }
    >;
    pickupTimes?: Map<string, BulkPickupTime>;
    trackingIndicators?: TrackingIndicatorsResponse;
  }>(
    hasAccess && currentGroupId ? `ogs-students-${currentGroupId}` : null,
    async () => {
      // Fetch students and room status in parallel for accurate filtering
      const [studentsResponse, roomStatusResponse] = await Promise.all([
        studentService.getStudents({
          groupId: currentGroupId!,
          includeArrivalTimes: true,
          token: session?.user?.token,
        }),
        // Fetch room status inline (don't use callback that sets state)
        fetch(`/api/groups/${currentGroupId}/students/room-status`, {
          headers: {
            Authorization: `Bearer ${session?.user?.token}`,
            "Content-Type": "application/json",
          },
        })
          .then(async (res) => {
            if (!res.ok) return null;
            const data = (await res.json()) as {
              data?: {
                student_room_status?: Record<
                  string,
                  {
                    in_group_room: boolean;
                    current_room_id?: number;
                    first_name?: string;
                    last_name?: string;
                    reason?: string;
                  }
                >;
              };
            };
            return data.data?.student_room_status ?? null;
          })
          .catch(() => null),
      ]);

      const students = studentsResponse.students || [];

      // Fetch pickup times and tracking indicators in parallel (prevents loading flash).
      // Arrival data is part of the student list response, so ogs-students-* invalidation is enough.
      let pickupTimesMap = new Map<string, BulkPickupTime>();
      let trackingIndicators: TrackingIndicatorsResponse = {
        labels: [],
        results: {},
      };
      if (students.length > 0) {
        const studentIds = students.map((s) => s.id.toString());
        const [pickupResult, trackingResult] = await Promise.all([
          fetchBulkPickupTimes(studentIds).catch(() => {
            logger.error("failed to fetch pickup times in SWR");
            return new Map<string, BulkPickupTime>();
          }),
          activeService.getTrackingIndicators(studentIds).catch(() => {
            return { labels: [], results: {} } as TrackingIndicatorsResponse;
          }),
        ]);
        pickupTimesMap = pickupResult;
        trackingIndicators = trackingResult;
      }

      return {
        groupId: currentGroupId!,
        students,
        roomStatus: roomStatusResponse ?? undefined,
        pickupTimes: pickupTimesMap,
        trackingIndicators,
      };
    },
    {
      keepPreviousData: true, // Prevent loading flash during refetch
      revalidateOnFocus: false, // Handled by global SSE
    },
  );

  // Sync SWR student data with local state
  // Also syncs room status and pickup times to keep UI in sync and prevent loading flash
  useEffect(() => {
    if (swrStudentsData?.groupId !== currentGroupId) {
      return;
    }

    if (swrStudentsData?.students) {
      const mappedStudents = swrStudentsData.students.map(mapStudentForOgsPage);
      setStudents(mappedStudents);
      if (currentGroupId) {
        const presentCount = countCheckedInStudents(mappedStudents);
        setGroupAttendanceCount(currentGroupId, {
          present: presentCount,
          total: mappedStudents.length,
        });
        setAllGroups((prev) =>
          prev.map((group) =>
            group.id === currentGroupId
              ? {
                  ...group,
                  student_count: mappedStudents.length,
                  present_count: presentCount,
                }
              : group,
          ),
        );
      }
    }
    if (swrStudentsData?.roomStatus) {
      setRoomStatus(swrStudentsData.roomStatus);
    }
    // Only set pickupTimes if it's a Map (the SWR fetcher returns a Map,
    // but test mocks may return the wrong type)
    if (swrStudentsData?.pickupTimes instanceof Map) {
      setPickupTimes(swrStudentsData.pickupTimes);
    }
  }, [swrStudentsData, currentGroupId, setGroupAttendanceCount]);

  // Ref to track current group without triggering unnecessary re-renders
  const currentGroupRef = useRef<OGSGroup | null>(null);
  useEffect(() => {
    currentGroupRef.current = currentGroup;
  }, [currentGroup]);

  // Ref to track current session token without triggering re-renders
  const sessionTokenRef = useRef(session?.user?.token);
  useEffect(() => {
    sessionTokenRef.current = session?.user?.token;
  }, [session?.user?.token]);

  // Load available users for transfer dropdown
  // Query "teacher", "staff", and "user" roles to cover all deployment configurations
  // Most production accounts use the "user" role (Nutzer)
  const loadAvailableUsers = useCallback(async () => {
    try {
      const users = await groupTransferService.getAllAvailableStaff();
      setAvailableUsers(users);
    } catch (error) {
      logger.error("failed to load available users", {
        error: error instanceof Error ? error.message : String(error),
      });
      setAvailableUsers([]);
    }
  }, []);

  // Check if current group has active transfers
  // Pass token to skip redundant getSession() call (saves ~600ms)
  const checkActiveTransfers = useCallback(
    async (groupId: string, token?: string) => {
      try {
        const transfers = await groupTransferService.getActiveTransfersForGroup(
          groupId,
          token,
        );
        setActiveTransfers(transfers);
      } catch (error) {
        logger.error("failed to check active transfers", {
          error: error instanceof Error ? error.message : String(error),
        });
        setActiveTransfers([]);
      }
    },
    [],
  );

  // Load users when modal opens
  // IMPORTANT: Use currentGroupId as dependency, not currentGroup object
  // Otherwise setAllGroups() creates new object references and triggers this effect again
  useEffect(() => {
    if (showTransferModal && currentGroupId) {
      loadAvailableUsers().catch((err: unknown) =>
        logger.error("failed to load available users", {
          error: err instanceof Error ? err.message : String(err),
        }),
      );
      checkActiveTransfers(currentGroupId).catch((err: unknown) =>
        logger.error("failed to check active transfers", {
          error: err instanceof Error ? err.message : String(err),
        }),
      );
    }
  }, [
    showTransferModal,
    currentGroupId,
    loadAvailableUsers,
    checkActiveTransfers,
  ]);

  // Handle group transfer
  const handleTransferGroup = async (
    targetPersonId: string,
    targetName: string,
  ) => {
    if (!currentGroup) return;

    await groupTransferService.transferGroup(currentGroup.id, targetPersonId);

    // Reload transfers for this group to show updated list
    await checkActiveTransfers(currentGroup.id);

    // NOTE: We intentionally do NOT reload groups here.
    // A transfer only creates a substitution record - it doesn't change group data.
    // Reloading groups could return them in a different order, causing the selection
    // to point to a different group and making the modal switch unexpectedly.

    // Show success toast
    showSuccessToast(
      `Gruppe "${currentGroup.name}" an ${targetName} übergeben`,
    );

    // Keep modal open to allow multiple transfers and show updated transfer list
  };

  // Handle cancel specific transfer by ID
  const handleCancelTransfer = async (substitutionId: string) => {
    if (!currentGroup) return;

    // Find the transfer to get recipient name
    const transfer = activeTransfers.find(
      (t) => t.substitutionId === substitutionId,
    );
    const recipientName = transfer?.targetName ?? "Betreuer";

    // Use the secure ownership-checked endpoint instead of direct substitution deletion
    // This ensures only the original group leader can cancel transfers
    await groupTransferService.cancelTransferBySubstitutionId(
      currentGroup.id,
      substitutionId,
    );

    // Reload transfers for this group
    await checkActiveTransfers(currentGroup.id);

    // NOTE: We intentionally do NOT reload groups here.
    // Canceling a transfer only deletes a substitution record - it doesn't change group data.
    // Reloading groups could return them in a different order, causing the selection
    // to point to a different group and making the modal switch unexpectedly.

    // Show success toast
    showSuccessToast(`Übergabe an ${recipientName} wurde zurückgenommen`);
  };

  // Helper function to load room status for current group
  const loadGroupRoomStatus = useCallback(
    async (groupId: string) => {
      try {
        const roomStatusResponse = await fetch(
          `/api/groups/${groupId}/students/room-status`,
          {
            headers: {
              Authorization: `Bearer ${sessionTokenRef.current}`,
              "Content-Type": "application/json",
            },
          },
        );

        if (roomStatusResponse.ok) {
          const response = (await roomStatusResponse.json()) as {
            success: boolean;
            message: string;
            data: {
              group_has_room: boolean;
              group_room_id?: number;
              student_room_status: Record<
                string,
                {
                  in_group_room: boolean;
                  current_room_id?: number;
                  first_name?: string;
                  last_name?: string;
                  reason?: string;
                }
              >;
            };
          };

          if (response.data?.student_room_status) {
            setRoomStatus(response.data.student_room_status);
          }
        }
      } catch (error) {
        logger.error("failed to load group room status", {
          error: error instanceof Error ? error.message : String(error),
        });
      }
    },
    [], // No dependencies - function is stable
  );

  // SSE is handled globally by TenantAuthWrapper - no page-level setup needed.
  // When student_checkin/checkout events occur, global SSE invalidates "student*" caches,
  // which triggers SWR refetch for ogs-students-* keys automatically.

  // Handle mobile detection
  useEffect(() => {
    const checkMobile = () => {
      setIsMobile(window.innerWidth < 768);
      setIsDesktop(window.innerWidth >= 1024);
    };
    checkMobile();
    window.addEventListener("resize", checkMobile);
    return () => window.removeEventListener("resize", checkMobile);
  }, []);

  // Function to switch between groups (by ID — stable across re-sorts)
  const switchToGroup = async (groupId: string) => {
    if (groupId === selectedGroupId) return;
    const selectedGroup = allGroups.find((g) => g.id === groupId);
    if (!selectedGroup) return;

    setIsLoading(true);
    setSelectedGroupId(groupId);
    setStudents([]); // Clear current students
    setRoomStatus({}); // Clear room status

    try {
      // Fetch students for the selected group
      // Pass token to skip redundant getSession() call (~600ms savings)
      const studentsResponse = await studentService.getStudents({
        groupId: selectedGroup.id,
        includeArrivalTimes: true,
        token: session?.user?.token,
      });
      const studentsData = studentsResponse.students || [];
      const presentCount = countCheckedInStudents(studentsData);

      setStudents(studentsData);
      setGroupAttendanceCount(groupId, {
        present: presentCount,
        total: studentsData.length,
      });

      // Update group with actual student count
      setAllGroups((prev) =>
        prev.map((group) =>
          group.id === groupId
            ? {
                ...group,
                student_count: studentsData.length,
                present_count: presentCount,
              }
            : group,
        ),
      );

      // Fetch room status and active transfers in parallel
      // Pass token to skip redundant getSession() call
      await Promise.all([
        loadGroupRoomStatus(selectedGroup.id),
        checkActiveTransfers(selectedGroup.id, session?.user?.token),
      ]);

      setError(null);
    } catch {
      setError("Fehler beim Laden der Gruppendaten.");
    } finally {
      setIsLoading(false);
    }
  };

  // Apply filters to students (ensure students is an array)
  const filteredStudents = (Array.isArray(students) ? students : []).filter(
    (student) =>
      matchesSearchFilter(student, searchTerm) &&
      matchesAttendanceFilter(student, attendanceFilter, roomStatus),
  );

  // Sort students based on selected sort mode
  const sortedStudents = useMemo(() => {
    const sorted = [...filteredStudents];

    const compareByName = (a: Student, b: Student) => {
      const lastCmp = (a.second_name ?? "").localeCompare(
        b.second_name ?? "",
        "de",
      );
      if (lastCmp !== 0) return lastCmp;
      return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
    };

    if (sortMode === "pickup") {
      return sorted.sort((a, b) => {
        const aHome = isHomeLocation(a.current_location);
        const bHome = isHomeLocation(b.current_location);

        // Zuhause immer ganz unten
        if (aHome && !bHome) return 1;
        if (!aHome && bHome) return -1;
        if (aHome && bHome) return compareByName(a, b);

        // Beide anwesend: nach Urgency-Gruppe sortieren
        const timeA = pickupTimes.get(a.id.toString())?.pickupTime;
        const timeB = pickupTimes.get(b.id.toString())?.pickupTime;
        const statusA = getStudentTimeStatus({
          plannedTime: timeA,
          actualTime: a.actual_pickup_time,
          now,
          sick: a.sick,
          classTrip: a.class_trip,
          excused: a.excused,
        });
        const statusB = getStudentTimeStatus({
          plannedTime: timeB,
          actualTime: b.actual_pickup_time,
          now,
          sick: b.sick,
          classTrip: b.class_trip,
          excused: b.excused,
        });
        const rankA = getTimeStatusSortRank(statusA);
        const rankB = getTimeStatusSortRank(statusB);

        // Verschiedene Urgency-Gruppen: überzogen zuerst
        if (rankA !== rankB) return rankA - rankB;

        // Gleiche Urgency-Gruppe: nach Abholzeit, dann Name
        if (timeA && timeB) {
          const timeCmp = timeA.localeCompare(timeB);
          if (timeCmp !== 0) return timeCmp;
        }

        return compareByName(a, b);
      });
    }

    if (sortMode === "arrival") {
      return sorted.sort((a, b) => {
        const aHome = isHomeLocation(a.current_location);
        const bHome = isHomeLocation(b.current_location);

        // Angekommene Kinder (nicht zu Hause) immer unten
        if (!aHome && bHome) return 1;
        if (aHome && !bHome) return -1;

        // Beide zu Hause: nach Ankunfts-Urgency sortieren
        const timeA = a.arrival_time;
        const timeB = b.arrival_time;
        const statusA = getStudentTimeStatus({
          plannedTime: timeA,
          actualTime: a.actual_arrival_time,
          now,
          sick: a.sick,
          classTrip: a.class_trip,
          excused: a.excused,
        });
        const statusB = getStudentTimeStatus({
          plannedTime: timeB,
          actualTime: b.actual_arrival_time,
          now,
          sick: b.sick,
          classTrip: b.class_trip,
          excused: b.excused,
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
    }

    // Alphabetisch (Standard): Nachname, dann Vorname
    return sorted.sort(compareByName);
  }, [filteredStudents, sortMode, pickupTimes, now]);

  const groupAbsenceOverview = useMemo(() => {
    const groupStudents = Array.isArray(students) ? students : [];
    const sickCount = groupStudents.filter((student) => student.sick).length;
    const excusedCount = groupStudents.filter(
      (student) => !student.sick && student.excused,
    ).length;

    return {
      totalStudents: groupStudents.length,
      sickCount,
      excusedCount,
    };
  }, [students]);

  const getCardGradient = useCallback(
    (student: Student) =>
      getPresenceCardGradient(
        student.current_location,
        isStudentInGroupRoom(student, currentGroup),
      ),
    [currentGroup],
  );

  // Prepare filter configurations for PageHeaderWithSearch
  const filterConfigs: FilterConfig[] = useMemo(
    () => [
      {
        id: "sort",
        label: "Sortierung",
        type: "buttons",
        value: sortMode,
        onChange: (value) =>
          setSortMode(value as "default" | "pickup" | "arrival"),
        options: [
          { value: "default", label: "Alphabetisch" },
          { value: "arrival", label: "Nächste Ankunft" },
          { value: "pickup", label: "Nächste Abholung" },
        ],
      },
      {
        id: "location",
        label: "Aufenthaltsort",
        type: "grid",
        value: attendanceFilter,
        onChange: (value) => setAttendanceFilter(value as string),
        options: [
          { value: "all", label: "Alle Orte", icon: "M4 6h16M4 12h16M4 18h16" },
          {
            value: "in_room",
            label: "Gruppenraum",
            icon: "M19 21V5a2 2 0 00-2-2H7a2 2 0 00-2 2v16m14 0h2m-2 0h-5m-9 0H3m2 0h5M9 7h1m-1 4h1m4-4h1m-1 4h1m-5 10v-5a1 1 0 011-1h2a1 1 0 011 1v5m-4 0h4",
          },
          {
            value: "foreign_room",
            label: "Fremder Raum",
            icon: "M8 14v3m4-3v3m4-3v3M3 21h18M3 10h18M3 7l9-4 9 4M4 10h16v11H4V10z",
          },
          {
            value: "transit",
            label: "Unterwegs",
            icon: "M13 10V3L4 14h7v7l9-11h-7z",
          },
          {
            value: "schoolyard",
            label: "Schulhof",
            icon: "M21 12a9 9 0 11-18 0 9 9 0 0118 0zM12 12a8 8 0 008 4M7.5 13.5a12 12 0 008.5 6.5M12 12a8 8 0 00-7.464 4.928M12.951 7.353a12 12 0 00-9.88 4.111M12 12a8 8 0 00-.536-8.928M15.549 15.147a12 12 0 001.38-10.611",
          },
          {
            value: "at_home",
            label: "Zuhause",
            icon: "M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6",
          },
        ],
      },
    ],
    [sortMode, attendanceFilter],
  );

  // Prepare active filters for display
  const activeFilters: ActiveFilter[] = useMemo(() => {
    const filters: ActiveFilter[] = [];

    if (sortMode !== "default") {
      filters.push({
        id: "sort",
        label: "Sortiert: Nächste Abholung",
        onRemove: () => setSortMode("default"),
      });
    }

    if (searchTerm) {
      filters.push({
        id: "search",
        label: `"${searchTerm}"`,
        onRemove: () => setSearchTerm(""),
      });
    }

    if (attendanceFilter !== "all") {
      const locationLabels: Record<string, string> = {
        in_room: "Gruppenraum",
        foreign_room: "Fremder Raum",
        transit: "Unterwegs",
        schoolyard: "Schulhof",
        at_home: "Zuhause",
      };
      filters.push({
        id: "location",
        label: locationLabels[attendanceFilter] ?? attendanceFilter,
        onRemove: () => setAttendanceFilter("all"),
      });
    }

    return filters;
  }, [sortMode, searchTerm, attendanceFilter]);

  if (isLoading || hasAccess === null) {
    return <OgsGroupsPageSkeleton />;
  }

  // If user doesn't have access, show empty state
  if (!hasAccess) {
    return (
      <div className="-mt-1.5 w-full">
        <PageHeaderWithSearch title="Meine Gruppe" />

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
                d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
            <div className="space-y-2">
              <h3 className="text-lg font-medium text-gray-900">
                Keine OGS-Gruppe zugeordnet
              </h3>
              <p className="text-gray-600">
                Du bist keiner OGS-Gruppe als Leiter:in zugeordnet. Wende dich
                an deine Verwaltung, um einer Gruppe zugewiesen zu werden.
              </p>
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Render helper for the substitution status pill. "In Vertretung" is a
  // passive informational badge — it just signals that the current user is
  // covering for someone else. Active actions ("Gruppe übergeben") moved into
  // the kebab-menu, and the check-in mode lives in the SchoolCheckinFab.
  const renderSubstitutionBadge = (variant: "desktop" | "mobile") => {
    if (!currentGroup?.viaSubstitution) return undefined;

    if (variant === "desktop") {
      return (
        <div className="flex h-10 items-center gap-2 px-4">
          <MotoConceptIcon concept="substitution" size={18} />
          <span className="text-sm font-medium text-gray-900">
            In Vertretung
          </span>
        </div>
      );
    }

    return (
      <div
        className="flex h-8 w-8 items-center justify-center"
        title="In Vertretung"
      >
        <MotoConceptIcon concept="substitution" size={18} />
      </div>
    );
  };

  // Build the kebab-menu items once per render. Skip when there's no current
  // group (loading state) so the menu doesn't appear with stale handlers.
  const overflowItems = currentGroup
    ? buildGroupOverflowItems({
        viaSubstitution: currentGroup.viaSubstitution ?? false,
        activeTransfersCount: activeTransfers.length,
        onOpenTransfer: () => setShowTransferModal(true),
      })
    : [];

  // Render helper for student grid content
  const renderStudentContent = () => {
    if (isLoading) {
      return <StudentCardGridSkeleton />;
    }
    if (students.length === 0) {
      return (
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
                d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"
              />
            </svg>
            <div className="space-y-1">
              <h3 className="text-lg font-medium text-gray-900">
                Keine Kinder in {currentGroup?.name ?? "dieser Gruppe"}
              </h3>
              <p className="text-sm text-gray-500">
                Es wurden noch keine Kinder zu dieser OGS-Gruppe hinzugefügt.
              </p>
              {allGroups.length > 1 && (
                <p className="mt-1 text-sm text-gray-500">
                  Versuchen Sie eine andere Gruppe auszuwählen.
                </p>
              )}
            </div>
          </div>
        </div>
      );
    }
    if (sortedStudents.length > 0) {
      return (
        <div>
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-3">
            {sortedStudents.map((student) => {
              const inGroupRoom = isStudentInGroupRoom(student, currentGroup);
              const cardGradient = getCardGradient(student);
              const studentPickup = pickupTimes.get(student.id.toString());
              const studentAbsence = getStudentAbsence({
                sick: student.sick,
                classTrip: student.class_trip,
                excused: student.excused,
              });

              const checkinState = deriveCheckinState(student.current_location);
              const studentIdStr = student.id.toString();
              const isGroupCardCheckinMode =
                isBinaryMode && schoolCheckin.isActive;
              return (
                <StudentCard
                  key={student.id}
                  studentId={student.id}
                  firstName={student.first_name}
                  lastName={student.second_name}
                  photoUrl={student.photo_url ?? null}
                  gradient={cardGradient}
                  onClick={() =>
                    router.push(`/students/${student.id}?from=/ogs-groups`)
                  }
                  checkinMode={isGroupCardCheckinMode}
                  checkinState={checkinState}
                  isCheckinPending={schoolCheckin.pendingIds.has(studentIdStr)}
                  onCheckinClick={() =>
                    void schoolCheckin.toggle(studentIdStr, checkinState)
                  }
                  locationBadge={
                    <StudentPresenceBadge
                      student={(() => {
                        const badgePlanning =
                          getStudentPresenceBadgePlanning(student);
                        return {
                          ...student,
                          not_arrival_today: badgePlanning.notArrivalToday,
                          not_arrival_reason: badgePlanning.notArrivalReason,
                        };
                      })()}
                      displayMode="roomName"
                      isGroupRoom={inGroupRoom}
                      variant="modern"
                      size="md"
                    />
                  }
                  extraContent={
                    <>
                      {student.pending_excused_note !== undefined && (
                        <StudentPendingExcusedRow
                          note={student.pending_excused_note}
                        />
                      )}
                      {studentAbsence && !student.actual_pickup_time ? (
                        <StudentAbsenceRow label={studentAbsence.label} />
                      ) : (
                        (() => {
                          const dayPlanningNotComingLabel =
                            getDayPlanningNotComingLabel(student);
                          if (
                            dayPlanningNotComingLabel &&
                            !student.actual_pickup_time
                          ) {
                            return (
                              <StudentAbsenceRow
                                label={dayPlanningNotComingLabel}
                              />
                            );
                          }
                          return (
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
                                pickupTime={studentPickup?.pickupTime}
                                actualTime={student.actual_pickup_time}
                                isException={
                                  studentPickup?.isException ?? false
                                }
                                notes={
                                  studentPickup
                                    ? combineTimeNotes(
                                        studentPickup.notes,
                                        studentPickup.dayNotes,
                                      )
                                    : undefined
                                }
                                now={now}
                              />
                            </>
                          );
                        })()
                      )}
                      {/* Check-in-only avatar-clearance spacer — in navigation
                          mode this surface now opts into an in-flow bottom row
                          (hint + avatar), so the card grows naturally. The
                          spacer remains necessary only for check-in mode,
                          where the hint disappears and the avatar still sits
                          as an absolute overlay. aria-hidden keeps screen
                          readers from announcing the empty box. */}
                      {isGroupCardCheckinMode &&
                      photosEnabled &&
                      (swrStudentsData?.trackingIndicators?.labels.length ??
                        0) > 0 ? (
                        <div aria-hidden className="h-9" />
                      ) : null}
                    </>
                  }
                  trackingIndicators={
                    swrStudentsData?.trackingIndicators?.labels.length ? (
                      <TrackingIndicators
                        labels={swrStudentsData.trackingIndicators.labels}
                        results={
                          swrStudentsData.trackingIndicators.results[
                            student.id
                          ] ?? []
                        }
                      />
                    ) : undefined
                  }
                />
              );
            })}
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
    <>
      <div className="w-full">
        {/* Page header — scrolls with the rest of the page (no sticky).
            Active filters surface as a count badge on the filter pill (no
            separate chips row), and the kebab menu carries the rare
            "Gruppe übergeben" action. */}
        <div className="-mx-1 px-1 pb-2 sm:mx-0 sm:px-0">
          <PageHeaderWithSearch
            title={
              isMobile && allGroups.length === 1
                ? currentGroup
                  ? formatGroupLabelWithAttendance(currentGroup)
                  : "Meine Gruppe"
                : "" // No title when multiple groups (tabs show group names) or on desktop
            }
            actionButton={renderSubstitutionBadge("desktop")}
            mobileActionButton={renderSubstitutionBadge("mobile")}
            overflowMenu={overflowItems}
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
            activeFilterDisplay="count"
            tabs={
              allGroups.length > 1 && !isDesktop
                ? {
                    items: allGroups.map((group) => ({
                      id: group.id,
                      label: formatGroupLabelWithAttendance(group),
                    })),
                    activeTab: currentGroup?.id ?? "",
                    onTabChange: (tabId) => {
                      const group = allGroups.find((g) => g.id === tabId);
                      if (group) {
                        localStorage.setItem("sidebar-last-group", tabId);
                        localStorage.setItem(
                          "sidebar-last-group-name",
                          group.name,
                        );
                        void switchToGroup(tabId);
                      }
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
              setAttendanceFilter("all");
              setSortMode("default");
            }}
          />
        </div>

        {/* Mobile Error Display — outside the sticky stack so it doesn't
            push everything down on small screens. */}
        {error && (
          <div className="mb-4 md:hidden">
            <Alert type="error" message={error} />
          </div>
        )}

        {/* Mobile (<md) check-in mode trigger — inline pill at the top of
            the card list when OFF; switches to a sticky bottom bar above
            the mobile nav when ON. Tablet keeps the floating FAB and
            desktop the header inline pill, both rendered below. */}
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

        {currentGroup ? (
          <GroupAbsenceOverview
            totalStudents={groupAbsenceOverview.totalStudents}
            sickCount={groupAbsenceOverview.sickCount}
            excusedCount={groupAbsenceOverview.excusedCount}
          />
        ) : null}

        {/* Student Grid. Bottom padding reserves room for the mobile
            sticky bar / tablet floating FAB so the last row of cards
            stays tappable; desktop has neither and gets pb-0. */}
        <div className={isBinaryMode ? "pb-24 lg:pb-0" : undefined}>
          {renderStudentContent()}
        </div>
      </div>

      {/* Tablet (md..lg) check-in mode trigger — floating FAB. Mobile
          renders the inline pill / sticky bar combo above; desktop
          renders the inline pill inside the page header. */}
      {isBinaryMode && (
        <div className="hidden md:block lg:hidden">
          <SchoolCheckinFab
            variant="floating"
            floatingUntil="lg"
            isActive={schoolCheckin.isActive}
            onToggle={schoolCheckin.toggleActive}
            successCount={schoolCheckin.successCount}
            pendingCount={schoolCheckin.pendingIds.size}
          />
        </div>
      )}

      {/* Group Transfer Modal */}
      <GroupTransferModal
        isOpen={showTransferModal}
        onClose={() => setShowTransferModal(false)}
        group={
          currentGroup
            ? {
                id: currentGroup.id,
                name: currentGroup.name,
                studentCount: currentGroup.student_count,
              }
            : null
        }
        availableUsers={
          userContext?.currentStaff?.personId
            ? availableUsers.filter(
                (u) => u.personId !== userContext.currentStaff!.personId,
              )
            : availableUsers
        }
        onTransfer={handleTransferGroup}
        existingTransfers={activeTransfers}
        onCancelTransfer={handleCancelTransfer}
        onRefreshTransfers={
          currentGroup
            ? async () => checkActiveTransfers(currentGroup.id)
            : undefined
        }
      />
    </>
  );
}

// Main component with Suspense wrapper
export default function OGSGroupPage() {
  return (
    <OpenCareModeGuard>
      <OGSGroupPageGuarded />
    </OpenCareModeGuard>
  );
}

function OGSGroupPageGuarded() {
  return (
    <RoleGuard variant="staffOnly" fallback={<OgsGroupsPageSkeleton />}>
      <Suspense fallback={<OgsGroupsPageSkeleton />}>
        <SSEErrorBoundary>
          <OGSGroupPageContent />
        </SSEErrorBoundary>
      </Suspense>
    </RoleGuard>
  );
}
