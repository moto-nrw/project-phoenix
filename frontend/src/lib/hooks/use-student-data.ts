"use client";

import { useCallback } from "react";
import { useSession } from "next-auth/react";
import { useSWRAuth } from "~/lib/swr";
import { studentService } from "~/lib/api";
import type { Student, SupervisorContact } from "~/lib/student-helpers";
import { userContextService } from "~/lib/usercontext-api";

/**
 * Extended Student type with additional detail page fields
 */
export interface ExtendedStudent extends Student {
  bus: boolean;
  current_room?: string;
  location_since?: string;
  birthday?: string;
  buskind?: boolean;
  attendance_rate?: number;
  extra_info?: string;
  supervisor_notes?: string;
  health_info?: string;
  pickup_status?: string;
  sick?: boolean;
  sick_since?: string;
}

interface StudentDataState {
  student: ExtendedStudent | null;
  loading: boolean;
  error: string | null;
  hasFullAccess: boolean;
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

  return {
    id: mappedStudent.id,
    first_name: mappedStudent.first_name ?? "",
    second_name: mappedStudent.second_name ?? "",
    name: mappedStudent.name,
    school_class: mappedStudent.school_class,
    group_id: mappedStudent.group_id ?? "",
    group_name: mappedStudent.group_name ?? "",
    current_location: mappedStudent.current_location,
    location_since: hasAccess
      ? (mappedStudent.location_since ?? undefined)
      : undefined,
    bus: mappedStudent.bus ?? false,
    current_room: undefined,
    birthday: mappedStudent.birthday ?? undefined,
    buskind: mappedStudent.bus ?? false,
    attendance_rate: undefined,
    extra_info: hasAccess ? (mappedStudent.extra_info ?? undefined) : undefined,
    supervisor_notes: hasAccess
      ? (mappedStudent.supervisor_notes ?? undefined)
      : undefined,
    health_info: mappedStudent.health_info ?? undefined,
    pickup_status: mappedStudent.pickup_status ?? undefined,
    sick: hasAccess ? (mappedStudent.sick ?? false) : false,
    sick_since: hasAccess ? (mappedStudent.sick_since ?? undefined) : undefined,
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
        userContextService.getMyEducationalGroups().catch(() => []),
        userContextService.getMySupervisedGroups().catch(() => []),
      ]);

      interface WrappedResponse {
        data?: unknown;
      }
      const wrappedResponse = studentResponse as WrappedResponse;
      const rawStudentData = wrappedResponse.data ?? studentResponse;

      const mappedStudent = rawStudentData as Student & {
        has_full_access?: boolean;
        group_supervisors?: SupervisorContact[];
      };

      const hasAccess = mappedStudent.has_full_access ?? false;
      const groupSupervisors = mappedStudent.group_supervisors ?? [];
      const extendedStudent = mapStudentResponse(studentResponse, hasAccess);

      const ogsGroupRoomNames = extractRoomNames(groups);
      const groupIds = groups.map((group) => group.id);
      const roomNames = extractRoomNames(supervisedGroups);

      return {
        student: extendedStudent,
        hasFullAccess: hasAccess,
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
    mutate().catch(() => {
      // Ignore revalidation errors
    });
  }, [mutate]);

  // Convert SWR state to component state
  const error = fetchError ? "Fehler beim Laden der Schülerdaten." : null;

  // Include session loading state to prevent transient error display.
  // When session is loading, SWR key is null, so isLoading is false even though
  // we're not ready to display data. This prevents "Student not found" flash.
  const loading = isLoading || sessionStatus === "loading";

  return {
    student: studentData?.student ?? null,
    loading,
    error,
    hasFullAccess: studentData?.hasFullAccess ?? true,
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
