import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";

const receivedProps = vi.fn();

vi.mock("~/components/guardians/student-guardian-manager", () => ({
  default: (props: Record<string, unknown>) => {
    receivedProps(props);
    return <div data-testid="guardian-manager" />;
  },
}));

import { StudentGuardiansTab } from "./student-guardians-tab";

function makeStudent(overrides: Partial<Student> = {}): Student {
  return {
    id: "42",
    name: "Max Muster",
    first_name: "Max",
    second_name: "Muster",
    school_class: "3a",
    current_location: "class",
    ...overrides,
  } as Student;
}

describe("StudentGuardiansTab", () => {
  it("renders StudentGuardianManager with props for users with full access", () => {
    const onChanged = vi.fn();
    render(
      <StudentGuardiansTab
        student={makeStudent({ has_full_access: true })}
        onChanged={onChanged}
      />,
    );

    expect(screen.getByTestId("guardian-manager")).toBeInTheDocument();
    expect(receivedProps).toHaveBeenCalledWith(
      expect.objectContaining({
        studentId: "42",
        readOnly: false,
        onUpdate: onChanged,
      }),
    );
  });

  it("renders manager when has_full_access is undefined (permission assumed)", () => {
    render(<StudentGuardiansTab student={makeStudent()} />);
    expect(screen.getByTestId("guardian-manager")).toBeInTheDocument();
  });

  it("renders manager read-only when user has read access but no write access", () => {
    const onChanged = vi.fn();
    render(
      <StudentGuardiansTab
        student={makeStudent({
          has_full_access: true,
          has_write_access: false,
        })}
        onChanged={onChanged}
      />,
    );

    expect(screen.getByTestId("guardian-manager")).toBeInTheDocument();
    expect(receivedProps).toHaveBeenCalledWith(
      expect.objectContaining({
        readOnly: true,
        onUpdate: undefined,
      }),
    );
  });

  it("shows GDPR hint when has_full_access is false", () => {
    render(
      <StudentGuardiansTab student={makeStudent({ has_full_access: false })} />,
    );

    expect(screen.getByText("Kein vollständiger Zugriff")).toBeInTheDocument();
    expect(screen.getByText(/Vollzugriff/)).toBeInTheDocument();
    expect(screen.queryByTestId("guardian-manager")).not.toBeInTheDocument();
  });
});
