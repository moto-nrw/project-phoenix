"use client";

import {
  fetchStudentDetail,
  type StudentDetailResponse,
  type ExtendedStudent,
} from "~/lib/student-detail-resource";
export type { ExtendedStudent } from "~/lib/student-detail-resource";
import { studentKeys } from "~/lib/swr/portal-keys";

import { useCallback } from "react";
import { useSession } from "next-auth/react";
import { hasSessionAccess } from "~/lib/session-access";
import { useSWRAuth } from "~/lib/swr";
import type { SupervisorContact } from "~/lib/student-helpers";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "StudentData" });

interface StudentDataState {
  student: ExtendedStudent | null;
  loading: boolean;
  /**
   * The failed load as thrown by the client (an `ApiError` with code and
   * request ID), or null. The page shows it over the shared error path
   * (#2513); no sentence is built here.
   */
  error: unknown;
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
  /** Class-trip authority, independent of Stammdaten writes. */
  hasAbsenceWriteAccess: boolean;
  /** Direct sick/excused reports; does not authorize parent-request review. */
  hasSickExcusedWriteAccess: boolean;
  attendanceLogEnabled: boolean;
  feedbackEnabled: boolean;
  supervisors: SupervisorContact[];
  myGroups: string[];
  myGroupRooms: string[];
  mySupervisedRooms: string[];
}

interface UseStudentDataResult extends StudentDataState {
  refreshData: () => void;
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
    studentId && hasSessionAccess(session)
      ? studentKeys.detail(studentId)
      : null,
    () => fetchStudentDetail(studentId),
    {
      keepPreviousData: false, // Never carry another child's detail or grants.
      revalidateOnFocus: false, // Handled by global SSE
    },
  );

  // refreshData now uses SWR's mutate
  const refreshData = useCallback(() => {
    // A failed reload lands in SWR's error state and from there on the
    // page's load error path; this catch only keeps the promise handled.
    mutate().catch((err) => {
      logger.debug("swr_revalidation_failed", {
        error: err instanceof Error ? err.message : String(err),
        scope: "student_detail",
      });
    });
  }, [mutate]);

  // Convert SWR state to component state
  const error: unknown = fetchError ?? null;

  // Include session loading state to prevent transient error display.
  // When session is loading, SWR key is null, so isLoading is false even though
  // we're not ready to display data. This prevents "Student not found" flash.
  const loading = isLoading || sessionStatus === "loading";

  return {
    student: studentData?.student ?? null,
    loading,
    error,
    hasFullAccess: studentData?.hasFullAccess ?? false,
    hasWriteAccess: studentData?.hasWriteAccess ?? false,
    hasAbsenceWriteAccess: studentData?.hasAbsenceWriteAccess ?? false,
    hasSickExcusedWriteAccess: studentData?.hasSickExcusedWriteAccess ?? false,
    attendanceLogEnabled: studentData?.attendanceLogEnabled ?? false,
    feedbackEnabled: studentData?.feedbackEnabled ?? false,
    supervisors: studentData?.supervisors ?? [],
    myGroups: studentData?.myGroups ?? [],
    myGroupRooms: studentData?.myGroupRooms ?? [],
    mySupervisedRooms: studentData?.mySupervisedRooms ?? [],
    refreshData,
  };
}
