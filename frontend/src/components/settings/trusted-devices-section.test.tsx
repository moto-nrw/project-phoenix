import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  waitFor,
  fireEvent,
  act,
} from "@testing-library/react";
import { TrustedDevicesSection } from "./trusted-devices-section";

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
    expect(
      screen.getByRole("button", { name: /Entfernen/ }),
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

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: /Entfernen/ }));
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

  it("surfaces error message when list fails", async () => {
    global.fetch = jsonResponse({ error: "boom" }, 500);
    render(<TrustedDevicesSection />);

    // Component renders an Alert + falls back to the empty-state copy
    // (devices=null after the catch). We just need to see the alert
    // surface; the exact text comes from `postJson` ("boom" via
    // extractErrorMessage) but we match loosely so a future copy tweak
    // doesn't break the test.
    await waitFor(() => {
      expect(screen.getByText(/boom/)).toBeInTheDocument();
    });
  });
});
