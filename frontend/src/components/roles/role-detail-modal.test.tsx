import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";

// Mock the Modal components
vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    children,
    isOpen,
  }: {
    children: React.ReactNode;
    isOpen: boolean;
  }) => (isOpen ? <div data-testid="modal">{children}</div> : null),
  ConfirmationModal: () => null,
}));

import { RoleDetailModal } from "./role-detail-modal";

describe("RoleDetailModal", () => {
  const mockOnClose = vi.fn();
  const mockOnEdit = vi.fn();
  const mockOnDelete = vi.fn();

  it("displays translated name for admin role", () => {
    const role = {
      id: "1",
      name: "admin",
      description: "System administrator",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(
      <RoleDetailModal
        isOpen={true}
        onClose={mockOnClose}
        role={role}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    // Check that "Administrator" appears (translated from "admin")
    expect(screen.getAllByText("Administrator").length).toBeGreaterThan(0);
  });

  it("displays translated description for system roles", () => {
    const role = {
      id: "1",
      name: "user",
      description: "Standard user",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(
      <RoleDetailModal
        isOpen={true}
        onClose={mockOnClose}
        role={role}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    // Check that translated description appears
    expect(
      screen.getAllByText("Standardbenutzer mit grundlegenden Berechtigungen")
        .length,
    ).toBeGreaterThan(0);
  });

  it("displays initials from translated name", () => {
    const role = {
      id: "1",
      name: "guest",
      description: "Guest access",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(
      <RoleDetailModal
        isOpen={true}
        onClose={mockOnClose}
        role={role}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    // Check that "GA" (first 2 chars of "Gast") appears as initials
    expect(screen.getByText("GA")).toBeInTheDocument();
  });

  it("returns null when role is null", () => {
    const { container } = render(
      <RoleDetailModal
        isOpen={true}
        onClose={mockOnClose}
        role={null}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it("displays original name for non-system roles", () => {
    const role = {
      id: "1",
      name: "custom_role",
      description: "A custom role",
      permissions: [],
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
    };

    render(
      <RoleDetailModal
        isOpen={true}
        onClose={mockOnClose}
        role={role}
        onEdit={mockOnEdit}
        onDelete={mockOnDelete}
      />,
    );

    expect(screen.getAllByText("custom_role").length).toBeGreaterThan(0);
  });
});
