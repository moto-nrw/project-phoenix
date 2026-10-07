import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { catalogText } from "~/test/error-catalog-text";
import { TenantPasswordChangeModal } from "./tenant-password-change-modal";

const originalFetch = global.fetch;

function reply(status: number, body: unknown): typeof global.fetch {
  return vi.fn().mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
  );
}

function fillAndSubmit() {
  fireEvent.change(screen.getByLabelText("Aktuelles Passwort"), {
    target: { value: "Old-passw0rd!" },
  });
  fireEvent.change(screen.getByLabelText("Neues Passwort"), {
    target: { value: "New-passw0rd!" },
  });
  fireEvent.change(screen.getByLabelText("Neues Passwort bestätigen"), {
    target: { value: "New-passw0rd!" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Passwort ändern" }));
}

describe("TenantPasswordChangeModal (#2517)", () => {
  let assign: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    vi.spyOn(console, "error").mockImplementation(() => undefined);
  });

  afterEach(() => {
    global.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("keeps a wrong current password in the dialog and marks the field", async () => {
    global.fetch = reply(401, {
      status: "error",
      error: "invalid credentials",
      code: "identity.current_password_wrong",
      errors: [{ field: "current_password", reason: "invalid credentials" }],
    });
    render(<TenantPasswordChangeModal isOpen onClose={vi.fn()} />);

    fillAndSubmit();

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
  });

  it("still sends an expired session to the login screen", async () => {
    global.fetch = reply(401, { error: "Nicht authentifiziert" });
    render(<TenantPasswordChangeModal isOpen onClose={vi.fn()} />);

    fillAndSubmit();

    await waitFor(() => expect(assign).toHaveBeenCalled());
  });

  it("marks the new password when the backend finds it too weak", async () => {
    global.fetch = reply(400, {
      error: "password too weak",
      code: "identity.password_too_weak",
      errors: [{ field: "new_password", reason: "too weak" }],
    });
    render(<TenantPasswordChangeModal isOpen onClose={vi.fn()} />);

    fillAndSubmit();

    expect(
      await screen.findByText(
        catalogText("identity.password_too_weak", "das Ändern des Passworts"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Neues Passwort")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
  });

  it("offers a retry after a server error that sends the form again", async () => {
    global.fetch = reply(500, { error: "boom" });
    render(<TenantPasswordChangeModal isOpen onClose={vi.fn()} />);

    fillAndSubmit();

    await screen.findByText(
      catalogText("general.server", "das Ändern des Passworts"),
    );
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => expect(global.fetch).toHaveBeenCalledTimes(2));
  });
});
