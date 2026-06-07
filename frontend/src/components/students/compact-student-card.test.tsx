import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { CompactStudentCard } from "./compact-student-card";

describe("CompactStudentCard", () => {
  it("renders the student's full name", () => {
    render(
      <CompactStudentCard
        studentId="42"
        firstName="Anna"
        lastName="Müller"
        schoolClass="3a"
        groupName="Bärengruppe"
      />,
    );

    expect(screen.getByText("Anna Müller")).toBeInTheDocument();
  });

  it("joins schoolClass and groupName with a middle dot", () => {
    render(
      <CompactStudentCard
        studentId="42"
        firstName="Anna"
        lastName="Müller"
        schoolClass="3a"
        groupName="Bärengruppe"
      />,
    );

    expect(screen.getByText("3a · Bärengruppe")).toBeInTheDocument();
  });

  it("shows only schoolClass when groupName is missing", () => {
    render(
      <CompactStudentCard
        studentId="42"
        firstName="Anna"
        lastName="Müller"
        schoolClass="3a"
      />,
    );

    expect(screen.getByText("3a")).toBeInTheDocument();
  });

  it("shows only groupName when schoolClass is missing", () => {
    render(
      <CompactStudentCard
        studentId="42"
        firstName="Anna"
        lastName="Müller"
        groupName="Bärengruppe"
      />,
    );

    expect(screen.getByText("Bärengruppe")).toBeInTheDocument();
  });

  it("omits the meta line entirely when both schoolClass and groupName are missing", () => {
    const { container } = render(
      <CompactStudentCard studentId="42" firstName="Anna" lastName="Müller" />,
    );

    // Only the name <p> should render, no second <p> for meta.
    expect(container.querySelectorAll("p")).toHaveLength(1);
  });

  it("calls onClick when the button is activated", async () => {
    const onClick = vi.fn();
    render(
      <CompactStudentCard
        studentId="42"
        firstName="Anna"
        lastName="Müller"
        schoolClass="3a"
        onClick={onClick}
      />,
    );

    screen.getByRole("button").click();

    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("exposes an aria-label and data-testid keyed by studentId", () => {
    render(
      <CompactStudentCard studentId={42} firstName="Anna" lastName="Müller" />,
    );

    expect(screen.getByTestId("compact-student-card-42")).toBeInTheDocument();
  });

  it("uses a button only when an onClick handler is provided", () => {
    const onClick = vi.fn();
    render(
      <CompactStudentCard
        studentId={42}
        firstName="Anna"
        lastName="Müller"
        onClick={onClick}
      />,
    );

    const button = screen.getByRole("button", {
      name: "Anna Müller - Profil öffnen",
    });
    expect(button).toHaveAttribute("data-testid", "compact-student-card-42");
    expect(button).toHaveAttribute("type", "button");
  });
});
