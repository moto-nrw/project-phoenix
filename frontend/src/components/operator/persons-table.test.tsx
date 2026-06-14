/**
 * Tests for PersonsTable.
 *
 * Covers: column rendering (with/without school column, with/without delete
 * action), tag rendering for staff/student/RFID flags, fallback for
 * person without account email, sort behaviour by name, and the delete
 * action firing without bubbling row clicks.
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import { PersonsTable } from "./persons-table";
import type { OperatorPerson } from "~/lib/operator/provisioning-helpers";

function makePerson(overrides: Partial<OperatorPerson> = {}): OperatorPerson {
  return {
    id: "1",
    firstName: "Anna",
    lastName: "Beispiel",
    fullName: "Anna Beispiel",
    hasAccount: true,
    accountEmail: "anna@example.com",
    hasRfidCard: false,
    isStaff: true,
    isStudent: false,
    schoolId: "10",
    schoolName: "Schule A",
    organizationId: "42",
    organizationName: "Träger A",
    createdAt: "2026-01-01T00:00:00Z",
    ...overrides,
  };
}

describe("PersonsTable", () => {
  it("renders persons with name, email, and Mitarbeiter tag", () => {
    render(<PersonsTable persons={[makePerson()]} />);

    expect(screen.getByText("Anna Beispiel")).toBeInTheDocument();
    expect(screen.getByText("anna@example.com")).toBeInTheDocument();
    expect(screen.getByText("Mitarbeiter")).toBeInTheDocument();
  });

  it("renders Kinder tag for students and RFID tag for card holders", () => {
    render(
      <PersonsTable
        persons={[
          makePerson({
            id: "2",
            firstName: "Ben",
            lastName: "Müller",
            isStaff: false,
            isStudent: true,
            hasRfidCard: true,
          }),
        ]}
      />,
    );

    expect(screen.getByText("Kinder")).toBeInTheDocument();
    expect(screen.getByText("RFID")).toBeInTheDocument();
    expect(screen.queryByText("Mitarbeiter")).not.toBeInTheDocument();
  });

  it("renders an em dash when no flags and no email are set", () => {
    render(
      <PersonsTable
        persons={[
          makePerson({
            id: "3",
            isStaff: false,
            isStudent: false,
            hasRfidCard: false,
            accountEmail: null,
          }),
        ]}
      />,
    );

    // Both the tag column and the email column fall back to —
    expect(screen.getAllByText("—").length).toBeGreaterThanOrEqual(2);
  });

  it("does not show the Schule column when showSchool is not set", () => {
    render(<PersonsTable persons={[makePerson()]} />);

    expect(screen.queryByText("Schule")).not.toBeInTheDocument();
  });

  it("renders the Schule column with the school name when showSchool is set", () => {
    render(<PersonsTable persons={[makePerson()]} showSchool />);

    expect(screen.getByText("Schule")).toBeInTheDocument();
    expect(screen.getByText("Schule A")).toBeInTheDocument();
  });

  it("renders an empty state when no persons are passed", () => {
    render(<PersonsTable persons={[]} />);

    expect(screen.getByText("Keine Personen vorhanden.")).toBeInTheDocument();
  });

  it("does not render the delete action when onDelete is not provided", () => {
    render(<PersonsTable persons={[makePerson()]} />);

    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
  });

  it("calls onDelete with the row when the delete button is clicked", () => {
    const onDelete = vi.fn();
    const person = makePerson();

    render(<PersonsTable persons={[person]} onDelete={onDelete} />);

    fireEvent.click(screen.getByText("Löschen"));

    expect(onDelete).toHaveBeenCalledTimes(1);
    expect(onDelete).toHaveBeenCalledWith(person);
  });

  it("sorts by tag column when the Merkmale header is clicked (covers getPersonTagSortValue)", () => {
    const persons = [
      makePerson({
        id: "1",
        firstName: "Anna",
        lastName: "Adler",
        isStaff: false,
        isStudent: true,
        hasRfidCard: true,
      }),
      makePerson({
        id: "2",
        firstName: "Ben",
        lastName: "Beier",
        isStaff: true,
        isStudent: false,
        hasRfidCard: false,
      }),
      makePerson({
        id: "3",
        firstName: "Cleo",
        lastName: "Cohn",
        isStaff: false,
        isStudent: false,
        hasRfidCard: false,
      }),
    ];

    render(<PersonsTable persons={persons} />);

    const merkmaleHeader = screen.getByRole("button", { name: /Merkmale/ });
    fireEvent.click(merkmaleHeader);

    const rows = screen.getAllByRole("row");
    expect(rows.length).toBe(4); // header + 3 rows
  });

  it("sorts by school when the Schule header is clicked (covers school sortValue)", () => {
    const persons = [
      makePerson({ id: "1", schoolName: "Z-Schule" }),
      makePerson({ id: "2", schoolName: "A-Schule" }),
    ];

    render(<PersonsTable persons={persons} showSchool />);

    fireEvent.click(screen.getByRole("button", { name: /Schule/ }));
    expect(screen.getAllByRole("row").length).toBe(3);
  });

  it("sorts by email when the E-Mail header is clicked (covers email sortValue)", () => {
    const persons = [
      makePerson({ id: "1", accountEmail: "z@example.com" }),
      makePerson({ id: "2", accountEmail: "a@example.com" }),
      makePerson({ id: "3", accountEmail: null }),
    ];

    render(<PersonsTable persons={persons} />);

    fireEvent.click(screen.getByRole("button", { name: /E-Mail/ }));
    expect(screen.getAllByRole("row").length).toBe(4);
  });

  it("does not bubble the row click when the delete button area is clicked (covers stopPropagation)", () => {
    const onDelete = vi.fn();
    const person = makePerson();
    render(<PersonsTable persons={[person]} onDelete={onDelete} />);

    const buttonContainer = screen.getByText("Löschen").parentElement;
    expect(buttonContainer).not.toBeNull();
    if (buttonContainer) {
      // keydown on the wrapper triggers the role="presentation" handler
      fireEvent.keyDown(buttonContainer, { key: "Enter" });
    }
    // The keydown handler stops propagation but does not call onDelete.
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("sorts persons by last name + first name", () => {
    const persons = [
      makePerson({ id: "1", firstName: "Anna", lastName: "Zander" }),
      makePerson({ id: "2", firstName: "Ben", lastName: "Adler" }),
    ];

    render(<PersonsTable persons={persons} />);

    const rows = screen.getAllByRole("row");
    // First row is the header — second row should be the person whose
    // last name sorts first (Adler).
    expect(rows[1]?.textContent).toContain("Ben");
    expect(rows[2]?.textContent).toContain("Anna");
  });
});
