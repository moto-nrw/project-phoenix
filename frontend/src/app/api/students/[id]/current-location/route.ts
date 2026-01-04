// app/api/students/[id]/current-location/route.ts
import type { NextRequest } from "next/server";
import { createGetHandler } from "~/lib/route-wrapper";
import { apiGet } from "~/lib/api-client";
import type { BackendStudent } from "~/lib/student-helpers";
import {
  LOCATION_STATUSES,
  isHomeLocation,
  isPresentLocation,
  normalizeLocation,
} from "~/lib/location-helper";
import axios from "axios";
import type { BackendRoom } from "~/lib/rooms-helpers";

// Define proper types for the API responses
interface StudentApiResponse {
  data: BackendStudent;
}

interface RoomStatusApiResponse {
  data: {
    student_room_status: Record<
      string,
      {
        current_room_id: number | null;
        check_in_time?: string;
      }
    >;
  };
}

interface RoomApiResponse {
  data: BackendRoom;
}

type RoomInfo = {
  id: string;
  name: string;
  roomNumber?: string;
  building?: string;
  floor?: number;
};

type GroupInfo = {
  id: string;
  name: string;
  roomId?: string; // Group's assigned room ID
};

type PresentLocation = {
  status: "present";
  location: string; // e.g. "Anwesend", "Unterwegs", or specific room label
  room: RoomInfo | null;
  group: GroupInfo | null;
  checkInTime: string | null;
  isGroupRoom: boolean;
};

type NotPresentLocation = {
  status: "not_present";
  location: "Zuhause";
  room: null;
  group: GroupInfo | null;
  checkInTime: null;
  isGroupRoom: false;
};

type UnknownLocation = {
  status: "unknown";
  location: "Unbekannt";
  room: null;
  group: null;
  checkInTime: null;
  isGroupRoom: false;
  errorCode?: "NETWORK" | "UNAUTHORIZED" | "FORBIDDEN" | "NOT_FOUND" | "SERVER";
};

type LocationResponse = PresentLocation | NotPresentLocation | UnknownLocation;

/**
 * isGroupRoom semantics
 * - true: The student is present and currently located in their group's assigned room (groupRoomId matches current_room_id).
 * - false: The student is present but not in their group's room (e.g., unterwegs or different room), or not present/unknown.
 * Usage guidance:
 * - UI may highlight a green badge when isGroupRoom is true and room != null.
 * - When present but in transit (Unterwegs) or when room details are not accessible due to permissions, expose status "present" with room = null and isGroupRoom = false.
 * - Do not infer presence solely from isGroupRoom; rely on discriminated union status.
 */

/**
 * Handler for GET /api/students/[id]/current-location
 * Returns the current location of a student including room details
 */
export const GET = createGetHandler(
  async (
    _request: NextRequest,
    token: string,
    params: Record<string, unknown>,
  ) => {
    const studentId = params.id as string;

    if (!studentId) {
      throw new Error("Student ID is required");
    }

    try {
      const { student, groupRoomId } = await fetchStudentAndGroup(
        studentId,
        token,
      );
      const normalizedLocation = normalizeLocation(student.current_location);

      if (isHomeLocation(normalizedLocation)) {
        return buildNotPresentResponse(student, groupRoomId);
      }

      if (isPresentLocation(normalizedLocation)) {
        return await handlePresentLocationCase(
          student,
          studentId,
          normalizedLocation,
          groupRoomId,
          token,
        );
      }

      return buildNotPresentResponse(student, groupRoomId);
    } catch (error) {
      return handleLocationFetchError(error);
    }
  },
);

// Fetches student and optionally group room ID
async function fetchStudentAndGroup(
  studentId: string,
  token: string,
): Promise<{ student: BackendStudent; groupRoomId: number | null }> {
  const studentResponse = await apiGet<StudentApiResponse>(
    `/api/students/${studentId}`,
    token,
  );
  const student = studentResponse.data.data;

  let groupRoomId: number | null = null;
  if (student.group_id) {
    try {
      const groupResponse = await apiGet<{ data: { room_id: number | null } }>(
        `/api/groups/${student.group_id}`,
        token,
      );
      groupRoomId = groupResponse.data.data.room_id;
    } catch (e) {
      console.error("Error fetching group room ID:", e);
    }
  }

  return { student, groupRoomId };
}

// Builds group info object from student data
function buildGroupInfo(
  student: BackendStudent,
  groupRoomId: number | null,
): GroupInfo | null {
  if (!student.group_id) {
    return null;
  }

  return {
    id: student.group_id.toString(),
    name: student.group_name ?? "Unknown Group",
    roomId: groupRoomId?.toString(),
  };
}

// Builds not present response
function buildNotPresentResponse(
  student: BackendStudent,
  groupRoomId: number | null,
): LocationResponse {
  return {
    status: "not_present",
    location: LOCATION_STATUSES.HOME,
    room: null,
    group: buildGroupInfo(student, groupRoomId),
    checkInTime: null,
    isGroupRoom: false,
  };
}

// Handles present location case with room status checks
async function handlePresentLocationCase(
  student: BackendStudent,
  studentId: string,
  normalizedLocation: string,
  groupRoomId: number | null,
  token: string,
): Promise<LocationResponse> {
  if (!student.group_id) {
    return buildPresentNoRoomResponse(student, normalizedLocation, groupRoomId);
  }

  const roomStatus = await tryGetStudentRoomStatus(
    student.group_id,
    studentId,
    token,
  );

  if (roomStatus) {
    return await buildPresentLocationWithRoom(
      student,
      studentId,
      normalizedLocation,
      groupRoomId,
      roomStatus,
      token,
    );
  }

  return buildPresentNoRoomResponse(student, normalizedLocation, groupRoomId);
}

// Tries to get room status for a student (may fail due to permissions)
// Returns null if student has no current room (transit state) or on error
async function tryGetStudentRoomStatus(
  groupId: number,
  studentId: string,
  token: string,
): Promise<{ current_room_id: number; check_in_time?: string } | null> {
  try {
    const roomStatusResponse = await apiGet<RoomStatusApiResponse>(
      `/api/groups/${groupId}/students/room-status`,
      token,
    );
    const roomStatusData = roomStatusResponse.data.data;
    const status = roomStatusData?.student_room_status?.[studentId];
    // Guard: only return status if current_room_id is present (not null/undefined)
    if (status?.current_room_id == null) {
      return null;
    }
    return {
      current_room_id: status.current_room_id,
      check_in_time: status.check_in_time,
    };
  } catch (error) {
    console.error("Error fetching room status (likely permissions):", error);
    return null;
  }
}

// Builds present location response with room details
async function buildPresentLocationWithRoom(
  student: BackendStudent,
  studentId: string,
  normalizedLocation: string,
  groupRoomId: number | null,
  roomStatus: { current_room_id: number; check_in_time?: string },
  token: string,
): Promise<LocationResponse> {
  const isInGroupRoom = groupRoomId === roomStatus.current_room_id;
  const group = buildGroupInfo(student, groupRoomId)!;

  try {
    const roomResponse = await apiGet<RoomApiResponse>(
      `/api/rooms/${roomStatus.current_room_id}`,
      token,
    );
    const roomData = roomResponse.data.data;

    if (roomData) {
      return {
        status: "present",
        location: normalizedLocation,
        room: {
          id: roomData.id.toString(),
          name: roomData.name ?? `Raum ${roomStatus.current_room_id}`,
          roomNumber: undefined,
          building: roomData.building,
          floor: roomData.floor,
        },
        group,
        checkInTime: roomStatus.check_in_time ?? null,
        isGroupRoom: isInGroupRoom,
      };
    }
  } catch (e) {
    console.error("Error fetching room details:", e);
  }

  // Fallback: room exists but couldn't fetch details
  return {
    status: "present",
    location: normalizedLocation,
    room: {
      id: roomStatus.current_room_id.toString(),
      name: `Raum ${roomStatus.current_room_id}`,
      roomNumber: undefined,
      building: undefined,
      floor: undefined,
    },
    group,
    checkInTime: roomStatus.check_in_time ?? null,
    isGroupRoom: isInGroupRoom,
  };
}

// Builds present-but-no-room location response (e.g., transit or room status unavailable)
function buildPresentNoRoomResponse(
  student: BackendStudent,
  normalizedLocation: string,
  groupRoomId: number | null,
): LocationResponse {
  return {
    status: "present",
    // Use the actual normalized location instead of hardcoded TRANSIT
    // This preserves the correct status when room details are unavailable
    location: normalizedLocation,
    room: null,
    group: buildGroupInfo(student, groupRoomId),
    checkInTime: null,
    isGroupRoom: false,
  };
}

// Handles location fetch errors and returns unknown response
function handleLocationFetchError(error: unknown): LocationResponse {
  console.error("Error fetching student current location:", error);

  const errorCode = mapAxiosErrorToCode(error);

  return {
    status: "unknown",
    location: LOCATION_STATUSES.UNKNOWN,
    room: null,
    group: null,
    checkInTime: null,
    isGroupRoom: false,
    errorCode,
  };
}

// Maps axios error to error code
function mapAxiosErrorToCode(
  error: unknown,
): UnknownLocation["errorCode"] | undefined {
  if (!axios.isAxiosError(error)) {
    return undefined;
  }

  if (!error.response) {
    return "NETWORK";
  }

  const status = error.response.status;
  if (status === 401) return "UNAUTHORIZED";
  if (status === 403) return "FORBIDDEN";
  if (status === 404) return "NOT_FOUND";
  return "SERVER";
}
