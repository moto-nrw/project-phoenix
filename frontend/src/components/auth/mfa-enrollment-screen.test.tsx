import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
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
    // Post-#1430: confirm now returns a full session token pair (instead
    // of 204). The screen stashes them and hands them to onComplete so
    // the caller can seed NextAuth.
    const startMock = mockResponse(204, null);
    const confirmMock = mockResponse(200, {
      access_token: "enroll-access",
      refresh_token: "enroll-refresh",
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
        access_token: "enroll-access",
        refresh_token: "enroll-refresh",
      });
    });
  });

  it("renders error when enroll/start fails", async () => {
    global.fetch = mockResponse(429, { error: "rate limited" });
    render(<MFAEnrollmentScreen {...props} onComplete={vi.fn()} />);

    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );

    await waitFor(() => {
      expect(
        screen.getByText("Zu viele Versuche. Bitte warten Sie einen Moment."),
      ).toBeInTheDocument();
    });
  });
});
