import { describe, it, expect, vi, beforeEach } from "vitest";
import {
  validateDataRetentionDays,
  validateStudentForm,
  handleStudentFormSubmit,
} from "./student-form-validation";

// Mock the Student type from student-helpers
vi.mock("~/lib/student-helpers", () => ({}));

interface MockStudent {
  id?: string;
  first_name?: string;
  second_name?: string;
  school_class?: string;
  data_retention_days?: number | null;
}

describe("validateDataRetentionDays", () => {
  it("returns error message when retention days is null", () => {
    const result = validateDataRetentionDays(null);
    expect(result).toBe("Aufbewahrungsdauer ist erforderlich (1-31 Tage)");
  });

  it("returns error message when retention days is undefined", () => {
    const result = validateDataRetentionDays(undefined);
    expect(result).toBe("Aufbewahrungsdauer ist erforderlich (1-31 Tage)");
  });

  it("returns error message when retention days is less than 1", () => {
    const result = validateDataRetentionDays(0);
    expect(result).toBe(
      "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen",
    );
  });

  it("returns error message when retention days is greater than 31", () => {
    const result = validateDataRetentionDays(32);
    expect(result).toBe(
      "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen",
    );
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
    const formData: MockStudent = {
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

  it("validates first name when required", () => {
    const formData: MockStudent = {
      first_name: "",
      second_name: "Mustermann",
      school_class: "5a",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, { firstName: true });

    expect(errors.first_name).toBe("Vorname ist erforderlich");
  });

  it("validates last name when required", () => {
    const formData: MockStudent = {
      first_name: "Max",
      second_name: "",
      school_class: "5a",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, { lastName: true });

    expect(errors.second_name).toBe("Nachname ist erforderlich");
  });

  it("validates school class when required", () => {
    const formData: MockStudent = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, { schoolClass: true });

    expect(errors.school_class).toBe("Klasse ist erforderlich");
  });

  it("validates trimmed values", () => {
    const formData: MockStudent = {
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

    expect(errors.first_name).toBe("Vorname ist erforderlich");
    expect(errors.second_name).toBe("Nachname ist erforderlich");
    expect(errors.school_class).toBe("Klasse ist erforderlich");
  });

  it("validates data retention days", () => {
    const formData: MockStudent = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "5a",
      data_retention_days: null,
    };

    const errors = validateStudentForm(formData, {});

    expect(errors.data_retention_days).toBe(
      "Aufbewahrungsdauer ist erforderlich (1-31 Tage)",
    );
  });

  it("validates invalid data retention days", () => {
    const formData: MockStudent = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "5a",
      data_retention_days: 35,
    };

    const errors = validateStudentForm(formData, {});

    expect(errors.data_retention_days).toBe(
      "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen",
    );
  });

  it("returns multiple errors when multiple fields invalid", () => {
    const formData: MockStudent = {
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

    expect(errors.first_name).toBe("Vorname ist erforderlich");
    expect(errors.second_name).toBe("Nachname ist erforderlich");
    expect(errors.school_class).toBe("Klasse ist erforderlich");
    expect(errors.data_retention_days).toBe(
      "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen",
    );
  });

  it("does not validate optional fields", () => {
    const formData: MockStudent = {
      first_name: "",
      second_name: "",
      school_class: "",
      data_retention_days: 30,
    };

    const errors = validateStudentForm(formData, {});

    expect(errors.first_name).toBeUndefined();
    expect(errors.second_name).toBeUndefined();
    expect(errors.school_class).toBeUndefined();
  });
});

describe("handleStudentFormSubmit", () => {
  let mockEvent: { preventDefault: ReturnType<typeof vi.fn> };
  let mockValidateForm: ReturnType<typeof vi.fn>;
  let mockOnSubmit: ReturnType<typeof vi.fn>;
  let mockSetLoading: ReturnType<typeof vi.fn>;
  let mockSetErrors: ReturnType<typeof vi.fn>;
  let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    mockEvent = { preventDefault: vi.fn() };
    mockValidateForm = vi.fn();
    mockOnSubmit = vi.fn();
    mockSetLoading = vi.fn();
    mockSetErrors = vi.fn();
    consoleErrorSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
  });

  it("prevents default form submission", async () => {
    mockValidateForm.mockReturnValue(false);

    await handleStudentFormSubmit(
      mockEvent as unknown as React.FormEvent,
      {},
      mockValidateForm,
      mockOnSubmit,
      mockSetLoading,
      mockSetErrors,
    );

    expect(mockEvent.preventDefault).toHaveBeenCalled();
  });

  it("returns early when validation fails", async () => {
    mockValidateForm.mockReturnValue(false);

    await handleStudentFormSubmit(
      mockEvent as unknown as React.FormEvent,
      {},
      mockValidateForm,
      mockOnSubmit,
      mockSetLoading,
      mockSetErrors,
    );

    expect(mockValidateForm).toHaveBeenCalled();
    expect(mockSetLoading).not.toHaveBeenCalled();
    expect(mockOnSubmit).not.toHaveBeenCalled();
  });

  it("successfully submits when validation passes", async () => {
    mockValidateForm.mockReturnValue(true);
    mockOnSubmit.mockResolvedValue(undefined);

    const formData: MockStudent = {
      first_name: "Max",
      second_name: "Mustermann",
      school_class: "5a",
      data_retention_days: 30,
    };

    await handleStudentFormSubmit(
      mockEvent as unknown as React.FormEvent,
      formData,
      mockValidateForm,
      mockOnSubmit,
      mockSetLoading,
      mockSetErrors,
    );

    expect(mockSetLoading).toHaveBeenCalledWith(true);
    expect(mockOnSubmit).toHaveBeenCalledWith(formData);
    expect(mockSetLoading).toHaveBeenCalledWith(false);
    expect(mockSetErrors).not.toHaveBeenCalled();
  });

  it("handles submission error", async () => {
    mockValidateForm.mockReturnValue(true);
    const error = new Error("Network error");
    mockOnSubmit.mockRejectedValue(error);

    await handleStudentFormSubmit(
      mockEvent as unknown as React.FormEvent,
      {},
      mockValidateForm,
      mockOnSubmit,
      mockSetLoading,
      mockSetErrors,
    );

    expect(consoleErrorSpy).toHaveBeenCalledWith(
      "Error saving student:",
      error,
    );
    expect(mockSetErrors).toHaveBeenCalledWith({
      submit: "Fehler beim Speichern. Bitte versuchen Sie es erneut.",
    });
  });

  it("always sets loading to false in finally block", async () => {
    mockValidateForm.mockReturnValue(true);
    mockOnSubmit.mockRejectedValue(new Error("Test error"));

    await handleStudentFormSubmit(
      mockEvent as unknown as React.FormEvent,
      {},
      mockValidateForm,
      mockOnSubmit,
      mockSetLoading,
      mockSetErrors,
    );

    // Verify setLoading called with true, then false
    expect(mockSetLoading).toHaveBeenCalledWith(true);
    expect(mockSetLoading).toHaveBeenCalledWith(false);
    expect(mockSetLoading).toHaveBeenCalledTimes(2);
  });

  it("sets loading to false even when submission succeeds", async () => {
    mockValidateForm.mockReturnValue(true);
    mockOnSubmit.mockResolvedValue(undefined);

    await handleStudentFormSubmit(
      mockEvent as unknown as React.FormEvent,
      {},
      mockValidateForm,
      mockOnSubmit,
      mockSetLoading,
      mockSetErrors,
    );

    expect(mockSetLoading).toHaveBeenCalledWith(true);
    expect(mockSetLoading).toHaveBeenCalledWith(false);
  });
});
