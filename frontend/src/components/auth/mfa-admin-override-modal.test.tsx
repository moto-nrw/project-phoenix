import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { catalogText } from "~/test/error-catalog-text";
import { MFAAdminOverrideModal } from "./mfa-admin-override-modal";

const toastSuccess = vi.fn();
const toastError = vi.fn();
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
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
  // A fresh Response per call: a body can only be read once.
  return vi.fn().mockImplementation(() =>
    Promise.resolve(
      new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    ),
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

  it("exposes a named dialog and locks background scrolling", () => {
    const { unmount } = render(<MFAAdminOverrideModal {...props} />);

    expect(
      screen.getByRole("dialog", {
        name: "Zwei-Faktor-Authentifizierung verwalten",
      }),
    ).toBeInTheDocument();
    expect(document.documentElement.style.overflow).toBe("hidden");

    unmount();
    expect(document.documentElement.style.overflow).toBe("");
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

  it("shows the catalog text of a 403 reject in the dialog", async () => {
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

    expect(
      await screen.findByText(
        catalogText("general.permission", "das Zurücksetzen der 2FA"),
      ),
    ).toBeInTheDocument();
    expect(toastError).not.toHaveBeenCalled();
  });

  // Without the stored state the override choice would show a guessed
  // "Standard" as current (#2517): the load error stands in its place.
  it("shows a failed state load with retry instead of a guessed setting", async () => {
    global.fetch = ok({ error: "boom" }, 500);
    render(<MFAAdminOverrideModal {...props} />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die 2FA-Einstellung"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Aktueller Status/)).not.toBeInTheDocument();
    expect(screen.getByRole("radio", { name: "Standard" })).toBeDisabled();

    global.fetch = ok({ override: "force_on", enrolled: true });
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText(/Aktueller Status/)).toBeInTheDocument();
    expect(
      screen.queryByText(catalogText("general.server", "die 2FA-Einstellung")),
    ).not.toBeInTheDocument();
  });
});
