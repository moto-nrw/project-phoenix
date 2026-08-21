// lib/ogs-group-live-api.ts
// Client for the aggregated OGS group live projection (#2056).
// GET /api/ogs-group-live returns everything the OGS-Gruppen page renders for
// one supervised group in a single backend request. The wire DTO is minimal by
// contract — it must never grow personal fields the page does not display.
import type { TrackingIndicatorsResponse } from "~/lib/active-helpers";
import type { GroupTransfer } from "~/lib/group-transfer-api";

// Wire types (Go DTOs from backend/api/students/ogs_group_live_handlers.go)

interface OgsLiveWireGroup {
  id: string;
  name: string;
  room_id?: string;
  room_name?: string;
  via_substitution: boolean;
}

export interface OgsLiveWireStudent {
  id: string;
  first_name: string;
  last_name: string;
  school_class: string;
  current_location: string;
  location_since?: string;
  current_room_color?: string | null;
  sick: boolean;
  sick_since?: string;
  excused: boolean;
  excused_since?: string;
  class_trip: boolean;
  class_trip_since?: string;
  day_planning_status?: "comes_today" | "not_coming_today";
  day_planning_reason?: string;
  day_planning_label?: string;
  pending_excused_note?: string;
  arrival_time?: string;
  arrival_is_exception?: boolean;
  arrival_notes?: string;
  actual_arrival_time?: string;
  actual_pickup_time?: string;
  photo_url?: string;
}

interface OgsLiveWireRoomStatus {
  in_group_room: boolean;
  current_room_id?: string;
}

interface OgsLiveWirePickupTime {
  student_id: string;
  pickup_time?: string;
  is_exception: boolean;
  notes?: string;
  day_notes?: Array<{ id: string; content: string }>;
}

interface OgsLiveWireTransfer {
  id: string;
  group_id: string;
  substitute_staff_id: string;
  substitute_name?: string;
  end_date: string;
}

export interface OgsGroupLiveWireResponse {
  groups: OgsLiveWireGroup[];
  group_id?: string;
  students: OgsLiveWireStudent[];
  room_status: Record<string, OgsLiveWireRoomStatus>;
  pickup_times: OgsLiveWirePickupTime[];
  tracking_indicators: TrackingIndicatorsResponse;
  transfers: OgsLiveWireTransfer[];
}

// View model consumed by the OGS-Gruppen page

interface OgsLiveGroup {
  id: string;
  name: string;
  roomId?: string;
  roomName?: string;
  viaSubstitution: boolean;
}

/** Today's effective pickup info for one student (subset the cards render). */
export interface OgsPickupInfo {
  pickupTime?: string;
  isException: boolean;
  notes?: string;
  dayNotes: Array<{ id: string; content: string }>;
}

export interface OgsLiveViewData {
  groups: OgsLiveGroup[];
  /** The group the live sections below belong to; null when the caller
   *  supervises no groups at all. */
  groupId: string | null;
  students: OgsLiveWireStudent[];
  roomStatus: Record<
    string,
    { in_group_room: boolean; current_room_id?: string }
  >;
  pickupTimes: Map<string, OgsPickupInfo>;
  trackingIndicators: TrackingIndicatorsResponse;
  transfers: GroupTransfer[];
}

const MAX_INT64 = 9_223_372_036_854_775_807n;

function isValidGroupId(groupId: string | null): groupId is string {
  return (
    groupId !== null &&
    /^[1-9]\d*$/.test(groupId) &&
    BigInt(groupId) <= MAX_INT64
  );
}

export function mapOgsGroupLiveResponse(
  wire: OgsGroupLiveWireResponse,
): OgsLiveViewData {
  const pickupTimes = new Map<string, OgsPickupInfo>();
  for (const pt of wire.pickup_times ?? []) {
    pickupTimes.set(pt.student_id, {
      pickupTime: pt.pickup_time,
      isException: pt.is_exception,
      notes: pt.notes,
      dayNotes: (pt.day_notes ?? []).map((note) => ({
        id: note.id,
        content: note.content,
      })),
    });
  }

  return {
    groups: (wire.groups ?? []).map((group) => ({
      id: group.id,
      name: group.name,
      roomId: group.room_id,
      roomName: group.room_name,
      viaSubstitution: group.via_substitution,
    })),
    groupId: wire.group_id ?? null,
    students: wire.students ?? [],
    roomStatus: wire.room_status ?? {},
    pickupTimes,
    trackingIndicators: wire.tracking_indicators ?? {
      labels: [],
      results: {},
    },
    transfers: (wire.transfers ?? []).map((transfer) => ({
      substitutionId: transfer.id,
      groupId: transfer.group_id,
      targetStaffId: transfer.substitute_staff_id,
      targetName: transfer.substitute_name ?? "Unbekannt",
      validUntil: transfer.end_date,
    })),
  };
}

/**
 * Fetches the aggregated live view. When a specific group was requested and
 * the backend answers 403 (e.g. a stale localStorage group the user no longer
 * supervises), one fallback request without group_id resolves the caller's
 * first supervised group instead — the page then re-syncs its selection from
 * the returned groupId.
 */
export async function fetchOgsGroupLive(
  token: string | undefined,
  groupId: string | null,
): Promise<OgsLiveViewData> {
  const load = async (withGroupId: string | null) => {
    const query = withGroupId
      ? `?group_id=${encodeURIComponent(withGroupId)}`
      : "";
    return fetch(`/api/ogs-group-live${query}`, {
      credentials: "include",
      headers: {
        Authorization: `Bearer ${token}`,
        "Content-Type": "application/json",
      },
    });
  };

  const requestedGroupId = isValidGroupId(groupId) ? groupId : null;
  let response = await load(requestedGroupId);
  if (response.status === 403 && requestedGroupId) {
    response = await load(null);
  }
  if (!response.ok) {
    throw new Error(`API error: ${response.status}`);
  }

  const json = (await response.json()) as {
    success: boolean;
    data: OgsGroupLiveWireResponse;
  };
  return mapOgsGroupLiveResponse(json.data);
}
