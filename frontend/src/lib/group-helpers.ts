import type { Group, CombinedGroup } from "./api";

export interface BackendGroup {
  id: number;
  name: string;
  room_id?: number;
  room?: {
    id: number;
    room_name: string;
  };
  representative_id?: number;
  representative?: {
    id: number;
    custom_user?: {
      first_name: string;
      second_name: string;
    };
  };
  students?: Array<{
    id: number;
    custom_user?: {
      first_name: string;
      second_name: string;
    };
  }>;
  supervisors?: Array<{
    id: number;
    custom_user?: {
      first_name: string;
      second_name: string;
    };
  }>;
  created_at: string;
  updated_at: string;
}

export interface BackendCombinedGroup {
  id: number;
  name: string;
  is_active: boolean;
  created_at: string;
  valid_until?: string;
  access_policy: string;
  specific_group_id?: number;
  specific_group?: BackendGroup;
  groups?: BackendGroup[];
  access_specialists?: Array<{
    id: number;
    custom_user?: {
      first_name: string;
      second_name: string;
    };
  }>;
}

/**
 * Maps a list of backend group responses to frontend group models
 */
export function mapGroupResponse(data: BackendGroup[]): Group[] {
  return data.map((group) => mapSingleGroupResponse(group));
}

/**
 * Maps a single backend group response to frontend group model
 */
export function mapSingleGroupResponse(group: BackendGroup): Group {
  return {
    id: group.id.toString(),
    name: group.name,
    room_id: group.room_id?.toString(),
    room_name: group.room?.room_name,
    representative_id: group.representative_id?.toString(),
    representative_name: group.representative?.custom_user
      ? `${group.representative.custom_user.first_name} ${group.representative.custom_user.second_name}`
      : undefined,
    student_count: group.students?.length ?? 0,
    supervisor_count: group.supervisors?.length ?? 0,
    created_at: group.created_at,
    updated_at: group.updated_at,
    students: group.students?.map((s) => ({
      id: s.id.toString(),
      name: s.custom_user
        ? `${s.custom_user.first_name} ${s.custom_user.second_name}`
        : "Unknown",
      // These fields would be populated from the actual student data if needed
      school_class: "",
      in_house: false,
    })),
    supervisors: group.supervisors?.map((s) => ({
      id: s.id.toString(),
      name: s.custom_user
        ? `${s.custom_user.first_name} ${s.custom_user.second_name}`
        : "Unknown",
    })),
  };
}

/**
 * Maps a list of backend combined group responses to frontend combined group models
 */
export function mapCombinedGroupResponse(
  data: BackendCombinedGroup[],
): CombinedGroup[] {
  return data.map((group) => mapSingleCombinedGroupResponse(group));
}

/**
 * Maps a single backend combined group response to frontend combined group model
 */
export function mapSingleCombinedGroupResponse(
  combinedGroup: BackendCombinedGroup,
): CombinedGroup {
  const now = new Date();
  const validUntil = combinedGroup.valid_until
    ? new Date(combinedGroup.valid_until)
    : undefined;
  const isExpired = validUntil ? validUntil < now : false;

  let timeUntilExpiration;
  if (validUntil && !isExpired) {
    const diffHours = Math.floor(
      (validUntil.getTime() - now.getTime()) / (1000 * 60 * 60),
    );
    if (diffHours > 24) {
      timeUntilExpiration = `≈${Math.floor(diffHours / 24)} days`;
    } else {
      timeUntilExpiration = `≈${diffHours} hours`;
    }
  }

  return {
    id: combinedGroup.id.toString(),
    name: combinedGroup.name,
    is_active: combinedGroup.is_active && !isExpired,
    created_at: combinedGroup.created_at,
    valid_until: combinedGroup.valid_until,
    access_policy: combinedGroup.access_policy as
      | "all"
      | "first"
      | "specific"
      | "manual",
    specific_group_id: combinedGroup.specific_group_id?.toString(),
    specific_group: combinedGroup.specific_group
      ? mapSingleGroupResponse(combinedGroup.specific_group)
      : undefined,
    groups: combinedGroup.groups?.map((g) => mapSingleGroupResponse(g)),
    access_specialists: combinedGroup.access_specialists?.map((s) => ({
      id: s.id.toString(),
      name: s.custom_user
        ? `${s.custom_user.first_name} ${s.custom_user.second_name}`
        : "Unknown",
    })),
    is_expired: isExpired,
    group_count: combinedGroup.groups?.length ?? 0,
    specialist_count: combinedGroup.access_specialists?.length ?? 0,
    time_until_expiration: timeUntilExpiration,
  };
}

/**
 * Prepares a group object for sending to the backend API
 */
export function prepareGroupForBackend(
  group: Partial<Group>,
): Record<string, unknown> {
  const backendGroup: Record<string, unknown> = {};

  // Required fields
  backendGroup.name = group.name ?? "";

  // Optional fields
  if (group.room_id) {
    backendGroup.room_id = parseInt(group.room_id, 10);
  }

  if (group.representative_id) {
    backendGroup.representative_id = parseInt(group.representative_id, 10);
  }

  return backendGroup;
}

/**
 * Prepares a combined group object for sending to the backend API
 */
export function prepareCombinedGroupForBackend(
  combinedGroup: Partial<CombinedGroup>,
): Record<string, unknown> {
  const backendCombinedGroup: Record<string, unknown> = {};

  // Required fields
  backendCombinedGroup.name = combinedGroup.name ?? "";
  backendCombinedGroup.access_policy = combinedGroup.access_policy ?? "manual";
  backendCombinedGroup.is_active = combinedGroup.is_active ?? true;

  // Optional fields
  if (combinedGroup.valid_until) {
    backendCombinedGroup.valid_until = combinedGroup.valid_until;
  }

  if (
    combinedGroup.specific_group_id &&
    combinedGroup.access_policy === "specific"
  ) {
    backendCombinedGroup.specific_group_id = parseInt(
      combinedGroup.specific_group_id,
      10,
    );
  }

  return backendCombinedGroup;
}
