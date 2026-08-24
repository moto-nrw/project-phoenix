import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { MFAAdminOverrideModal } from "./mfa-admin-override-modal";

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: toastSuccess,
    error: toastError,
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));

const originalFetch = global.fetch;

function ok(body: unknown, status = 200): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function noContent(): typeof global.fetch {
  return vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
}

const props = {
  isOpen: true,
  onClose: vi.fn(),
  bearerToken: "tok-1",
  accountId: "42",
  accountLabel: "Anna Beispiel",
};

describe("MFAAdminOverrideModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    global.fetch = ok({ override: "none", enrolled: false });
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("renders the reset form with reason field when open", () => {
    render(<MFAAdminOverrideModal {...props} />);
    expect(screen.getByText("Anna Beispiel")).toBeInTheDocument();
    expect(screen.getByLabelText(/Grund/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /2FA endgültig zurücksetzen/ }),
    ).toBeInTheDocument();
  });

  it("blocks submit when reason is shorter than 3 characters", async () => {
    global.fetch = noContent();
    render(<MFAAdminOverrideModal {...props} />);

    fireEvent.change(screen.getByLabelText(/Grund/), {
      target: { value: "ok" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: /2FA endgültig zurücksetzen/ }),
    );

    await waitFor(() => {
      expect(screen.getByText(/mindestens 3 Zeichen/)).toBeInTheDocument();
    });
    // The modal fetches the current MFA admin state on open (GET /mfa) so
    // it can display "currently force_off / force_on / standard" in the
    // header. A validation failure on the reset form must NOT trigger the
    // destructive DELETE — assert that specifically.
    const deleteCalls = (
      global.fetch as ReturnType<typeof vi.fn>
    ).mock.calls.filter(
      ([, init]) => (init as RequestInit | undefined)?.method === "DELETE",
    );
    expect(deleteCalls).toHaveLength(0);
  });

  it("calls DELETE /auth/accounts/{id}/mfa with the reason and shows the confirmation step", async () => {
    global.fetch = noContent();
    render(<MFAAdminOverrideModal {...props} />);

    fireEvent.change(screen.getByLabelText(/Grund/), {
      target: { value: "Mailbox gesperrt" },
    });
    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: /2FA endgültig zurücksetzen/ }),
      );
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/accounts/42/mfa",
        expect.objectContaining({
          method: "DELETE",
          body: JSON.stringify({ reason: "Mailbox gesperrt" }),
        }),
      );
    });
    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalled();
    });
    expect(screen.getByText(/vollständig entfernt/)).toBeInTheDocument();
  });

  it("shows German error from a 403 reject", async () => {
    global.fetch = ok({ error: "Forbidden" }, 403);
    render(<MFAAdminOverrideModal {...props} />);

    fireEvent.change(screen.getByLabelText(/Grund/), {
      target: { value: "Test-Grund" },
    });
    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: /2FA endgültig zurücksetzen/ }),
      );
    });

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
  });
});
