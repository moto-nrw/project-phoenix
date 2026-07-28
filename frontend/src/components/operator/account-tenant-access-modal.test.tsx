import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountTenantAccessModal } from "./account-tenant-access-modal";

const {
  mockToastSuccess,
  mockToastError,
  mockList,
  mockGrant,
  mockUpdateRole,
  mockRevoke,
  mockListSchoolSummaries,
  mockListSystemRoles,
  MockApiError,
} = vi.hoisted(() => ({
  mockToastSuccess: vi.fn(),
  mockToastError: vi.fn(),
  mockList: vi.fn(),
  mockGrant: vi.fn(),
  mockUpdateRole: vi.fn(),
  mockRevoke: vi.fn(),
  mockListSchoolSummaries: vi.fn(),
  mockListSystemRoles: vi.fn(),
  MockApiError: class MockAccountTenantAccessApiError extends Error {
    status: number;
    constructor(message: string, status: number) {
      super(message);
      this.name = "AccountTenantAccessApiError";
      this.status = status;
    }
  },
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

vi.mock("~/components/ui/modal", async () => {
  const { createElement } = await import("react");
  return {
    ConfirmationModal: ({
      isOpen,
      title,
      children,
      onConfirm,
      confirmText,
    }: {
      isOpen: boolean;
      title: string;
      children: ReactNode;
      onConfirm: () => void;
      confirmText?: string;
    }) =>
      isOpen
        ? createElement(
            "div",
            { "data-testid": "confirm-modal" },
            createElement("h2", null, title),
            createElement("div", null, children),
            createElement(
              "button",
              { onClick: onConfirm },
              confirmText ?? "Bestätigen",
            ),
          )
        : null,
  };
});

vi.mock("~/components/ui/custom-select", async () => {
  const { createElement } = await import("react");
  return {
    CustomSelect: ({
      value,
      options,
      onChange,
      ariaLabel,
      id,
      placeholder,
    }: {
      value: string;
      options: readonly { value: string; label: string }[];
      onChange: (value: string) => void;
      ariaLabel?: string;
      id?: string;
      placeholder?: string;
    }) =>
      createElement(
        "select",
        {
          "aria-label": ariaLabel ?? id ?? placeholder,
          value,
          id,
          onChange: (event: { target: { value: string } }) =>
            onChange(event.target.value),
        },
        [
          createElement("option", { key: "__empty", value: "" }, "—"),
          ...options.map((option) =>
            createElement(
              "option",
              { key: option.value, value: option.value },
              option.label,
            ),
          ),
        ],
      ),
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

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({ success: mockToastSuccess, error: mockToastError }),
}));

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/operator/account-tenant-access-api", () => ({
  accountTenantAccessService: {
    list: mockList,
    grant: mockGrant,
    updateRole: mockUpdateRole,
    revoke: mockRevoke,
  },
  AccountTenantAccessApiError: MockApiError,
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listSchoolSummaries: mockListSchoolSummaries,
    listSystemRoles: mockListSystemRoles,
  },
}));

function access(overrides: Record<string, unknown> = {}) {
  return {
    tenantId: "2",
    schoolName: "OGS Nord",
    schoolSlug: "ogs-nord",
    schoolActive: true,
    organizationId: "1",
    organizationName: "Träger Köln",
    status: "active",
    activatedAt: null,
    deactivatedAt: null,
    hasPerson: true,
    hasStaff: true,
    roles: [{ id: "1", name: "admin" }],
    ...overrides,
  };
}

function renderModal(props: Record<string, unknown> = {}) {
  return render(
    <AccountTenantAccessModal
      isOpen={true}
      onClose={vi.fn()}
      accountId="42"
      accountLabel="Ada Lovelace"
      accountEmail="ada@example.com"
      {...props}
    />,
  );
}

describe("AccountTenantAccessModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([access()]);
    mockListSchoolSummaries.mockResolvedValue([
      {
        id: "2",
        name: "OGS Nord",
        organizationName: "Träger Köln",
        active: true,
        deletedAt: null,
      },
      {
        id: "3",
        name: "OGS Süd",
        organizationName: "Träger Köln",
        active: true,
        deletedAt: null,
      },
    ]);
    mockListSystemRoles.mockResolvedValue([
      { id: "1", name: "admin", isSystem: true },
      { id: "2", name: "user", isSystem: true },
      { id: "6", name: "guardian", isSystem: true },
    ]);
  });

  it("lists the schools the account can reach", async () => {
    renderModal();

    await waitFor(() => expect(mockList).toHaveBeenCalledWith("42"));
    expect(screen.getByText("OGS Nord")).toBeInTheDocument();
    // Organization + role summary of the granted school.
    expect(screen.getByText(/Träger Köln • Verwaltung/)).toBeInTheDocument();
  });

  it("offers only schools the account is not active at", async () => {
    renderModal();

    const schoolSelect = await screen.findByLabelText("account-access-school");
    const options = Array.from(schoolSelect.querySelectorAll("option")).map(
      (option) => option.textContent,
    );

    expect(options).toContain("OGS Süd (Träger Köln)");
    expect(options).not.toContain("OGS Nord (Träger Köln)");
  });

  it("never offers the guardian role", async () => {
    renderModal();

    const roleSelect = await screen.findByLabelText("account-access-role");
    const options = Array.from(roleSelect.querySelectorAll("option")).map(
      (option) => option.textContent,
    );

    expect(options).toContain("Verwaltung");
    expect(options).toContain("Betreuung");
    expect(options).not.toContain("guardian");
  });

  it("grants access with the selected school and role", async () => {
    mockGrant.mockResolvedValue([access(), access({ tenantId: "3" })]);
    const onUpdated = vi.fn();
    renderModal({ onUpdated });

    const schoolSelect = await screen.findByLabelText("account-access-school");
    fireEvent.change(schoolSelect, { target: { value: "3" } });
    fireEvent.change(screen.getByLabelText("account-access-role"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByText("Zugang erteilen"));

    await waitFor(() =>
      expect(mockGrant).toHaveBeenCalledWith("42", {
        schoolId: "3",
        roleId: "1",
        firstName: undefined,
        lastName: undefined,
      }),
    );
    expect(mockToastSuccess).toHaveBeenCalled();
    await waitFor(() => expect(onUpdated).toHaveBeenCalled());
  });

  it("surfaces the backend reason when a grant is rejected", async () => {
    mockGrant.mockRejectedValue(
      new MockApiError("Diese Rolle existiert an der Zielschule nicht", 400),
    );
    renderModal();

    const schoolSelect = await screen.findByLabelText("account-access-school");
    fireEvent.change(schoolSelect, { target: { value: "3" } });
    fireEvent.change(screen.getByLabelText("account-access-role"), {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByText("Zugang erteilen"));

    await waitFor(() =>
      expect(screen.getByTestId("alert")).toHaveTextContent(
        "Diese Rolle existiert an der Zielschule nicht",
      ),
    );
  });

  it("changes the role of an existing school access", async () => {
    mockUpdateRole.mockResolvedValue([
      access({ roles: [{ id: "2", name: "user" }] }),
    ]);
    renderModal();

    const roleSelect = await screen.findByLabelText("Rolle an OGS Nord");
    fireEvent.change(roleSelect, { target: { value: "2" } });

    await waitFor(() =>
      expect(mockUpdateRole).toHaveBeenCalledWith("42", "2", "2"),
    );
  });

  it("warns that revoking the last school access deactivates the account", async () => {
    renderModal();

    fireEvent.click(await screen.findByText("Entziehen"));

    expect(screen.getByTestId("confirm-modal")).toHaveTextContent(
      "letzte aktive Schulzugang",
    );
  });

  it("revokes access after confirmation", async () => {
    mockRevoke.mockResolvedValue([access({ status: "inactive", roles: [] })]);
    renderModal();

    fireEvent.click(await screen.findByText("Entziehen"));
    fireEvent.click(screen.getByText("Zugang entziehen"));

    await waitFor(() => expect(mockRevoke).toHaveBeenCalledWith("42", "2"));
  });

  it("asks for a name when the account has no person record anywhere", async () => {
    mockList.mockResolvedValue([access({ hasPerson: false, hasStaff: false })]);
    renderModal();

    expect(await screen.findByLabelText(/Vorname/)).toBeInTheDocument();
    expect(screen.getByLabelText(/Nachname/)).toBeInTheDocument();
  });
});
