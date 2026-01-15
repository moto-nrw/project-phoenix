import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import {
  getDefaultValueForField,
  hasPrivacyConsentFields,
  extractPrivacyConsent,
  isEmptyValue,
  validateNumberMin,
  validateField,
  validateFormFields,
  DatabaseForm,
} from "./database-form";
import type { FormField, FormSection } from "./database-form";

// =============================================================================
// getDefaultValueForField Tests
// =============================================================================

describe("getDefaultValueForField", () => {
  it("returns false for checkbox fields", () => {
    const field: FormField = {
      name: "test",
      label: "Test",
      type: "checkbox",
    };
    expect(getDefaultValueForField(field)).toBe(false);
  });

  it("returns empty array for multiselect fields", () => {
    const field: FormField = {
      name: "test",
      label: "Test",
      type: "multiselect",
    };
    expect(getDefaultValueForField(field)).toEqual([]);
  });

  it("returns 30 for data_retention_days number field", () => {
    const field: FormField = {
      name: "data_retention_days",
      label: "Aufbewahrungstage",
      type: "number",
    };
    expect(getDefaultValueForField(field)).toBe(30);
  });

  it("returns 0 for other number fields", () => {
    const field: FormField = {
      name: "age",
      label: "Alter",
      type: "number",
    };
    expect(getDefaultValueForField(field)).toBe(0);
  });

  it("returns empty string for text fields", () => {
    const field: FormField = {
      name: "name",
      label: "Name",
      type: "text",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });

  it("returns empty string for email fields", () => {
    const field: FormField = {
      name: "email",
      label: "E-Mail",
      type: "email",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });

  it("returns empty string for select fields", () => {
    const field: FormField = {
      name: "role",
      label: "Rolle",
      type: "select",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });

  it("returns empty string for textarea fields", () => {
    const field: FormField = {
      name: "description",
      label: "Beschreibung",
      type: "textarea",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });

  it("returns empty string for password fields", () => {
    const field: FormField = {
      name: "password",
      label: "Passwort",
      type: "password",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });

  it("returns empty string for date fields", () => {
    const field: FormField = {
      name: "birthdate",
      label: "Geburtsdatum",
      type: "date",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });

  it("returns empty string for custom fields", () => {
    const field: FormField = {
      name: "custom",
      label: "Custom",
      type: "custom",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });
});

// =============================================================================
// hasPrivacyConsentFields Tests
// =============================================================================

describe("hasPrivacyConsentFields", () => {
  it("returns true when privacy_consent_accepted field exists", () => {
    const sections: FormSection[] = [
      {
        title: "Datenschutz",
        fields: [
          {
            name: "privacy_consent_accepted",
            label: "Einwilligung",
            type: "checkbox",
          },
        ],
      },
    ];
    expect(hasPrivacyConsentFields(sections)).toBe(true);
  });

  it("returns true when data_retention_days field exists", () => {
    const sections: FormSection[] = [
      {
        title: "Datenschutz",
        fields: [
          {
            name: "data_retention_days",
            label: "Aufbewahrungstage",
            type: "number",
          },
        ],
      },
    ];
    expect(hasPrivacyConsentFields(sections)).toBe(true);
  });

  it("returns true when both privacy consent fields exist", () => {
    const sections: FormSection[] = [
      {
        title: "Datenschutz",
        fields: [
          {
            name: "privacy_consent_accepted",
            label: "Einwilligung",
            type: "checkbox",
          },
          {
            name: "data_retention_days",
            label: "Aufbewahrungstage",
            type: "number",
          },
        ],
      },
    ];
    expect(hasPrivacyConsentFields(sections)).toBe(true);
  });

  it("returns false when no privacy consent fields exist", () => {
    const sections: FormSection[] = [
      {
        title: "Persönliche Daten",
        fields: [
          { name: "first_name", label: "Vorname", type: "text" },
          { name: "last_name", label: "Nachname", type: "text" },
        ],
      },
    ];
    expect(hasPrivacyConsentFields(sections)).toBe(false);
  });

  it("returns false for empty sections array", () => {
    expect(hasPrivacyConsentFields([])).toBe(false);
  });

  it("returns false when sections have no fields", () => {
    const sections: FormSection[] = [{ title: "Empty Section", fields: [] }];
    expect(hasPrivacyConsentFields(sections)).toBe(false);
  });

  it("searches across multiple sections", () => {
    const sections: FormSection[] = [
      {
        title: "Persönliche Daten",
        fields: [{ name: "first_name", label: "Vorname", type: "text" }],
      },
      {
        title: "Datenschutz",
        fields: [
          {
            name: "privacy_consent_accepted",
            label: "Einwilligung",
            type: "checkbox",
          },
        ],
      },
    ];
    expect(hasPrivacyConsentFields(sections)).toBe(true);
  });
});

// =============================================================================
// extractPrivacyConsent Tests
// =============================================================================

describe("extractPrivacyConsent", () => {
  it("returns consent when data has valid structure", () => {
    const responseData = {
      data: {
        accepted: true,
        data_retention_days: 30,
      },
    };
    const result = extractPrivacyConsent(responseData);
    expect(result).toEqual({
      accepted: true,
      data_retention_days: 30,
    });
  });

  it("returns null for null input", () => {
    expect(extractPrivacyConsent(null)).toBeNull();
  });

  it("returns null for undefined input", () => {
    expect(extractPrivacyConsent(undefined)).toBeNull();
  });

  it("returns null for non-object input", () => {
    expect(extractPrivacyConsent("string")).toBeNull();
    expect(extractPrivacyConsent(123)).toBeNull();
    expect(extractPrivacyConsent(true)).toBeNull();
  });

  it("returns null when data property is missing", () => {
    const responseData = { other: "value" };
    expect(extractPrivacyConsent(responseData)).toBeNull();
  });

  it("returns null when data is null", () => {
    const responseData = { data: null };
    expect(extractPrivacyConsent(responseData)).toBeNull();
  });

  it("returns null when data is not an object", () => {
    const responseData = { data: "string" };
    expect(extractPrivacyConsent(responseData)).toBeNull();
  });

  it("returns null when accepted field is missing", () => {
    const responseData = {
      data: {
        data_retention_days: 30,
      },
    };
    expect(extractPrivacyConsent(responseData)).toBeNull();
  });

  it("returns null when data_retention_days field is missing", () => {
    const responseData = {
      data: {
        accepted: true,
      },
    };
    expect(extractPrivacyConsent(responseData)).toBeNull();
  });

  it("returns consent with false accepted value", () => {
    const responseData = {
      data: {
        accepted: false,
        data_retention_days: 7,
      },
    };
    const result = extractPrivacyConsent(responseData);
    expect(result).toEqual({
      accepted: false,
      data_retention_days: 7,
    });
  });
});

// =============================================================================
// isEmptyValue Tests
// =============================================================================

describe("isEmptyValue", () => {
  it("returns true for undefined", () => {
    expect(isEmptyValue(undefined)).toBe(true);
  });

  it("returns true for null", () => {
    expect(isEmptyValue(null)).toBe(true);
  });

  it("returns true for empty string", () => {
    expect(isEmptyValue("")).toBe(true);
  });

  it("returns false for non-empty string", () => {
    expect(isEmptyValue("hello")).toBe(false);
  });

  it("returns false for zero", () => {
    expect(isEmptyValue(0)).toBe(false);
  });

  it("returns false for false boolean", () => {
    expect(isEmptyValue(false)).toBe(false);
  });

  it("returns false for empty array", () => {
    expect(isEmptyValue([])).toBe(false);
  });

  it("returns false for empty object", () => {
    expect(isEmptyValue({})).toBe(false);
  });

  it("returns false for whitespace string", () => {
    expect(isEmptyValue("  ")).toBe(false);
  });
});

// =============================================================================
// validateNumberMin Tests
// =============================================================================

describe("validateNumberMin", () => {
  it("returns null when number value meets minimum", () => {
    expect(validateNumberMin(5, 1, "Alter")).toBeNull();
    expect(validateNumberMin(10, 10, "Alter")).toBeNull();
  });

  it("returns error when number value is below minimum", () => {
    const result = validateNumberMin(0, 1, "Alter");
    expect(result).toBe("Alter muss mindestens 1 sein.");
  });

  it("returns error for negative value when minimum is positive", () => {
    const result = validateNumberMin(-5, 0, "Wert");
    expect(result).toBe("Wert muss mindestens 0 sein.");
  });

  it("handles string values that can be parsed", () => {
    expect(validateNumberMin("10", 5, "Anzahl")).toBeNull();
    expect(validateNumberMin("3", 5, "Anzahl")).toBe(
      "Anzahl muss mindestens 5 sein.",
    );
  });

  it("returns error for non-parseable string values", () => {
    const result = validateNumberMin("abc", 1, "Wert");
    expect(result).toBe("Wert muss mindestens 1 sein.");
  });

  it("returns error for empty string", () => {
    const result = validateNumberMin("", 1, "Wert");
    expect(result).toBe("Wert muss mindestens 1 sein.");
  });
});

// =============================================================================
// validateField Tests
// =============================================================================

describe("validateField", () => {
  it("returns error for required field with empty value", () => {
    const field: FormField = {
      name: "name",
      label: "Name",
      type: "text",
      required: true,
    };
    expect(validateField(field, "")).toBe("Name ist erforderlich.");
    expect(validateField(field, null)).toBe("Name ist erforderlich.");
    expect(validateField(field, undefined)).toBe("Name ist erforderlich.");
  });

  it("returns null for required field with valid value", () => {
    const field: FormField = {
      name: "name",
      label: "Name",
      type: "text",
      required: true,
    };
    expect(validateField(field, "John")).toBeNull();
  });

  it("returns null for optional field with empty value", () => {
    const field: FormField = {
      name: "nickname",
      label: "Spitzname",
      type: "text",
      required: false,
    };
    expect(validateField(field, "")).toBeNull();
    expect(validateField(field, null)).toBeNull();
  });

  it("validates number field with min constraint", () => {
    const field: FormField = {
      name: "age",
      label: "Alter",
      type: "number",
      required: true,
      min: 1,
    };
    expect(validateField(field, 0)).toBe("Alter muss mindestens 1 sein.");
    expect(validateField(field, 1)).toBeNull();
    expect(validateField(field, 25)).toBeNull();
  });

  it("runs custom validation function", () => {
    const field: FormField = {
      name: "email",
      label: "E-Mail",
      type: "email",
      validation: (value) => {
        if (typeof value !== "string" || !value.includes("@")) {
          return "Ungültige E-Mail-Adresse.";
        }
        return null;
      },
    };
    expect(validateField(field, "invalid")).toBe("Ungültige E-Mail-Adresse.");
    expect(validateField(field, "test@example.com")).toBeNull();
  });

  it("required check runs before custom validation", () => {
    const customValidation = vi.fn().mockReturnValue(null);
    const field: FormField = {
      name: "test",
      label: "Test",
      type: "text",
      required: true,
      validation: customValidation,
    };
    expect(validateField(field, "")).toBe("Test ist erforderlich.");
    expect(customValidation).not.toHaveBeenCalled();
  });

  it("runs custom validation after required check passes", () => {
    const customValidation = vi.fn().mockReturnValue("Custom error");
    const field: FormField = {
      name: "test",
      label: "Test",
      type: "text",
      required: true,
      validation: customValidation,
    };
    expect(validateField(field, "value")).toBe("Custom error");
    expect(customValidation).toHaveBeenCalledWith("value");
  });
});

// =============================================================================
// validateFormFields Tests
// =============================================================================

describe("validateFormFields", () => {
  it("returns null when all fields are valid", () => {
    const sections: FormSection[] = [
      {
        title: "Personal",
        fields: [
          {
            name: "first_name",
            label: "Vorname",
            type: "text",
            required: true,
          },
          {
            name: "last_name",
            label: "Nachname",
            type: "text",
            required: true,
          },
        ],
      },
    ];
    const formData = { first_name: "John", last_name: "Doe" };
    expect(validateFormFields(sections, formData)).toBeNull();
  });

  it("returns first error when validation fails", () => {
    const sections: FormSection[] = [
      {
        title: "Personal",
        fields: [
          {
            name: "first_name",
            label: "Vorname",
            type: "text",
            required: true,
          },
          {
            name: "last_name",
            label: "Nachname",
            type: "text",
            required: true,
          },
        ],
      },
    ];
    const formData = { first_name: "", last_name: "Doe" };
    expect(validateFormFields(sections, formData)).toBe(
      "Vorname ist erforderlich.",
    );
  });

  it("validates across multiple sections", () => {
    const sections: FormSection[] = [
      {
        title: "Personal",
        fields: [
          {
            name: "first_name",
            label: "Vorname",
            type: "text",
            required: true,
          },
        ],
      },
      {
        title: "Contact",
        fields: [
          { name: "email", label: "E-Mail", type: "email", required: true },
        ],
      },
    ];
    const formData = { first_name: "John", email: "" };
    expect(validateFormFields(sections, formData)).toBe(
      "E-Mail ist erforderlich.",
    );
  });

  it("returns null for empty sections", () => {
    expect(validateFormFields([], {})).toBeNull();
  });

  it("handles sections with no required fields", () => {
    const sections: FormSection[] = [
      {
        title: "Optional",
        fields: [
          {
            name: "nickname",
            label: "Spitzname",
            type: "text",
            required: false,
          },
        ],
      },
    ];
    expect(validateFormFields(sections, { nickname: "" })).toBeNull();
  });
});

// =============================================================================
// DatabaseForm Component Tests
// =============================================================================

describe("DatabaseForm", () => {
  const mockTheme = {
    primary: "teal-500",
    secondary: "blue-600",
    accent: "blue" as const,
    background: "blue-50",
    border: "blue-200",
    textAccent: "blue-800",
    icon: null,
    avatarGradient: "from-teal-400 to-blue-500",
  };

  const mockSections: FormSection[] = [
    {
      title: "Test Section",
      fields: [
        {
          name: "test_field",
          label: "Test Field",
          type: "text",
          required: true,
        },
      ],
    },
  ];

  const defaultProps = {
    theme: mockTheme,
    sections: mockSections,
    onSubmit: vi.fn(),
    onCancel: vi.fn(),
    submitLabel: "Speichern",
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("renders form sections and fields", () => {
    render(<DatabaseForm {...defaultProps} />);

    expect(screen.getByText("Test Section")).toBeInTheDocument();
    expect(screen.getByLabelText(/Test Field/)).toBeInTheDocument();
  });

  it("renders submit and cancel buttons", () => {
    render(<DatabaseForm {...defaultProps} />);

    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Abbrechen" }),
    ).toBeInTheDocument();
  });

  it("calls onCancel when cancel button is clicked", () => {
    const onCancel = vi.fn();
    render(<DatabaseForm {...defaultProps} onCancel={onCancel} />);

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("shows validation error on submit with empty required field", async () => {
    const onSubmit = vi.fn();
    render(<DatabaseForm {...defaultProps} onSubmit={onSubmit} />);

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(
        screen.getByText("Test Field ist erforderlich."),
      ).toBeInTheDocument();
    });
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("calls onSubmit with form data when validation passes", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    render(<DatabaseForm {...defaultProps} onSubmit={onSubmit} />);

    const input = screen.getByLabelText(/Test Field/);
    fireEvent.change(input, { target: { value: "test value" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ test_field: "test value" }),
      );
    });
  });

  it("displays external error when provided", () => {
    render(<DatabaseForm {...defaultProps} error="External error message" />);

    expect(screen.getByText("External error message")).toBeInTheDocument();
  });

  it("disables buttons when isLoading is true", () => {
    render(<DatabaseForm {...defaultProps} isLoading={true} />);

    expect(
      screen.getByRole("button", { name: /wird gespeichert/i }),
    ).toBeDisabled();
    expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
  });

  it("shows loading text on submit button when isLoading", () => {
    render(<DatabaseForm {...defaultProps} isLoading={true} />);

    expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();
  });

  it("renders checkbox fields correctly", () => {
    const sectionsWithCheckbox: FormSection[] = [
      {
        title: "Settings",
        fields: [{ name: "active", label: "Aktiv", type: "checkbox" }],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithCheckbox} />);

    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).toBeInTheDocument();
    expect(checkbox).not.toBeChecked();
  });

  it("handles checkbox change", () => {
    const sectionsWithCheckbox: FormSection[] = [
      {
        title: "Settings",
        fields: [{ name: "active", label: "Aktiv", type: "checkbox" }],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithCheckbox} />);

    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);
    expect(checkbox).toBeChecked();
  });

  it("renders select fields with options", () => {
    const sectionsWithSelect: FormSection[] = [
      {
        title: "Auswahl",
        fields: [
          {
            name: "role",
            label: "Rolle",
            type: "select",
            options: [
              { value: "admin", label: "Administrator" },
              { value: "user", label: "Benutzer" },
            ],
          },
        ],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithSelect} />);

    const select = screen.getByRole("combobox");
    expect(select).toBeInTheDocument();
    expect(screen.getByText("Administrator")).toBeInTheDocument();
    expect(screen.getByText("Benutzer")).toBeInTheDocument();
  });

  it("renders textarea fields", () => {
    const sectionsWithTextarea: FormSection[] = [
      {
        title: "Details",
        fields: [
          { name: "description", label: "Beschreibung", type: "textarea" },
        ],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithTextarea} />);

    const textarea = screen.getByRole("textbox");
    expect(textarea).toBeInTheDocument();
    expect(textarea.tagName.toLowerCase()).toBe("textarea");
  });

  it("renders number fields", () => {
    const sectionsWithNumber: FormSection[] = [
      {
        title: "Zahlen",
        fields: [
          { name: "count", label: "Anzahl", type: "number", min: 0, max: 100 },
        ],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithNumber} />);

    const numberInput = screen.getByRole("spinbutton");
    expect(numberInput).toBeInTheDocument();
    expect(numberInput).toHaveAttribute("min", "0");
    expect(numberInput).toHaveAttribute("max", "100");
  });

  it("handles number field change", () => {
    const sectionsWithNumber: FormSection[] = [
      {
        title: "Zahlen",
        fields: [{ name: "count", label: "Anzahl", type: "number" }],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithNumber} />);

    const numberInput = screen.getByRole("spinbutton");
    fireEvent.change(numberInput, { target: { value: "42" } });
    expect(numberInput).toHaveValue(42);
  });

  it("renders with initial data", async () => {
    render(
      <DatabaseForm
        {...defaultProps}
        initialData={{ test_field: "initial value" }}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText(/Test Field/)).toHaveValue("initial value");
    });
  });

  it("renders helper text when provided", () => {
    const sectionsWithHelper: FormSection[] = [
      {
        title: "Test",
        fields: [
          {
            name: "field",
            label: "Field",
            type: "text",
            helperText: "Dies ist ein Hilfetext",
          },
        ],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithHelper} />);

    expect(screen.getByText("Dies ist ein Hilfetext")).toBeInTheDocument();
  });

  it("renders section subtitle when provided", () => {
    const sectionsWithSubtitle: FormSection[] = [
      {
        title: "Main Title",
        subtitle: "This is a subtitle",
        fields: [{ name: "field", label: "Field", type: "text" }],
      },
    ];

    render(<DatabaseForm {...defaultProps} sections={sectionsWithSubtitle} />);

    expect(screen.getByText("This is a subtitle")).toBeInTheDocument();
  });

  it("displays error from submit failure", async () => {
    const onSubmit = vi.fn().mockRejectedValue(new Error("Server error"));
    const sectionsOptional: FormSection[] = [
      {
        title: "Test",
        fields: [{ name: "field", label: "Field", type: "text" }],
      },
    ];

    render(
      <DatabaseForm
        {...defaultProps}
        sections={sectionsOptional}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(screen.getByText("Server error")).toBeInTheDocument();
    });
  });

  it("renders sticky action bar when stickyActions is true", () => {
    render(<DatabaseForm {...defaultProps} stickyActions={true} />);

    const stickyContainer = document.querySelector(".sticky");
    expect(stickyContainer).toBeInTheDocument();
  });
});
