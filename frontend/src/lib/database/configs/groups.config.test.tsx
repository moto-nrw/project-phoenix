/**
 * Tests for Groups Configuration
 * Tests group config structure and response mapping
 */
import { describe, it, expect, vi } from "vitest";
import { groupsConfig } from "./groups.config";
import type { Group } from "@/lib/group-helpers";

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
    expect((countBadge?.label as (entity: Group) => string)(mockGroup)).toBe(
      "15 Schüler",
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
    expect(
      (supervisorBadge?.label as (entity: Group) => string)(mockGroup),
    ).toBe("2 Gruppenleiter/innen");
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
    expect(subtitle).toBe("1 Gruppenleiter/in");
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
});
