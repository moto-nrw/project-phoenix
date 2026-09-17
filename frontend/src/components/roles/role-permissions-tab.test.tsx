import "@testing-library/jest-dom/vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Permission, Role } from "~/lib/auth-helpers";
import { RolePermissionsTab } from "./role-permissions-tab";

const {
  mockToastSuccess,
  mockGetPermissions,
  mockGetRolePermissions,
  mockReplaceRolePermissions,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockGetPermissions: vi.fn((): Promise<Permission[]> => Promise.resolve([])),
  mockGetRolePermissions: vi.fn((): Promise<Permission[]> =>
    Promise.resolve([]),
  ),
  mockReplaceRolePermissions: vi.fn(() => Promise.resolve()),
}));

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: vi.fn() }),
}));

vi.mock("~/lib/auth-service", () => ({
  authService: {
    getPermissions: mockGetPermissions,
    getRolePermissions: mockGetRolePermissions,
    replaceRolePermissions: mockReplaceRolePermissions,
  },
}));

vi.mock("~/lib/permission-labels", () => ({
  localizeAction: (action: string) => action,
  localizeDescription: (
    _resource: string,
    _action: string,
    description?: string,
  ) => description ?? "",
  localizeResource: (resource: string) => resource,
  formatPermissionDisplay: (resource: string, action: string) =>
    `${resource}:${action}`,
}));

function permission(
  id: string,
  resource: string,
  action: string,
  description = "",
): Permission {
  return {
    id,
    name: `${resource}.${action}`,
    description,
    resource,
    action,
    createdAt: "",
    updatedAt: "",
  };
}

const allPermissions = [
  permission("p1", "students", "read", "Kinder ansehen"),
  permission("p2", "students", "update"),
  permission("p3", "rooms", "read"),
];

const role: Role = {
  id: "1",
  name: "Vertretungslehrkraft",
  description: "",
  isSystem: false,
  createdAt: "",
  updatedAt: "",
};

describe("RolePermissionsTab", () => {
  const onSaved = vi.fn(async () => undefined);
  const onCancelEdit = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockGetPermissions.mockResolvedValue(allPermissions);
    mockGetRolePermissions.mockResolvedValue([allPermissions[0]!]);
  });

  it("lists the assigned permissions by resource in the view state", async () => {
    render(
      <RolePermissionsTab
        role={role}
        editing={false}
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByText("1 Berechtigungen zugewiesen."),
    ).toBeInTheDocument();
    expect(screen.getByText("students:read")).toBeInTheDocument();
    expect(screen.queryByText("rooms:read")).not.toBeInTheDocument();
    // Ansicht: keine Kästchen, kein Speichern.
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(mockGetPermissions).not.toHaveBeenCalled();
  });

  it("names the next step when nothing is assigned yet", async () => {
    mockGetRolePermissions.mockResolvedValue([]);
    render(
      <RolePermissionsTab
        role={role}
        editing={false}
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByText(/Noch keine Berechtigungen zugewiesen/),
    ).toBeInTheDocument();
  });

  it("keeps the catalogue when the earlier read-only request finishes late", async () => {
    let resolveReadOnly: (permissions: Permission[]) => void = () => undefined;
    mockGetRolePermissions
      .mockImplementationOnce(
        () =>
          new Promise<Permission[]>((resolve) => {
            resolveReadOnly = resolve;
          }),
      )
      .mockResolvedValueOnce([allPermissions[0]!]);
    const { rerender } = render(
      <RolePermissionsTab
        role={role}
        editing={false}
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    rerender(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByText("1 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();

    await act(async () => resolveReadOnly([allPermissions[0]!]));

    await waitFor(() => {
      expect(screen.getByLabelText("rooms:read")).toBeInTheDocument();
    });
  });

  it("replaces the complete selection with one save request", async () => {
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByText("1 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();
    const save = screen.getByRole("button", { name: "Speichern" });
    expect(save).toBeDisabled();

    fireEvent.click(screen.getByLabelText("students:read"));
    fireEvent.click(screen.getByLabelText("rooms:read"));
    expect(
      screen.getByText("1 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();
    expect(save).toBeEnabled();
    fireEvent.click(save);

    await waitFor(() => {
      expect(mockReplaceRolePermissions).toHaveBeenCalledWith("1", ["p3"]);
    });
    expect(mockReplaceRolePermissions).toHaveBeenCalledTimes(1);
    await waitFor(() => {
      expect(onSaved).toHaveBeenCalledOnce();
    });
    expect(mockToastSuccess).toHaveBeenCalledWith(
      "Berechtigungen gespeichert.",
    );
  });

  it("toggles a whole resource group and filters by search", async () => {
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );
    await screen.findByText("1 von 3 Berechtigungen ausgewählt.");

    // students:read war schon gewählt, students:update kommt dazu; rooms
    // bleibt unberührt.
    fireEvent.click(
      screen.getByLabelText("Alle Berechtigungen für students auswählen"),
    );
    expect(
      screen.getByText("2 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("students:update")).toBeChecked();
    expect(screen.getByLabelText("rooms:read")).not.toBeChecked();

    fireEvent.change(screen.getByLabelText("Berechtigungen suchen"), {
      target: { value: "rooms" },
    });
    expect(screen.queryByLabelText("students:read")).not.toBeInTheDocument();
    expect(screen.getByLabelText("rooms:read")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Alle auswählen" }),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Berechtigungen suchen"), {
      target: { value: "gibtesnicht" },
    });
    expect(
      screen.getByText("Keine Berechtigungen gefunden."),
    ).toBeInTheDocument();
  });

  it("starts the draft at the saved state again after cancelling", async () => {
    const { rerender } = render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );
    await screen.findByText("1 von 3 Berechtigungen ausgewählt.");

    fireEvent.click(screen.getByLabelText("rooms:read"));
    expect(
      screen.getByText("2 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();

    // Abbrechen: der Aufrufer beendet den Bearbeiten-Zustand.
    rerender(
      <RolePermissionsTab
        role={role}
        editing={false}
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );
    await screen.findByText("1 Berechtigungen zugewiesen.");

    rerender(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByText("1 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("rooms:read")).not.toBeChecked();
  });

  it("keeps a click that lands while the permissions load commits", async () => {
    let resolveAssigned: (permissions: Permission[]) => void = () => undefined;
    mockGetRolePermissions.mockImplementationOnce(
      () =>
        new Promise<Permission[]>((resolve) => {
          resolveAssigned = resolve;
        }),
    );
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    // Ein MutationObserver feuert als Microtask direkt nach dem Commit, der
    // die Liste einsetzt, also vor den passiven Effekten desselben Commits.
    // Genau in diesem Fenster landete der Klick, den der Reset-Effekt auf CI
    // verworfen hat (#3346).
    const clicked = new Promise<void>((resolve) => {
      const observer = new MutationObserver(() => {
        const groupToggle = screen.queryByLabelText<HTMLInputElement>(
          "Alle Berechtigungen für students auswählen",
        );
        if (!groupToggle) return;
        observer.disconnect();
        groupToggle.click();
        resolve();
      });
      observer.observe(document.body, { childList: true, subtree: true });
    });

    // Nur ausserhalb von act behält React diese Reihenfolge: act zieht Commit
    // und passive Effekte zusammen, der Klick käme also nie dazwischen.
    const actEnvironment = globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = false;
    try {
      resolveAssigned([allPermissions[0]!]);
      await clicked;
    } finally {
      actEnvironment.IS_REACT_ACT_ENVIRONMENT = true;
    }
    // Zieht die passiven Effekte des Commits nach, die den Klick verworfen
    // haben.
    await act(async () => undefined);

    expect(
      screen.getByText("2 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("students:update")).toBeChecked();
  });

  it("shows the save error in the alert and keeps the draft", async () => {
    mockReplaceRolePermissions.mockRejectedValueOnce(new Error("offline"));
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );
    await screen.findByText("1 von 3 Berechtigungen ausgewählt.");

    fireEvent.click(screen.getByLabelText("rooms:read"));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Berechtigungen konnten nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("rooms:read")).toBeChecked();
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("hands cancel back to the caller", async () => {
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );
    await screen.findByText("1 von 3 Berechtigungen ausgewählt.");

    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onCancelEdit).toHaveBeenCalledOnce();
  });

  it("reports when the permissions could not be loaded", async () => {
    mockGetRolePermissions.mockRejectedValueOnce(new Error("offline"));
    render(
      <RolePermissionsTab
        role={role}
        editing={false}
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByText(
        "Die Berechtigungen konnten nicht geladen werden.",
      ),
    ).toBeInTheDocument();
  });

  it("keeps cancellation available when the edit catalogue cannot be loaded", async () => {
    mockGetRolePermissions.mockRejectedValueOnce(new Error("offline"));
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByText(
        "Die Berechtigungen konnten nicht geladen werden.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onCancelEdit).toHaveBeenCalledOnce();
  });
});
