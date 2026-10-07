import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { TenantPasswordResetModal } from "./tenant-password-reset-modal";

const { requestPasswordReset } = vi.hoisted(() => ({
  requestPasswordReset: vi.fn(),
}));
vi.mock("~/lib/auth-api", () => ({ requestPasswordReset }));

const STORAGE_KEY = "passwordResetRateLimitUntil";

function submitAddress() {
  fireEvent.change(screen.getByRole("textbox"), {
    target: { value: "staff@example.com" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Link senden" }));
}

describe("TenantPasswordResetModal (#2517)", () => {
  beforeEach(() => {
    localStorage.removeItem(STORAGE_KEY);
    vi.spyOn(console, "error").mockImplementation(() => undefined);
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    requestPasswordReset.mockReset();
    localStorage.removeItem(STORAGE_KEY);
  });

  it("reads the wait from the code and shows the catalog text with a countdown", async () => {
    const limited = new ApiError("rate limited", 429, {
      code: "identity.password_reset_rate_limited",
    });
    limited.retryAfterSeconds = 90;
    requestPasswordReset.mockRejectedValue(limited);
    render(<TenantPasswordResetModal isOpen onClose={vi.fn()} />);

    submitAddress();

    expect(
      await screen.findByText(
        catalogText(
          "identity.password_reset_rate_limited",
          "das Senden des Links",
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent(/01:(30|29)/);
    expect(screen.getByRole("button", { name: "Link senden" })).toBeDisabled();
    expect(Number(localStorage.getItem(STORAGE_KEY))).toBeGreaterThan(
      Date.now(),
    );
  });

  it("shows a server error with retry that sends the address again", async () => {
    requestPasswordReset.mockRejectedValueOnce(new ApiError("boom", 500));
    requestPasswordReset.mockResolvedValueOnce({ message: "ok" });
    render(<TenantPasswordResetModal isOpen onClose={vi.fn()} />);

    submitAddress();

    await screen.findByText(
      catalogText("general.server", "das Senden des Links"),
    );
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() =>
      expect(requestPasswordReset).toHaveBeenNthCalledWith(
        2,
        "staff@example.com",
      ),
    );
    expect(await screen.findByText("E-Mail versendet!")).toBeInTheDocument();
  });
});
