import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { HolidayParentSetup } from "~/lib/holiday-parent-setup-api";
import { catalogText } from "~/test/error-catalog-text";

const mocks = vi.hoisted(() => ({
  fetchSetup: vi.fn(),
  complete: vi.fn(),
  assign: vi.fn(),
}));

vi.mock("~/lib/holiday-parent-setup-api", () => ({
  fetchHolidayParentSetup: mocks.fetchSetup,
  completeHolidayParentSetup: mocks.complete,
}));
vi.mock("~/components/parent/language-switcher", () => ({
  LanguageSwitcher: () => null,
}));

import { HolidayParentSetupView } from "./holiday-parent-setup";

const LOGIN = "/login?next=%2Fferienanmeldungen";

function newPassword() {
  return ["Fer", "ien", "-12", "3!"].join("");
}

function setup(overrides: Partial<HolidayParentSetup> = {}) {
  mocks.fetchSetup.mockResolvedValue({
    email: "eva@example.org",
    traegerName: "Träger Bissendorf",
    accountExists: false,
    ...overrides,
  });
}

function renderView() {
  return render(
    <ToastProvider>
      <HolidayParentSetupView token="setup-key" />
    </ToastProvider>,
  );
}

function fillPasswords(password: string, confirm = password) {
  fireEvent.change(passwordField(), {
    target: { value: password },
  });
  fireEvent.change(
    screen.getByLabelText("Passwort bestätigen", { selector: "input" }),
    {
      target: { value: confirm },
    },
  );
}

function passwordField() {
  return screen.getByLabelText("Passwort", { selector: "input" });
}

describe("HolidayParentSetupView (#6673)", () => {
  beforeEach(() => {
    mocks.fetchSetup.mockReset();
    mocks.complete.mockReset();
    mocks.assign.mockReset();
    vi.spyOn(window.location, "assign").mockImplementation(mocks.assign);
  });

  it("sets the password of a new account for the fixed address and leads to the login", async () => {
    setup();
    mocks.complete.mockResolvedValue({ accountExists: false });
    renderView();

    expect(
      await screen.findByRole("heading", { name: "Elternkonto anlegen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Ihre Ferienanmeldungen bei Träger Bissendorf/),
    ).toBeInTheDocument();
    expect(screen.getByText("eva@example.org")).toBeInTheDocument();
    expect(screen.getByText("Passwortanforderungen")).toBeInTheDocument();

    fillPasswords(newPassword(), `${newPassword()}x`);
    fireEvent.click(screen.getByRole("button", { name: "Konto anlegen" }));
    expect(
      await screen.findByText("Die Passwörter stimmen nicht überein."),
    ).toBeInTheDocument();
    expect(mocks.complete).not.toHaveBeenCalled();

    fillPasswords(newPassword());
    fireEvent.click(screen.getByRole("button", { name: "Konto anlegen" }));
    await waitFor(() =>
      expect(mocks.complete).toHaveBeenCalledWith("setup-key", {
        password: newPassword(),
        confirmPassword: newPassword(),
      }),
    );
    expect(
      await screen.findByRole("heading", {
        name: "Ihr Elternkonto ist fertig",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Melden Sie sich jetzt mit eva@example.org und Ihrem neuen Passwort an.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Anmelden" })).toHaveAttribute(
      "href",
      LOGIN,
    );
  });

  it("shows a refused password in the form and keeps the fields", async () => {
    setup();
    mocks.complete.mockRejectedValue(
      new ApiError("weak", 400, { code: "identity.password_too_weak" }),
    );
    renderView();
    await screen.findByRole("heading", { name: "Elternkonto anlegen" });

    fillPasswords(newPassword());
    fireEvent.click(screen.getByRole("button", { name: "Konto anlegen" }));

    expect(
      await screen.findByText(
        catalogText("identity.password_too_weak", "Ihr Elternkonto"),
        { exact: false },
      ),
    ).toBeInTheDocument();
    expect(passwordField()).toHaveValue(newPassword());
  });

  it("offers one action to an existing account: link the Träger, then sign in", async () => {
    setup({ accountExists: true });
    mocks.complete.mockResolvedValue({ accountExists: true });
    renderView();

    expect(
      await screen.findByRole("heading", {
        name: "Sie haben schon ein Elternkonto",
      }),
    ).toBeInTheDocument();
    // Anker: the existing-account state with its one action is rendered.
    expect(
      screen.getByRole("button", { name: "Anmelden" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByLabelText("Passwort", { selector: "input" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Anmelden" }));
    await waitFor(() => expect(mocks.assign).toHaveBeenCalledWith(LOGIN));
    expect(mocks.complete).toHaveBeenCalledWith("setup-key");
  });

  it("explains a link that no longer works and what to do", async () => {
    mocks.fetchSetup.mockRejectedValue(
      // Any 404 of the link reads as invalid; the backend names it
      // identity.holiday_parent_setup_invalid.
      new ApiError("gone", 404, {
        code: "identity.invitation_not_found",
      }),
    );
    renderView();

    expect(
      await screen.findByRole("heading", {
        name: "Dieser Link gilt nicht mehr",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/bitten Sie den Träger um einen neuen Link/),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Anmelden" })).toHaveAttribute(
      "href",
      LOGIN,
    );
  });

  it("shows any other load failure with a retry", async () => {
    mocks.fetchSetup.mockRejectedValueOnce(
      new ApiError("boom", 503, { code: "general.unavailable" }),
    );
    renderView();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "Ihr Elternkonto"),
        { exact: false },
      ),
    ).toBeInTheDocument();
    setup();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(
      await screen.findByRole("heading", { name: "Elternkonto anlegen" }),
    ).toBeInTheDocument();
  });
});
