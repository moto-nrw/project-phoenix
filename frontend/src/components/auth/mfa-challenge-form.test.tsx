import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { MFAChallengeForm } from "./mfa-challenge-form";

vi.mock("~/components/ui/alert", () => ({
  Alert: ({ message }: { message: string }) => (
    <div role="alert">{message}</div>
  ),
}));

const originalFetch = global.fetch;

function mockOk(body: unknown): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify(body), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

function mockErr(
  status: number,
  error: string,
  code?: string,
): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(JSON.stringify({ error, ...(code ? { code } : {}) }), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );
}

describe("MFAChallengeForm", () => {
  const defaultProps = {
    scope: "tenant" as const,
    challengeToken: "challenge-token-abc",
    maskedEmail: "j***@example.com",
    resendCooldownSeconds: 60,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    global.fetch = originalFetch;
  });

  it("renders 6 digit inputs and the masked email", () => {
    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);
    expect(screen.getByText("j***@example.com")).toBeInTheDocument();
    expect(screen.getAllByRole("textbox").length).toBeGreaterThanOrEqual(6);
  });

  it("auto-submits when all 6 digits are entered", async () => {
    global.fetch = mockOk({
      access_token: "access-tok",
      refresh_token: "refresh-tok",
    });
    const onSuccess = vi.fn();

    render(<MFAChallengeForm {...defaultProps} onSuccess={onSuccess} />);

    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "123456" } });
    });

    await waitFor(() => {
      expect(global.fetch).toHaveBeenCalledWith(
        "/api/auth/mfa/verify",
        expect.objectContaining({
          method: "POST",
          credentials: "include",
          body: JSON.stringify({
            challenge_token: "challenge-token-abc",
            code: "123456",
            remember_device: false,
          }),
        }),
      );
    });
    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalledWith({
        access_token: "access-tok",
        refresh_token: "refresh-tok",
      });
    });
  });

  it("forwards remember_device when the checkbox is ticked", async () => {
    global.fetch = mockOk({
      access_token: "tok",
      refresh_token: "rtok",
    });

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    const checkbox = screen.getByLabelText("Auf diesem Gerät 90 Tage merken");
    fireEvent.click(checkbox);

    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "987654" } });
    });

    await waitFor(() => {
      const call = vi.mocked(global.fetch).mock.calls[0];
      expect(call?.[0]).toBe("/api/auth/mfa/verify");
      const init = call?.[1] as RequestInit;
      expect(init.body).toContain('"remember_device":true');
    });
  });

  it("renders German error when verify returns 401 (invalid)", async () => {
    global.fetch = mockErr(401, "invalid code");

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "111111" } });
    });

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        "Der eingegebene Code ist ungültig. Bitte erneut versuchen.",
      );
    });
  });

  it("delegates a coded portal handoff error to the embedding login flow", async () => {
    global.fetch = mockErr(403, "school portal required", "use_school_portal");
    const onError = vi.fn().mockResolvedValue(true);

    render(
      <MFAChallengeForm
        {...defaultProps}
        onSuccess={vi.fn()}
        onError={onError}
      />,
    );

    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "222222" } });
    });

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith(
        expect.objectContaining({ code: "use_school_portal", status: 403 }),
      );
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("does not render any recovery-code affordance", () => {
    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);
    expect(
      screen.queryByText(/Wiederherstellungscode/i),
    ).not.toBeInTheDocument();
  });

  it("disables resend until cooldown elapses", () => {
    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);
    const resend = screen.getByText(/Neuen Code anfordern \(60s\)/);
    expect(resend.closest("button")).toBeDisabled();
  });
});
