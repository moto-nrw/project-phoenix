import { describe, it, expect } from "vitest";
import type { BackendGroup, Group } from "./group-helpers";
import {
  mapGroupResponse,
  mapGroupsResponse,
  mapSingleGroupResponse,
  prepareGroupForBackend,
  formatGroupName,
  formatGroupLocation,
  formatGroupRepresentative,
} from "./group-helpers";
import { suppressConsole } from "~/test/helpers/console";
import { buildBackendGroup } from "~/test/fixtures/groups";

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
      "expected array for backendGroups",
      { received: "string" },
    );
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
