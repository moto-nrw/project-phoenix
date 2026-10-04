import { describe, it, expect } from "vitest";
import {
  validateDataRetentionDays,
  validateStudentForm,
} from "./student-form-validation";
import type { Student } from "~/lib/student-helpers";

describe("validateDataRetentionDays", () => {
  it("returns error message when retention days is null", () => {
    const result = validateDataRetentionDays(null);
    expect(result).toBe("Bitte geben Sie eine Zahl von 1 bis 31 ein.");
  });

  it("returns error message when retention days is undefined", () => {
    const result = validateDataRetentionDays(undefined);
    expect(result).toBe("Bitte geben Sie eine Zahl von 1 bis 31 ein.");
  });

  it("returns error message when retention days is less than 1", () => {
    const result = validateDataRetentionDays(0);
    expect(result).toBe("Bitte geben Sie eine Zahl von 1 bis 31 ein.");
  });

  it("returns error message when retention days is greater than 31", () => {
    const result = validateDataRetentionDays(32);
    expect(result).toBe("Bitte geben Sie eine Zahl von 1 bis 31 ein.");
  });

  it("returns undefined for valid retention days (1)", () => {
    const result = validateDataRetentionDays(1);
    expect(result).toBeUndefined();
  });

  it("returns undefined for valid retention days (31)", () => {
    const result = validateDataRetentionDays(31);
    expect(result).toBeUndefined();
  });

  it("returns undefined for valid retention days (15)", () => {
    const result = validateDataRetentionDays(15);
    expect(result).toBeUndefined();
  });
});

describe("validateStudentForm", () => {
  it("returns empty object when all required fields are valid", () => {
    const formData: Partial<Student> = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "5a",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, {
      firstName: true,
      lastName: true,
      schoolClass: true,
    });

    expect(errors).toEqual({});
  });

  it("requires a companion note when an accompanied day is selected", () => {
    const errors = validateStudentForm({
      data_retention_days: 30,
      allowed_departure_modes: { mon: ["accompanied"] },
      departure_companion_note: "   ",
    });

    expect(errors.departure_companion_note).toBe(
      "Bitte geben Sie an, mit wem das Kind nach Hause geht.",
    );
  });

  it("accepts a filled companion note for an accompanied day", () => {
    const errors = validateStudentForm({
      data_retention_days: 30,
      allowed_departure_modes: { mon: ["accompanied"] },
      departure_companion_note: "Geschwisterkind Lena",
    });

    expect(errors.departure_companion_note).toBeUndefined();
  });

  it("accepts links covering every accompanied day without a note", () => {
    const errors = validateStudentForm(
      {
        data_retention_days: 30,
        allowed_departure_modes: { mon: ["accompanied"], tue: ["accompanied"] },
      },
      {},
      { companionLinkDays: ["mon", "tue"] },
    );

    expect(errors.departure_companion_note).toBeUndefined();
  });

  it("requires a note for an accompanied day no link covers", () => {
    // A Monday link answers nothing for an accompanied Tuesday — the cover is
    // per weekday, mirroring the backend's Student.Validate.
    const errors = validateStudentForm(
      {
        data_retention_days: 30,
        allowed_departure_modes: { mon: ["accompanied"], tue: ["accompanied"] },
      },
      {},
      { companionLinkDays: ["mon"] },
    );

    expect(errors.departure_companion_note).toBe(
      "Bitte geben Sie an, mit wem das Kind nach Hause geht.",
    );
  });

  it("skips the companion check while the stored links are unknown", () => {
    const errors = validateStudentForm(
      {
        data_retention_days: 30,
        allowed_departure_modes: { mon: ["accompanied"] },
      },
      {},
      { companionLinkDays: "unknown" },
    );

    expect(errors.departure_companion_note).toBeUndefined();
  });

  it("does not require a companion note without an accompanied day", () => {
    const errors = validateStudentForm({
      data_retention_days: 30,
      allowed_departure_modes: { mon: ["bus"] },
    });

    expect(errors.departure_companion_note).toBeUndefined();
  });

  it("validates first name when required", () => {
    const formData: Partial<Student> = {
      first_name: "",
      second_name: "Mustermann",
      school_class: "5a",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, { firstName: true });

    expect(errors.first_name).toBe("Bitte geben Sie den Vornamen ein.");
  });

  it("validates last name when required", () => {
    const formData: Partial<Student> = {
      first_name: "Max",
      second_name: "",
      school_class: "5a",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, { lastName: true });

    expect(errors.last_name).toBe("Bitte geben Sie den Nachnamen ein.");
  });

  it("validates school class when required", () => {
    const formData: Partial<Student> = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, { schoolClass: true });

    expect(errors.school_class).toBe("Bitte geben Sie die Klasse ein.");
  });

  it("validates trimmed values", () => {
    const formData: Partial<Student> = {
      first_name: "  ",
      second_name: "  ",
      school_class: "  ",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, {
      firstName: true,
      lastName: true,
      schoolClass: true,
    });

    expect(errors.first_name).toBe("Bitte geben Sie den Vornamen ein.");
    expect(errors.last_name).toBe("Bitte geben Sie den Nachnamen ein.");
    expect(errors.school_class).toBe("Bitte geben Sie die Klasse ein.");
  });

  it("validates undefined data retention days", () => {
    const formData: Partial<Student> = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "5a",
    };

    const errors = validateStudentForm(formData, {});

    expect(errors.data_retention_days).toBe(
      "Bitte geben Sie eine Zahl von 1 bis 31 ein.",
    );
  });

  it("validates invalid data retention days", () => {
    const formData: Partial<Student> = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "5a",
      data_retention_days: 35,
    };

    const errors = validateStudentForm(formData, {});

    expect(errors.data_retention_days).toBe(
      "Bitte geben Sie eine Zahl von 1 bis 31 ein.",
    );
  });

  it("returns multiple errors when multiple fields invalid", () => {
    const formData: Partial<Student> = {
      first_name: "",
      second_name: "",
      school_class: "",
      data_retention_days: 0,
    };

    const errors = validateStudentForm(formData, {
      firstName: true,
      lastName: true,
      schoolClass: true,
    });

    expect(errors.first_name).toBe("Bitte geben Sie den Vornamen ein.");
    expect(errors.last_name).toBe("Bitte geben Sie den Nachnamen ein.");
    expect(errors.school_class).toBe("Bitte geben Sie die Klasse ein.");
    expect(errors.data_retention_days).toBe(
      "Bitte geben Sie eine Zahl von 1 bis 31 ein.",
    );
  });

  it("does not validate optional fields", () => {
    const formData: Partial<Student> = {
      first_name: "",
      second_name: "",
      school_class: "",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, {});

    expect(errors.first_name).toBeUndefined();
    expect(errors.last_name).toBeUndefined();
    expect(errors.school_class).toBeUndefined();
  });
});
