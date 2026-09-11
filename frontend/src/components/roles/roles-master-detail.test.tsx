import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("~/components/ui/hooks/useIsMobile", () => ({
  useIsMobile: vi.fn(() => false),
}));

class MockResizeObserver {
  observe = vi.fn();
  unobserve = vi.fn();
  disconnect = vi.fn();
}

vi.stubGlobal("ResizeObserver", MockResizeObserver);

import { RolesMasterDetail } from "./roles-master-detail";
import type { Role } from "@/lib/auth-helpers";

const customRole: Role = {
  id: "1",
  name: "Vertretungslehrkraft",
  description: "Vertritt im Krankheitsfall",
  isSystem: false,
  baseRole: "teacher",
  createdAt: "2026-01-01",
  updatedAt: "2026-01-02",
  permissions: [
    {
      id: "p1",
      name: "permission.read",
      description: "",
      resource: "x",
      action: "read",
      createdAt: "",
      updatedAt: "",
    },
  ],
};

const systemRole: Role = {
  id: "2",
  name: "admin",
  description: "Administrator",
  isSystem: true,
  baseRole: "admin",
  createdAt: "2026-01-01",
  updatedAt: "2026-01-02",
};

const unclassifiedRole: Role = {
  id: "3",
  name: "Helfer",
  description: "",
  isSystem: false,
  createdAt: "2026-01-01",
  updatedAt: "2026-01-02",
};

describe("RolesMasterDetail", () => {
  const onSelect = vi.fn();
  const onSaveRole = vi.fn(async () => undefined);
  const onDeleteClick = vi.fn();
  const onManagePermissions = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows the empty detail state when nothing is selected", () => {
    render(
      <RolesMasterDetail
        roles={[customRole]}
        selectedId={null}
        selectedRole={null}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onManagePermissions={onManagePermissions}
      />,
    );

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.getByText("Vertretungslehrkraft")).toBeInTheDocument();
  });

  it("renders the selected role detail with permissions count and base role", () => {
    render(
      <RolesMasterDetail
        roles={[customRole]}
        selectedId="1"
        selectedRole={customRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onManagePermissions={onManagePermissions}
      />,
    );

    expect(screen.getAllByText("Vertretungslehrkraft").length).toBeGreaterThan(
      0,
    );
    expect(screen.getByText("1 Berechtigungen")).toBeInTheDocument();
  });

  it("opens the inline edit form and triggers delete and manage-permissions for non-system roles", () => {
    render(
      <RolesMasterDetail
        roles={[customRole]}
        selectedId="1"
        selectedRole={customRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onManagePermissions={onManagePermissions}
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));
    expect(onDeleteClick).toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: /^Berechtigungen$/ }));
    expect(onManagePermissions).toHaveBeenCalled();

    // Bearbeitet wird im Detailbereich, nicht in einem Modal daneben: der
    // Knopf schaltet das Formular ein, mit Speichern und Abbrechen unten.
    fireEvent.click(screen.getByText("Bearbeiten"));
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Abbrechen" }),
    ).toBeInTheDocument();
  });

  it("hides edit, delete, and permission actions for system roles", () => {
    render(
      <RolesMasterDetail
        roles={[systemRole]}
        selectedId="2"
        selectedRole={systemRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onManagePermissions={onManagePermissions}
      />,
    );

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Berechtigungen/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "System-Rollen können nicht bearbeitet oder gelöscht werden.",
      ),
    ).toBeInTheDocument();
  });

  it("shows the 'Zuordnung fehlt' chip for roles without a base role", () => {
    render(
      <RolesMasterDetail
        roles={[unclassifiedRole]}
        selectedId="3"
        selectedRole={unclassifiedRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onManagePermissions={onManagePermissions}
      />,
    );

    expect(screen.getByText("Zuordnung fehlt")).toBeInTheDocument();
  });

  it("groups roles under a single 'Alle Rollen' bucket and renders a Stammdaten tab", () => {
    render(
      <RolesMasterDetail
        roles={[customRole, systemRole, unclassifiedRole]}
        selectedId="1"
        selectedRole={customRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onManagePermissions={onManagePermissions}
      />,
    );

    expect(screen.getByText("Alle Rollen")).toBeInTheDocument();
    expect(screen.getByRole("tab", { name: "Stammdaten" })).toBeInTheDocument();
  });

  it("renders the loading spinner instead of role data while detailLoading is true", () => {
    render(
      <RolesMasterDetail
        roles={[customRole]}
        selectedId="1"
        selectedRole={customRole}
        detailLoading={true}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onManagePermissions={onManagePermissions}
      />,
    );

    expect(
      screen.getByText("Rollendaten werden geladen..."),
    ).toBeInTheDocument();
    // The Stammdaten data fields must NOT render while loading. "Berechtigungen"
    // is the InfoSection-only label for the permission count grid; the header
    // subtitle has its own count chip.
    expect(screen.queryByText("Rollendetails")).not.toBeInTheDocument();
  });
});
