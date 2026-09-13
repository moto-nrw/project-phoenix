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

// Der Reiter lädt über authService; hier zählt nur, dass der Detailbereich
// ihn mit dem Bearbeiten-Zustand des offenen Reiters versorgt.
vi.mock("~/components/roles/role-permissions-tab", () => ({
  RolePermissionsTab: ({
    editing,
    onCancelEdit,
  }: {
    editing: boolean;
    onCancelEdit: () => void;
  }) => (
    <div data-testid="role-permissions-tab" data-editing={editing}>
      {editing ? (
        <button type="button" onClick={onCancelEdit}>
          Abbrechen
        </button>
      ) : null}
    </div>
  ),
}));

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
  const onPermissionsSaved = vi.fn(async () => undefined);

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
        onPermissionsSaved={onPermissionsSaved}
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
        onPermissionsSaved={onPermissionsSaved}
      />,
    );

    expect(screen.getAllByText("Vertretungslehrkraft").length).toBeGreaterThan(
      0,
    );
    expect(screen.getByText("1 Berechtigungen")).toBeInTheDocument();
  });

  it("opens the inline edit form and triggers delete for non-system roles", () => {
    render(
      <RolesMasterDetail
        roles={[customRole]}
        selectedId="1"
        selectedRole={customRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onPermissionsSaved={onPermissionsSaved}
      />,
    );

    fireEvent.click(screen.getByText("Löschen"));
    expect(onDeleteClick).toHaveBeenCalled();

    // Berechtigungen sind ein zweiter Reiter, kein Knopf im Kopf (#3116).
    expect(
      screen.queryByRole("button", { name: /^Berechtigungen$/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: "Berechtigungen" }),
    ).toBeInTheDocument();

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

  it("switches the permissions tab into its own edit state", () => {
    render(
      <RolesMasterDetail
        roles={[customRole]}
        selectedId="1"
        selectedRole={customRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onPermissionsSaved={onPermissionsSaved}
      />,
    );

    // Radix Tabs aktivieren auf mousedown, nicht auf click.
    fireEvent.mouseDown(screen.getByRole("tab", { name: "Berechtigungen" }), {
      button: 0,
    });
    const tab = screen.getByTestId("role-permissions-tab");
    expect(tab).toHaveAttribute("data-editing", "false");

    fireEvent.click(screen.getByText("Bearbeiten"));
    expect(screen.getByTestId("role-permissions-tab")).toHaveAttribute(
      "data-editing",
      "true",
    );
    // Der Kopf zeigt im Bearbeiten-Zustand weder Bearbeiten noch Löschen.
    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(screen.getByTestId("role-permissions-tab")).toHaveAttribute(
      "data-editing",
      "false",
    );
    expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
  });

  it("ends the edit state when the tab changes", () => {
    render(
      <RolesMasterDetail
        roles={[customRole]}
        selectedId="1"
        selectedRole={customRole}
        detailLoading={false}
        onSelect={onSelect}
        onSaveRole={onSaveRole}
        onDeleteClick={onDeleteClick}
        onPermissionsSaved={onPermissionsSaved}
      />,
    );

    fireEvent.click(screen.getByText("Bearbeiten"));
    expect(
      screen.getByRole("button", { name: "Speichern" }),
    ).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByRole("tab", { name: "Berechtigungen" }), {
      button: 0,
    });
    expect(screen.getByTestId("role-permissions-tab")).toHaveAttribute(
      "data-editing",
      "false",
    );
    expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
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
        onPermissionsSaved={onPermissionsSaved}
      />,
    );

    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
    // Die Berechtigungen einer Systemrolle bleiben ansehbar.
    expect(
      screen.getByRole("tab", { name: "Berechtigungen" }),
    ).toBeInTheDocument();
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
        onPermissionsSaved={onPermissionsSaved}
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
        onPermissionsSaved={onPermissionsSaved}
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
        onPermissionsSaved={onPermissionsSaved}
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
