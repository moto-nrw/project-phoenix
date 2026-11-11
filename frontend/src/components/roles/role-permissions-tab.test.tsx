import "@testing-library/jest-dom/vitest";
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { deferred } from "~/test/helpers/form-modal-pending";
import { ApiError } from "~/lib/api-error";
import type { Permission, Role } from "~/lib/auth-helpers";
import { catalogText } from "~/test/error-catalog-text";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
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

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: mockToastSuccess, error: vi.fn() }),
}));

vi.mock("~/lib/auth-service", () => ({
  authService: {
    getPermissions: mockGetPermissions,
    getRolePermissions: mockGetRolePermissions,
    replaceRolePermissions: mockReplaceRolePermissions,
  },
}));

vi.mock("~/lib/permission-labels", async (importOriginal) => ({
  // Platform and risk rules stay real (#4574); only the wording is stubbed.
  ...(await importOriginal<typeof import("~/lib/permission-labels")>()),
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
      await screen.findByText("1 Berechtigung zugewiesen."),
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
      "Die Berechtigungen der Rolle sind gespeichert.",
    );
  });

  it("locks editing until the saved permissions and parent finish reloading", async () => {
    const reload = deferred<Permission[]>();
    const parentReload = deferred();
    mockGetRolePermissions
      .mockResolvedValueOnce([allPermissions[0]!])
      .mockReturnValueOnce(reload.promise);
    const saved = vi.fn(() => parentReload.promise);
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={saved}
        onCancelEdit={onCancelEdit}
      />,
    );
    const checkbox = await screen.findByLabelText("rooms:read");
    fireEvent.click(checkbox);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(mockGetRolePermissions).toHaveBeenCalledTimes(2),
    );
    expect(checkbox).toBeDisabled();
    await act(async () =>
      reload.resolve([allPermissions[0]!, allPermissions[2]!]),
    );
    expect(saved).toHaveBeenCalledOnce();
    expect(checkbox).toBeDisabled();
    await act(async () => parentReload.resolve());
    expect(checkbox).toBeEnabled();
    expect(checkbox).toBeChecked();
  });

  // #6652: the permissions hold once the write is through; a failed reload is
  // a load error that says the change is saved, and its retry only reloads.
  it("keeps saved permissions saved when the reload fails", async () => {
    const retry = deferred<Permission[]>();
    mockGetRolePermissions
      .mockReset()
      .mockResolvedValueOnce([allPermissions[0]!])
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockReturnValueOnce(retry.promise);
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );
    await screen.findByText("1 von 3 Berechtigungen ausgewählt.");
    fireEvent.click(screen.getByLabelText("students:read"));
    fireEvent.click(screen.getByLabelText("rooms:read"));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige der Berechtigungen")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    await waitFor(() => expect(onSaved).toHaveBeenCalledOnce());
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(mockGetRolePermissions).toHaveBeenCalledTimes(3),
    );
    expect(screen.getByLabelText("rooms:read")).toBeDisabled();
    await act(async () => retry.resolve([allPermissions[2]!]));
    expect(screen.getByLabelText("rooms:read")).toBeEnabled();
    expect(mockReplaceRolePermissions).toHaveBeenCalledTimes(1);
    expect(onSaved).toHaveBeenCalledOnce();
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
    const studentsGroup = screen.getByRole("button", { name: "students" });
    expect(studentsGroup).toHaveAccessibleDescription("1/2");

    // students:read war schon gewählt, students:update kommt dazu; rooms
    // bleibt unberührt.
    fireEvent.click(
      screen.getByLabelText("Alle Berechtigungen für students auswählen"),
    );
    expect(
      screen.getByText("2 von 3 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();
    expect(studentsGroup).toHaveAccessibleDescription("2/2");
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
    await screen.findByText("1 Berechtigung zugewiesen.");

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
    let signalClicked: () => void = () => undefined;
    const clicked = new Promise<void>((resolve) => {
      signalClicked = resolve;
    });
    const observer = new MutationObserver(() => {
      const groupToggle = screen.queryByLabelText<HTMLInputElement>(
        "Alle Berechtigungen für students auswählen",
      );
      if (!groupToggle) return;
      observer.disconnect();
      groupToggle.click();
      signalClicked();
    });
    observer.observe(document.body, { childList: true, subtree: true });

    // Erscheint die Liste nicht, muss das Warten von selbst enden: sonst
    // bliebe das zurückgesetzte IS_REACT_ACT_ENVIRONMENT für den restlichen
    // Lauf stehen und liesse spätere Tests scheitern.
    let expired: ReturnType<typeof setTimeout> | undefined;
    const givesUp = new Promise<never>((_, reject) => {
      expired = setTimeout(
        () => reject(new Error("Die Berechtigungsliste ist nicht erschienen.")),
        2000,
      );
    });

    // Nur ausserhalb von act behält React diese Reihenfolge: act zieht Commit
    // und passive Effekte zusammen, der Klick käme also nie dazwischen.
    const actEnvironment = globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean };
    actEnvironment.IS_REACT_ACT_ENVIRONMENT = false;
    try {
      resolveAssigned([allPermissions[0]!]);
      await Promise.race([clicked, givesUp]);
    } finally {
      clearTimeout(expired);
      observer.disconnect();
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

  it("keeps unsaved permission changes when retrying a failed post-save reload", async () => {
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
    mockGetRolePermissions.mockRejectedValueOnce(new ApiError("down", 503));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    const reloadText = `${catalogText("general.unavailable", "die Anzeige der Berechtigungen")} ${SAVED_BUT_STALE_SUFFIX}`;
    expect(await screen.findByText(reloadText)).toBeVisible();
    await waitFor(() =>
      expect(screen.getByLabelText("students:update")).toBeEnabled(),
    );
    fireEvent.click(screen.getByLabelText("students:update"));
    mockGetRolePermissions.mockResolvedValueOnce([
      allPermissions[0]!,
      allPermissions[2]!,
    ]);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() =>
      expect(screen.queryByText(reloadText)).not.toBeInTheDocument(),
    );
    expect(screen.getByLabelText("students:update")).toBeChecked();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
    expect(mockReplaceRolePermissions).toHaveBeenCalledTimes(1);
  });

  it("clears obsolete post-save feedback after a successful edit-mode read", async () => {
    const props = { role, onSaved, onCancelEdit };
    const { rerender } = render(<RolePermissionsTab {...props} editing />);
    await screen.findByText("1 von 3 Berechtigungen ausgewählt.");
    fireEvent.click(screen.getByLabelText("rooms:read"));
    mockGetRolePermissions.mockRejectedValueOnce(new ApiError("down", 503));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    const reloadText = `${catalogText("general.unavailable", "die Anzeige der Berechtigungen")} ${SAVED_BUT_STALE_SUFFIX}`;
    expect(await screen.findByText(reloadText)).toBeVisible();
    rerender(<RolePermissionsTab {...props} editing={false} />);
    expect(await screen.findByText("1 Berechtigung zugewiesen.")).toBeVisible();
    expect(screen.queryByText(reloadText)).not.toBeInTheDocument();
    rerender(<RolePermissionsTab {...props} editing />);
    expect(await screen.findByLabelText("students:update")).toBeEnabled();
    expect(
      screen.queryByRole("button", { name: "Wiederholen" }),
    ).not.toBeInTheDocument();
  });

  it("shows the save error in the alert and keeps the draft", async () => {
    mockReplaceRolePermissions.mockRejectedValueOnce(
      new ApiError("offline", 503, { code: "general.unavailable" }),
    );
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
        catalogText("general.unavailable", "das Speichern der Berechtigungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("rooms:read")).toBeChecked();
    expect(onSaved).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => {
      expect(mockReplaceRolePermissions).toHaveBeenCalledTimes(2);
    });
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
    mockGetRolePermissions.mockRejectedValueOnce(
      new ApiError("offline", 503, { code: "general.unavailable" }),
    );
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
        catalogText("general.unavailable", "die Liste der Berechtigungen"),
      ),
    ).toBeInTheDocument();
  });

  it("keeps cancellation available when the edit catalogue cannot be loaded", async () => {
    mockGetRolePermissions.mockRejectedValueOnce(
      new ApiError("offline", 503, { code: "general.unavailable" }),
    );
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
        catalogText("general.unavailable", "die Liste der Berechtigungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(onCancelEdit).toHaveBeenCalledOnce();
  });
  it("names the Verwaltung role as the owner of a held admin:*", async () => {
    const adminWildcard = permission("p10", "admin", "*");
    mockGetPermissions.mockResolvedValue([...allPermissions, adminWildcard]);
    mockGetRolePermissions.mockResolvedValue([adminWildcard]);
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByRole("checkbox", { name: "admin:*" }),
    ).toHaveAccessibleDescription(
      "Nur für die Rolle Verwaltung. Sie können es entfernen, aber nicht wieder vergeben.",
    );
  });
  it("offers no reserved permission and keeps one the role already holds removable", async () => {
    const adminWildcard = permission("p10", "admin", "*");
    const systemManage = permission("p11", "system", "manage");
    mockGetPermissions.mockResolvedValue([
      ...allPermissions,
      adminWildcard,
      systemManage,
    ]);
    mockGetRolePermissions.mockResolvedValue([
      allPermissions[0]!,
      systemManage,
    ]);
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    const held = await screen.findByRole("checkbox", {
      name: "system:manage",
    });
    expect(held).toBeChecked();
    expect(held).toHaveAccessibleDescription(
      "Nur für das moto-Team. Sie können es entfernen, aber nicht wieder vergeben.",
    );
    expect(
      screen.getByRole("checkbox", { name: "students:read" }),
    ).toBeVisible();
    expect(
      screen.queryByRole("checkbox", { name: "admin:*" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("2 von 4 Berechtigungen ausgewählt."),
    ).toBeInTheDocument();
  });

  it("explains what a risky permission allows", async () => {
    const usersManage = permission("p20", "users", "manage");
    mockGetPermissions.mockResolvedValue([...allPermissions, usersManage]);
    render(
      <RolePermissionsTab
        role={role}
        editing
        onSaved={onSaved}
        onCancelEdit={onCancelEdit}
      />,
    );

    expect(
      await screen.findByRole("checkbox", { name: "users:manage" }),
    ).toHaveAccessibleDescription(
      "Diese Person kann sich und anderen jede Rolle der Schule geben.",
    );
    expect(
      screen.getByRole("checkbox", { name: "rooms:read" }),
    ).not.toHaveAccessibleDescription();
  });
});
