import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { SupervisorContactCard } from "./SupervisorContactCard";

describe("SupervisorContactCard", () => {
  it("returns null when supervisors is empty", () => {
    const { container } = render(
      <SupervisorContactCard supervisors={[]} studentName="Max" />,
    );

    expect(container.innerHTML).toBe("");
  });

  it("renders supervisor name and role badge", () => {
    render(
      <SupervisorContactCard
        supervisors={[
          {
            id: 1,
            first_name: "Anna",
            last_name: "Schmidt",
            email: "anna@test.com",
            role: "supervisor",
          },
        ]}
        studentName="Max Mustermann"
      />,
    );

    expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
    expect(screen.getByText("Gruppenleitung")).toBeInTheDocument();
  });

  it("renders title", () => {
    render(
      <SupervisorContactCard
        supervisors={[
          {
            id: 1,
            first_name: "Anna",
            last_name: "Schmidt",
            email: "anna@test.com",
            role: "supervisor",
          },
        ]}
        studentName="Max"
      />,
    );

    expect(screen.getByText("Ansprechpartner")).toBeInTheDocument();
  });

  it("renders email when provided", () => {
    render(
      <SupervisorContactCard
        supervisors={[
          {
            id: 1,
            first_name: "Anna",
            last_name: "Schmidt",
            email: "anna@test.com",
            role: "supervisor",
          },
        ]}
        studentName="Max"
      />,
    );

    expect(screen.getByText("anna@test.com")).toBeInTheDocument();
    expect(screen.getByText("Kontakt aufnehmen")).toBeInTheDocument();
  });

  it("renders multiple supervisors", () => {
    render(
      <SupervisorContactCard
        supervisors={[
          {
            id: 1,
            first_name: "Anna",
            last_name: "Schmidt",
            email: "anna@test.com",
            role: "supervisor",
          },
          {
            id: 2,
            first_name: "Peter",
            last_name: "Müller",
            email: "peter@test.com",
            role: "supervisor",
          },
        ]}
        studentName="Max"
      />,
    );

    expect(screen.getByText("Anna Schmidt")).toBeInTheDocument();
    expect(screen.getByText("Peter Müller")).toBeInTheDocument();
  });

  it("hides contact button when no email", () => {
    render(
      <SupervisorContactCard
        supervisors={[
          {
            id: 1,
            first_name: "Anna",
            last_name: "Schmidt",
            role: "supervisor",
          },
        ]}
        studentName="Max"
      />,
    );

    expect(screen.queryByText("Kontakt aufnehmen")).not.toBeInTheDocument();
  });
});
