import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { RoleManagementModal } from "./role-management-modal";

const {
  mockToastSuccess,
  mockGetAccountRoles,
  mockGetRoles,
  mockAssignRoleToAccount,
  mockRemoveRoleFromAccount,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockGetAccountRoles: vi.fn(),
  mockGetRoles: vi.fn(),
  mockAssignRoleToAccount: vi.fn(),
  mockRemoveRoleFromAccount: vi.fn(),
}));

vi.mock("~/components/ui/form-modal", async () => {
  const { createElement } = await import("react");

  return {
    FormModal: ({
      isOpen,
      title,
      footer,
      children,
    }: {
      isOpen: boolean;
      title: string;
      footer: ReactNode;
      children: ReactNode;
    }) =>
      isOpen
        ? createElement(
            "div",
            { "data-testid": "form-modal" },
            createElement("h1", null, title),
            createElement("div", null, children),
            createElement("div", null, footer),
          )
        : null,
  };
});

vi.mock("~/components/ui/alert", async () => {
  const { createElement } = await import("react");

  return {
    Alert: ({ message }: { message?: string }) =>
      message
        ? createElement("div", { "data-testid": "alert" }, message)
        : null,
  };
});

vi.mock("~/components/ui/detail-modal-components", async () => {
  const { createElement } = await import("react");

  return {
    DataField: ({ label, children }: { label: string; children: ReactNode }) =>
      createElement(
        "div",
        null,
        createElement("span", null, label),
        createElement("span", null, children),
      ),
    DataGrid: ({ children }: { children: ReactNode }) =>
      createElement("div", null, children),
    InfoSection: ({
      title,
      children,
    }: {
      title: string;
      children: ReactNode;
    }) =>
      createElement(
        "section",
        null,
        createElement("h2", null, title),
        children,
      ),
    DetailIcons: {
      person: createElement("span", null, "person"),
    },
  };
});

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
  }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({
    error: vi.fn(),
  }),
}));

vi.mock("~/lib/auth-service", () => ({
  authService: {
    getAccountRoles: mockGetAccountRoles,
    getRoles: mockGetRoles,
    assignRoleToAccount: mockAssignRoleToAccount,
    removeRoleFromAccount: mockRemoveRoleFromAccount,
  },
}));

function role(id: string, name: string) {
  return {
    id,
    name,
    description: "",
    isSystem: true,
    createdAt: "",
    updatedAt: "",
  };
}

const ALL_ROLES = [role("1", "admin"), role("2", "user")];

describe("RoleManagementModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGetRoles.mockResolvedValue(ALL_ROLES);
  });

  it("loads the current role and assignable role options on open", async () => {
    mockGetAccountRoles.mockResolvedValue([role("2", "user")]);

    render(
      <RoleManagementModal
        isOpen={true}
        onClose={vi.fn()}
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(mockGetAccountRoles).toHaveBeenCalledWith("42");
    });
    expect(screen.getAllByText("Betreuer").length).toBeGreaterThan(0);

    fireEvent.click(screen.getByLabelText("Neue Rolle"));
    expect(
      screen.getByRole("option", { name: "Administrator" }),
    ).toBeInTheDocument();
  });

  it("assigns the new role before removing the old one on save", async () => {
    mockGetAccountRoles.mockResolvedValue([role("2", "user")]);
    mockAssignRoleToAccount.mockResolvedValue(undefined);
    mockRemoveRoleFromAccount.mockResolvedValue(undefined);

    const onUpdated = vi.fn();
    render(
      <RoleManagementModal
        isOpen={true}
        onClose={vi.fn()}
        accountId="42"
        accountLabel="Ada Lovelace"
        onUpdated={onUpdated}
      />,
    );

    await waitFor(() => {
      expect(mockGetAccountRoles).toHaveBeenCalledWith("42");
    });

    fireEvent.click(screen.getByLabelText("Neue Rolle"));
    fireEvent.click(screen.getByRole("option", { name: "Administrator" }));
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(mockRemoveRoleFromAccount).toHaveBeenCalledWith("42", "2");
    });

    const assignOrder = mockAssignRoleToAccount.mock.invocationCallOrder[0];
    const removeOrder = mockRemoveRoleFromAccount.mock.invocationCallOrder[0];
    expect(assignOrder).toBeDefined();
    expect(removeOrder).toBeDefined();
    expect(assignOrder as number).toBeLessThan(removeOrder as number);
    expect(mockAssignRoleToAccount).toHaveBeenCalledWith("42", "1");
    expect(onUpdated).toHaveBeenCalled();
  });

  it("disables save when the selected role matches the current role", async () => {
    mockGetAccountRoles.mockResolvedValue([role("2", "user")]);

    render(
      <RoleManagementModal
        isOpen={true}
        onClose={vi.fn()}
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(screen.getByText("Speichern")).toBeDisabled();
    });

    expect(mockAssignRoleToAccount).not.toHaveBeenCalled();
  });

  it("shows an error alert without losing state when saving fails", async () => {
    mockGetAccountRoles.mockResolvedValue([role("2", "user")]);
    mockAssignRoleToAccount.mockRejectedValue(new Error("boom"));

    render(
      <RoleManagementModal
        isOpen={true}
        onClose={vi.fn()}
        accountId="42"
        accountLabel="Ada Lovelace"
      />,
    );

    await waitFor(() => {
      expect(mockGetAccountRoles).toHaveBeenCalledWith("42");
    });

    fireEvent.click(screen.getByLabelText("Neue Rolle"));
    fireEvent.click(screen.getByRole("option", { name: "Administrator" }));
    fireEvent.click(screen.getByText("Speichern"));

    await waitFor(() => {
      expect(screen.getByTestId("alert")).toHaveTextContent(
        "Die Rolle konnte nicht aktualisiert werden.",
      );
    });
    expect(mockRemoveRoleFromAccount).not.toHaveBeenCalled();
  });
});
