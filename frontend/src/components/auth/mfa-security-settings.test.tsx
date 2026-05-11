import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { render, screen, fireEvent, waitFor } from "@testing-library/react";
import { MFASecuritySettings } from "./mfa-security-settings";

vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    success: vi.fn(),
    error: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));

vi.mock("./mfa-enrollment-screen", () => ({
  MFAEnrollmentScreen: () => <div data-testid="enrollment-screen" />,
}));

const originalFetch = global.fetch;

function ok(body: unknown): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function chain(statusBody: unknown, devicesBody: unknown): typeof global.fetch {
  return vi.fn(async (url: RequestInfo | URL) => {
    const u = typeof url === "string" ? url : url.toString();
    if (u.endsWith("/status")) {
      return new Response(JSON.stringify(statusBody), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    if (u.endsWith("/trusted-devices")) {
      return new Response(JSON.stringify(devicesBody), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      });
    }
    return new Response(null, { status: 204 });
  }) as unknown as typeof global.fetch;
}

const props = {
  scope: "tenant" as const,
  bearerToken: "tok-1",
  userEmail: "user@example.com",
};

describe("MFASecuritySettings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("renders inactive state and exposes the activate button", async () => {
    global.fetch = chain(
      {
        enrolled: false,
        unused_recovery_codes: 0,
        mode_required: false,
      },
      [],
    );
    render(<MFASecuritySettings {...props} />);

    await waitFor(() => {
      expect(screen.getByText("Inaktiv")).toBeInTheDocument();
    });
    expect(
      screen.getByRole("button", { name: "2FA aktivieren" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Vertrauenswürdige Geräte")).toBeNull();
  });

  it("renders active state with last-used + recovery counts", async () => {
    global.fetch = chain(
      {
        enrolled: true,
        last_used_at: "2026-05-10T12:00:00Z",
        unused_recovery_codes: 7,
        mode_required: false,
      },
      [
        {
          id: 42,
          user_agent: "Mozilla/5.0",
          ip_address: "1.2.3.4",
          expires_at: "2026-06-10T12:00:00Z",
        },
      ],
    );
    render(<MFASecuritySettings {...props} />);

    await waitFor(() => {
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });
    expect(screen.getByText("7 ungenutzt")).toBeInTheDocument();
    expect(screen.getByText("Vertrauenswürdige Geräte")).toBeInTheDocument();
    expect(screen.getByText("Mozilla/5.0")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Neue Wiederherstellungscodes" }),
    ).toBeInTheDocument();
  });

  it("disables 'Deaktivieren' when mode_required is true", async () => {
    global.fetch = chain(
      {
        enrolled: true,
        unused_recovery_codes: 5,
        mode_required: true,
      },
      [],
    );
    render(<MFASecuritySettings {...props} />);

    await waitFor(() => {
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });
    expect(
      screen.getByRole("button", { name: "2FA deaktivieren" }),
    ).toBeDisabled();
    expect(
      screen.getByText(
        "2FA ist für Ihr Konto verpflichtend und kann nicht deaktiviert werden.",
      ),
    ).toBeInTheDocument();
  });

  it("opens disable confirm modal when allowed", async () => {
    global.fetch = chain(
      {
        enrolled: true,
        unused_recovery_codes: 5,
        mode_required: false,
      },
      [],
    );
    render(<MFASecuritySettings {...props} />);

    await waitFor(() => {
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "2FA deaktivieren" }));
    expect(
      screen.getByRole("heading", {
        name: "Zwei-Faktor-Authentifizierung deaktivieren?",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Endgültig deaktivieren" }),
    ).toBeInTheDocument();
  });

  it("opens regenerate confirm modal", async () => {
    global.fetch = chain(
      {
        enrolled: true,
        unused_recovery_codes: 3,
        mode_required: false,
      },
      [],
    );
    render(<MFASecuritySettings {...props} />);

    await waitFor(() => {
      expect(screen.getByText("Aktiv")).toBeInTheDocument();
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Neue Wiederherstellungscodes" }),
    );
    expect(
      screen.getByRole("heading", {
        name: "Neue Wiederherstellungscodes erzeugen?",
      }),
    ).toBeInTheDocument();
  });

  it("renders 'no trusted devices' empty state", async () => {
    global.fetch = chain(
      {
        enrolled: true,
        unused_recovery_codes: 8,
        mode_required: false,
      },
      [],
    );
    render(<MFASecuritySettings {...props} />);
    await waitFor(() => {
      expect(
        screen.getByText("Keine vertrauenswürdigen Geräte hinterlegt."),
      ).toBeInTheDocument();
    });
  });
});

// Suppress unused-import warning
void ok;
