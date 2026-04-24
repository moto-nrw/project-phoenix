import { describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { Student } from "~/lib/student-helpers";
import { StudentListItem } from "./student-list-item";

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

describe("StudentListItem", () => {
  it("renders first + second name when both provided", () => {
    render(
      <StudentListItem
        student={makeStudent()}
        isSelected={false}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("Max Muster")).toBeInTheDocument();
  });

  it("falls back to name field when no second_name", () => {
    render(
      <StudentListItem
        student={makeStudent({
          first_name: undefined,
          second_name: undefined,
          name: "FallbackName",
        })}
        isSelected={false}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("FallbackName")).toBeInTheDocument();
  });

  it("falls back to 'Unbekannt' when no name info", () => {
    render(
      <StudentListItem
        student={makeStudent({
          first_name: undefined,
          second_name: undefined,
          name: "",
        })}
        isSelected={false}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("Unbekannt")).toBeInTheDocument();
  });

  it("shows group name in subtitle", () => {
    render(
      <StudentListItem
        student={makeStudent({ group_name: "Füchse" })}
        isSelected={false}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText(/Füchse/)).toBeInTheDocument();
  });

  it("appends arrival summary after group name", () => {
    render(
      <StudentListItem
        student={makeStudent({ group_name: "Füchse" })}
        isSelected={false}
        onSelect={vi.fn()}
        arrivalSummary="Mo-Fr 08:00"
      />,
    );

    expect(screen.getByText("Füchse · Mo-Fr 08:00")).toBeInTheDocument();
  });

  it("shows 'keine Ankunft' badge and alert icon when hasArrival is false", () => {
    render(
      <StudentListItem
        student={makeStudent({ group_name: "Füchse" })}
        isSelected={false}
        onSelect={vi.fn()}
        hasArrival={false}
      />,
    );

    expect(screen.getByText(/keine Ankunft/)).toBeInTheDocument();
    expect(screen.getByLabelText("Ankunft fehlt")).toBeInTheDocument();
  });

  it("prefers arrivalSummary over missing arrival message", () => {
    render(
      <StudentListItem
        student={makeStudent()}
        isSelected={false}
        onSelect={vi.fn()}
        hasArrival={false}
        arrivalSummary="Eigene Zeit"
      />,
    );

    expect(screen.getByText("Eigene Zeit")).toBeInTheDocument();
    expect(screen.queryByText(/keine Ankunft/)).not.toBeInTheDocument();
  });

  it("shows em dash when no subtitle data", () => {
    render(
      <StudentListItem
        student={makeStudent({ group_name: undefined })}
        isSelected={false}
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("applies selected styles and aria-current", () => {
    render(
      <StudentListItem
        student={makeStudent()}
        isSelected={true}
        onSelect={vi.fn()}
      />,
    );

    const button = screen.getByRole("button");
    expect(button).toHaveAttribute("aria-current", "true");
  });

  it("calls onSelect when clicked", () => {
    const onSelect = vi.fn();
    render(
      <StudentListItem
        student={makeStudent()}
        isSelected={false}
        onSelect={onSelect}
      />,
    );

    fireEvent.click(screen.getByRole("button"));
    expect(onSelect).toHaveBeenCalled();
  });
});
