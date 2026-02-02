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
  PickupStatusSection,
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

    expect(screen.getByText("Group A")).toBeInTheDocument();
    expect(screen.getByText("Group B")).toBeInTheDocument();
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
  it("renders bus status checkbox", () => {
    const onChange = vi.fn();
    render(<BusStatusSection value={true} onChange={onChange} />);

    const checkbox = screen.getByRole("checkbox");
    expect(checkbox).toBeChecked();
  });

  it("calls onChange when checkbox toggled", () => {
    const onChange = vi.fn();
    render(<BusStatusSection value={false} onChange={onChange} />);

    const checkbox = screen.getByRole("checkbox");
    fireEvent.click(checkbox);

    expect(onChange).toHaveBeenCalledWith(true);
  });
});

describe("PickupStatusSection", () => {
  it("renders pickup status select", () => {
    const onChange = vi.fn();
    render(<PickupStatusSection value="Wird abgeholt" onChange={onChange} />);

    expect(screen.getByDisplayValue("Wird abgeholt")).toBeInTheDocument();
  });

  it("calls onChange when selection changes", () => {
    const onChange = vi.fn();
    render(<PickupStatusSection value="" onChange={onChange} />);

    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "Geht alleine nach Hause" } });

    expect(onChange).toHaveBeenCalledWith("Geht alleine nach Hause");
  });

  it("handles null value for empty selection", () => {
    const onChange = vi.fn();
    render(<PickupStatusSection value="Wird abgeholt" onChange={onChange} />);

    const select = screen.getByRole("combobox");
    fireEvent.change(select, { target: { value: "" } });

    expect(onChange).toHaveBeenCalledWith(null);
  });
});
