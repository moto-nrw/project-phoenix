/**
 * Tests for StudentCommonFormSections Component
 * Tests rendering of common form sections
 */
import { render } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { StudentCommonFormSections } from "./student-common-form-sections";
import type { Student } from "@/lib/api";

// Mock the form field components
vi.mock("./student-form-fields", () => ({
  HealthInfoSection: ({
    value,
    onChange,
  }: {
    value?: string | null;
    onChange: (v: string) => void;
  }) => (
    <div data-testid="health-info">
      <textarea
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        data-testid="health-info-textarea"
      />
    </div>
  ),
  AdditionalInfoSection: ({
    value,
    onChange,
  }: {
    value?: string | null;
    onChange: (v: string) => void;
  }) => (
    <div data-testid="additional-info">
      <textarea
        value={value ?? ""}
        onChange={(e) => onChange(e.target.value)}
        data-testid="additional-info-textarea"
      />
    </div>
  ),
  PrivacyConsentSection: () => <div data-testid="privacy-consent" />,
}));

describe("StudentCommonFormSections", () => {
  const mockFormData: Partial<Student> = {
    health_info: "Test health info",
    supervisor_notes: "Test notes",
    extra_info: "Test extra",
  };

  const noFieldError = () => undefined;
  const mockOnChange = vi.fn();

  it("renders all form sections", () => {
    const { getByTestId, queryByTestId } = render(
      <StudentCommonFormSections
        formData={mockFormData}
        fieldError={noFieldError}
        onChange={mockOnChange}
      />,
    );

    expect(getByTestId("health-info")).toBeInTheDocument();
    expect(getByTestId("additional-info")).toBeInTheDocument();
    expect(getByTestId("privacy-consent")).toBeInTheDocument();
    expect(queryByTestId("supervisor-notes")).not.toBeInTheDocument();
  });
});
