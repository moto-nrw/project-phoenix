import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactElement, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { InfoDisplay } from "~/lib/display-helpers";
import { catalogText } from "~/test/error-catalog-text";

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
    data: { user: { roles: ["admin"], permissions: [] } },
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

describe("InfoDisplaysPage error path (#2517)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.swr = { data: [display], error: undefined, isLoading: false };
  });

  it("shows a failed list load with the catalog text and retry", async () => {
    mocks.swr = {
      data: undefined,
      error: new ApiError("down", 503, { code: "general.unavailable" }),
      isLoading: false,
    };

    render(<InfoDisplaysPage />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Info-Displays"),
      ),
    ).toBeInTheDocument();
    // #2517: keine Zählung aus einer Liste, die nie geladen wurde.
    expect(screen.queryByText(/0 Displays/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(mocks.mutate).toHaveBeenCalled();
  });

  it("keeps a refused create in the dialog and marks the name", async () => {
    mocks.createDisplay.mockRejectedValueOnce(
      new ApiError("invalid display input", 400, {
        code: "general.input",
        errors: [{ field: "name", reason: "invalid" }],
      }),
    );

    render(<InfoDisplaysPage />);
    fireEvent.click(screen.getByRole("button", { name: "Neues Display" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.change(within(dialog).getByLabelText("Name des Displays"), {
      target: { value: "Flur" },
    });
    fireEvent.click(within(dialog).getByRole("button", { name: "Erstellen" }));

    expect(
      await within(dialog).findByText(
        catalogText("general.input", "das Display"),
      ),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(
        within(dialog).getByLabelText("Name des Displays"),
      ).toHaveAttribute("aria-invalid", "true");
    });
  });
});
