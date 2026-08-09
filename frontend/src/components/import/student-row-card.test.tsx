import { render, screen } from "@testing-library/react";
import { describe, it, expect } from "vitest";
import { StudentRowCard } from "./student-row-card";

describe("StudentRowCard", () => {
  const baseStudent = {
    row: 1,
    status: "new" as const,
    errors: [],
    first_name: "Max",
    last_name: "Mustermann",
    meta: ["1a", "Gruppe A", "Anna Mustermann"],
  };

  it("renders student name", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(screen.getByText("Max Mustermann")).toBeInTheDocument();
  });

  it("displays row number", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(screen.getByText("1")).toBeInTheDocument();
  });

  it("uses index+1 when row is 0", () => {
    const student = { ...baseStudent, row: 0 };
    render(<StudentRowCard student={student} index={4} />);

    expect(screen.getByText("5")).toBeInTheDocument();
  });

  it("displays every meta chip", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(screen.getByText("1a")).toBeInTheDocument();
    expect(screen.getByText("Gruppe A")).toBeInTheDocument();
    expect(screen.getByText("Anna Mustermann")).toBeInTheDocument();
  });

  it("shows 'Neu' badge for new status", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(screen.getByText("Neu")).toBeInTheDocument();
  });

  it("shows 'Vorhanden' badge for existing status", () => {
    const student = { ...baseStudent, status: "existing" as const };
    render(<StudentRowCard student={student} index={0} />);

    expect(screen.getByText("Vorhanden")).toBeInTheDocument();
  });

  it("shows 'Fehler' badge for error status", () => {
    const student = { ...baseStudent, status: "error" as const };
    render(<StudentRowCard student={student} index={0} />);

    expect(screen.getByText("Fehler")).toBeInTheDocument();
  });

  it("shows 'Warnung' badge for warning status", () => {
    const student = { ...baseStudent, status: "warning" as const };
    render(<StudentRowCard student={student} index={0} />);

    expect(screen.getByText("Warnung")).toBeInTheDocument();
  });

  it("displays errors when present", () => {
    const student = {
      ...baseStudent,
      errors: ["Missing field", "Invalid format"],
    };
    render(<StudentRowCard student={student} index={0} />);

    expect(
      screen.getByText("Missing field, Invalid format"),
    ).toBeInTheDocument();
  });

  it("hides errors section when no errors", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(screen.queryByText(/Missing field/)).not.toBeInTheDocument();
  });

  it("skips empty meta entries", () => {
    const student = { ...baseStudent, meta: ["1a", "", "Anna Mustermann"] };
    render(<StudentRowCard student={student} index={0} />);

    expect(screen.getByText("1a")).toBeInTheDocument();
    expect(screen.getByText("Anna Mustermann")).toBeInTheDocument();
  });

  it("renders no meta row when every entry is empty", () => {
    const student = { ...baseStudent, meta: ["", "", ""] };
    const { container } = render(
      <StudentRowCard student={student} index={0} />,
    );

    expect(screen.queryByText("1a")).not.toBeInTheDocument();
    expect(screen.queryByText("Anna Mustermann")).not.toBeInTheDocument();
    expect(container.querySelector(".text-gray-500")).not.toBeInTheDocument();
  });
});
