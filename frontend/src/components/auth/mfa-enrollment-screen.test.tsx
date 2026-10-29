import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { catalogText } from "~/test/error-catalog-text";
import { MFAEnrollmentScreen } from "./mfa-enrollment-screen";

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => (
    <div role="alert">{message}</div>
  ),
}));

vi.mock("~/components/ui/button", () => ({
  Button: ({
    children,
    onClick,
    type = "button",
    disabled,
  }: {
    children: React.ReactNode;
    onClick?: () => void;
    type?: "button" | "submit" | "reset";
    disabled?: boolean;
  }) => (
    <button type={type} onClick={onClick} disabled={disabled}>
      {children}
    </button>
  ),
}));

vi.mock("~/components/ui/wizard-stepper", () => ({
  WizardStepper: ({ steps, current }: { steps: string[]; current: number }) => (
    <ol data-testid="wizard-stepper">
      {steps.map((label, idx) => (
        <li key={label} aria-current={idx === current ? "step" : undefined}>
          {label}
        </li>
      ))}
    </ol>
  ),
}));

const originalFetch = global.fetch;

function mockResponse(status: number, body: unknown): typeof global.fetch {
  if (status === 204) {
    return vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
  }
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

const props = {
  scope: "tenant" as const,
  bearerToken: "bearer-xyz",
  userEmail: "admin@example.com",
};

describe("MFAEnrollmentScreen", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("starts in intro step and triggers enroll/start when the button is clicked", async () => {
    global.fetch = mockResponse(204, null);
    render(<MFAEnrollmentScreen {...props} onComplete={vi.fn()} />);

    expect(
      screen.getByRole("heading", {
        name: "Zwei-Faktor-Authentifizierung einrichten",
      }),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/mfa/enroll/start",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
        }),
      );
    });
    await waitFor(() => {
      expect(
        screen.getByRole("group", { name: "6-stelliger Bestätigungscode" }),
      ).toBeInTheDocument();
    });
  });

  it("auto-submits enroll/confirm when 6 digits are entered and shows the success screen", async () => {
    // Tenant confirmation installs the session in the BFF and returns status.
    const startMock = mockResponse(204, null);
    const confirmMock = mockResponse(200, {
      status: "authenticated",
    });
    let callCount = 0;
    global.fetch = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        callCount++;
        if (callCount === 1) return startMock(input, init);
        return confirmMock(input, init);
      },
    ) as unknown as typeof global.fetch;

    const onComplete = vi.fn();
    render(<MFAEnrollmentScreen {...props} onComplete={onComplete} />);

    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );

    await waitFor(() => {
      expect(
        screen.getByRole("group", { name: "6-stelliger Bestätigungscode" }),
      ).toBeInTheDocument();
    });

    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "987654" } });
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/mfa/enroll/confirm",
        expect.objectContaining({
          // remember_device is now part of the payload (defaults to false).
          body: JSON.stringify({ code: "987654", remember_device: false }),
        }),
      );
    });

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "2FA erfolgreich aktiviert" }),
      ).toBeInTheDocument();
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zum Dashboard" }),
    );
    await waitFor(() => {
      expect(onComplete).toHaveBeenCalledWith({
        status: "authenticated",
      });
    });
  });

  it("keeps a failed final session read visible and allows Continue to retry", async () => {
    global.fetch = vi.fn(async (url) =>
      String(url).endsWith("/start")
        ? new Response(null, { status: 204 })
        : Response.json({ status: "authenticated" }),
    );
    const onComplete = vi
      .fn()
      .mockRejectedValueOnce(new Error("metadata unavailable"))
      .mockResolvedValueOnce(undefined);
    render(<MFAEnrollmentScreen {...props} onComplete={onComplete} />);
    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );
    fireEvent.change(await screen.findByLabelText("Stelle 1"), {
      target: { value: "123456" },
    });
    await screen.findByRole("heading", { name: "2FA erfolgreich aktiviert" });
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zum Dashboard" }),
    );
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expect(onComplete).toHaveBeenCalledWith({ status: "authenticated" });
    fireEvent.click(
      screen.getByRole("button", { name: "Weiter zum Dashboard" }),
    );
    await waitFor(() => expect(onComplete).toHaveBeenCalledTimes(2));
  });

  it("keeps a refused confirmation code in the form", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    const startMock = mockResponse(204, null);
    const confirmMock = mockResponse(401, {
      status: 401,
      detail: "invalid code",
      code: "identity.mfa_code_invalid",
    });
    let callCount = 0;
    global.fetch = vi.fn(
      async (input: RequestInfo | URL, init?: RequestInit) => {
        callCount++;
        if (callCount === 1) return startMock(input, init);
        return confirmMock(input, init);
      },
    ) as unknown as typeof global.fetch;
    render(<MFAEnrollmentScreen {...props} onComplete={vi.fn()} />);

    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );
    await waitFor(() => {
      expect(
        screen.getByRole("group", { name: "6-stelliger Bestätigungscode" }),
      ).toBeInTheDocument();
    });
    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "111111" } });
    });

    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("identity.mfa_code_invalid", "die Prüfung des Codes"),
        ),
      ).toBeInTheDocument();
    });
    expect(assign).not.toHaveBeenCalled();
    assign.mockRestore();
  });

  // An expired enrollment token answers 401 without a refusal code. The
  // screen no longer invents identity.invalid_credentials (ADR 0006); the
  // person has to log in again.
  it("sends an expired enrollment session to the login screen", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    global.fetch = mockResponse(401, { status: 401, detail: "Unauthorized" });
    render(<MFAEnrollmentScreen {...props} onComplete={vi.fn()} />);

    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );

    await waitFor(() => {
      expect(assign).toHaveBeenCalledWith(
        expect.stringContaining("error=SessionExpired"),
      );
    });
    assign.mockRestore();
  });

  it("renders error when enroll/start fails", async () => {
    global.fetch = mockResponse(429, {
      error: "rate limited",
      code: "identity.mfa_blocked",
    });
    render(<MFAEnrollmentScreen {...props} onComplete={vi.fn()} />);

    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );

    await waitFor(() => {
      expect(
        screen.getByText(
          catalogText("identity.mfa_blocked", "das Senden des Codes"),
        ),
      ).toBeInTheDocument();
    });
  });
});
