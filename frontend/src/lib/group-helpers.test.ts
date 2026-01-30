import { describe, it, expect } from "vitest";
import type {
  BackendGroup,
  BackendCombinedGroup,
  Group,
  CombinedGroup,
} from "./group-helpers";
import {
  mapGroupResponse,
  mapGroupsResponse,
  mapCombinedGroupResponse,
  mapCombinedGroupsResponse,
  mapSingleGroupResponse,
  mapSingleCombinedGroupResponse,
  prepareGroupForBackend,
  prepareCombinedGroupForBackend,
  formatGroupName,
  formatGroupLocation,
  formatGroupRepresentative,
  formatCombinedGroupStatus,
  formatCombinedGroupValidity,
  getAccessPolicyName,
} from "./group-helpers";
import { suppressConsole } from "~/test/helpers/console";
import { buildBackendGroup } from "~/test/fixtures";

// Sample backend group for testing
const sampleBackendGroup = buildBackendGroup({
  id: 1,
  name: "Klasse 3a",
  room_id: 10,
  room: {
    id: 10,
    name: "Raum 101",
  },
  representative_id: 5,
  representative: {
    id: 5,
    staff_id: 50,
    first_name: "Maria",
    last_name: "Schmidt",
    full_name: "Maria Schmidt",
    specialization: "Mathematics",
    role: "teacher",
  },
  teachers: [
    {
      id: 5,
      staff_id: 50,
      first_name: "Maria",
      last_name: "Schmidt",
      full_name: "Maria Schmidt",
      specialization: "Mathematics",
      role: "teacher",
    },
    {
      id: 6,
      staff_id: 51,
      first_name: "Peter",
      last_name: "Mueller",
      full_name: "Peter Mueller",
      specialization: "German",
      role: "teacher",
    },
  ],
  student_count: 25,
  supervisor_count: 2,
  students: [
    {
      id: 1,
      name: "Max Mustermann",
      school_class: "3a",
      current_location: "Anwesend",
    },
    { id: 2, name: "Anna Schmidt", school_class: "3a", current_location: null },
  ],
});

const sampleBackendCombinedGroup: BackendCombinedGroup = {
  id: 100,
  name: "Combined Group A",
  is_active: true,
  created_at: "2024-01-01T00:00:00Z",
  valid_until: "2024-12-31T23:59:59Z",
  access_policy: "all",
  specific_group_id: undefined,
  groups: [sampleBackendGroup],
  access_specialists: [{ id: 1, name: "Dr. Smith" }],
  is_expired: false,
  group_count: 1,
  specialist_count: 1,
  time_until_expiration: "11 months",
};

describe("mapGroupResponse", () => {
  it("maps backend group to frontend structure", () => {
    const result = mapGroupResponse(sampleBackendGroup);

    expect(result.id).toBe("1"); // int64 → string
    expect(result.name).toBe("Klasse 3a");
    expect(result.room_id).toBe("10"); // int64 → string
    expect(result.room_name).toBe("Raum 101");
    expect(result.representative_id).toBe("5");
    expect(result.representative_name).toBe("Maria Schmidt");
    expect(result.student_count).toBe(25);
    expect(result.supervisor_count).toBe(2);
    expect(result.created_at).toBe("2024-01-01T00:00:00Z");
    expect(result.updated_at).toBe("2024-01-15T12:00:00Z");
  });

  it("maps representative details when available", () => {
    const result = mapGroupResponse(sampleBackendGroup);

    expect(result.representative).toBeDefined();
    expect(result.representative?.id).toBe("5");
    expect(result.representative?.staffId).toBe("50");
    expect(result.representative?.firstName).toBe("Maria");
    expect(result.representative?.lastName).toBe("Schmidt");
    expect(result.representative?.fullName).toBe("Maria Schmidt");
  });

  it("maps students array with location normalization", () => {
    const result = mapGroupResponse(sampleBackendGroup);

    expect(result.students).toHaveLength(2);
    expect(result.students?.[0]?.id).toBe("1");
    expect(result.students?.[0]?.name).toBe("Max Mustermann");
    expect(result.students?.[0]?.current_location).toBe("Anwesend");
    // null location should be normalized to "Unbekannt"
    expect(result.students?.[1]?.current_location).toBe("Unbekannt");
  });

  it("maps teachers to supervisors and extracts teacher_ids", () => {
    const result = mapGroupResponse(sampleBackendGroup);

    expect(result.supervisors).toHaveLength(2);
    expect(result.supervisors?.[0]?.id).toBe("5");
    expect(result.supervisors?.[0]?.name).toBe("Maria Schmidt");
    expect(result.supervisors?.[1]?.id).toBe("6");
    expect(result.supervisors?.[1]?.name).toBe("Peter Mueller");

    expect(result.teacher_ids).toEqual(["5", "6"]);
  });

  it("handles group without room", () => {
    const groupWithoutRoom: BackendGroup = {
      ...sampleBackendGroup,
      room_id: undefined,
      room: undefined,
    };

    const result = mapGroupResponse(groupWithoutRoom);

    expect(result.room_id).toBeUndefined();
    expect(result.room_name).toBeUndefined();
  });

  it("handles group without representative", () => {
    const groupWithoutRep: BackendGroup = {
      ...sampleBackendGroup,
      representative_id: undefined,
      representative: undefined,
    };

    const result = mapGroupResponse(groupWithoutRep);

    expect(result.representative_id).toBeUndefined();
    expect(result.representative_name).toBeUndefined();
    expect(result.representative).toBeUndefined();
  });

  it("handles group without students array", () => {
    const groupWithoutStudents: BackendGroup = {
      ...sampleBackendGroup,
      students: undefined,
    };

    const result = mapGroupResponse(groupWithoutStudents);

    expect(result.students).toBeUndefined();
  });

  it("handles group without teachers array", () => {
    const groupWithoutTeachers: BackendGroup = {
      ...sampleBackendGroup,
      teachers: undefined,
    };

    const result = mapGroupResponse(groupWithoutTeachers);

    expect(result.supervisors).toBeUndefined();
    expect(result.teacher_ids).toBeUndefined();
  });

  it("handles student with missing name", () => {
    const groupWithUnnamedStudent: BackendGroup = {
      ...sampleBackendGroup,
      students: [{ id: 1, school_class: "3a" }],
    };

    const result = mapGroupResponse(groupWithUnnamedStudent);

    expect(result.students?.[0]?.name).toBe("Unnamed Student");
  });
});

describe("mapGroupsResponse", () => {
  const consoleSpies = suppressConsole("error");

  it("maps array of backend groups", () => {
    const groups = [sampleBackendGroup, { ...sampleBackendGroup, id: 2 }];

    const result = mapGroupsResponse(groups);

    expect(result).toHaveLength(2);
    expect(result[0]?.id).toBe("1");
    expect(result[1]?.id).toBe("2");
  });

  it("handles empty array", () => {
    const result = mapGroupsResponse([]);

    expect(result).toEqual([]);
  });

  it("returns empty array for non-array input", () => {
    const result = mapGroupsResponse("invalid" as unknown as BackendGroup[]);

    expect(result).toEqual([]);
    expect(consoleSpies.error).toHaveBeenCalledWith(
      "Expected array for backendGroups, got:",
      "invalid",
    );
  });
});

describe("mapCombinedGroupResponse", () => {
  it("maps backend combined group to frontend structure", () => {
    const result = mapCombinedGroupResponse(sampleBackendCombinedGroup);

    expect(result.id).toBe("100");
    expect(result.name).toBe("Combined Group A");
    expect(result.is_active).toBe(true);
    expect(result.created_at).toBe("2024-01-01T00:00:00Z");
    expect(result.valid_until).toBe("2024-12-31T23:59:59Z");
    expect(result.access_policy).toBe("all");
    expect(result.is_expired).toBe(false);
    expect(result.group_count).toBe(1);
    expect(result.specialist_count).toBe(1);
    expect(result.time_until_expiration).toBe("11 months");
  });

  it("maps nested groups", () => {
    const result = mapCombinedGroupResponse(sampleBackendCombinedGroup);

    expect(result.groups).toHaveLength(1);
    expect(result.groups?.[0]?.id).toBe("1");
    expect(result.groups?.[0]?.name).toBe("Klasse 3a");
  });

  it("maps access specialists", () => {
    const result = mapCombinedGroupResponse(sampleBackendCombinedGroup);

    expect(result.access_specialists).toHaveLength(1);
    expect(result.access_specialists?.[0]?.id).toBe("1");
    expect(result.access_specialists?.[0]?.name).toBe("Dr. Smith");
  });

  it("handles combined group with specific_group", () => {
    const withSpecificGroup: BackendCombinedGroup = {
      ...sampleBackendCombinedGroup,
      access_policy: "specific",
      specific_group_id: 1,
      specific_group: sampleBackendGroup,
    };

    const result = mapCombinedGroupResponse(withSpecificGroup);

    expect(result.access_policy).toBe("specific");
    expect(result.specific_group_id).toBe("1");
    expect(result.specific_group?.id).toBe("1");
    expect(result.specific_group?.name).toBe("Klasse 3a");
  });

  it("handles combined group without optional fields", () => {
    const minimalCombinedGroup: BackendCombinedGroup = {
      id: 100,
      name: "Minimal Group",
      is_active: false,
      created_at: "2024-01-01T00:00:00Z",
      access_policy: "manual",
    };

    const result = mapCombinedGroupResponse(minimalCombinedGroup);

    expect(result.groups).toBeUndefined();
    expect(result.access_specialists).toBeUndefined();
    expect(result.valid_until).toBeUndefined();
    expect(result.specific_group_id).toBeUndefined();
  });
});

describe("mapCombinedGroupsResponse", () => {
  it("maps array of backend combined groups", () => {
    const groups = [
      sampleBackendCombinedGroup,
      { ...sampleBackendCombinedGroup, id: 101, name: "Combined Group B" },
    ];

    const result = mapCombinedGroupsResponse(groups);

    expect(result).toHaveLength(2);
    expect(result[0]?.id).toBe("100");
    expect(result[1]?.id).toBe("101");
  });
});

describe("mapSingleGroupResponse", () => {
  it("extracts and maps group from { data: group } wrapper", () => {
    const response = { data: sampleBackendGroup };

    const result = mapSingleGroupResponse(response);

    expect(result.id).toBe("1");
    expect(result.name).toBe("Klasse 3a");
  });
});

describe("mapSingleCombinedGroupResponse", () => {
  it("extracts and maps combined group from { data: group } wrapper", () => {
    const response = { data: sampleBackendCombinedGroup };

    const result = mapSingleCombinedGroupResponse(response);

    expect(result.id).toBe("100");
    expect(result.name).toBe("Combined Group A");
  });
});

describe("prepareGroupForBackend", () => {
  it("converts frontend group to backend format", () => {
    const frontendGroup: Partial<Group> = {
      id: "1",
      name: "Klasse 3a",
      room_id: "10",
      representative_id: "5",
      teacher_ids: ["5", "6"],
    };

    const result = prepareGroupForBackend(frontendGroup);

    expect(result.id).toBe(1); // string → number
    expect(result.name).toBe("Klasse 3a");
    expect(result.room_id).toBe(10);
    expect(result.representative_id).toBe(5);
    expect(result.teacher_ids).toEqual([5, 6]); // string[] → number[]
  });

  it("handles group without optional ids", () => {
    const frontendGroup: Partial<Group> = {
      name: "New Group",
    };

    const result = prepareGroupForBackend(frontendGroup);

    expect(result.id).toBeUndefined();
    expect(result.name).toBe("New Group");
    expect(result.room_id).toBeUndefined();
    expect(result.representative_id).toBeUndefined();
  });

  it("handles empty teacher_ids", () => {
    const frontendGroup: Partial<Group> = {
      name: "Test Group",
      teacher_ids: [],
    };

    const result = prepareGroupForBackend(frontendGroup);

    expect(result.teacher_ids).toEqual([]);
  });
});

describe("prepareCombinedGroupForBackend", () => {
  it("converts frontend combined group to backend format", () => {
    const frontendGroup: Partial<CombinedGroup> = {
      id: "100",
      name: "Combined Group A",
      is_active: true,
      valid_until: "2024-12-31T23:59:59Z",
      access_policy: "all",
      groups: [{ id: "1", name: "Klasse 3a", isOccupied: false } as Group],
      access_specialists: [{ id: "1", name: "Dr. Smith" }],
    };

    const result = prepareCombinedGroupForBackend(frontendGroup);

    expect(result.id).toBe(100);
    expect(result.name).toBe("Combined Group A");
    expect(result.is_active).toBe(true);
    expect(result.valid_until).toBe("2024-12-31T23:59:59Z");
    expect(result.access_policy).toBe("all");
  });

  it("creates full BackendGroup objects for nested groups", () => {
    const frontendGroup: Partial<CombinedGroup> = {
      groups: [
        {
          id: "1",
          name: "Klasse 3a",
          created_at: "2024-01-01T00:00:00Z",
          updated_at: "2024-01-15T12:00:00Z",
        } as Group,
      ],
    };

    const result = prepareCombinedGroupForBackend(frontendGroup);

    expect(result.groups).toHaveLength(1);
    expect(result.groups?.[0]?.id).toBe(1);
    expect(result.groups?.[0]?.name).toBe("Klasse 3a");
    expect(result.groups?.[0]?.created_at).toBe("2024-01-01T00:00:00Z");
  });

  it("generates timestamps for groups without dates", () => {
    const frontendGroup: Partial<CombinedGroup> = {
      groups: [{ id: "1", name: "Test" } as Group],
    };

    const result = prepareCombinedGroupForBackend(frontendGroup);

    expect(result.groups?.[0]?.created_at).toBeDefined();
    expect(result.groups?.[0]?.updated_at).toBeDefined();
  });

  it("converts access_specialists ids to numbers", () => {
    const frontendGroup: Partial<CombinedGroup> = {
      access_specialists: [
        { id: "1", name: "Dr. Smith" },
        { id: "2", name: "Prof. Jones" },
      ],
    };

    const result = prepareCombinedGroupForBackend(frontendGroup);

    expect(result.access_specialists?.[0]?.id).toBe(1);
    expect(result.access_specialists?.[1]?.id).toBe(2);
  });

  it("handles specific_group_id conversion", () => {
    const frontendGroup: Partial<CombinedGroup> = {
      access_policy: "specific",
      specific_group_id: "5",
    };

    const result = prepareCombinedGroupForBackend(frontendGroup);

    expect(result.specific_group_id).toBe(5);
  });
});

describe("formatGroupName", () => {
  it("returns group name when available", () => {
    const group: Group = {
      id: "1",
      name: "Klasse 3a",
    };

    const result = formatGroupName(group);

    expect(result).toBe("Klasse 3a");
  });

  it("returns 'Unnamed Group' when name is empty", () => {
    const group: Group = {
      id: "1",
      name: "",
    };

    const result = formatGroupName(group);

    expect(result).toBe("Unnamed Group");
  });
});

describe("formatGroupLocation", () => {
  it("returns room name when available", () => {
    const group: Group = {
      id: "1",
      name: "Test",
      room_name: "Raum 101",
    };

    const result = formatGroupLocation(group);

    expect(result).toBe("Raum 101");
  });

  it("returns 'No Room Assigned' when room_name is undefined", () => {
    const group: Group = {
      id: "1",
      name: "Test",
    };

    const result = formatGroupLocation(group);

    expect(result).toBe("No Room Assigned");
  });
});

describe("formatGroupRepresentative", () => {
  it("returns representative name when available", () => {
    const group: Group = {
      id: "1",
      name: "Test",
      representative_name: "Maria Schmidt",
    };

    const result = formatGroupRepresentative(group);

    expect(result).toBe("Maria Schmidt");
  });

  it("returns 'No Representative' when representative_name is undefined", () => {
    const group: Group = {
      id: "1",
      name: "Test",
    };

    const result = formatGroupRepresentative(group);

    expect(result).toBe("No Representative");
  });
});

describe("formatCombinedGroupStatus", () => {
  it("returns 'Inactive' for inactive groups", () => {
    const group: CombinedGroup = {
      id: "1",
      name: "Test",
      is_active: false,
      access_policy: "all",
    };

    const result = formatCombinedGroupStatus(group);

    expect(result).toBe("Inactive");
  });

  it("returns 'Expired' for active but expired groups", () => {
    const group: CombinedGroup = {
      id: "1",
      name: "Test",
      is_active: true,
      is_expired: true,
      access_policy: "all",
    };

    const result = formatCombinedGroupStatus(group);

    expect(result).toBe("Expired");
  });

  it("returns 'Active' for active non-expired groups", () => {
    const group: CombinedGroup = {
      id: "1",
      name: "Test",
      is_active: true,
      is_expired: false,
      access_policy: "all",
    };

    const result = formatCombinedGroupStatus(group);

    expect(result).toBe("Active");
  });
});

describe("formatCombinedGroupValidity", () => {
  it("returns valid_until when available", () => {
    const group: CombinedGroup = {
      id: "1",
      name: "Test",
      is_active: true,
      valid_until: "2024-12-31T23:59:59Z",
      access_policy: "all",
    };

    const result = formatCombinedGroupValidity(group);

    expect(result).toBe("2024-12-31T23:59:59Z");
  });

  it("returns 'No expiration' when valid_until is undefined", () => {
    const group: CombinedGroup = {
      id: "1",
      name: "Test",
      is_active: true,
      access_policy: "all",
    };

    const result = formatCombinedGroupValidity(group);

    expect(result).toBe("No expiration");
  });
});

describe("getAccessPolicyName", () => {
  it("translates known policy names", () => {
    expect(getAccessPolicyName("all")).toBe("All Specialists");
    expect(getAccessPolicyName("first")).toBe("First Specialist");
    expect(getAccessPolicyName("specific")).toBe("Specific Specialist");
    expect(getAccessPolicyName("manual")).toBe("Manual Assignment");
  });

  it("returns original value for unknown policies", () => {
    expect(getAccessPolicyName("custom")).toBe("custom");
    expect(getAccessPolicyName("unknown_policy")).toBe("unknown_policy");
  });
});
