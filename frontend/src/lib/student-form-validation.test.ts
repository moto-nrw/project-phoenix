import { describe, it, expect } from "vitest";
import { validateStudentForm } from "./student-form-validation";
import type { Student } from "~/lib/student-helpers";

describe("validateStudentForm retention days", () => {
  const RETENTION_HINT = "Bitte geben Sie eine Zahl von 1 bis 31 ein.";

  it.each([null, undefined, 0, 32])("marks %s as outside 1 to 31", (days) => {
    expect(
      // The number field hands null for an emptied input.
      validateStudentForm({
        data_retention_days: days,
      } as unknown as Partial<Student>),
    ).toMatchObject({ data_retention_days: RETENTION_HINT });
  });

  it.each([1, 15, 31])("accepts %i", (days) => {
    expect(validateStudentForm({ data_retention_days: days })).toEqual({});
  });

  // A form that leaves the consent pair out writes none (the school default
  // applies), so there is nothing to check.
  it("checks nothing when the form carries no retention", () => {
    expect(validateStudentForm({})).toEqual({});
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
