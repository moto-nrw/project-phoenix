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
import { ApiError } from "~/lib/api-error";
import { MealCaterer } from "./meal-caterer";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";

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

  // Bauart 2 Regel 4: Nachtragen ist eine Kopf-Aktion mit FormModal, kein
  // Formular im Kartenkörper.
  it("invites the caterer by mail in a dialog and lists the open invitation (ADR 0077)", async () => {
    mocks.getMealCaterer
      .mockResolvedValueOnce({ supply: null, invitations: [] })
      .mockResolvedValue({ supply: null, invitations: [invitation] });
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });

    expect(
      await screen.findByText(/Laden Sie Ihren Caterer per E-Mail ein/),
    ).toBeInTheDocument();
    expect(
      screen.queryByLabelText(/Name des Caterers/),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Caterer einladen" }));
    const form = await screen.findByRole("dialog");
    fireEvent.change(within(form).getByLabelText(/Name des Caterers/), {
      target: { value: "Küche am Markt" },
    });
    fireEvent.change(
      within(form).getByLabelText(/E-Mail-Adresse des Caterers/),
      { target: { value: "kueche@example.com" } },
    );
    fireEvent.click(
      within(form).getByRole("button", { name: "Einladung senden" }),
    );

    await waitFor(() =>
      expect(mocks.inviteMealCaterer).toHaveBeenCalledWith({
        email: "kueche@example.com",
        catererName: "Küche am Markt",
        deliveryAddress: "",
      }),
    );
    expect(
      await screen.findByText(
        "Die Einladung ist unterwegs an kueche@example.com.",
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole("dialog")).not.toBeInTheDocument(),
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
    fireEvent.click(screen.getByRole("menuitem", { name: "Widerrufen" }));
    expect(mocks.revokeMealCatererInvitation).not.toHaveBeenCalled();
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      Array.from(dialog.querySelectorAll("button")).find(
        (button) => button.textContent === "Widerrufen",
      )!,
    );
    await waitFor(() =>
      expect(mocks.revokeMealCatererInvitation).toHaveBeenCalledWith("5"),
    );
    expect(
      await screen.findByText("Die Einladung ist widerrufen."),
    ).toBeInTheDocument();
  });

  it("connects the footer submit button and fields to the invitation form", async () => {
    mocks.getMealCaterer.mockResolvedValue({ supply: null, invitations: [] });
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });
    fireEvent.click(
      await screen.findByRole("button", { name: "Caterer einladen" }),
    );
    const dialog = await screen.findByRole("dialog");
    const name =
      within(dialog).getByLabelText<HTMLInputElement>(/Name des Caterers/);
    const email = within(dialog).getByLabelText<HTMLInputElement>(
      /E-Mail-Adresse des Caterers/,
    );
    const submit = within(dialog).getByRole<HTMLButtonElement>("button", {
      name: "Einladung senden",
    });
    fireEvent.change(name, { target: { value: "Küche am Markt" } });
    fireEvent.change(email, { target: { value: "kueche@example.com" } });
    expect(submit).toHaveAttribute("type", "submit");
    expect(name.form).toBeInstanceOf(HTMLFormElement);
    expect(email.form).toBe(name.form);
    expect(submit.form).toBe(name.form);
    // Happy DOM's keyboard simulation does not find external submit buttons.
    // Exercise the native submission event after checking form ownership.
    fireEvent.submit(name.form!);
    await waitFor(() =>
      expect(mocks.inviteMealCaterer).toHaveBeenCalledExactlyOnceWith({
        email: "kueche@example.com",
        catererName: "Küche am Markt",
        deliveryAddress: "",
      }),
    );
  });

  it("keeps a failed invitation in the open dialog with the field error", async () => {
    mocks.getMealCaterer.mockResolvedValue({ supply: null, invitations: [] });
    mocks.inviteMealCaterer.mockRejectedValueOnce(
      new ApiError("invalid", 400, {
        code: "general.input",
        errors: [{ field: "email", reason: "invalid" }],
      }),
    );
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });

    fireEvent.click(
      await screen.findByRole("button", { name: "Caterer einladen" }),
    );
    const form = await screen.findByRole("dialog");
    fireEvent.change(within(form).getByLabelText(/Name des Caterers/), {
      target: { value: "Küche am Markt" },
    });
    fireEvent.change(
      within(form).getByLabelText(/E-Mail-Adresse des Caterers/),
      { target: { value: "kueche@example.com" } },
    );
    fireEvent.click(
      within(form).getByRole("button", { name: "Einladung senden" }),
    );

    await waitFor(() =>
      expect(
        within(form).getByLabelText(/E-Mail-Adresse des Caterers/),
      ).toHaveAttribute("aria-invalid", "true"),
    );
    expect(screen.getByRole("dialog")).toBe(form);
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
      screen.queryByRole("button", { name: "Caterer einladen" }),
    ).not.toBeInTheDocument();
    // Die seltene, zerstörende Aktion steht im Menü des Kartenkopfs.
    expect(
      screen.queryByRole("button", { name: "Belieferung beenden" }),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Weitere Aktionen" }));
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Belieferung beenden" }),
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
      screen.queryByRole("button", { name: "Weitere Aktionen" }),
    ).not.toBeInTheDocument();
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
      screen.queryByRole("button", { name: "Caterer einladen" }),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("kueche@example.com")).not.toBeInTheDocument();
  });
});

// #6652: a write holds once it is through; a failed reload keeps the shown
// caterer and says the change is saved, and its retry only reloads.
describe("reload after a write (#6652)", () => {
  const reloadText = `${catalogText("general.unavailable", "die Anzeige des Caterers")} ${SAVED_BUT_STALE_SUFFIX}`;
  const supply = {
    catererName: "Küche am Markt",
    deliveryAddress: "Schulstraße 1",
    startedAt: "2026-09-01T08:00:00Z",
  };
  const down = () => new ApiError("down", 503, { code: "general.unavailable" });

  beforeEach(() => {
    vi.clearAllMocks();
    mocks.getMealCaterer.mockReset();
    mocks.inviteMealCaterer.mockReset().mockResolvedValue(invitation);
    mocks.revokeMealCatererInvitation.mockReset().mockResolvedValue(undefined);
    mocks.endMealCatererSupply.mockReset().mockResolvedValue(undefined);
  });

  async function expectReloadOnly(
    write: ReturnType<typeof vi.fn>,
    actionObject: string,
    shown: string | RegExp,
  ) {
    expect(await screen.findByText(reloadText)).toBeInTheDocument();
    // Anker: der bisher gezeigte Stand steht noch da.
    expect(screen.getAllByText(shown)[0]).toBeInTheDocument();
    expect(
      screen.queryByText(catalogText("general.unavailable", actionObject)),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText(catalogText("general.unavailable", "der Caterer")),
    ).not.toBeInTheDocument();
    expect(write).toHaveBeenCalledTimes(1);

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mocks.getMealCaterer).toHaveBeenCalledTimes(3));
    expect(write).toHaveBeenCalledTimes(1);
  }

  it("keeps a sent invitation sent when the reload fails", async () => {
    mocks.getMealCaterer
      .mockResolvedValueOnce({ supply: null, invitations: [] })
      .mockRejectedValueOnce(down())
      .mockResolvedValue({ supply: null, invitations: [invitation] });
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });

    fireEvent.click(
      await screen.findByRole("button", { name: "Caterer einladen" }),
    );
    const form = await screen.findByRole("dialog");
    fireEvent.change(within(form).getByLabelText(/Name des Caterers/), {
      target: { value: "Küche am Markt" },
    });
    fireEvent.change(
      within(form).getByLabelText(/E-Mail-Adresse des Caterers/),
      { target: { value: "kueche@example.com" } },
    );
    fireEvent.click(
      within(form).getByRole("button", { name: "Einladung senden" }),
    );

    await expectReloadOnly(
      mocks.inviteMealCaterer,
      "die Einladung",
      /Laden Sie Ihren Caterer per E-Mail ein/,
    );
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(await screen.findAllByText("Offene Einladungen")).not.toHaveLength(
      0,
    );
  });

  it("keeps a revoked invitation revoked when the reload fails", async () => {
    mocks.getMealCaterer
      .mockResolvedValueOnce({ supply: null, invitations: [invitation] })
      .mockRejectedValueOnce(down())
      .mockResolvedValue({ supply: null, invitations: [] });
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });

    fireEvent.click(
      within(await screen.findByRole("table")).getByRole("button", {
        name: "Aktionen für die Einladung an kueche@example.com",
      }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Widerrufen" }));
    const dialog = await screen.findByRole("dialog");
    fireEvent.click(
      Array.from(dialog.querySelectorAll("button")).find(
        (button) => button.textContent === "Widerrufen",
      )!,
    );

    await expectReloadOnly(
      mocks.revokeMealCatererInvitation,
      "die Einladung",
      "Küche am Markt",
    );
    await waitFor(() =>
      expect(screen.queryAllByText("Offene Einladungen")).toHaveLength(0),
    );
  });

  it("keeps an ended supply ended when the reload fails", async () => {
    mocks.getMealCaterer
      .mockResolvedValueOnce({ supply, invitations: [] })
      .mockRejectedValueOnce(down())
      .mockResolvedValue({ supply: null, invitations: [] });
    render(<MealCaterer canManageSupply />, { wrapper: ToastProvider });

    fireEvent.click(
      await screen.findByRole("button", { name: "Weitere Aktionen" }),
    );
    fireEvent.click(
      screen.getByRole("menuitem", { name: "Belieferung beenden" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Beenden" }));

    await expectReloadOnly(
      mocks.endMealCatererSupply,
      "das Ende der Belieferung",
      "Küche am Markt",
    );
    expect(
      await screen.findByText(/Laden Sie Ihren Caterer per E-Mail ein/),
    ).toBeInTheDocument();
  });
});
