"use client";

import {
  useState,
  useEffect,
  Suspense,
  useMemo,
  useCallback,
  useRef,
  type JSX,
} from "react";
import { useSession } from "next-auth/react";
import { useRouter, useSearchParams } from "next/navigation";
import { useSetBreadcrumb } from "~/lib/breadcrumb-context";
import { Alert } from "~/components/ui/alert";
import { PageHeaderWithSearch } from "~/components/ui/page-header";
import type { FilterConfig, ActiveFilter } from "~/components/ui/page-header";
import { studentService } from "~/lib/api";
import type { Student } from "~/lib/api";
import {
  LOCATION_STATUSES,
  isHomeLocation,
  isSchoolyardLocation,
  isTransitLocation,
  parseLocation,
} from "~/lib/location-helper";
import {
  getPickupUrgency,
  isStudentInGroupRoom,
  matchesSearchFilter,
  matchesAttendanceFilter,
} from "./ogs-group-helpers";
import type { OGSGroup, PickupUrgency } from "./ogs-group-helpers";
import { SSEErrorBoundary } from "~/components/sse/SSEErrorBoundary";
import { GroupTransferModal } from "~/components/groups/group-transfer-modal";
import { groupTransferService } from "~/lib/group-transfer-api";
import type { StaffWithRole, GroupTransfer } from "~/lib/group-transfer-api";
import { useToast } from "~/contexts/ToastContext";
import { useSWRAuth } from "~/lib/swr";

import { Loading } from "~/components/ui/loading";
import { LocationBadge } from "@/components/ui/location-badge";
import { EmptyStudentResults } from "~/components/ui/empty-student-results";
import {
  StudentCard,
  StudentInfoRow,
  PickupTimeIcon,
  ExceptionIcon,
} from "~/components/students/student-card";
import { fetchBulkPickupTimes } from "~/lib/pickup-schedule-api";
import type { BulkPickupTime } from "~/lib/pickup-schedule-api";
import { Clock, AlertTriangle } from "lucide-react";

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
  sick_since?: string;
  sick_until?: string;
  location_since?: string;
  group_id?: number;
  group_name?: string;
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

function renderPickupIcon(urgency: PickupUrgency): JSX.Element {
  if (urgency === "overdue") {
    return <AlertTriangle className="h-3.5 w-3.5 text-red-500" />;
  }
  if (urgency === "soon") {
    return <Clock className="h-3.5 w-3.5 animate-pulse text-orange-500" />;
  }
  // normal / none — default gray clock
  return <PickupTimeIcon />;
}

function OGSGroupPageContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { data: session, status } = useSession({
    required: true,
    onUnauthenticated() {
      router.push("/");
    },
  });

  const { success: showSuccessToast } = useToast();

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
  const [sortMode, setSortMode] = useState<"default" | "pickup">("default");

  // Current time for urgency calculation (updates every minute)
  const [now, setNow] = useState(() => new Date());

  useEffect(() => {
    const interval = setInterval(() => setNow(new Date()), 60_000);
    return () => clearInterval(interval);
  }, []);

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
      console.log("⏱️ [OGS-GROUPS] SWR fetching dashboard via BFF...");
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

      console.log(
        `⏱️ [OGS-GROUPS] SWR fetch complete: ${(performance.now() - start).toFixed(0)}ms`,
      );
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
      const mappedStudents: Student[] = studentsData.map((s) => ({
        id: s.id.toString(),
        name: `${s.first_name} ${s.last_name}`.trim(),
        first_name: s.first_name,
        second_name: s.last_name, // Backend uses last_name
        school_class: s.school_class ?? "",
        current_location: s.current_location ?? "",
        location_since: s.location_since,
        group_id: s.group_id?.toString(),
        group_name: s.group_name,
      }));
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
  }, [dashboardData, selectedGroupId]);

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
  }>(
    hasAccess && currentGroupId ? `ogs-students-${currentGroupId}` : null,
    async () => {
      // Fetch students and room status in parallel for accurate filtering
      const [studentsResponse, roomStatusResponse] = await Promise.all([
        studentService.getStudents({
          groupId: currentGroupId!,
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

      // Fetch pickup times for all students (prevents loading flash)
      let pickupTimesMap = new Map<string, BulkPickupTime>();
      if (students.length > 0) {
        const studentIds = students.map((s) => s.id.toString());
        pickupTimesMap = await fetchBulkPickupTimes(studentIds).catch(() => {
          console.error("Failed to fetch pickup times in SWR");
          return new Map<string, BulkPickupTime>();
        });
      }

      return {
        students,
        roomStatus: roomStatusResponse ?? undefined,
        pickupTimes: pickupTimesMap,
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
    if (swrStudentsData?.students) {
      setStudents(swrStudentsData.students);
    }
    if (swrStudentsData?.roomStatus) {
      setRoomStatus(swrStudentsData.roomStatus);
    }
    // Only set pickupTimes if it's a Map (the SWR fetcher returns a Map,
    // but test mocks may return the wrong type)
    if (swrStudentsData?.pickupTimes instanceof Map) {
      setPickupTimes(swrStudentsData.pickupTimes);
    }
  }, [swrStudentsData]);

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
      console.error("Error loading available users:", error);
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
        console.error("Error checking active transfers:", error);
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
      loadAvailableUsers().catch(console.error);
      checkActiveTransfers(currentGroupId).catch(console.error);
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
        console.error("Error loading group room status:", error);
      }
    },
    [], // No dependencies - function is stable
  );

  // SSE is handled globally by AuthWrapper - no page-level setup needed.
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
        token: session?.user?.token,
      });
      const studentsData = studentsResponse.students || [];

      setStudents(studentsData);

      // Update group with actual student count
      setAllGroups((prev) =>
        prev.map((group) =>
          group.id === groupId
            ? { ...group, student_count: studentsData.length }
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

    if (sortMode === "pickup") {
      // Pickup sort: anwesend mit Abholzeit (nach Zeit) → anwesend ohne Abholzeit → zuhause
      return sorted.sort((a, b) => {
        const aHome = isHomeLocation(a.current_location);
        const bHome = isHomeLocation(b.current_location);

        // Zuhause immer ganz unten
        if (aHome && !bHome) return 1;
        if (!aHome && bHome) return -1;
        if (aHome && bHome) return 0;

        // Beide anwesend: nach Abholzeit sortieren
        const timeA = pickupTimes.get(a.id.toString())?.pickupTime;
        const timeB = pickupTimes.get(b.id.toString())?.pickupTime;

        // Ohne Abholzeit nach den mit Abholzeit
        if (!timeA && !timeB) return 0;
        if (!timeA) return 1;
        if (!timeB) return -1;

        return timeA.localeCompare(timeB);
      });
    }

    // Alphabetisch (Standard): Nachname, dann Vorname
    return sorted.sort((a, b) => {
      const lastCmp = (a.second_name ?? "").localeCompare(
        b.second_name ?? "",
        "de",
      );
      if (lastCmp !== 0) return lastCmp;
      return (a.first_name ?? "").localeCompare(b.first_name ?? "", "de");
    });
  }, [filteredStudents, sortMode, pickupTimes]);

  const getCardGradient = useCallback(
    (student: Student) => {
      if (isStudentInGroupRoom(student, currentGroup)) {
        return "from-emerald-50/80 to-green-100/80";
      }

      if (isSchoolyardLocation(student.current_location)) {
        return "from-amber-50/80 to-yellow-100/80";
      }

      if (isTransitLocation(student.current_location)) {
        return "from-fuchsia-50/80 to-pink-100/80";
      }

      if (isHomeLocation(student.current_location)) {
        return "from-red-50/80 to-rose-100/80";
      }

      const parsedLocation = parseLocation(student.current_location);
      if (
        parsedLocation.room ||
        parsedLocation.status === LOCATION_STATUSES.PRESENT
      ) {
        return "from-blue-50/80 to-cyan-100/80";
      }

      return "from-slate-50/80 to-gray-100/80";
    },
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
        onChange: (value) => setSortMode(value as "default" | "pickup"),
        options: [
          { value: "default", label: "Alphabetisch" },
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

  if (status === "loading" || isLoading || hasAccess === null) {
    return <Loading fullPage={false} />;
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

  // Render helper for desktop action button
  const renderDesktopActionButton = () => {
    if (isMobile || !currentGroup) return undefined;
    if (currentGroup.viaSubstitution) {
      return (
        <div className="flex h-10 items-center gap-2 rounded-full border border-orange-200 bg-orange-50 px-4">
          <svg
            className="h-5 w-5 text-orange-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
            />
          </svg>
          <span className="text-sm font-medium text-orange-900">
            In Vertretung
          </span>
        </div>
      );
    }
    return (
      <button
        onClick={() => setShowTransferModal(true)}
        className="group relative flex h-10 items-center gap-2 rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] px-4 text-white shadow-lg transition-all duration-150 hover:scale-105 hover:shadow-xl active:scale-95"
        aria-label="Gruppe übergeben"
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
            d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
          />
        </svg>
        <span className="relative text-sm font-semibold">
          {activeTransfers.length > 0
            ? `Gruppe übergeben (${activeTransfers.length})`
            : "Gruppe übergeben"}
        </span>
      </button>
    );
  };

  // Render helper for mobile action button
  const renderMobileActionButton = () => {
    if (!isMobile || !currentGroup) return undefined;
    if (currentGroup.viaSubstitution) {
      return (
        <div
          className="flex h-8 w-8 items-center justify-center rounded-full border border-orange-200 bg-orange-50"
          title="In Vertretung"
        >
          <svg
            className="h-4 w-4 text-orange-600"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth={2.5}
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
            />
          </svg>
        </div>
      );
    }
    return (
      <button
        onClick={() => setShowTransferModal(true)}
        className="relative flex h-8 w-8 items-center justify-center rounded-full bg-gradient-to-br from-[#83CD2D] to-[#70b525] text-white shadow-md transition-all duration-150 active:scale-90"
        aria-label="Gruppe übergeben"
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
            d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4"
          />
        </svg>
        {activeTransfers.length > 0 && (
          <span className="absolute -top-1 -right-1 flex h-4 w-4 items-center justify-center rounded-full bg-white text-[10px] font-bold text-[#70b525] shadow-sm">
            {activeTransfers.length}
          </span>
        )}
      </button>
    );
  };

  // Render helper for student grid content
  const renderStudentContent = () => {
    if (isLoading) {
      return <Loading fullPage={false} />;
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
                Keine Schüler in {currentGroup?.name ?? "dieser Gruppe"}
              </h3>
              <p className="text-sm text-gray-500">
                Es wurden noch keine Schüler zu dieser OGS-Gruppe hinzugefügt.
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
              const isAtHome = isHomeLocation(student.current_location);
              const urgency = isAtHome
                ? ("none" as PickupUrgency)
                : getPickupUrgency(studentPickup?.pickupTime, now);

              return (
                <StudentCard
                  key={student.id}
                  studentId={student.id}
                  firstName={student.first_name}
                  lastName={student.second_name}
                  gradient={cardGradient}
                  onClick={() =>
                    router.push(`/students/${student.id}?from=/ogs-groups`)
                  }
                  locationBadge={
                    <LocationBadge
                      student={student}
                      displayMode="roomName"
                      isGroupRoom={inGroupRoom}
                      variant="modern"
                      size="md"
                    />
                  }
                  extraContent={
                    studentPickup?.pickupTime ? (
                      <StudentInfoRow
                        icon={
                          studentPickup.isException ? (
                            <ExceptionIcon />
                          ) : (
                            renderPickupIcon(urgency)
                          )
                        }
                      >
                        Abholung: {studentPickup.pickupTime} Uhr
                        {studentPickup.dayNotes?.length > 0 && (
                          <span className="ml-1 text-gray-500">
                            (
                            {studentPickup.dayNotes
                              .map((n) => n.content)
                              .join(", ")}
                            )
                          </span>
                        )}
                      </StudentInfoRow>
                    ) : null
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
        {/* PageHeaderWithSearch - Title only on mobile */}
        <PageHeaderWithSearch
          title={
            isMobile && allGroups.length === 1
              ? (currentGroup?.name ?? "Meine Gruppe")
              : "" // No title when multiple groups (tabs show group names) or on desktop
          }
          actionButton={renderDesktopActionButton()}
          mobileActionButton={renderMobileActionButton()}
          tabs={
            allGroups.length > 1 && !isDesktop
              ? {
                  items: allGroups.map((group) => ({
                    id: group.id,
                    label: group.name,
                    count: group.student_count,
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

        {/* Mobile Error Display */}
        {error && (
          <div className="mb-4 md:hidden">
            <Alert type="error" message={error} />
          </div>
        )}

        {/* Student Grid - Mobile Optimized */}
        {renderStudentContent()}
      </div>

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
        availableUsers={availableUsers}
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
    <Suspense fallback={<Loading fullPage={false} />}>
      <SSEErrorBoundary>
        <OGSGroupPageContent />
      </SSEErrorBoundary>
    </Suspense>
  );
}
