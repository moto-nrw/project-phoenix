import {
  fireEvent,
  render as renderPlain,
  screen,
  waitFor,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { AccountTenantAccessModal } from "./account-tenant-access-modal";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const {
  mockList,
  mockGrant,
  mockUpdateRole,
  mockRevoke,
  mockListAssignableRoles,
  mockListSchoolSummaries,
} = vi.hoisted(() => ({
  mockList: vi.fn(),
  mockGrant: vi.fn(),
  mockUpdateRole: vi.fn(),
  mockRevoke: vi.fn(),
  mockListAssignableRoles: vi.fn(),
  mockListSchoolSummaries: vi.fn(),
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
      disabled,
    }: {
      value: string;
      options: readonly { value: string; label: string }[];
      onChange: (value: string) => void;
      ariaLabel?: string;
      id?: string;
      placeholder?: string;
      disabled?: boolean;
    }) =>
      createElement(
        "select",
        {
          "aria-label": ariaLabel ?? id ?? placeholder,
          value,
          id,
          onChange: (event: { target: { value: string } }) =>
            onChange(event.target.value),
          disabled,
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

vi.mock("~/lib/logger", () => ({
  createLogger: () => ({ error: vi.fn(), warn: vi.fn() }),
}));

vi.mock("~/lib/operator/account-tenant-access-api", () => ({
  accountTenantAccessService: {
    list: mockList,
    grant: mockGrant,
    updateRole: mockUpdateRole,
    revoke: mockRevoke,
    listAssignableRoles: mockListAssignableRoles,
  },
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: {
    listSchoolSummaries: mockListSchoolSummaries,
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
    roles: [{ id: "1", name: "admin", isSystem: true, baseRole: null }],
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
    mockListAssignableRoles.mockResolvedValue([
      { id: "1", name: "admin" },
      { id: "2", name: "user" },
      // lehrkraft (#1772) is assignable since the class day view shipped.
      { id: "9", name: "lehrkraft" },
    ]);
  });

  it("lists the schools the account can reach", async () => {
    renderModal();

    await waitFor(() => expect(mockList).toHaveBeenCalledWith("42"));
    expect(screen.getByText("OGS Nord")).toBeInTheDocument();
    // Organization + role summary of the granted school.
    expect(screen.getByText(/Träger Köln • Verwaltung/)).toBeInTheDocument();
  });

  it("ignores an earlier account's load result after the account changes", async () => {
    let resolveFirstLoad!: (value: ReturnType<typeof access>[]) => void;
    const firstLoad = new Promise<ReturnType<typeof access>[]>((resolve) => {
      resolveFirstLoad = resolve;
    });
    mockList
      .mockImplementationOnce(() => firstLoad)
      .mockResolvedValueOnce([
        access({
          tenantId: "3",
          schoolName: "OGS Süd",
          schoolSlug: "ogs-sued",
        }),
      ]);

    const { rerender } = renderModal({ accountId: "1" });
    await waitFor(() => expect(mockList).toHaveBeenCalledWith("1"));

    rerender(
      <AccountTenantAccessModal
        isOpen={true}
        onClose={vi.fn()}
        accountId="2"
        accountLabel="Grace Hopper"
        accountEmail="grace@example.com"
      />,
    );

    expect(await screen.findByText("OGS Süd")).toBeInTheDocument();
    resolveFirstLoad([access({ schoolName: "OGS Nord" })]);

    await waitFor(() => {
      expect(screen.getByText("OGS Süd")).toBeInTheDocument();
      expect(screen.queryByText("OGS Nord")).not.toBeInTheDocument();
    });
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

  it("offers lehrkraft but never the guardian role", async () => {
    renderModal();

    const schoolSelect = await screen.findByLabelText("account-access-school");
    fireEvent.change(schoolSelect, { target: { value: "3" } });
    const roleSelect = screen.getByLabelText("account-access-role");
    await waitFor(() =>
      expect(mockListAssignableRoles).toHaveBeenCalledWith("42", "3"),
    );
    await waitFor(() =>
      expect(roleSelect.querySelector('option[value="1"]')).toBeInTheDocument(),
    );
    const options = Array.from(roleSelect.querySelectorAll("option")).map(
      (option) => option.textContent,
    );

    expect(options).toContain("Verwaltung");
    expect(options).toContain("Betreuung");
    // lehrkraft (#1772) is assignable since the class day view shipped —
    // offered under its German label.
    expect(options).toContain("Lehrkraft");
    expect(options).not.toContain("guardian");
  });

  it("grants access with the selected school and role", async () => {
    mockGrant.mockResolvedValue([access(), access({ tenantId: "3" })]);
    const onUpdated = vi.fn();
    renderModal({ onUpdated });

    const schoolSelect = await screen.findByLabelText("account-access-school");
    fireEvent.change(schoolSelect, { target: { value: "3" } });
    const roleSelect = screen.getByLabelText("account-access-role");
    await waitFor(() =>
      expect(roleSelect.querySelector('option[value="1"]')).toBeInTheDocument(),
    );
    fireEvent.change(roleSelect, {
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
    expect(
      await screen.findByText("Schulzugang wurde erteilt."),
    ).toBeInTheDocument();
    await waitFor(() => expect(onUpdated).toHaveBeenCalled());
  });

  // #2519: the catalog text by code, in the grant section, not the backend sentence.
  it("shows a rejected grant in its section", async () => {
    mockGrant.mockRejectedValue(
      new ApiError("role does not exist at target school", 400, {
        code: "general.input",
      }),
    );
    renderModal();

    const schoolSelect = await screen.findByLabelText("account-access-school");
    fireEvent.change(schoolSelect, { target: { value: "3" } });
    const roleSelect = screen.getByLabelText("account-access-role");
    await waitFor(() =>
      expect(roleSelect.querySelector('option[value="1"]')).toBeInTheDocument(),
    );
    fireEvent.change(roleSelect, {
      target: { value: "1" },
    });
    fireEvent.click(screen.getByText("Zugang erteilen"));

    expect(
      await screen.findByText(
        catalogText("general.input", "die Vergabe des Schulzugangs"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("role does not exist at target school"),
    ).toBeNull();
  });

  it("shows a failed role change as a toast with retry", async () => {
    mockUpdateRole
      .mockRejectedValueOnce(new ApiError("down", 503))
      .mockResolvedValueOnce([access()]);
    renderModal();

    const roleSelect = await screen.findByLabelText("Rolle an OGS Nord");
    fireEvent.change(roleSelect, { target: { value: "2" } });

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Rolle an OGS Nord"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockUpdateRole).toHaveBeenCalledTimes(2));
  });

  it("keeps a failed revoke inside the confirmation", async () => {
    mockRevoke.mockRejectedValue(
      new ApiError("conflict", 409, { code: "general.business_rejection" }),
    );
    renderModal();

    fireEvent.click(await screen.findByText("Entziehen"));
    fireEvent.click(screen.getByText("Zugang entziehen"));

    const confirm = screen.getByTestId("confirm-modal");
    await waitFor(() =>
      expect(confirm.textContent).toContain(
        catalogText(
          "general.business_rejection",
          "das Entziehen des Schulzugangs",
        ),
      ),
    );
  });

  it("shows a failed load with retry", async () => {
    mockList
      .mockRejectedValueOnce(new ApiError("down", 503))
      .mockResolvedValueOnce([access()]);
    renderModal();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Schulzugänge"),
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2));
  });

  it("never offers lehrkraft as target for an existing non-lehrkraft access", async () => {
    renderModal();

    // Ein Wechsel auf Lehrkraft würde das Betreuungsprofil (users.teachers)
    // samt aktiver Gruppen-Aufsichten stranden — gesperrt wie im
    // role-management-modal.
    const roleSelect = await screen.findByLabelText("Rolle an OGS Nord");
    const options = Array.from(roleSelect.querySelectorAll("option")).map(
      (option) => option.textContent,
    );

    expect(options).toContain("Verwaltung");
    expect(options).toContain("Betreuung");
    expect(options).not.toContain("Lehrkraft");
  });

  it("keeps an existing lehrkraft role read-only", async () => {
    mockList.mockResolvedValue([
      access({
        roles: [{ id: "9", name: "lehrkraft", isSystem: true, baseRole: null }],
      }),
    ]);
    renderModal();

    // Der aktuelle Wert muss anzeigbar bleiben; der Wechsel Richtung
    const roleSelect = await screen.findByLabelText("Rolle an OGS Nord");
    const options = Array.from(roleSelect.querySelectorAll("option")).map(
      (option) => option.textContent,
    );

    expect(options).toContain("Lehrkraft");
    expect(options).not.toContain("Betreuung");
    expect(options).not.toContain("Verwaltung");
    expect((roleSelect as HTMLSelectElement).value).toBe("9");
    expect(roleSelect).toBeDisabled();
  });

  it("changes the role of an existing school access", async () => {
    mockUpdateRole.mockResolvedValue([
      access({
        roles: [{ id: "2", name: "user", isSystem: false, baseRole: null }],
      }),
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

  it("disables revocation for system roles managed by their dedicated flow", async () => {
    mockList.mockResolvedValue([
      access({
        roles: [{ id: "2", name: "user", isSystem: true, baseRole: null }],
      }),
    ]);
    renderModal();

    const revokeButton = await screen.findByText("Entziehen");
    expect(revokeButton).toBeDisabled();
    expect(revokeButton).toHaveAttribute(
      "title",
      "Diese Rolle wird über ihren eigenen Verwaltungsablauf entfernt.",
    );
  });

  it("allows revoking a custom role named like a system role", async () => {
    mockList.mockResolvedValue([
      access({
        roles: [{ id: "2", name: "user", isSystem: false, baseRole: null }],
      }),
    ]);
    renderModal();

    expect(await screen.findByText("Entziehen")).toBeEnabled();
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

  it("disables the role selector when school roles cannot be loaded", async () => {
    mockListAssignableRoles.mockImplementation((_accountId, schoolId) =>
      schoolId === "3"
        ? Promise.reject(new ApiError("network down", 503))
        : Promise.resolve([
            { id: "1", name: "admin" },
            { id: "2", name: "user" },
          ]),
    );
    renderModal();

    fireEvent.change(await screen.findByLabelText("account-access-school"), {
      target: { value: "3" },
    });

    const roleSelect = screen.getByLabelText("account-access-role");
    await waitFor(() => expect(roleSelect).toBeDisabled());
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Rollen"),
      ),
    ).toBeInTheDocument();
  });
});
