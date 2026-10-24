import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  getMealCaterer: vi.fn(),
  inviteMealCaterer: vi.fn(),
  revokeMealCatererInvitation: vi.fn(),
  endMealCatererSupply: vi.fn(),
}));

vi.mock("~/lib/meal-caterer-api", () => mocks);

import { ToastProvider } from "~/contexts/ToastContext";
import { MealCaterer } from "./meal-caterer";

const invitation = {
  id: "5",
  email: "kueche@example.com",
  catererName: "Küche am Markt",
  deliveryAddress: "",
  status: "open",
  expiresAt: "2026-09-23T10:00:00Z",
  createdAt: "2026-09-09T10:00:00Z",
};

describe("MealCaterer", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.inviteMealCaterer.mockResolvedValue(invitation);
    mocks.revokeMealCatererInvitation.mockResolvedValue(undefined);
    mocks.endMealCatererSupply.mockResolvedValue(undefined);
  });

  it("invites the caterer by mail and lists the open invitation (ADR 0077)", async () => {
    mocks.getMealCaterer
      .mockResolvedValueOnce({ supply: null, invitations: [] })
      .mockResolvedValue({ supply: null, invitations: [invitation] });
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });

    fireEvent.change(await screen.findByLabelText(/Name des Caterers/), {
      target: { value: "Küche am Markt" },
    });
    fireEvent.change(screen.getByLabelText(/E-Mail-Adresse des Caterers/), {
      target: { value: "kueche@example.com" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Einladung senden" }));

    await waitFor(() =>
      expect(mocks.inviteMealCaterer).toHaveBeenCalledWith({
        email: "kueche@example.com",
        catererName: "Küche am Markt",
        deliveryAddress: "",
      }),
    );
    // Liste fürs Handy und Tabelle stehen beide im DOM (stackedOnMobile).
    expect(await screen.findAllByText("Offene Einladungen")).not.toHaveLength(
      0,
    );
    expect(screen.queryByText(/einladung\//i)).not.toBeInTheDocument();

    fireEvent.click(
      within(screen.getByRole("table")).getByRole("button", {
        name: "Aktionen für die Einladung an kueche@example.com",
      }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Zurücknehmen" }));
    expect(mocks.revokeMealCatererInvitation).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      Array.from(dialog.querySelectorAll("button")).find(
        (button) => button.textContent === "Zurücknehmen",
      )!,
    );
    await waitFor(() =>
      expect(mocks.revokeMealCatererInvitation).toHaveBeenCalledWith("5"),
    );
  });

  it("ends the running supply only after the confirmation", async () => {
    mocks.getMealCaterer.mockResolvedValue({
      supply: {
        catererName: "Küche am Markt",
        deliveryAddress: "Schulstraße 1",
        startedAt: "2026-09-01T08:00:00Z",
      },
      invitations: [],
    });
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });

    expect(await screen.findByText("Küche am Markt")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Einladung senden" }),
    ).not.toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Belieferung beenden" }),
    );
    expect(mocks.endMealCatererSupply).not.toHaveBeenCalled();
    fireEvent.click(await screen.findByRole("button", { name: "Beenden" }));
    await waitFor(() => expect(mocks.endMealCatererSupply).toHaveBeenCalled());
  });

  it("shows the supply to non-admins without ending it", async () => {
    mocks.getMealCaterer.mockResolvedValue({
      supply: {
        catererName: "Küche am Markt",
        deliveryAddress: "Schulstraße 1",
        startedAt: "2026-09-01T08:00:00Z",
      },
      invitations: [],
    });
    render(<MealCaterer canManageSupply={false} />, { wrapper: ToastProvider });

    expect(await screen.findByText("Küche am Markt")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Belieferung beenden" }),
    ).not.toBeInTheDocument();
  });

  it("tells non-admins that only admins invite a caterer", async () => {
    mocks.getMealCaterer.mockResolvedValue({
      supply: null,
      invitations: [invitation],
    });
    render(<MealCaterer canManageSupply={false} />, { wrapper: ToastProvider });

    expect(
      await screen.findByText(/Einen Caterer einladen können nur Admins/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Einladung senden" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("kueche@example.com")).not.toBeInTheDocument();
  });
});
