/**
 * Tests for Student Form Field Components
 * Tests reusable form field components
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import {
  PersonalInfoSection,
  HealthInfoSection,
  SupervisorNotesSection,
  AdditionalInfoSection,
  PrivacyConsentSection,
  BusStatusSection,
  EnrollmentConsentsSection,
  PickupStatusSection,
  DepartureSection,
} from "./student-form-fields";
import type { Student } from "@/lib/api";

describe("PersonalInfoSection", () => {
  const mockFormData: Partial<Student> = {
    first_name: "Max",
    second_name: "Mustermann",
    school_class: "5A",
  };

  const mockOnChange = vi.fn();
  const mockErrors = {};

  it("renders all personal info fields", () => {
    render(
      <PersonalInfoSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
      />,
    );

    expect(screen.getByDisplayValue("Max")).toBeInTheDocument();
    expect(screen.getByDisplayValue("Mustermann")).toBeInTheDocument();
    expect(screen.getByDisplayValue("5A")).toBeInTheDocument();
  });

  it("calls onChange when input changes", () => {
    render(
      <PersonalInfoSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
      />,
    );

    const firstNameInput = screen.getByDisplayValue("Max");
    fireEvent.change(firstNameInput, { target: { value: "Maxine" } });

    expect(mockOnChange).toHaveBeenCalledWith("first_name", "Maxine");
  });

  it("displays error messages", () => {
    const errors = { first_name: "Required field" };
    render(
      <PersonalInfoSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={errors}
      />,
    );

    expect(screen.getByText("Required field")).toBeInTheDocument();
  });

  it("renders group select when groups provided", () => {
    const groups = [
      { value: "1", label: "Group A" },
      { value: "2", label: "Group B" },
    ];

    render(
      <PersonalInfoSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
        groups={groups}
      />,
    );

    fireEvent.click(screen.getByRole("combobox"));
    expect(screen.getByRole("option", { name: "Group A" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Group B" })).toBeInTheDocument();
  });
});

describe("HealthInfoSection", () => {
  it("renders health info textarea", () => {
    const onChange = vi.fn();
    render(<HealthInfoSection value="Peanut allergy" onChange={onChange} />);

    expect(screen.getByDisplayValue("Peanut allergy")).toBeInTheDocument();
  });

  it("calls onChange when textarea changes", () => {
    const onChange = vi.fn();
    render(<HealthInfoSection value="" onChange={onChange} />);

    const textarea = screen.getByPlaceholderText(/Allergien, Medikamente/);
    fireEvent.change(textarea, { target: { value: "New info" } });

    expect(onChange).toHaveBeenCalledWith("New info");
  });
});

describe("SupervisorNotesSection", () => {
  it("renders supervisor notes textarea", () => {
    const onChange = vi.fn();
    render(
      <SupervisorNotesSection value="Important note" onChange={onChange} />,
    );

    expect(screen.getByDisplayValue("Important note")).toBeInTheDocument();
  });

  it("calls onChange when textarea changes", () => {
    const onChange = vi.fn();
    render(<SupervisorNotesSection value="" onChange={onChange} />);

    const textarea = screen.getByPlaceholderText(/Interne Notizen/);
    fireEvent.change(textarea, { target: { value: "New note" } });

    expect(onChange).toHaveBeenCalledWith("New note");
  });
});

describe("AdditionalInfoSection", () => {
  it("renders additional info textarea", () => {
    const onChange = vi.fn();
    render(<AdditionalInfoSection value="Extra details" onChange={onChange} />);

    expect(screen.getByDisplayValue("Extra details")).toBeInTheDocument();
  });

  it("calls onChange when textarea changes", () => {
    const onChange = vi.fn();
    render(<AdditionalInfoSection value="" onChange={onChange} />);

    const textarea = screen.getByPlaceholderText(/Weitere Informationen/);
    fireEvent.change(textarea, { target: { value: "New info" } });

    expect(onChange).toHaveBeenCalledWith("New info");
  });
});

describe("PrivacyConsentSection", () => {
  const mockFormData: Partial<Student> = {
    privacy_consent_accepted: true,
    data_retention_days: 30,
  };

  const mockOnChange = vi.fn();
  const mockErrors = {};

  it("renders privacy consent checkbox", () => {
    render(
      <PrivacyConsentSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
      />,
    );

    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).toBeChecked();
  });

  it("renders data retention input", () => {
    render(
      <PrivacyConsentSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
      />,
    );

    const input = screen.getByDisplayValue("30");
    expect(input).toBeInTheDocument();
  });

  it("calls onChange when checkbox toggled", () => {
    render(
      <PrivacyConsentSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
      />,
    );

    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);

    expect(mockOnChange).toHaveBeenCalledWith(
      "privacy_consent_accepted",
      false,
    );
  });

  it("calls onChange when retention days changed", () => {
    render(
      <PrivacyConsentSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
      />,
    );

    const input = screen.getByDisplayValue("30");
    fireEvent.change(input, { target: { value: "15" } });

    expect(mockOnChange).toHaveBeenCalledWith("data_retention_days", 15);
  });

  it("handles empty retention days input", () => {
    render(
      <PrivacyConsentSection
        formData={mockFormData}
        onChange={mockOnChange}
        errors={mockErrors}
      />,
    );

    const input = screen.getByDisplayValue("30");
    fireEvent.change(input, { target: { value: "" } });

    expect(mockOnChange).toHaveBeenCalledWith("data_retention_days", null);
  });
});

describe("BusStatusSection", () => {
  it("renders selected bus weekdays", () => {
    const onChange = vi.fn();
    render(
      <BusStatusSection
        value={true}
        days={{ mon: true, wed: true }}
        onChange={onChange}
      />,
    );

    expect(
      screen.getByRole("checkbox", { name: "Montag Buskind" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Mittwoch Buskind" }),
    ).toBeChecked();
  });

  it("calls onChange with weekday map when toggled", () => {
    const onChange = vi.fn();
    render(<BusStatusSection value={false} days={{}} onChange={onChange} />);

    const checkbox = screen.getByRole("checkbox", { name: "Montag Buskind" });
    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith({ mon: true });
  });
});

describe("DepartureSection", () => {
  it("marks the allowed modes per weekday", () => {
    const onChange = vi.fn();
    render(
      <DepartureSection
        days={{ mon: ["bus", "pickup"], wed: ["pickup"] }}
        onChange={onChange}
      />,
    );

    expect(screen.getByRole("checkbox", { name: "Montag: Bus" })).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Montag: Abgeholt" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Mittwoch: Abgeholt" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Dienstag: Zu Fuß" }),
    ).not.toBeChecked();
  });

  it("adds a weekday mode", () => {
    const onChange = vi.fn();
    render(<DepartureSection days={{}} onChange={onChange} />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Montag: Bus" }));
    expect(onChange).toHaveBeenCalledWith({ mon: ["bus"] });
  });

  it("clears the weekday when the last selected mode is removed", () => {
    const onChange = vi.fn();
    render(<DepartureSection days={{ mon: ["bus"] }} onChange={onChange} />);

    fireEvent.click(screen.getByRole("checkbox", { name: "Montag: Bus" }));
    expect(onChange).toHaveBeenCalledWith({});
  });
});

describe("EnrollmentConsentsSection", () => {
  it("renders nothing when no enrollment consents are stamped", () => {
    const { container } = render(<EnrollmentConsentsSection />);

    expect(container).toBeEmptyDOMElement();
  });

  it("renders stamped and missing enrollment consents", () => {
    render(
      <EnrollmentConsentsSection
        agbAcceptedAt="2026-01-02T10:00:00Z"
        dataProcessingAcceptedAt="2026-01-03T10:00:00Z"
        emailContactAcceptedAt={null}
        photoConsentGivenAt="2026-01-04T10:00:00Z"
      />,
    );

    expect(
      screen.getByText("Einwilligungen bei Anmeldung"),
    ).toBeInTheDocument();
    expect(screen.getByText("AGB")).toBeInTheDocument();
    expect(screen.getByText("Datenverarbeitung (DSGVO)")).toBeInTheDocument();
    expect(screen.getByText("E-Mail-Kontakt")).toBeInTheDocument();
    expect(
      screen.getByText("Fotos bei Schulveranstaltungen"),
    ).toBeInTheDocument();
    expect(screen.getByText("Nicht erteilt")).toBeInTheDocument();
    expect(screen.getByText("02.01.2026")).toBeInTheDocument();
    expect(screen.getByText("03.01.2026")).toBeInTheDocument();
    expect(screen.getByText("04.01.2026")).toBeInTheDocument();
  });
});

describe("PickupStatusSection", () => {
  it("renders selected pickup weekdays", () => {
    const onChange = vi.fn();
    render(
      <PickupStatusSection
        days={{ mon: true, wed: true }}
        onChange={onChange}
      />,
    );

    expect(
      screen.getByRole("checkbox", { name: "Montag wird abgeholt" }),
    ).toBeChecked();
    expect(
      screen.getByRole("checkbox", { name: "Mittwoch wird abgeholt" }),
    ).toBeChecked();
  });

  it("calls onChange with weekday map when toggled", () => {
    const onChange = vi.fn();
    render(<PickupStatusSection days={{}} onChange={onChange} />);

    const checkbox = screen.getByRole("checkbox", {
      name: "Montag wird abgeholt",
    });
    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith({ mon: true });
  });

  it("unchecking a selected day removes it from the map", () => {
    const onChange = vi.fn();
    render(<PickupStatusSection days={{ mon: true }} onChange={onChange} />);

    const checkbox = screen.getByRole("checkbox", {
      name: "Montag wird abgeholt",
    });
    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith({ mon: false });
  });
});
