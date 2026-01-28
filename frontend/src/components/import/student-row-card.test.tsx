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
    school_class: "1a",
    group_name: "Gruppe A",
    guardian_info: "Anna Mustermann",
    health_info: "",
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

  it("displays school class", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(screen.getByText("1a")).toBeInTheDocument();
  });

  it("displays group name", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(screen.getByText("Gruppe A")).toBeInTheDocument();
  });

  it("displays guardian info", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

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

    expect(screen.getByText("Missing field, Invalid format")).toBeInTheDocument();
  });

  it("hides errors section when no errors", () => {
    render(<StudentRowCard student={baseStudent} index={0} />);

    expect(
      screen.queryByText(/Missing field/),
    ).not.toBeInTheDocument();
  });

  it("hides separator when school_class is empty", () => {
    const student = { ...baseStudent, school_class: "" };
    render(<StudentRowCard student={student} index={0} />);

    expect(screen.queryByText("1a")).not.toBeInTheDocument();
  });

  it("hides group name when empty", () => {
    const student = { ...baseStudent, group_name: "" };
    render(<StudentRowCard student={student} index={0} />);

    expect(screen.queryByText("Gruppe A")).not.toBeInTheDocument();
  });

  it("hides guardian info when empty", () => {
    const student = { ...baseStudent, guardian_info: "" };
    render(<StudentRowCard student={student} index={0} />);

    expect(screen.queryByText("Anna Mustermann")).not.toBeInTheDocument();
  });
});
