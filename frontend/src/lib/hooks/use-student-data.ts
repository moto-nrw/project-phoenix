"use client";

import { useEffect, useState, useCallback } from "react";
import { useSession } from "next-auth/react";
import { useSSE } from "~/lib/hooks/use-sse";
import type { SSEEvent } from "~/lib/sse-types";
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
 * Custom hook for fetching and managing student detail page data
 * Handles student data, groups, SSE updates, and access control
 */
export function useStudentData(studentId: string): UseStudentDataResult {
  const { data: session } = useSession();

  const [state, setState] = useState<StudentDataState>({
    student: null,
    loading: true,
    error: null,
    hasFullAccess: true,
    supervisors: [],
    myGroups: [],
    myGroupRooms: [],
    mySupervisedRooms: [],
  });
  const [groupsLoaded, setGroupsLoaded] = useState(false);
  const [refreshTrigger, setRefreshTrigger] = useState(0);

  const refreshData = useCallback(() => {
    setRefreshTrigger((prev) => prev + 1);
  }, []);

  // Load groups first (before student data)
  useEffect(() => {
    const loadMyGroups = async () => {
      if (!session?.user?.token) {
        setState((prev) => ({
          ...prev,
          myGroups: [],
          myGroupRooms: [],
          mySupervisedRooms: [],
        }));
        setGroupsLoaded(true);
        return;
      }

      try {
        const groups = await userContextService.getMyEducationalGroups();
        const ogsGroupRoomNames = groups
          .map((group) => group.room?.name)
          .filter((name): name is string => Boolean(name));

        const supervisedGroups =
          await userContextService.getMySupervisedGroups();
        const roomNames = supervisedGroups
          .map((group) => group.room?.name)
          .filter((name): name is string => Boolean(name));

        setState((prev) => ({
          ...prev,
          myGroups: groups.map((group) => group.id),
          myGroupRooms: ogsGroupRoomNames,
          mySupervisedRooms: roomNames,
        }));
      } catch (err) {
        console.error("Error loading supervisor groups:", err);
      } finally {
        setGroupsLoaded(true);
      }
    };

    void loadMyGroups();
  }, [session?.user?.token]);

  // Fetch student data after groups are loaded
  useEffect(() => {
    const fetchStudent = async () => {
      setState((prev) => ({ ...prev, loading: true, error: null }));

      try {
        const response = await studentService.getStudent(studentId);

        interface WrappedResponse {
          data?: unknown;
        }
        const wrappedResponse = response as WrappedResponse;
        const studentData = wrappedResponse.data ?? response;

        const mappedStudent = studentData as Student & {
          has_full_access?: boolean;
          group_supervisors?: SupervisorContact[];
        };

        const hasAccess = mappedStudent.has_full_access ?? false;
        const groupSupervisors = mappedStudent.group_supervisors ?? [];
        const extendedStudent = mapStudentResponse(response, hasAccess);

        setState((prev) => ({
          ...prev,
          student: extendedStudent,
          hasFullAccess: hasAccess,
          supervisors: groupSupervisors,
          loading: false,
        }));
      } catch (err) {
        console.error("Error fetching student:", err);
        setState((prev) => ({
          ...prev,
          error: "Fehler beim Laden der Schülerdaten.",
          loading: false,
        }));
      }
    };

    if (groupsLoaded) {
      void fetchStudent();
    }
  }, [studentId, refreshTrigger, groupsLoaded]);

  // SSE event handler - refresh when this student checks in/out
  const handleSSEEvent = useCallback(
    (event: SSEEvent) => {
      const isCheckInOrOut =
        event.type === "student_checkin" || event.type === "student_checkout";
      if (isCheckInOrOut && event.data.student_id === studentId) {
        refreshData();
      }
    },
    [studentId, refreshData],
  );

  // SSE connection for real-time location updates
  useSSE("/api/sse/events", {
    onMessage: handleSSEEvent,
    enabled: groupsLoaded,
  });

  return {
    ...state,
    refreshData,
  };
}

/**
 * Determines if checkout section should be shown for a student
 */
export function shouldShowCheckoutSection(
  student: ExtendedStudent,
  myGroups: string[],
  mySupervisedRooms: string[],
): boolean {
  const isInMyGroup = Boolean(
    student.group_id && myGroups.includes(student.group_id),
  );
  const isInMySupervisedRoom = Boolean(
    student.current_location &&
    mySupervisedRooms.some((room) => student.current_location?.includes(room)),
  );
  const isCheckedIn = Boolean(
    student.current_location && !student.current_location.startsWith("Zuhause"),
  );

  return (isInMyGroup || isInMySupervisedRoom) && isCheckedIn;
}
