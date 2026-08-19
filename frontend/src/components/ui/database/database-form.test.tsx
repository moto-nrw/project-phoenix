import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import useSWR from "swr";
import {
  getDefaultValueForField,
  hasPrivacyConsentFields,
  extractPrivacyConsent,
  isEmptyValue,
  validateNumberMin,
  validateNumberMax,
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

  it("returns an empty string for optional number fields", () => {
    const field: FormField = {
      name: "age",
      label: "Alter",
      type: "number",
    };
    expect(getDefaultValueForField(field)).toBe("");
  });

  it("returns 0 for required number fields", () => {
    const field: FormField = {
      name: "age",
      label: "Alter",
      type: "number",
      required: true,
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

describe("validateNumberMax", () => {
  it("returns null when the value meets the maximum", () => {
    expect(validateNumberMax(50, 50, "Anzahl")).toBeNull();
  });

  it("returns an error when the value exceeds the maximum", () => {
    expect(validateNumberMax(51, 50, "Anzahl")).toBe(
      "Anzahl darf höchstens 50 sein.",
    );
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

  it("rejects fractional values for integer number fields", () => {
    const field: FormField = {
      name: "capacity",
      label: "Kapazität",
      type: "number",
      min: 1,
    };

    expect(validateField(field, 1.5)).toBe(
      "Kapazität muss eine ganze Zahl sein.",
    );
    expect(validateField(field, "1.5")).toBe(
      "Kapazität muss eine ganze Zahl sein.",
    );
    expect(validateField(field, 2)).toBeNull();
  });

  it("validates a non-empty optional number field", () => {
    const field: FormField = {
      name: "capacity",
      label: "Maximale Belegung",
      type: "number",
      min: 1,
    };

    expect(validateField(field, "")).toBeNull();
    expect(validateField(field, 0)).toBe(
      "Maximale Belegung muss mindestens 1 sein.",
    );
    expect(validateField(field, 43)).toBeNull();
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

  it("returns first error with field name when validation fails", () => {
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
    expect(validateFormFields(sections, formData)).toEqual({
      message: "Vorname ist erforderlich.",
      fieldName: "first_name",
    });
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
    expect(validateFormFields(sections, formData)).toEqual({
      message: "E-Mail ist erforderlich.",
      fieldName: "email",
    });
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
    sections: mockSections,
    onSubmit: vi.fn(),
    onCancel: vi.fn(),
    submitLabel: "Speichern",
  };

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(useSWR).mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
      isValidating: false,
      mutate: vi.fn(),
    });
  });

  it("renders form sections and fields", () => {
    render(<DatabaseForm {...defaultProps} />);

    expect(screen.getByText("Test Section")).toBeInTheDocument();
    expect(screen.getByLabelText(/Test Field/)).toBeInTheDocument();
  });

  // componentProps ist der Weg, eine geteilte Custom-Feld-Komponente mit
  // Presets zu benutzen (#2405: derselbe Farb-Picker, für den Schulhof mit
  // orangener Vorschau), statt sie in eine zweite Komponente zu wickeln.
  // Fällt die Durchreichung weg, rendert das Feld still den Standard.
  describe("componentProps", () => {
    const CustomField = ({ label, hint }: { label: string; hint?: string }) => (
      <div>
        <span>{label}</span>
        <span>{hint ?? "Standardhinweis"}</span>
      </div>
    );

    const sectionsWithCustom = (
      componentProps?: Record<string, unknown>,
    ): FormSection[] => [
      {
        title: "Test Section",
        fields: [
          {
            name: "custom_field",
            label: "Farbe",
            type: "custom",
            component: CustomField,
            componentProps,
          },
        ],
      },
    ];

    it("reicht sie an das Custom-Feld durch", () => {
      render(
        <DatabaseForm
          {...defaultProps}
          sections={sectionsWithCustom({ hint: "Eigener Hinweis" })}
        />,
      );

      expect(screen.getByText("Eigener Hinweis")).toBeInTheDocument();
    });

    it("fällt ohne sie auf die Defaults der Komponente zurück", () => {
      render(
        <DatabaseForm {...defaultProps} sections={sectionsWithCustom()} />,
      );

      expect(screen.getByText("Standardhinweis")).toBeInTheDocument();
    });

    it("lässt die formeigenen Props gewinnen", () => {
      // Sonst könnte ein Preset value/onChange/label überschreiben und das
      // Feld vom Formularzustand abkoppeln.
      render(
        <DatabaseForm
          {...defaultProps}
          sections={sectionsWithCustom({ label: "Überschrieben" })}
        />,
      );

      expect(screen.getByText("Farbe")).toBeInTheDocument();
      expect(screen.queryByText("Überschrieben")).not.toBeInTheDocument();
    });
  });

  // sectionLevel existiert, weil dieses Formular in zwei Kontexten laeuft:
  // im Master-Detail-Bereich (Default 2) und ueber DatabaseFormModal in
  // einem Modal, dessen Titel h3 ist (dort 4). Ohne diese Tests koennte die
  // Durchreichung in einem der beiden Zweige wegfallen, ohne dass etwas rot
  // wird - der concept-lose Zweig rendert eine eigene Ueberschrift.
  it.each([
    ["ohne concept", mockSections],
    ["mit concept", [{ ...mockSections[0]!, concept: "rooms" as const }]],
  ])("nutzt %s standardmaessig h2", (_label, sections) => {
    render(<DatabaseForm {...defaultProps} sections={sections} />);

    expect(
      screen.getByRole("heading", { level: 2, name: "Test Section" }),
    ).toBeInTheDocument();
  });

  it.each([
    ["ohne concept", mockSections],
    ["mit concept", [{ ...mockSections[0]!, concept: "rooms" as const }]],
  ])(
    "reicht sectionLevel %s bis zur Ueberschrift durch",
    (_label, sections) => {
      render(
        <DatabaseForm {...defaultProps} sections={sections} sectionLevel={4} />,
      );

      expect(
        screen.getByRole("heading", { level: 4, name: "Test Section" }),
      ).toBeInTheDocument();
    },
  );

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
    fireEvent.click(select);
    expect(
      screen.getByRole("option", { name: "Administrator" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "Benutzer" }),
    ).toBeInTheDocument();
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

  it("submits an emptied optional number field", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const sectionsWithCapacity: FormSection[] = [
      {
        title: "Raum",
        fields: [
          {
            name: "capacity",
            label: "Maximale Belegung",
            type: "number",
            min: 1,
          },
        ],
      },
    ];

    render(
      <DatabaseForm
        {...defaultProps}
        sections={sectionsWithCapacity}
        initialData={{ capacity: 2 }}
        onSubmit={onSubmit}
      />,
    );

    const capacityInput = screen.getByRole("spinbutton", {
      name: "Maximale Belegung",
    });
    fireEvent.change(capacityInput, { target: { value: "" } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({ capacity: "" }),
      ),
    );
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

  it("preserves a null initial number as an empty field", async () => {
    const sections: FormSection[] = [
      {
        title: "Kapazität",
        fields: [
          {
            name: "max_participant",
            label: "Maximale Teilnehmer",
            type: "number",
            required: false,
            min: 1,
          },
        ],
      },
    ];
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <DatabaseForm
        {...defaultProps}
        sections={sections}
        initialData={{ max_participant: null }}
        onSubmit={onSubmit}
      />,
    );

    const input = screen.getByRole("spinbutton");
    await waitFor(() => expect(input).toHaveValue(null));

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(onSubmit).toHaveBeenCalledWith({ max_participant: null }),
    );
  });

  it("preserves unsaved edits when privacy consent revalidates", async () => {
    const sections: FormSection[] = [
      {
        title: "Stammdaten",
        fields: [
          { name: "test_field", label: "Test Field", type: "text" },
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
    const initialData = { id: "student-1", test_field: "initial value" };
    const initialConsent = {
      data: { accepted: false, data_retention_days: 30 },
      error: undefined,
      isLoading: false,
      isValidating: false,
      mutate: vi.fn(),
    };
    vi.mocked(useSWR).mockReturnValue(initialConsent);

    const { rerender } = render(
      <DatabaseForm
        {...defaultProps}
        sections={sections}
        initialData={initialData}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Test Field")).toHaveValue("initial value");
      expect(screen.getByLabelText("Einwilligung")).not.toBeChecked();
      expect(screen.getByLabelText("Aufbewahrungstage")).toHaveValue(30);
    });

    fireEvent.change(screen.getByLabelText("Test Field"), {
      target: { value: "unsaved edit" },
    });
    fireEvent.click(screen.getByLabelText("Einwilligung"));
    fireEvent.change(screen.getByLabelText("Aufbewahrungstage"), {
      target: { value: "45" },
    });

    vi.mocked(useSWR).mockReturnValue({
      ...initialConsent,
      data: { accepted: false, data_retention_days: 90 },
    });
    rerender(
      <DatabaseForm
        {...defaultProps}
        sections={sections}
        initialData={initialData}
      />,
    );

    await waitFor(() => {
      expect(screen.getByLabelText("Test Field")).toHaveValue("unsaved edit");
      expect(screen.getByLabelText("Einwilligung")).toBeChecked();
      expect(screen.getByLabelText("Aufbewahrungstage")).toHaveValue(45);
    });
  });

  // The student proxy writes the Datenschutz pair BEFORE the student PUT, so
  // anything this payload carries is stored even when the student write is
  // refused. While editing, an untouched pair is only an echo of the fetched
  // server consent — sending it back would overwrite a change made since.
  const privacySections: FormSection[] = [
    {
      title: "Stammdaten",
      fields: [
        { name: "test_field", label: "Test Field", type: "text" },
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

  const withConsent = (accepted: boolean, days: number) => ({
    data: { accepted, data_retention_days: days },
    error: undefined,
    isLoading: false,
    isValidating: false,
    mutate: vi.fn(),
  });

  it("omits the untouched privacy pair when editing a student", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useSWR).mockReturnValue(withConsent(true, 90));

    render(
      <DatabaseForm
        {...defaultProps}
        sections={privacySections}
        initialData={{ id: "student-1", test_field: "initial value" }}
        onSubmit={onSubmit}
      />,
    );

    await waitFor(() =>
      expect(screen.getByLabelText("Aufbewahrungstage")).toHaveValue(90),
    );
    fireEvent.change(screen.getByLabelText("Test Field"), {
      target: { value: "unrelated edit" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    const submitted = onSubmit.mock.calls[0]![0] as Record<string, unknown>;
    expect(submitted.test_field).toBe("unrelated edit");
    expect("privacy_consent_accepted" in submitted).toBe(false);
    expect("data_retention_days" in submitted).toBe(false);
  });

  // Both fields travel together once one of them was touched: the proxy's
  // consent PUT upserts the pair, so a lone field resets the other.
  it("submits both privacy fields once one of them was edited", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    vi.mocked(useSWR).mockReturnValue(withConsent(false, 90));

    render(
      <DatabaseForm
        {...defaultProps}
        sections={privacySections}
        initialData={{ id: "student-1", test_field: "initial value" }}
        onSubmit={onSubmit}
      />,
    );

    await waitFor(() =>
      expect(screen.getByLabelText("Aufbewahrungstage")).toHaveValue(90),
    );
    fireEvent.click(screen.getByLabelText("Einwilligung"));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    const submitted = onSubmit.mock.calls[0]![0] as Record<string, unknown>;
    expect(submitted.privacy_consent_accepted).toBe(true);
    expect(submitted.data_retention_days).toBe(90);
  });

  // On create there is no stored consent to echo, so the defaults must travel.
  it("keeps the privacy pair when creating a student", async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);

    render(
      <DatabaseForm
        {...defaultProps}
        sections={privacySections}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.change(screen.getByLabelText("Test Field"), {
      target: { value: "neues Kind" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => expect(onSubmit).toHaveBeenCalled());
    const submitted = onSubmit.mock.calls[0]![0] as Record<string, unknown>;
    expect(submitted.privacy_consent_accepted).toBe(false);
    expect(submitted.data_retention_days).toBe(30);
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
    // Suppress expected console.error from component's error handling
    const consoleSpy = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

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

    consoleSpy.mockRestore();
  });

  it("renders sticky action bar when stickyActions is true", () => {
    render(<DatabaseForm {...defaultProps} stickyActions={true} />);

    const stickyContainer = document.querySelector(".sticky");
    expect(stickyContainer).toBeInTheDocument();
  });

  describe("double-submit prevention", () => {
    it("prevents double-submit by disabling button immediately", async () => {
      let resolveSubmit: (value: void) => void;
      const submitPromise = new Promise<void>((resolve) => {
        resolveSubmit = resolve;
      });
      const onSubmit = vi.fn(() => submitPromise);

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

      const submitButton = screen.getByRole("button", { name: "Speichern" });

      // Click submit button
      fireEvent.click(submitButton);

      // Button should show loading state
      await waitFor(() => {
        expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();
      });

      // Click again while submitting
      fireEvent.click(screen.getByText("Wird gespeichert..."));

      // onSubmit should only have been called once
      expect(onSubmit).toHaveBeenCalledTimes(1);

      // Resolve the submission
      resolveSubmit!();

      await waitFor(() => {
        expect(screen.getByText("Speichern")).toBeInTheDocument();
      });
    });

    it("resets isSubmitting state after validation error", async () => {
      const onSubmit = vi.fn().mockResolvedValue(undefined);

      render(<DatabaseForm {...defaultProps} onSubmit={onSubmit} />);

      const submitButton = screen.getByRole("button", { name: "Speichern" });

      // First submit - validation fails (required field is empty)
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(
          screen.getByText("Test Field ist erforderlich."),
        ).toBeInTheDocument();
      });

      // Button should be back to normal state (not showing loading)
      expect(screen.getByText("Speichern")).toBeInTheDocument();
      expect(submitButton).not.toBeDisabled();

      // Now fill in the field and submit again - should work
      const input = screen.getByLabelText(/Test Field/);
      fireEvent.change(input, { target: { value: "test value" } });
      fireEvent.click(submitButton);

      await waitFor(() => {
        expect(onSubmit).toHaveBeenCalledTimes(1);
      });
    });

    it("resets isSubmitting state after submit error", async () => {
      // Suppress expected console.error from component's error handling
      const consoleSpy = vi
        .spyOn(console, "error")
        .mockImplementation(() => undefined);

      const onSubmit = vi.fn().mockRejectedValue(new Error("Network error"));

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

      const submitButton = screen.getByRole("button", { name: "Speichern" });

      // Submit the form
      fireEvent.click(submitButton);

      // Wait for error to be displayed
      await waitFor(() => {
        expect(screen.getByText("Network error")).toBeInTheDocument();
      });

      // Button should be back to normal state
      expect(screen.getByText("Speichern")).toBeInTheDocument();

      // Can submit again
      fireEvent.click(submitButton);
      await waitFor(() => {
        expect(onSubmit).toHaveBeenCalledTimes(2);
      });

      consoleSpy.mockRestore();
    });

    it("prevents submit while parent isLoading is true", () => {
      const onSubmit = vi.fn();

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
          isLoading={true}
        />,
      );

      // Button should be disabled
      const submitButton = screen.getByRole("button", {
        name: /wird gespeichert/i,
      });
      expect(submitButton).toBeDisabled();

      // Try to click (should be blocked)
      fireEvent.click(submitButton);

      // onSubmit should not have been called
      expect(onSubmit).not.toHaveBeenCalled();
    });

    it("highlights the failing field with red border and label on validation error", async () => {
      const sectionsMulti: FormSection[] = [
        {
          title: "Test Section",
          fields: [
            {
              name: "first_field",
              label: "First Field",
              type: "text",
              required: false,
            },
            {
              name: "second_field",
              label: "Second Field",
              type: "text",
              required: true,
            },
          ],
        },
      ];

      render(<DatabaseForm {...defaultProps} sections={sectionsMulti} />);

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(
          screen.getByText("Second Field ist erforderlich."),
        ).toBeInTheDocument();
      });

      // The failing field's label should have red text
      const failingLabel = screen.getByText(/Second Field\*/);
      expect(failingLabel.className).toContain("text-red-600");

      // The non-failing field's label should remain gray
      const passingLabel = screen.getByText("First Field");
      expect(passingLabel.className).toContain("text-gray-700");
    });

    it("clears field highlighting on next submit attempt", async () => {
      const onSubmit = vi.fn().mockResolvedValue(undefined);

      render(<DatabaseForm {...defaultProps} onSubmit={onSubmit} />);

      // First submit — validation fails
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(
          screen.getByText("Test Field ist erforderlich."),
        ).toBeInTheDocument();
      });

      const label = screen.getByText(/Test Field\*/);
      expect(label.className).toContain("text-red-600");

      // Fill in and re-submit
      const input = screen.getByLabelText(/Test Field/);
      fireEvent.change(input, { target: { value: "fixed" } });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(onSubmit).toHaveBeenCalledTimes(1);
      });

      // Label should no longer be red
      expect(label.className).toContain("text-gray-700");
    });

    it("scrolls to error banner on validation failure", async () => {
      const scrollIntoViewMock = vi.fn();
      Element.prototype.scrollIntoView = scrollIntoViewMock;

      render(<DatabaseForm {...defaultProps} />);

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(
          screen.getByText("Test Field ist erforderlich."),
        ).toBeInTheDocument();
      });

      expect(scrollIntoViewMock).toHaveBeenCalledWith({
        behavior: "smooth",
        block: "start",
      });
    });

    it("shows loading state during async submission", async () => {
      let resolveSubmit: (value: void) => void;
      const submitPromise = new Promise<void>((resolve) => {
        resolveSubmit = resolve;
      });
      const onSubmit = vi.fn(() => submitPromise);

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

      const submitButton = screen.getByRole("button", { name: "Speichern" });
      fireEvent.click(submitButton);

      // Should show loading state
      await waitFor(() => {
        expect(screen.getByText("Wird gespeichert...")).toBeInTheDocument();
      });

      // Cancel button should also be disabled
      const cancelButton = screen.getByRole("button", { name: "Abbrechen" });
      expect(cancelButton).toBeDisabled();

      // Resolve and check state returns to normal
      resolveSubmit!();

      await waitFor(() => {
        expect(screen.getByText("Speichern")).toBeInTheDocument();
      });
    });
  });
});
