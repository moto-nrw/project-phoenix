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

const START = { challenge_token: "challenge-abc" };

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
    global.fetch = mockResponse(200, START);
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

  it("confirms enroll/confirm with the button and shows the success screen", async () => {
    const startMock = mockResponse(200, START);
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
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/mfa/enroll/confirm",
        expect.objectContaining({
          body: JSON.stringify({
            challenge_token: "challenge-abc",
            code: "987654",
            remember_device: false,
          }),
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

  it("remembers the device at setup only through the second button (#6955)", async () => {
    global.fetch = vi.fn(async (url) =>
      String(url).endsWith("/start")
        ? Response.json(START)
        : Response.json({ status: "authenticated" }),
    );
    render(
      <MFAEnrollmentScreen
        {...props}
        trustedDeviceEnabled
        trustedDeviceDays={90}
        onComplete={vi.fn()}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );
    fireEvent.change(await screen.findByLabelText("Stelle 1"), {
      target: { value: "123456" },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: "Bestätigen und dieses Gerät 90 Tage merken",
      }),
    );
    await screen.findByRole("heading", { name: "2FA erfolgreich aktiviert" });
    const confirm = vi
      .mocked(global.fetch)
      .mock.calls.find(([url]) => String(url).endsWith("/enroll/confirm"));
    const init = confirm?.[1];
    expect(JSON.parse(String(init?.body))).toMatchObject({
      code: "123456",
      remember_device: true,
    });
  });

  it("keeps a failed final session read visible and allows Continue to retry", async () => {
    global.fetch = vi.fn(async (url) =>
      String(url).endsWith("/start")
        ? Response.json(START)
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
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
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
    const startMock = mockResponse(200, START);
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
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));
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

  it("sends no confirm when enroll/start names no challenge", async () => {
    global.fetch = mockResponse(204, null);
    render(<MFAEnrollmentScreen {...props} onComplete={vi.fn()} />);

    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "das Senden des Codes"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("group", { name: "6-stelliger Bestätigungscode" }),
    ).not.toBeInTheDocument();
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it("keeps a rejected confirm request in the form", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    global.fetch = vi.fn(async (url) =>
      String(url).endsWith("/start")
        ? Response.json(START)
        : Response.json(
            { status: 400, detail: "challenge_token required" },
            { status: 400 },
          ),
    );
    render(<MFAEnrollmentScreen {...props} onComplete={vi.fn()} />);
    fireEvent.click(
      screen.getByRole("button", { name: "Code an meine E-Mail senden" }),
    );
    fireEvent.change(await screen.findByLabelText("Stelle 1"), {
      target: { value: "123456" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Bestätigen" }));

    expect(
      await screen.findByText(
        catalogText("general.input", "die Prüfung des Codes"),
      ),
    ).toBeInTheDocument();
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
