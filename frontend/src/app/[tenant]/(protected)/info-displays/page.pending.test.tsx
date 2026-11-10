// #6533: Schließen während des Anlegens ist gesperrt; ein Fehler entsperrt
// den Dialog wieder. Echter FormModal.
import {
  act,
  fireEvent,
  render as rtlRender,
  screen,
  within,
} from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { InfoDisplay } from "~/lib/display-helpers";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

const mocks = vi.hoisted(() => ({
  swr: {
    data: undefined as InfoDisplay[] | undefined,
    error: undefined as unknown,
    isLoading: false,
  },
  mutate: vi.fn(() => Promise.resolve()),
  createDisplay: vi.fn(),
  deleteDisplay: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "admin-1", roles: ["admin"], permissions: [] } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/auth-utils", () => ({
  isAdmin: () => true,
  hasPermission: () => true,
}));

vi.mock("~/lib/hooks/use-require-permission", () => ({
  useRequirePermission: () => ({ isLoading: false }),
}));

vi.mock("~/components/tenant/display-mode-guard", () => ({
  DisplayModeGuard: ({ children }: { children: ReactNode }) => <>{children}</>,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({ tenantSlug: "test" }),
  useTenantSlugSafe: () => "test",
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: () => ({ ...mocks.swr, mutate: mocks.mutate }),
}));

vi.mock("~/lib/display-api", () => ({
  listDisplays: vi.fn(),
  createDisplay: mocks.createDisplay,
  updateDisplay: vi.fn(),
  regenerateDisplayToken: vi.fn(),
  deleteDisplay: mocks.deleteDisplay,
}));

import InfoDisplaysPage from "./page";

function render(ui: ReactElement) {
  return rtlRender(ui, { wrapper: ToastProvider });
}

const display: InfoDisplay = {
  id: "5",
  name: "Eingang",
  isActive: true,
  createdAt: "2026-09-01T08:00:00Z",
  updatedAt: "2026-09-01T08:00:00Z",
} as InfoDisplay;

describe("InfoDisplaysPage name dialog pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.swr = { data: [display], error: undefined, isLoading: false };
  });

  it("keeps the dialog shut while creating and unlocks it after a failure", async () => {
    const answer = deferred<never>();
    mocks.createDisplay.mockReturnValue(answer.promise);

    render(<InfoDisplaysPage />);
    fireEvent.click(screen.getByRole("button", { name: "Neues Display" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name des Displays"), {
      target: { value: "Flur" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Erstellen" }));

    expectSaveLocksDialog("Neues Info-Display");

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await within(dialog).findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked("Neues Info-Display");
    expect(within(dialog).getByLabelText("Name des Displays")).toHaveValue(
      "Flur",
    );
  });
});
