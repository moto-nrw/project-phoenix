import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import GuardianList from "./guardian-list";

vi.mock("@/lib/guardian-helpers", () => ({
  getGuardianFullName: (g: { firstName?: string; lastName?: string }) =>
    `${g.firstName ?? ""} ${g.lastName ?? ""}`.trim(),
  getRelationshipTypeLabel: (type: string) => {
    const labels: Record<string, string> = {
      mother: "Mutter",
      father: "Vater",
      guardian: "Vormund",
    };
    return labels[type] ?? type;
  },
}));

vi.mock("~/components/simple/student", () => ({
  ModernContactActions: ({
    email,
    phone,
    studentName,
  }: {
    email?: string;
    phone?: string;
    studentName: string;
  }) => (
    <div data-testid="contact-actions">
      <span data-testid="contact-email">{email}</span>
      <span data-testid="contact-phone">{phone}</span>
      <span data-testid="contact-name">{studentName}</span>
    </div>
  ),
}));

const mockGuardians = [
  {
    id: "1",
    firstName: "Anna",
    lastName: "Müller",
    email: "anna@example.com",
    phone: "01234567890",
    mobilePhone: "01234567891",
    isPrimary: true,
    relationshipType: "mother",
    isEmergencyContact: true,
  },
  {
    id: "2",
    firstName: "Hans",
    lastName: "Müller",
    email: "hans@example.com",
    phone: null,
    mobilePhone: "09876543210",
    isPrimary: false,
    relationshipType: "father",
    isEmergencyContact: false,
  },
];

describe("GuardianList", () => {
  it("renders empty state when no guardians", () => {
    render(<GuardianList guardians={[]} />);

    expect(
      screen.getByText("Keine Erziehungsberechtigten zugewiesen"),
    ).toBeInTheDocument();
  });

  it("renders guardian names", () => {
    render(<GuardianList guardians={mockGuardians} />);

    expect(screen.getAllByText("Anna Müller").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Hans Müller").length).toBeGreaterThan(0);
  });

  it("shows primary badge for primary guardian", () => {
    render(<GuardianList guardians={mockGuardians} />);

    expect(screen.getByText("Primär")).toBeInTheDocument();
  });

  it("shows relationship type when showRelationship is true", () => {
    render(<GuardianList guardians={mockGuardians} showRelationship={true} />);

    expect(screen.getByText("Mutter")).toBeInTheDocument();
    expect(screen.getByText("Vater")).toBeInTheDocument();
  });

  it("hides relationship type when showRelationship is false", () => {
    render(<GuardianList guardians={mockGuardians} showRelationship={false} />);

    expect(screen.queryByText("Mutter")).not.toBeInTheDocument();
    expect(screen.queryByText("Vater")).not.toBeInTheDocument();
  });

  it("displays contact information", () => {
    render(<GuardianList guardians={mockGuardians} />);

    expect(screen.getAllByText("anna@example.com").length).toBeGreaterThan(0);
    expect(screen.getAllByText("01234567890").length).toBeGreaterThan(0);
    expect(screen.getAllByText("01234567891").length).toBeGreaterThan(0);
  });

  it("shows 'Nicht angegeben' for missing phone", () => {
    render(<GuardianList guardians={mockGuardians} />);

    // Hans has no phone, should show Nicht angegeben
    expect(screen.getAllByText("Nicht angegeben").length).toBeGreaterThan(0);
  });

  it("shows emergency contact indicator", () => {
    render(<GuardianList guardians={mockGuardians} showRelationship={true} />);

    expect(screen.getByText("Notfallkontakt")).toBeInTheDocument();
  });

  it("renders edit buttons when not readOnly", () => {
    const onEdit = vi.fn();
    render(<GuardianList guardians={mockGuardians} onEdit={onEdit} />);

    const editButtons = screen.getAllByTitle("Bearbeiten");
    expect(editButtons.length).toBe(2);
  });

  it("renders delete buttons when not readOnly", () => {
    const onDelete = vi.fn();
    render(<GuardianList guardians={mockGuardians} onDelete={onDelete} />);

    const deleteButtons = screen.getAllByTitle("Entfernen");
    expect(deleteButtons.length).toBe(2);
  });

  it("hides action buttons when readOnly", () => {
    render(
      <GuardianList
        guardians={mockGuardians}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        readOnly={true}
      />,
    );

    expect(screen.queryByTitle("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.queryByTitle("Entfernen")).not.toBeInTheDocument();
  });

  it("calls onEdit when edit button clicked", () => {
    const onEdit = vi.fn();
    render(<GuardianList guardians={mockGuardians} onEdit={onEdit} />);

    const editButtons = screen.getAllByTitle("Bearbeiten");
    fireEvent.click(editButtons[0]!);

    expect(onEdit).toHaveBeenCalledWith(mockGuardians[0]);
  });

  it("calls onDelete when delete button clicked", () => {
    const onDelete = vi.fn();
    render(<GuardianList guardians={mockGuardians} onDelete={onDelete} />);

    const deleteButtons = screen.getAllByTitle("Entfernen");
    fireEvent.click(deleteButtons[0]!);

    expect(onDelete).toHaveBeenCalledWith(mockGuardians[0]);
  });

  it("renders ModernContactActions for each guardian", () => {
    render(<GuardianList guardians={mockGuardians} />);

    const contactActions = screen.getAllByTestId("contact-actions");
    expect(contactActions.length).toBe(2);
  });
});
