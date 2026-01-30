import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import { PersonalInfoCard } from "./PersonalInfoCard";

vi.mock("~/components/ui/info-card", () => ({
  InfoItem: ({ label, value }: { label: string; value: React.ReactNode }) => (
    <div data-testid={`info-item-${label}`}>
      <span>{label}</span>
      <span>{value}</span>
    </div>
  ),
}));

const baseStudent = {
  name: "Max Mustermann",
  school_class: "3a",
  group_name: "Gruppe A",
  birthday: "2015-06-15T00:00:00Z",
  buskind: true,
  pickup_status: "Wird abgeholt",
  health_info: "Nussallergie",
  sick: false,
};

describe("PersonalInfoCard", () => {
  it("renders title", () => {
    render(
      <PersonalInfoCard
        student={baseStudent}
        isEditing={false}
        hasFullAccess={true}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Persönliche Informationen")).toBeInTheDocument();
  });

  it("shows edit button when hasFullAccess and not editing", () => {
    render(
      <PersonalInfoCard
        student={baseStudent}
        isEditing={false}
        hasFullAccess={true}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByTitle("Bearbeiten")).toBeInTheDocument();
  });

  it("hides edit button when editing", () => {
    render(
      <PersonalInfoCard
        student={baseStudent}
        isEditing={true}
        hasFullAccess={true}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.queryByTitle("Bearbeiten")).not.toBeInTheDocument();
  });

  it("shows read-only badge when no full access", () => {
    render(
      <PersonalInfoCard
        student={baseStudent}
        isEditing={false}
        hasFullAccess={false}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Nur Ansicht")).toBeInTheDocument();
  });

  it("renders student info items", () => {
    render(
      <PersonalInfoCard
        student={baseStudent}
        isEditing={false}
        hasFullAccess={true}
        onEdit={vi.fn()}
      />,
    );

    expect(
      screen.getByTestId("info-item-Vollständiger Name"),
    ).toBeInTheDocument();
    expect(screen.getByTestId("info-item-Klasse")).toBeInTheDocument();
    expect(screen.getByTestId("info-item-Gruppe")).toBeInTheDocument();
  });

  it("shows sick badge when student is sick with full access", () => {
    const sickStudent = {
      ...baseStudent,
      sick: true,
      sick_since: "2025-01-20T00:00:00Z",
    };
    render(
      <PersonalInfoCard
        student={sickStudent}
        isEditing={false}
        hasFullAccess={true}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Krank")).toBeInTheDocument();
  });

  it("shows healthy badge when student is not sick with full access", () => {
    render(
      <PersonalInfoCard
        student={baseStudent}
        isEditing={false}
        hasFullAccess={true}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByText("Nicht krankgemeldet")).toBeInTheDocument();
  });

  it("shows supervisor notes when hasFullAccess", () => {
    const studentWithNotes = {
      ...baseStudent,
      supervisor_notes: "Wichtige Notiz",
    };
    render(
      <PersonalInfoCard
        student={studentWithNotes}
        isEditing={false}
        hasFullAccess={true}
        onEdit={vi.fn()}
      />,
    );

    expect(screen.getByTestId("info-item-Betreuernotizen")).toBeInTheDocument();
  });

  it("hides supervisor notes when no full access", () => {
    const studentWithNotes = {
      ...baseStudent,
      supervisor_notes: "Wichtige Notiz",
    };
    render(
      <PersonalInfoCard
        student={studentWithNotes}
        isEditing={false}
        hasFullAccess={false}
        onEdit={vi.fn()}
      />,
    );

    expect(
      screen.queryByTestId("info-item-Betreuernotizen"),
    ).not.toBeInTheDocument();
  });
});
