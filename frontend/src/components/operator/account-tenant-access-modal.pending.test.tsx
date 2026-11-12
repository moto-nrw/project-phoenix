// #6533: Schließen während einer Änderung ist gesperrt, eine späte Antwort
// verändert nicht die Zugänge eines anderen Kontos. Echter FormModal.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const mocks = vi.hoisted(() => ({
  list: vi.fn(),
  grant: vi.fn(),
  roles: vi.fn(),
  schools: vi.fn(),
}));

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

vi.mock("~/lib/operator/account-tenant-access-api", () => ({
  accountTenantAccessService: {
    list: mocks.list,
    grant: mocks.grant,
    updateRole: vi.fn(),
    revoke: vi.fn(),
    listAssignableRoles: mocks.roles,
  },
}));
vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: { listSchoolSummaries: mocks.schools },
}));

import { AccountTenantAccessModal } from "./account-tenant-access-modal";

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

function modal(accountId: string, onClose = vi.fn()) {
  return (
    <AccountTenantAccessModal
      isOpen
      onClose={onClose}
      accountId={accountId}
      accountLabel={accountId === "42" ? "Ada Lovelace" : "Grace Hopper"}
      accountEmail="person@example.com"
    />
  );
}

async function renderAndGrant() {
  const answer = deferred<ReturnType<typeof access>[]>();
  mocks.grant.mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const view = render(modal("42", onClose), { wrapper: ToastProvider });
  const schoolSelect = await screen.findByLabelText("account-access-school");
  fireEvent.change(schoolSelect, { target: { value: "3" } });
  const roleSelect = screen.getByLabelText("account-access-role");
  await waitFor(() =>
    expect(roleSelect.querySelector('option[value="1"]')).toBeInTheDocument(),
  );
  fireEvent.change(roleSelect, { target: { value: "1" } });
  fireEvent.click(screen.getByRole("button", { name: "Zugang erteilen" }));
  return { ...view, answer, onClose };
}

describe("AccountTenantAccessModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.list.mockImplementation(async (accountId: string) =>
      accountId === "42"
        ? [access()]
        : [access({ tenantId: "5", schoolName: "OGS Süd" })],
    );
    mocks.schools.mockResolvedValue([
      {
        id: "2",
        name: "OGS Nord",
        organizationName: "Träger Köln",
        active: true,
        deletedAt: null,
      },
      {
        id: "3",
        name: "OGS West",
        organizationName: "Träger Köln",
        active: true,
        deletedAt: null,
      },
    ]);
    mocks.roles.mockResolvedValue([
      { id: "1", name: "admin", isSystem: true, schoolPortal: false },
    ]);
  });

  it("keeps the dialog shut while granting and unlocks it after a failure", async () => {
    const { answer, onClose } = await renderAndGrant();

    expectSaveLocksDialog("Schulzugänge: Ada Lovelace", ["Schließen"]);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Schulzugänge: Ada Lovelace");
    expect(onClose).not.toHaveBeenCalled();
  });

  it("leaves the schools of another account alone with a late answer", async () => {
    const { answer, rerender } = await renderAndGrant();
    expect(mocks.grant).toHaveBeenCalledOnce();

    rerender(modal("7"));
    // Anker: der Dialog zeigt die Zugänge des anderen Kontos.
    expect(await screen.findByText("OGS Süd")).toBeInTheDocument();

    await act(async () => {
      answer.resolve([
        access(),
        access({ tenantId: "3", schoolName: "OGS West" }),
      ]);
    });

    expect(screen.getByText("OGS Süd")).toBeInTheDocument();
    expect(screen.queryByText("OGS West")).not.toBeInTheDocument();
  });
});
