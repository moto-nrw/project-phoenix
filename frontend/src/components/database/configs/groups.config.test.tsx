/**
 * Tests for Groups Configuration
 * Tests group config structure and response mapping
 */
import { describe, it, expect, vi } from "vitest";
import { groupsConfig } from "./groups.config";
import type { Group } from "@/lib/group-helpers";

const mockFetch = vi.fn();
global.fetch = mockFetch as unknown as typeof fetch;

// Mock mapGroupResponse
vi.mock("@/lib/group-helpers", async () => {
  const actual = await vi.importActual("@/lib/group-helpers");
  return {
    ...actual,
    mapGroupResponse: vi.fn((data: unknown) => data),
  };
});

describe("groupsConfig", () => {
  it("exports a valid entity config", () => {
    expect(groupsConfig).toBeDefined();
    expect(groupsConfig.name).toEqual({
      singular: "Gruppe",
      plural: "Gruppen",
    });
  });

  it("has correct API configuration", () => {
    expect(groupsConfig.api.basePath).toBe("/api/groups");
  });

  it("has form sections configured", () => {
    expect(groupsConfig.form.sections).toHaveLength(1);
    expect(groupsConfig.form.sections[0]?.title).toBe("Gruppendetails");
  });

  it("has required form fields", () => {
    const fields = groupsConfig.form.sections[0]?.fields ?? [];
    const fieldNames = fields.map((f) => f.name);

    expect(fieldNames).toContain("name");
    expect(fieldNames).toContain("room_id");
    expect(fieldNames).toContain("teacher_ids");
  });

  it("transforms data before submit with empty room_id", () => {
    const data: Partial<Group> = {
      name: "Test Group",
      room_id: "",
    };

    const transformed = groupsConfig.form.transformBeforeSubmit?.(data);
    // Empty string is preserved (not converted to undefined with ??)
    expect(transformed?.room_id).toBe("");
  });

  it("transforms null room_id to undefined", () => {
    const data = {
      name: "Test Group",
      room_id: null as unknown as string | undefined,
    };

    const transformed = groupsConfig.form.transformBeforeSubmit?.(data);
    expect(transformed?.room_id).toBeUndefined();
  });

  it("has detail header configuration", () => {
    const mockGroup: Group = {
      id: "1",
      name: "Group Blue",
      room_name: "Room 101",
      student_count: 15,
    };

    expect(groupsConfig.detail.header?.title(mockGroup)).toBe("Group Blue");
    expect(groupsConfig.detail.header?.subtitle?.(mockGroup)).toBe("Room 101");
  });

  it("shows no room message when not assigned", () => {
    const mockGroup: Group = {
      id: "1",
      name: "Group Blue",
      student_count: 0,
    };

    expect(groupsConfig.detail.header?.subtitle?.(mockGroup)).toBe(
      "Kein Raum zugewiesen",
    );
  });

  it("shows student count badge", () => {
    const mockGroup: Group = {
      id: "1",
      name: "Group Blue",
      student_count: 15,
    };

    const badges = groupsConfig.detail.header?.badges ?? [];
    const countBadge = badges[0];
    expect(countBadge).toBeDefined();
    expect((countBadge!.label as (entity: Group) => string)(mockGroup)).toBe(
      "15 Kinder",
    );
  });

  it("shows room badge when assigned", () => {
    const mockGroup: Group = {
      id: "1",
      name: "Group Blue",
      room_name: "Room 101",
      student_count: 0,
    };

    const badges = groupsConfig.detail.header?.badges ?? [];
    const roomBadge = badges.find((b) => b.label === "Raum zugewiesen");
    expect(roomBadge?.showWhen(mockGroup)).toBe(true);
  });

  it("shows supervisor count badge", () => {
    const mockGroup: Group = {
      id: "1",
      name: "Group Blue",
      student_count: 0,
      supervisors: [
        { id: "1", name: "Max Mustermann" },
        { id: "2", name: "Jane Doe" },
      ],
    };

    const badges = groupsConfig.detail.header?.badges ?? [];
    const supervisorBadge = badges[2];
    expect(supervisorBadge).toBeDefined();
    expect(
      (supervisorBadge!.label as (entity: Group) => string)(mockGroup),
    ).toBe("2 Gruppenleitungen");
  });

  it("maps request data correctly", () => {
    const data: Partial<Group> = {
      name: "Test Group",
      room_id: "5",
      teacher_ids: ["1", "2"],
    };

    const mapped = groupsConfig.service?.mapRequest?.(data);
    expect(mapped).toEqual({
      name: "Test Group",
      room_id: 5,
      teacher_ids: [1, 2],
    });
  });

  it("has list configuration", () => {
    expect(groupsConfig.list.title).toBe("Gruppe auswählen");
    expect(groupsConfig.list.searchStrategy).toBe("frontend");
  });

  it("displays group in list with supervisors", () => {
    const mockGroup: Group = {
      id: "1",
      name: "Group Blue",
      student_count: 15,
      supervisors: [{ id: "1", name: "Max Mustermann" }],
    };

    const subtitle = groupsConfig.list.item.subtitle?.(mockGroup);
    expect(subtitle).toBe("1 Gruppenleitung");
  });

  it("displays no supervisor message in list", () => {
    const mockGroup: Group = {
      id: "1",
      name: "Group Blue",
      student_count: 0,
      supervisors: [],
    };

    const subtitle = groupsConfig.list.item.subtitle?.(mockGroup);
    expect(subtitle).toBe("Keine Gruppenleitung");
  });

  it("has custom labels", () => {
    expect(groupsConfig.labels?.createButton).toBe("Neue Gruppe erstellen");
    expect(groupsConfig.labels?.deleteConfirmation).toContain("löschen");
  });

  it("loads caregiver teacher options from wrapped responses", async () => {
    mockFetch.mockResolvedValueOnce({
      json: async () => ({
        data: [
          {
            id: "10",
            first_name: "Ada",
            last_name: "Lovelace",
            specialization: "Springer",
            teacher_id: "77",
          },
        ],
      }),
    });

    const teacherField = groupsConfig.form.sections[0]?.fields.find(
      (field) => field.name === "teacher_ids",
    );
    const loadOptions =
      teacherField && typeof teacherField.options === "function"
        ? teacherField.options
        : undefined;
    const options = await loadOptions?.();

    expect(mockFetch).toHaveBeenCalledWith("/api/staff/by-role?role=user");
    expect(options).toEqual([
      { value: "77", label: "Ada Lovelace (Springer)" },
    ]);
  });

  it("loads caregiver teacher options from array responses and skips staff without teacher ids", async () => {
    mockFetch.mockResolvedValueOnce({
      json: async () => [
        {
          id: "10",
          full_name: "Ada Lovelace",
          teacher_id: "77",
        },
        {
          id: "11",
          full_name: "Ignored Staff",
        },
      ],
    });

    const teacherField = groupsConfig.form.sections[0]?.fields.find(
      (field) => field.name === "teacher_ids",
    );
    const loadOptions =
      teacherField && typeof teacherField.options === "function"
        ? teacherField.options
        : undefined;
    const options = await loadOptions?.();

    expect(options).toEqual([{ value: "77", label: "Ada Lovelace" }]);
  });

  it("returns an empty teacher option list when caregiver lookup fails", async () => {
    mockFetch.mockRejectedValueOnce(new Error("boom"));

    const teacherField = groupsConfig.form.sections[0]?.fields.find(
      (field) => field.name === "teacher_ids",
    );
    const loadOptions =
      teacherField && typeof teacherField.options === "function"
        ? teacherField.options
        : undefined;
    const options = await loadOptions?.();

    expect(options).toEqual([]);
  });

  it("loads room options from the rooms api", async () => {
    mockFetch.mockResolvedValueOnce({
      json: async () => ({
        data: [
          { id: 7, name: "Raum Sonnenblume" },
          { id: 9, name: "Raum Regenbogen" },
        ],
      }),
    });

    const roomField = groupsConfig.form.sections[0]?.fields.find(
      (field) => field.name === "room_id",
    );
    const loadOptions =
      roomField && typeof roomField.options === "function"
        ? roomField.options
        : undefined;
    const options = await loadOptions?.();

    expect(mockFetch).toHaveBeenCalledWith("/api/rooms");
    expect(options).toEqual([
      { value: "", label: "Kein Gruppenraum" },
      { value: "7", label: "Raum Sonnenblume" },
      { value: "9", label: "Raum Regenbogen" },
    ]);
  });

  it("falls back to the empty room option when room loading fails", async () => {
    mockFetch.mockRejectedValueOnce(new Error("rooms unavailable"));

    const roomField = groupsConfig.form.sections[0]?.fields.find(
      (field) => field.name === "room_id",
    );
    const loadOptions =
      roomField && typeof roomField.options === "function"
        ? roomField.options
        : undefined;
    const options = await loadOptions?.();

    expect(options).toEqual([{ value: "", label: "Kein Gruppenraum" }]);
  });

  it("unwraps wrapped response payloads before mapping", async () => {
    const { mapGroupResponse } = await import("@/lib/group-helpers");

    groupsConfig.service?.mapResponse?.({
      status: "success",
      data: { id: 12, name: "Wrapped Group" },
    });

    expect(vi.mocked(mapGroupResponse)).toHaveBeenCalledWith({
      id: 12,
      name: "Wrapped Group",
    });
  });
});
