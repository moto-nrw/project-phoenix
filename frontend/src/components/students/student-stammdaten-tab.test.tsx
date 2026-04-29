import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

const { handleStudentFormSubmitMock, validateStudentFormMock } = vi.hoisted(
  () => ({
    handleStudentFormSubmitMock: vi.fn(),
    validateStudentFormMock: vi.fn(),
  }),
);

vi.mock("~/lib/student-form-validation", () => ({
  handleStudentFormSubmit: handleStudentFormSubmitMock,
  validateStudentForm: validateStudentFormMock,
}));

vi.mock("./student-form-fields", () => ({
  PersonalInfoSection: ({
    formData,
    onChange,
  }: {
    formData: Partial<Student>;
    onChange: (field: keyof Student, value: string | boolean | number) => void;
  }) => (
    <div data-testid="personal-info">
      <input
        data-testid="first-name"
        value={formData.first_name ?? ""}
        onChange={(e) => onChange("first_name", e.target.value)}
      />
    </div>
  ),
  BusStatusSection: ({
    value,
    onChange,
  }: {
    value?: boolean;
    onChange: (v: boolean) => void;
  }) => (
    <button
      type="button"
      data-testid="bus-toggle"
      onClick={() => onChange(!value)}
    >
      bus:{String(value)}
    </button>
  ),
  PickupStatusSection: ({
    value,
    onChange,
  }: {
    value?: string;
    onChange: (v: string) => void;
  }) => (
    <input
      data-testid="pickup-status"
      value={value ?? ""}
      onChange={(e) => onChange(e.target.value)}
    />
  ),
}));

vi.mock("./student-common-form-sections", () => ({
  StudentCommonFormSections: ({
    errors,
  }: {
    errors: Record<string, string>;
  }) => (
    <div
      data-testid="common-sections"
      data-error-count={Object.keys(errors).length}
    />
  ),
}));

vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    ...props
  }: React.ButtonHTMLAttributes<HTMLButtonElement> & {
    variant?: string;
  }) => <button {...props}>{children}</button>,
}));

import { StudentStammdatenTab } from "./student-stammdaten-tab";

function makeStudent(overrides: Partial<Student> = {}): Student {
  return {
    id: "1",
    name: "Max Muster",
    first_name: "Max",
    second_name: "Muster",
    school_class: "3a",
    current_location: "class",
    ...overrides,
  } as Student;
}

describe("StudentStammdatenTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    validateStudentFormMock.mockReturnValue({});
    handleStudentFormSubmitMock.mockImplementation(
      async (
        event: Event & { preventDefault: () => void },
        _formData: Partial<Student>,
        validate: () => boolean,
        save: (data: Partial<Student>) => Promise<void>,
        setSaving: (v: boolean) => void,
        setErrors: (e: Record<string, string>) => void,
      ) => {
        event.preventDefault();
        if (!validate()) return;
        setSaving(true);
        try {
          await save(_formData);
        } catch (err) {
          setErrors({ submit: err instanceof Error ? err.message : "err" });
        } finally {
          setSaving(false);
        }
      },
    );
  });

  it("submit button is disabled when form is unchanged", () => {
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect(screen.getByRole("button", { name: /Speichern/ })).toBeDisabled();
  });

  it("enables submit once a field changes", () => {
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "New" },
    });

    expect(screen.getByRole("button", { name: /Speichern/ })).toBeEnabled();
  });

  it("calls onSave with submitted form data", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "Updated" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalled());
    expect(onSave.mock.calls[0]![0]).toMatchObject({ first_name: "Updated" });
  });

  it("shows submit error from validator", () => {
    validateStudentFormMock.mockReturnValue({ first_name: "bad" });
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    // Rerender with dirty form to simulate trigger submit
    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "X" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    expect(screen.getByTestId("common-sections")).toHaveAttribute(
      "data-error-count",
      "1",
    );
  });

  it("displays submit error banner when submit error exists", async () => {
    const onSave = vi.fn().mockRejectedValue(new Error("Save failed"));

    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "X" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() =>
      expect(screen.getByText("Save failed")).toBeInTheDocument(),
    );
  });

  it("rebuilds draft when student prop changes", () => {
    const { rerender } = render(
      <StudentStammdatenTab
        student={makeStudent({ first_name: "Old" })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect((screen.getByTestId("first-name") as HTMLInputElement).value).toBe(
      "Old",
    );

    rerender(
      <StudentStammdatenTab
        student={makeStudent({ first_name: "New" })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    expect((screen.getByTestId("first-name") as HTMLInputElement).value).toBe(
      "New",
    );
  });

  it("toggles the bus value", () => {
    render(
      <StudentStammdatenTab
        student={makeStudent({ bus: false })}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    const btn = screen.getByTestId("bus-toggle");
    expect(btn).toHaveTextContent("bus:false");
    fireEvent.click(btn);
    expect(btn).toHaveTextContent("bus:true");
  });

  it("clears field-level error when the field changes", () => {
    validateStudentFormMock.mockReturnValue({ first_name: "required" });
    render(
      <StudentStammdatenTab
        student={makeStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );

    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "X" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));
    expect(screen.getByTestId("common-sections")).toHaveAttribute(
      "data-error-count",
      "1",
    );

    // Typing again clears the first_name error
    fireEvent.change(screen.getByTestId("first-name"), {
      target: { value: "XY" },
    });
    expect(screen.getByTestId("common-sections")).toHaveAttribute(
      "data-error-count",
      "0",
    );
  });
});
