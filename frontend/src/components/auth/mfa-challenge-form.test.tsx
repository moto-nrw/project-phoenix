import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  render,
  screen,
  fireEvent,
  waitFor,
  act,
} from "@testing-library/react";
import { catalogText } from "~/test/error-catalog-text";
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
  detail: string,
  code?: string,
): typeof global.fetch {
  return vi.fn().mockResolvedValue(
    new Response(
      JSON.stringify({ status, detail, ...(code ? { code } : {}) }),
      {
        status,
        headers: { "Content-Type": "application/json" },
      },
    ),
  );
}

/** Types the code and presses the button the person chose (#6955). */
async function enterCode(code: string, button = "Bestätigen") {
  const inputs = screen.getAllByRole("textbox").slice(0, 6);
  await act(async () => {
    fireEvent.change(inputs[0]!, { target: { value: code } });
  });
  await act(async () => {
    fireEvent.click(screen.getByRole("button", { name: button }));
  });
}

function verifyBody(): Record<string, unknown> {
  const call = vi.mocked(global.fetch).mock.calls[0];
  expect(call?.[0]).toBe("/api/auth/mfa/verify");
  const init = call?.[1];
  return JSON.parse(String(init?.body)) as Record<string, unknown>;
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

  it("confirms only with a button and never remembers without the choice", async () => {
    global.fetch = mockOk({
      status: "authenticated",
    });
    const onSuccess = vi.fn();

    render(
      <MFAChallengeForm
        {...defaultProps}
        trustedDeviceEnabled
        trustedDeviceDays={90}
        onSuccess={onSuccess}
      />,
    );

    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "123456" } });
    });
    expect(global.fetch).not.toHaveBeenCalled();

    await enterCode("123456", "Nur dieses Mal bestätigen");

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
        status: "authenticated",
      });
    });
  });

  it("offers two buttons, preselects neither and explains the choice", () => {
    render(
      <MFAChallengeForm
        {...defaultProps}
        trustedDeviceEnabled
        trustedDeviceDays={90}
        onSuccess={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Nur dieses Mal bestätigen" }),
    ).toHaveAttribute("type", "submit");
    const remember = screen.getByRole("button", {
      name: "Bestätigen und dieses Gerät 90 Tage merken",
    });
    expect(remember).toHaveAttribute("type", "button");
    expect(remember).toHaveAccessibleDescription(
      /Nur auf Ihrem eigenen Gerät merken, nicht auf einem geteilten Tablet\. Unter „Profil“/,
    );
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  it("remembers the device only through the second button", async () => {
    global.fetch = mockOk({ status: "authenticated" });

    render(
      <MFAChallengeForm
        {...defaultProps}
        trustedDeviceEnabled
        trustedDeviceDays={30}
        onSuccess={vi.fn()}
      />,
    );

    await enterCode("987654", "Bestätigen und dieses Gerät 30 Tage merken");

    await waitFor(() => {
      expect(verifyBody()).toMatchObject({
        code: "987654",
        remember_device: true,
      });
    });
  });

  it("confirms only this time on Enter in the code field", async () => {
    global.fetch = mockOk({ status: "authenticated" });

    render(
      <MFAChallengeForm
        {...defaultProps}
        trustedDeviceEnabled
        trustedDeviceDays={90}
        onSuccess={vi.fn()}
      />,
    );

    const inputs = screen.getAllByRole("textbox").slice(0, 6);
    await act(async () => {
      fireEvent.change(inputs[0]!, { target: { value: "135790" } });
    });
    await act(async () => {
      fireEvent.submit(inputs[5]!.closest("form")!);
    });

    await waitFor(() => {
      expect(verifyBody()).toMatchObject({
        code: "135790",
        remember_device: false,
      });
    });
  });

  it("offers only Bestätigen where the device may not be remembered", async () => {
    global.fetch = mockOk({ status: "authenticated" });

    render(
      <MFAChallengeForm
        {...defaultProps}
        trustedDeviceEnabled={false}
        onSuccess={vi.fn()}
      />,
    );

    expect(
      screen.queryByRole("button", { name: /merken/ }),
    ).not.toBeInTheDocument();
    await enterCode("246802");
    await waitFor(() => {
      expect(verifyBody()).toMatchObject({ remember_device: false });
    });
  });

  it("asks for the missing digits instead of sending an incomplete code", async () => {
    global.fetch = mockOk({ status: "authenticated" });

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    await enterCode("123");

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Bitte geben Sie alle 6 Stellen des Codes ein.",
    );
    expect(global.fetch).not.toHaveBeenCalled();
  });

  // A refused code answers 401 with its code (#2517): the catalog text
  // shows in place, no jump to the login screen.
  it("renders the catalog text when verify refuses the code", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    global.fetch = mockErr(401, "invalid code", "identity.mfa_code_invalid");

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    await enterCode("111111");

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        catalogText("identity.mfa_code_invalid", "die Prüfung des Codes"),
      );
    });
    expect(assign).not.toHaveBeenCalled();
    assign.mockRestore();
  });

  // The token exchange after a correct code refuses an inactive account or a
  // revoked school with its own code: that also stays in the form.
  it("keeps a refused credential of the token exchange in the form", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    global.fetch = mockErr(
      401,
      "account inactive",
      "identity.session_account_inactive",
    );

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    await enterCode("111111");

    await waitFor(() => {
      expect(screen.getByRole("alert")).toHaveTextContent(
        catalogText(
          "identity.session_account_inactive",
          "die Prüfung des Codes",
        ),
      );
    });
    expect(assign).not.toHaveBeenCalled();
    assign.mockRestore();
  });

  // A 401 without a refusal code means the challenge itself ran out. The
  // form no longer invents identity.invalid_credentials for it (ADR 0006):
  // it is an expired login and leads to the login screen.
  it("sends a 401 without a refusal code to the login screen", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    global.fetch = mockErr(401, "Unauthorized");

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    await enterCode("111111");

    await waitFor(() => {
      expect(assign).toHaveBeenCalledWith(
        expect.stringContaining("error=SessionExpired"),
      );
    });
    expect(
      screen.queryByText(
        catalogText("identity.invalid_credentials", "die Prüfung des Codes"),
      ),
    ).not.toBeInTheDocument();
    assign.mockRestore();
  });

  it("delegates a coded portal handoff error to the embedding login flow", async () => {
    global.fetch = mockErr(
      403,
      "school portal required",
      "identity.use_school_portal",
    );
    const onError = vi.fn().mockResolvedValue(true);

    render(
      <MFAChallengeForm
        {...defaultProps}
        onSuccess={vi.fn()}
        onError={onError}
      />,
    );

    await enterCode("222222");

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith(
        expect.objectContaining({
          code: "identity.use_school_portal",
          status: 403,
        }),
      );
    });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("disables resend until cooldown elapses", () => {
    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);
    const resend = screen.getByText(/Neuen Code anfordern \(60s\)/);
    expect(resend.closest("button")).toBeDisabled();
  });
});
