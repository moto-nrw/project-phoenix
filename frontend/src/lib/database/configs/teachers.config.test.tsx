/**
 * Tests for Teachers Configuration
 * Tests teacher config structure and mapping functions
 */
import { describe, it, expect, vi } from "vitest";
import { teachersConfig } from "./teachers.config";
import type { Teacher } from "@/lib/teacher-api";

// Mock teacher-api
vi.mock("@/lib/teacher-api", () => ({
  teacherService: {
    createTeacher: vi.fn(),
    updateTeacher: vi.fn(),
  },
}));

describe("teachersConfig", () => {
  it("exports a valid entity config", () => {
    expect(teachersConfig).toBeDefined();
    expect(teachersConfig.name).toEqual({
      singular: "Pädagogische Fachkraft",
      plural: "Pädagogische Fachkräfte",
    });
  });

  it("has correct API configuration", () => {
    expect(teachersConfig.api.basePath).toBe("/api/staff");
  });

  it("has form sections configured", () => {
    expect(teachersConfig.form.sections).toHaveLength(3);
  });

  it("has required form fields", () => {
    const allFields = teachersConfig.form.sections.flatMap((s) => s.fields);
    const fieldNames = allFields.map((f) => f.name);

    expect(fieldNames).toContain("first_name");
    expect(fieldNames).toContain("last_name");
    expect(fieldNames).toContain("email");
    expect(fieldNames).toContain("specialization");
    expect(fieldNames).toContain("password");
  });

  it("has default values", () => {
    expect(teachersConfig.form.defaultValues?.specialization).toBe("");
    expect(teachersConfig.form.defaultValues?.role).toBe("");
  });

  it("validates required fields", () => {
    const validation = teachersConfig.form.validation?.({
      first_name: "",
      last_name: "",
    });

    expect(validation).toBeDefined();
    expect(validation?.first_name).toBeDefined();
    expect(validation?.last_name).toBeDefined();
  });

  it("validates password for new teacher", () => {
    const validation = teachersConfig.form.validation?.({
      first_name: "Max",
      last_name: "Mustermann",
      specialization: "Math",
    });

    expect(validation?.password).toBeDefined();
  });

  it("does not require password for existing teacher", () => {
    const validation = teachersConfig.form.validation?.({
      id: "1",
      first_name: "Max",
      last_name: "Mustermann",
      specialization: "Math",
    });

    expect(validation).toBeNull();
  });

  it("has detail header configuration", () => {
    const mockTeacher: Teacher = {
      id: "1",
      name: "Max Mustermann",
      first_name: "Max",
      last_name: "Mustermann",
      specialization: "Mathematics",
      email: "max@example.com",
    };

    expect(teachersConfig.detail.header?.title(mockTeacher)).toBe(
      "Max Mustermann",
    );
  });

  it("falls back to first_name and last_name", () => {
    const mockTeacher = {
      id: "1",
      first_name: "Max",
      last_name: "Mustermann",
      specialization: "Mathematics",
    } as unknown as Teacher;

    expect(teachersConfig.detail.header?.title(mockTeacher)).toBe(
      "Max Mustermann",
    );
  });

  it("shows specialization and role in subtitle", () => {
    const mockTeacher: Teacher = {
      id: "1",
      name: "Max Mustermann",
      first_name: "Max",
      last_name: "Mustermann",
      specialization: "Mathematics",
      role: "Gruppenleiter",
    };

    const subtitle = teachersConfig.detail.header?.subtitle?.(mockTeacher);
    expect(subtitle).toContain("Mathematics");
    expect(subtitle).toContain("Gruppenleiter");
  });

  it("has list configuration", () => {
    expect(teachersConfig.list.title).toBe("Pädagogische Fachkraft auswählen");
    expect(teachersConfig.list.searchStrategy).toBe("frontend");
  });

  it("displays teacher in list", () => {
    const mockTeacher: Teacher = {
      id: "1",
      name: "Max Mustermann",
      first_name: "Max",
      last_name: "Mustermann",
      specialization: "Mathematics",
    };

    const title = teachersConfig.list.item.title(mockTeacher);
    expect(title).toBe("Max Mustermann");
  });

  it("shows avatar initials", () => {
    const mockTeacher: Teacher = {
      id: "1",
      name: "Max Mustermann",
      first_name: "Max",
      last_name: "Mustermann",
      specialization: "Mathematics",
    };

    const initials = teachersConfig.list.item.avatar?.text(mockTeacher);
    expect(initials).toBe("MM");
  });

  it("has custom labels", () => {
    expect(teachersConfig.labels?.createButton).toBe(
      "Neue pädagogische Fachkraft erstellen",
    );
    expect(teachersConfig.labels?.deleteConfirmation).toContain("löschen");
  });

  it("has onCreateSuccess callback", () => {
    expect(teachersConfig.onCreateSuccess).toBeDefined();
  });
});
