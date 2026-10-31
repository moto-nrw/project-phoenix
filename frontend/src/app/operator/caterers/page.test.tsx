/**
 * Tests for the operator caterer list (#4497): the caterers with the state
 * of their two-factor sign-in and the reset of it.
 */
import { fireEvent, render, screen, within } from "@testing-library/react";
import { SWRConfig } from "~/lib/swr";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";

const { mockListCaterers } = vi.hoisted(() => ({
  mockListCaterers: vi.fn(),
}));

// The list's loading and empty states come from SWR itself.
vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({ status: "authenticated", data: { user: { id: "1" } } }),
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("~/lib/operator/provisioning-api", () => ({
  operatorProvisioningService: { listCaterers: mockListCaterers },
}));

import OperatorCaterersPage from "./page";

function renderPage() {
  return render(
    <SWRConfig value={{ provider: () => new Map(), dedupingInterval: 0 }}>
      <ToastProvider>
        <OperatorCaterersPage />
      </ToastProvider>
    </SWRConfig>,
  );
}

describe("OperatorCaterersPage (#4497)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("lists the caterers with their two-factor sign-in", async () => {
    mockListCaterers.mockResolvedValue([
      {
        id: "5",
        accountId: "50",
        name: "Küche am Markt",
        email: "kueche@example.test",
        mfaEnabled: true,
      },
      {
        id: "6",
        accountId: "60",
        name: "Bistro Nord",
        email: "bistro@example.test",
        mfaEnabled: false,
      },
    ]);
    renderPage();

    const row = (await screen.findByText("Küche am Markt")).closest("tr");
    expect(row).not.toBeNull();
    expect(within(row!).getByText("kueche@example.test")).toBeInTheDocument();
    expect(within(row!).getByText("An")).toBeInTheDocument();
    const other = screen.getByText("Bistro Nord").closest("tr");
    expect(within(other!).getByText("Aus")).toBeInTheDocument();
    // Only a switched-on second factor can be reset.
    expect(
      screen.getByRole("button", { name: "Aktionen für kueche@example.test" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", {
        name: "Aktionen für bistro@example.test",
      }),
    ).not.toBeInTheDocument();
  });

  it("opens the reset with the caterer's reason dialog", async () => {
    mockListCaterers.mockResolvedValue([
      {
        id: "5",
        accountId: "50",
        name: "Küche am Markt",
        email: "kueche@example.test",
        mfaEnabled: true,
      },
    ]);
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für kueche@example.test",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Zwei-Faktor zurücksetzen" }),
    );

    expect(await screen.findByLabelText(/Grund/)).toBeInTheDocument();
    expect(screen.getByText(/seine Passkeys bleiben/)).toBeInTheDocument();
  });

  it("says when no caterer exists yet", async () => {
    mockListCaterers.mockResolvedValue([]);
    renderPage();

    expect(await screen.findByText("Noch keine Caterer")).toBeInTheDocument();
  });
});
