import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { TrustedDevicesSection } from "./trusted-devices-section";
import { catalogText } from "~/test/error-catalog-text";

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

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { token: "test-token" }, expires: "2099-01-01" },
    status: "authenticated",
    update: vi.fn(),
  }),
}));

const originalFetch = global.fetch;

function jsonResponse(body: unknown, status = 200): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function noContentResponse(): typeof global.fetch {
  return vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
}

const sampleDevice = {
  id: 42,
  user_agent:
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
  ip_address: "203.0.113.7",
  created_at: "2026-05-01T10:00:00Z",
  expires_at: "2026-08-01T10:00:00Z",
  last_used_at: "2026-05-14T12:00:00Z",
};

describe("TrustedDevicesSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("renders empty state when no devices exist", async () => {
    global.fetch = jsonResponse([]);
    render(<TrustedDevicesSection />);

    await waitFor(() => {
      expect(
        screen.getByText(/Sie haben aktuell keine vertrauten Geräte/),
      ).toBeInTheDocument();
    });
    expect(global.fetch).toHaveBeenCalledWith(
      "/api/auth/mfa/trusted-devices",
      expect.objectContaining({ method: "GET" }),
    );
  });

  it("renders a device row with shortened User-Agent + revoke button", async () => {
    global.fetch = jsonResponse([sampleDevice]);
    render(<TrustedDevicesSection />);

    await waitFor(() => {
      expect(screen.getByText("Chrome auf macOS")).toBeInTheDocument();
    });
    expect(screen.getByText(/203\.0\.113\.7/)).toBeInTheDocument();
    // Zeilenaktionen liegen im Kebab der Zeile (Bauart 1 Regel 4).
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Chrome auf macOS" }),
    );
    expect(
      screen.getByRole("menuitem", { name: /Entfernen/ }),
    ).toBeInTheDocument();
  });

  it("hits the operator endpoint when scope is operator", async () => {
    // Operator endpoints are wrapped in OperatorEnvelope { status, data }.
    global.fetch = jsonResponse({ status: "success", data: [] });
    render(<TrustedDevicesSection scope="operator" />);

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/operator/auth/mfa/trusted-devices",
        expect.objectContaining({ method: "GET" }),
      );
    });
  });

  it("calls DELETE on revoke + reloads the list", async () => {
    const listMock = jsonResponse([sampleDevice]);
    const deleteMock = noContentResponse();
    const reloadMock = jsonResponse([]);
    let call = 0;
    global.fetch = vi.fn(async (input, init) => {
      call++;
      if (call === 1) return listMock(input as RequestInfo, init);
      if (call === 2) return deleteMock(input as RequestInfo, init);
      return reloadMock(input as RequestInfo, init);
    }) as unknown as typeof global.fetch;

    render(<TrustedDevicesSection />);

    await waitFor(() => {
      expect(screen.getByText("Chrome auf macOS")).toBeInTheDocument();
    });

    // #3109: the row menu entry opens the ConfirmDeleteModal; the device is
    // only revoked after the two-step confirmation inside the dialog.
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Chrome auf macOS" }),
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Entfernen" }));
    expect(global.fetch).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("heading", { name: "Gerät entfernen?" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Ja, entfernen" }));
    await act(async () => {
      fireEvent.click(
        screen.getByRole("button", { name: "Endgültig entfernen" }),
      );
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/mfa/trusted-devices/42",
        expect.objectContaining({ method: "DELETE" }),
      );
    });
    await waitFor(() => {
      expect(toastSuccess).toHaveBeenCalled();
    });
  });

  // #2517: catalog text in place of the list, never the backend sentence
  // and never the empty state.
  it("surfaces a failed load with the catalog text", async () => {
    global.fetch = jsonResponse({ error: "boom" }, 500);
    render(<TrustedDevicesSection />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der vertrauten Geräte"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/boom/)).toBeNull();
    expect(
      screen.queryByText(
        "Sie haben aktuell keine vertrauten Geräte gespeichert.",
      ),
    ).toBeNull();
  });
});
