import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";
import { StudentDetailHeader } from "./student-detail-header";

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

describe("StudentDetailHeader", () => {
  it("shows name, initials, class and group", () => {
    render(
      <StudentDetailHeader student={makeStudent({ group_name: "Füchse" })} />,
    );

    expect(screen.getByText("MM")).toBeInTheDocument();
    expect(screen.getByText("Max Muster")).toBeInTheDocument();
    expect(screen.getByText("3a · Füchse")).toBeInTheDocument();
  });

  it("falls back to '?' initials when no first/last name", () => {
    render(
      <StudentDetailHeader
        student={makeStudent({
          first_name: undefined,
          second_name: undefined,
          name: "FooBar",
        })}
      />,
    );

    expect(screen.getByText("?")).toBeInTheDocument();
    expect(screen.getByText("FooBar")).toBeInTheDocument();
  });

  it("shows 'Unbekannt' when no name info at all", () => {
    render(
      <StudentDetailHeader
        student={makeStudent({
          first_name: undefined,
          second_name: undefined,
          name: "",
        })}
      />,
    );

    expect(screen.getByText("Unbekannt")).toBeInTheDocument();
  });

  it("shows fallback meta when no class and no group", () => {
    render(
      <StudentDetailHeader
        student={makeStudent({
          school_class: "",
          group_name: undefined,
        })}
      />,
    );

    expect(screen.getByText("Keine Klasse hinterlegt")).toBeInTheDocument();
  });

  it("only shows available meta parts", () => {
    render(
      <StudentDetailHeader
        student={makeStudent({
          school_class: "3a",
          group_name: undefined,
        })}
      />,
    );

    expect(screen.getByText("3a")).toBeInTheDocument();
  });

  it("renders a warning badge when warning supplied", () => {
    render(<StudentDetailHeader student={makeStudent()} warning="Achtung" />);

    expect(screen.getByText("Achtung")).toBeInTheDocument();
  });

  it("omits warning badge when warning is null/empty", () => {
    render(<StudentDetailHeader student={makeStudent()} warning={null} />);

    expect(screen.queryByText("Achtung")).not.toBeInTheDocument();
  });

  it("renders action slot when provided", () => {
    render(
      <StudentDetailHeader
        student={makeStudent()}
        actions={<button type="button">Go</button>}
      />,
    );

    expect(screen.getByRole("button", { name: "Go" })).toBeInTheDocument();
  });
});
