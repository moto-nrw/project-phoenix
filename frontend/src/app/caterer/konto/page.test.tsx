import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { catalogText } from "~/test/error-catalog-text";

const logout = vi.fn(() => Promise.resolve());

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { email: "kueche@example.com" } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/shell-auth-context", () => ({
  useShellAuth: () => ({ logout }),
}));

import CatererAccountPage from "./page";

const originalFetch = global.fetch;

function reply(status: number, body?: unknown): typeof global.fetch {
  return vi.fn().mockImplementation(() =>
    Promise.resolve(
      new Response(body === undefined ? null : JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
}

function openAndSubmit() {
  fireEvent.click(screen.getByRole("button", { name: "Passwort ändern" }));
  fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
    target: { value: "Old-passw0rd!" },
  });
  fireEvent.change(screen.getByLabelText("Neues Passwort"), {
    target: { value: "New-passw0rd!" },
  });
  fireEvent.change(screen.getByLabelText("Neues Passwort bestätigen"), {
    target: { value: "New-passw0rd!" },
  });
  const submit = screen
    .getAllByRole("button", { name: "Passwort ändern" })
    .find((button) => button.getAttribute("type") === "submit");
  if (!submit) throw new Error("submit button missing");
  fireEvent.click(submit);
}

describe("Caterer-Konto (#4496)", () => {
  beforeEach(() => {
    logout.mockClear();
    vi.spyOn(console, "error").mockImplementation(() => undefined);
  });

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("shows the sign-in address and the password change", () => {
    render(<CatererAccountPage />);

    expect(screen.getByText("kueche@example.com")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Nach dem Ändern melden Sie sich auf allen Geräten neu an.",
      ),
    ).toBeInTheDocument();
  });

  it("sends the change to the caterer route and signs out afterwards", async () => {
    global.fetch = reply(204);
    render(<CatererAccountPage />);

    openAndSubmit();

    await waitFor(() =>
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/caterer/account/password",
        expect.objectContaining({ method: "POST" }),
      ),
    );
    expect(
      await screen.findByText("Passwort erfolgreich geändert!"),
    ).toBeInTheDocument();
    await waitFor(() => expect(logout).toHaveBeenCalledTimes(1), {
      timeout: 3000,
    });
  });

  it("keeps a wrong current password in the dialog", async () => {
    global.fetch = reply(401, {
      status: 401,
      detail: "current password is incorrect",
      code: "identity.current_password_wrong",
      errors: [{ field: "current_password", reason: "invalid credentials" }],
    });
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    render(<CatererAccountPage />);

    openAndSubmit();

    expect(
      await screen.findByText(
        catalogText(
          "identity.current_password_wrong",
          "das Ändern des Passworts",
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Aktuelles Passwort")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    expect(assign).not.toHaveBeenCalled();
    expect(logout).not.toHaveBeenCalled();
  });
});
