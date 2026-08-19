"use client";

import { useCallback } from "react";
import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import { studentService } from "~/lib/api";
import type { Student, SupervisorContact } from "~/lib/student-helpers";
import { busDaysHaveAny } from "~/lib/student-helpers";
import { userContextService } from "~/lib/usercontext-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentData" });

/**
 * Extended Student type with additional detail page fields
 */
export interface ExtendedStudent extends Student {
  bus: boolean;
  /** Confirmation to widen a linked child's own departure plan on save. */
  extend_companion_plans?: boolean;
  current_room?: string;
  location_since?: string;
  birthday?: string;
  buskind?: boolean;
  extra_info?: string;
  supervisor_notes?: string;
  health_info?: string;
  address_street?: string;
  address_city?: string;
  address_postal_code?: string;
  pickup_status?: string;
  sick?: boolean;
  sick_since?: string;
  excused?: boolean;
  excused_since?: string;
  class_trip?: boolean;
  class_trip_since?: string;
}

interface StudentDataState {
  student: ExtendedStudent | null;
  loading: boolean;
  error: string | null;
  /**
   * READ access to this child's data — the backend's `has_full_access`, which
   * resolves to `authorize.CanReadStudent`: true for admins and every verified
   * staff member (#2329), false for guest/guardian accounts, whose view stays
   * redacted. Gate read-only surfaces on THIS flag — every backend read path
   * for a child uses the same predicate, so they cannot disagree.
   */
  hasFullAccess: boolean;
  /** WRITE access (`has_write_access`): admins and verified staff (#2329). */
  hasWriteAccess: boolean;
  /**
   * WRITE access to this child's absence statuses only (krank / entschuldigt /
   * Klassenfahrt) — the backend's `has_absence_write_access`. A superset of
   * `hasWriteAccess`: a school running ohne feste Gruppen
   * (`operations.group_mode = open_care`) lets staff holding `users:absence`
   * report absences for children they cannot otherwise edit (#2232).
   * Gate ONLY the absence actions on it, never a Stammdaten affordance.
   */
  hasAbsenceWriteAccess: boolean;
  attendanceLogEnabled: boolean;
  supervisors: SupervisorContact[];
  myGroups: string[];
  myGroupRooms: string[];
  mySupervisedRooms: string[];
}

interface UseStudentDataResult extends StudentDataState {
  refreshData: () => void;
}

/**
 * Maps raw student response to ExtendedStudent with proper access control
 */
function mapStudentResponse(
  response: unknown,
  hasAccess: boolean,
  canManageAbsence: boolean,
): ExtendedStudent {
  interface WrappedResponse {
    data?: unknown;
    success?: boolean;
    message?: string;
  }
  const wrappedResponse = response as WrappedResponse;
  const studentData = wrappedResponse.data ?? response;

  const mappedStudent = studentData as Student & {
    has_full_access?: boolean;
    group_supervisors?: SupervisorContact[];
  };

  const canSeeAbsenceStatus = hasAccess || canManageAbsence;

  // Spread upstream-mapped student so future Student fields propagate without
  // touching this whitelist. Explicit lines below either coerce nullish to a
  // default (first_name/group_id/...) or apply access-control gating.
  return {
    ...mappedStudent,
    first_name: mappedStudent.first_name ?? "",
    second_name: mappedStudent.second_name ?? "",
    group_id: mappedStudent.group_id ?? "",
    group_name: mappedStudent.group_name ?? "",
    // bus / buskind are derived from bus_days, the single source of truth (#1582).
    bus: busDaysHaveAny(mappedStudent.bus_days),
    buskind: busDaysHaveAny(mappedStudent.bus_days),
    current_room: undefined,
    location_since: hasAccess
      ? (mappedStudent.location_since ?? undefined)
      : undefined,
    extra_info: hasAccess ? (mappedStudent.extra_info ?? undefined) : undefined,
    address_street: hasAccess
      ? (mappedStudent.address_street ?? undefined)
      : undefined,
    address_city: hasAccess
      ? (mappedStudent.address_city ?? undefined)
      : undefined,
    address_postal_code: hasAccess
      ? (mappedStudent.address_postal_code ?? undefined)
      : undefined,
    supervisor_notes: hasAccess
      ? (mappedStudent.supervisor_notes ?? undefined)
      : undefined,
    // The absence statuses follow read access OR the absence-write gate: they
    // are the state the Krank-/Entschuldigt-Aktionen toggle, so blanking them
    // for a caller who may write them would always render "melden" and turn a
    // clearing click into a fresh report (#2232). The backend hands these
    // fields to every authenticated staff member anyway.
    sick: canSeeAbsenceStatus ? (mappedStudent.sick ?? false) : false,
    sick_since: canSeeAbsenceStatus
      ? (mappedStudent.sick_since ?? undefined)
      : undefined,
    excused: canSeeAbsenceStatus ? (mappedStudent.excused ?? false) : false,
    excused_since: canSeeAbsenceStatus
      ? (mappedStudent.excused_since ?? undefined)
      : undefined,
    class_trip: canSeeAbsenceStatus
      ? (mappedStudent.class_trip ?? false)
      : false,
    class_trip_since: canSeeAbsenceStatus
      ? (mappedStudent.class_trip_since ?? undefined)
      : undefined,
    actual_arrival_time: hasAccess
      ? (mappedStudent.actual_arrival_time ?? undefined)
      : undefined,
    actual_pickup_time: hasAccess
      ? (mappedStudent.actual_pickup_time ?? undefined)
      : undefined,
    has_full_access: hasAccess,
    // Photo URL is visible to anyone with users:read who can see the
    // student. The backend returns the authenticated proxy URL, so the
    // browser can render <img src> with the same-origin session cookie.
    // Forgetting this field on the detail-page hook was the bug where the
    // big xl avatar in StudentDetailHeader stayed empty even though all
    // other surfaces (master-detail header, search cards, OGS-room cards)
    // showed the photo correctly.
    photo_url: mappedStudent.photo_url ?? undefined,
    photo_consent_given: mappedStudent.photo_consent_given ?? undefined,
    photo_consent_given_at: mappedStudent.photo_consent_given_at ?? undefined,
    photo_consent_given_by: mappedStudent.photo_consent_given_by ?? undefined,
  };
}

/**
 * Helper to extract room names from groups
 */
function extractRoomNames(
  groups: Array<{ room?: { name?: string } }>,
): string[] {
  return groups.map((group) => group.room?.name).filter(Boolean) as string[];
}

interface StudentDetailResponse {
  student: ExtendedStudent;
  hasFullAccess: boolean;
  hasWriteAccess: boolean;
  hasAbsenceWriteAccess: boolean;
  attendanceLogEnabled: boolean;
  supervisors: SupervisorContact[];
  myGroups: string[];
  myGroupRooms: string[];
  mySupervisedRooms: string[];
}

/**
 * Custom hook for fetching and managing student detail page data
 * Uses SWR for caching and automatic revalidation via global SSE
 */
export function useStudentData(studentId: string): UseStudentDataResult {
  const { data: session, status: sessionStatus } = useSession();

  // SWR-based student data fetching with caching
  // Cache key "student-detail-{id}" will be invalidated by global SSE on student_checkin/checkout events
  const {
    data: studentData,
    isLoading,
    error: fetchError,
    mutate,
  } = useSWRAuth<StudentDetailResponse>(
    studentId && session?.user?.token ? `student-detail-${studentId}` : null,
    async () => {
      // Fetch student data and user context in parallel
      const [studentResponse, groups, supervisedGroups] = await Promise.all([
        studentService.getStudent(studentId),
        userContextService.getMyEducationalGroups().catch((err) => {
          logger.debug("fetch_educational_groups_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [];
        }),
        userContextService.getMySupervisedGroups().catch((err) => {
          logger.debug("fetch_supervised_groups_failed", {
            error: err instanceof Error ? err.message : String(err),
          });
          return [];
        }),
      ]);

      interface WrappedResponse {
        data?: unknown;
      }
      const wrappedResponse = studentResponse as WrappedResponse;
      const rawStudentData = wrappedResponse.data ?? studentResponse;

      const mappedStudent = rawStudentData as Student & {
        has_full_access?: boolean;
        has_write_access?: boolean;
        has_absence_write_access?: boolean;
        group_supervisors?: SupervisorContact[];
        attendance_log_enabled?: boolean;
      };

      const hasAccess = mappedStudent.has_full_access ?? false;
      const hasWriteAccess = mappedStudent.has_write_access ?? false;
      // Falls back to the Stammdaten flag so an older backend keeps today's
      // behavior instead of hiding the Krankmeldung action (#2232).
      const hasAbsenceWriteAccess =
        mappedStudent.has_absence_write_access ?? hasWriteAccess;
      const attendanceLogEnabled =
        mappedStudent.attendance_log_enabled ?? false;
      const groupSupervisors = mappedStudent.group_supervisors ?? [];
      const extendedStudent = mapStudentResponse(
        studentResponse,
        hasAccess,
        hasAbsenceWriteAccess,
      );

      const ogsGroupRoomNames = extractRoomNames(groups);
      const groupIds = groups.map((group) => group.id);
      const roomNames = extractRoomNames(supervisedGroups);

      return {
        student: extendedStudent,
        hasFullAccess: hasAccess,
        hasWriteAccess,
        hasAbsenceWriteAccess,
        attendanceLogEnabled,
        supervisors: groupSupervisors,
        myGroups: groupIds,
        myGroupRooms: ogsGroupRoomNames,
        mySupervisedRooms: roomNames,
      };
    },
    {
      keepPreviousData: true, // Show cached data while revalidating
      revalidateOnFocus: false, // Handled by global SSE
    },
  );

  // refreshData now uses SWR's mutate
  const refreshData = useCallback(() => {
    mutate().catch((err) => {
      logger.debug("swr_revalidation_failed", {
        error: err instanceof Error ? err.message : String(err),
        scope: "student_detail",
      });
    });
  }, [mutate]);

  // Convert SWR state to component state
  const error = fetchError ? "Fehler beim Laden der Kinderdaten." : null;

  // Include session loading state to prevent transient error display.
  // When session is loading, SWR key is null, so isLoading is false even though
  // we're not ready to display data. This prevents "Student not found" flash.
  const loading = isLoading || sessionStatus === "loading";

  return {
    student: studentData?.student ?? null,
    loading,
    error,
    hasFullAccess: studentData?.hasFullAccess ?? true,
    hasWriteAccess: studentData?.hasWriteAccess ?? false,
    hasAbsenceWriteAccess: studentData?.hasAbsenceWriteAccess ?? false,
    attendanceLogEnabled: studentData?.attendanceLogEnabled ?? false,
    supervisors: studentData?.supervisors ?? [],
    myGroups: studentData?.myGroups ?? [],
    myGroupRooms: studentData?.myGroupRooms ?? [],
    mySupervisedRooms: studentData?.mySupervisedRooms ?? [],
    refreshData,
  };
}

/**
 * Determines if checkout section should be shown for a student
 * Any authenticated staff member can checkout any checked-in student
 */
export function shouldShowCheckoutSection(
  student: ExtendedStudent,
  _myGroups: string[],
  _mySupervisedRooms: string[],
): boolean {
  // Show checkout button for any checked-in student (not at home)
  const isCheckedIn = Boolean(
    student.current_location && !student.current_location.startsWith("Zuhause"),
  );

  return isCheckedIn;
}
