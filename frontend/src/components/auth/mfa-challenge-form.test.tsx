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

async function enterCode(code: string) {
  const inputs = screen.getAllByRole("textbox").slice(0, 6);
  await act(async () => {
    fireEvent.change(inputs[0]!, { target: { value: code } });
  });
}

// The accessible name is the label followed by the button's description.
const REMEMBER = /^Bestätigen und dieses Gerät 90 Tage merken/;
const ONCE = /^Nur dieses Mal bestätigen/;

/** Types the code and confirms without remembering the device. */
async function submitCodeOnce(code: string) {
  await enterCode(code);
  fireEvent.click(screen.getByRole("button", { name: ONCE }));
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

  // #6955: the person decides with one of two buttons. A complete code
  // alone sends nothing, so no device is remembered without a choice.
  it("waits for a choice once the code is complete", async () => {
    global.fetch = mockOk({ status: "authenticated" });

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    // Blocked with a reason the person can read, so `aria-disabled`.
    const remember = screen.getByRole("button", { name: REMEMBER });
    const once = screen.getByRole("button", { name: ONCE });
    expect(remember).toHaveAttribute("aria-disabled", "true");
    expect(once).toHaveAttribute("aria-disabled", "true");
    fireEvent.click(remember);
    fireEvent.click(once);

    await enterCode("123456");

    expect(remember).not.toHaveAttribute("aria-disabled");
    expect(once).not.toHaveAttribute("aria-disabled");
    expect(remember).toBeEnabled();
    expect(once).toBeEnabled();
    expect(global.fetch).not.toHaveBeenCalled();
  });

  it("confirms without remembering the device on „Nur dieses Mal“", async () => {
    global.fetch = mockOk({ status: "authenticated" });
    const onSuccess = vi.fn();

    render(<MFAChallengeForm {...defaultProps} onSuccess={onSuccess} />);
    await submitCodeOnce("123456");

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
      expect(onSuccess).toHaveBeenCalledWith({ status: "authenticated" });
    });
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  it("remembers the device only on the button that says so", async () => {
    global.fetch = mockOk({ status: "authenticated" });

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);
    await enterCode("987654");
    fireEvent.click(screen.getByRole("button", { name: REMEMBER }));

    await waitFor(() => {
      const call = vi.mocked(global.fetch).mock.calls[0];
      expect(call?.[0]).toBe("/api/auth/mfa/verify");
      const init = call?.[1] as RequestInit;
      expect(init.body).toContain('"remember_device":true');
    });
    expect(global.fetch).toHaveBeenCalledTimes(1);
  });

  // Tooltips do not show on a tablet: each button says on its face for
  // whom it is meant.
  it("says on each button for whom it is meant", () => {
    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);
    expect(screen.getByRole("button", { name: REMEMBER })).toHaveTextContent(
      "Nur auf Ihrem eigenen Gerät.",
    );
    expect(screen.getByRole("button", { name: ONCE })).toHaveTextContent(
      "Für Geräte, die mehrere Personen nutzen.",
    );
  });

  it("names the school's day count on the remember button", () => {
    render(
      <MFAChallengeForm
        {...defaultProps}
        trustedDeviceDays={14}
        onSuccess={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("button", {
        name: /^Bestätigen und dieses Gerät 14 Tage merken/,
      }),
    ).toBeInTheDocument();
  });

  it.each([
    ["tenant", "in Ihrem Profil"],
    ["school", "in den Einstellungen"],
    ["traeger", "unter „Konto“"],
    ["caterer", "unter „Konto“"],
    ["operator", "in Ihrem Profil"],
  ] as const)(
    "tells a %s account where the remembered device can be removed",
    (scope, place) => {
      render(
        <MFAChallengeForm
          {...defaultProps}
          scope={scope}
          onSuccess={vi.fn()}
        />,
      );
      expect(
        screen.getByText(`Gemerkte Geräte entfernen Sie ${place}.`),
      ).toBeInTheDocument();
    },
  );

  // A school that switched remembered devices off has nothing to choose:
  // the code is checked as soon as it is complete, and never remembered.
  it("checks the code right away where devices cannot be remembered", async () => {
    global.fetch = mockOk({ status: "authenticated" });
    const onSuccess = vi.fn();

    render(
      <MFAChallengeForm
        {...defaultProps}
        trustedDeviceEnabled={false}
        onSuccess={onSuccess}
      />,
    );
    expect(
      screen.getByText(/Der Code wird automatisch geprüft/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /merken/ }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: ONCE }),
    ).not.toBeInTheDocument();

    await enterCode("123456");

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalledWith({ status: "authenticated" });
    });
    const init = vi.mocked(global.fetch).mock.calls[0]?.[1] as RequestInit;
    expect(init.body).toContain('"remember_device":false');
  });

  it("empties the code and locks both buttons after a refused code", async () => {
    global.fetch = mockErr(401, "invalid code", "identity.mfa_code_invalid");

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);
    await submitCodeOnce("111111");

    await waitFor(() => {
      expect(screen.getByRole("alert")).toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: ONCE })).toHaveAttribute(
        "aria-disabled",
        "true",
      );
    });
    expect(screen.getByRole("button", { name: REMEMBER })).toHaveAttribute(
      "aria-disabled",
      "true",
    );
    expect(screen.getByLabelText("Stelle 1")).toHaveValue("");
  });

  // A refused code answers 401 with its code (#2517): the catalog text
  // shows in place, no jump to the login screen.
  it("renders the catalog text when verify refuses the code", async () => {
    const assign = vi
      .spyOn(window.location, "assign")
      .mockImplementation(() => undefined);
    global.fetch = mockErr(401, "invalid code", "identity.mfa_code_invalid");

    render(<MFAChallengeForm {...defaultProps} onSuccess={vi.fn()} />);

    await submitCodeOnce("111111");

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

    await submitCodeOnce("111111");

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

    await submitCodeOnce("111111");

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

    await submitCodeOnce("222222");

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
