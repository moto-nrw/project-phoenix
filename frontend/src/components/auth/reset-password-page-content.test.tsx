import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import {
  ResetPasswordPageContent,
  type ResetPasswordPageCopy,
} from "./reset-password-page-content";

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  token: null as string | null,
  tenant: null as unknown,
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => ({
    get: (key: string) => (key === "token" ? mocks.token : null),
  }),
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => mocks.tenant,
}));

vi.mock("~/lib/tenant-api", () => ({
  loginImageSrc: (path: string) => `/resolved${path}`,
}));

// next/image is not globally mocked — render a plain img so the brand branch
// renders without Next's image optimization machinery.
vi.mock("next/image", () => ({
  default: (props: Record<string, unknown>) => {
    const { src, alt } = props as { src: string; alt: string };
    // eslint-disable-next-line @next/next/no-img-element
    return <img src={src} alt={alt} />;
  },
}));

// ---------------------------------------------------------------------------
// Sentinel copy — distinct strings so assertions can't accidentally match
// unrelated UI text and each copy-driven branch is unambiguous.
// ---------------------------------------------------------------------------

const COPY: ResetPasswordPageCopy = {
  missingToken: "MISSING_TOKEN",
  invalidToken: "INVALID_TOKEN",
  passwordTooShort: "TOO_SHORT",
  passwordMissingUppercase: "NO_UPPERCASE",
  passwordMissingLowercase: "NO_LOWERCASE",
  passwordMissingNumber: "NO_NUMBER",
  passwordMissingSpecial: "NO_SPECIAL",
  passwordMismatch: "MISMATCH",
  genericError: "GENERIC_ERROR",
  invalidRequest: "INVALID_REQUEST",
  expiredLink: "EXPIRED_LINK",
  notFoundLink: "NOT_FOUND_LINK",
  successEyebrow: "SUCCESS_EYEBROW",
  successTitle: "SUCCESS_TITLE",
  successSubtitle: "SUCCESS_SUBTITLE",
  successBody: "SUCCESS_BODY",
  formEyebrow: "FORM_EYEBROW",
  formTitle: "FORM_TITLE",
  formSubtitle: "FORM_SUBTITLE",
  passwordLabel: "PW_LABEL",
  confirmPasswordLabel: "CONFIRM_LABEL",
  showPassword: "SHOW_PW",
  hidePassword: "HIDE_PW",
  requirementsTitle: "REQUIREMENTS_TITLE",
  requirements: ["REQ_1", "REQ_2", "REQ_3", "REQ_4"],
  submitting: "SUBMITTING",
  submit: "SUBMIT",
};

const VALID_PASSWORD = "Str0ngP@ssword";

function renderContent(
  overrides: Partial<{
    confirmReset: (
      token: string,
      password: string,
      confirmPassword: string,
    ) => Promise<{ message: string }>;
    successRedirectPath: string;
    backHref: string;
    backLabel: string;
  }> = {},
) {
  return render(
    <ResetPasswordPageContent
      confirmReset={
        overrides.confirmReset ?? vi.fn().mockResolvedValue({ message: "ok" })
      }
      successRedirectPath={overrides.successRedirectPath ?? "/login"}
      backHref={overrides.backHref ?? "/login"}
      backLabel={overrides.backLabel ?? "BACK_LABEL"}
      copy={COPY}
    />,
  );
}

function fillPasswords(password: string, confirm = password) {
  fireEvent.change(screen.getByLabelText(COPY.passwordLabel), {
    target: { value: password },
  });
  fireEvent.change(screen.getByLabelText(COPY.confirmPasswordLabel), {
    target: { value: confirm },
  });
}

function submit() {
  fireEvent.click(screen.getByRole("button", { name: COPY.submit }));
}

beforeEach(() => {
  vi.clearAllMocks();
  mocks.token = "valid-token";
  mocks.tenant = null;
});

// ---------------------------------------------------------------------------
// Token handling
// ---------------------------------------------------------------------------

describe("ResetPasswordPageContent — token handling", () => {
  it("renders the form with an enabled submit when a token is present", () => {
    renderContent();

    expect(screen.getByText(COPY.formTitle)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: COPY.submit })).toBeEnabled();
    expect(screen.getByLabelText(COPY.passwordLabel)).toBeEnabled();
  });

  it("shows the missing-token error and disables the form when no token is in the URL", () => {
    mocks.token = null;
    renderContent();

    expect(screen.getByText(COPY.missingToken)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: COPY.submit })).toBeDisabled();
    expect(screen.getByLabelText(COPY.passwordLabel)).toBeDisabled();
  });

  it("renders the password requirements list", () => {
    renderContent();

    expect(screen.getByText(COPY.requirementsTitle)).toBeInTheDocument();
    for (const requirement of COPY.requirements) {
      expect(screen.getByText(requirement)).toBeInTheDocument();
    }
  });
});

// ---------------------------------------------------------------------------
// Client-side password validation — one assertion per branch in
// validatePassword(), in the order the function checks them.
// ---------------------------------------------------------------------------

describe("ResetPasswordPageContent — password validation", () => {
  it.each([
    ["too short", "Aa1!", COPY.passwordTooShort],
    ["missing uppercase", "abcdefg1!", COPY.passwordMissingUppercase],
    ["missing lowercase", "ABCDEFG1!", COPY.passwordMissingLowercase],
    ["missing number", "Abcdefgh!", COPY.passwordMissingNumber],
    ["missing special char", "Abcdefg12", COPY.passwordMissingSpecial],
  ])(
    "blocks submit and shows the %s error",
    async (_label, password, expectedError) => {
      const confirmReset = vi.fn();
      renderContent({ confirmReset });

      fillPasswords(password);
      submit();

      expect(await screen.findByText(expectedError)).toBeInTheDocument();
      expect(confirmReset).not.toHaveBeenCalled();
    },
  );

  it("blocks submit when the two passwords differ", async () => {
    const confirmReset = vi.fn();
    renderContent({ confirmReset });

    fillPasswords(VALID_PASSWORD, `${VALID_PASSWORD}X`);
    submit();

    expect(await screen.findByText(COPY.passwordMismatch)).toBeInTheDocument();
    expect(confirmReset).not.toHaveBeenCalled();
  });

  it("disables submit and never calls the network when the token is absent", () => {
    // Without a token the form is inert: the submit button is disabled and the
    // missing-token error is shown, so a tokenless confirm can never be sent.
    mocks.token = null;
    const confirmReset = vi.fn();
    renderContent({ confirmReset });

    expect(screen.getByText(COPY.missingToken)).toBeInTheDocument();
    fillPasswords(VALID_PASSWORD);
    submit();

    expect(screen.getByRole("button", { name: COPY.submit })).toBeDisabled();
    expect(confirmReset).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Submission success
// ---------------------------------------------------------------------------

describe("ResetPasswordPageContent — successful reset", () => {
  it("calls confirmReset with the token and both passwords, then shows the success screen", async () => {
    const confirmReset = vi.fn().mockResolvedValue({ message: "ok" });
    renderContent({ confirmReset });

    fillPasswords(VALID_PASSWORD);
    submit();

    expect(await screen.findByText(COPY.successTitle)).toBeInTheDocument();
    expect(confirmReset).toHaveBeenCalledWith(
      "valid-token",
      VALID_PASSWORD,
      VALID_PASSWORD,
    );
    expect(screen.getByText(COPY.successBody)).toBeInTheDocument();
  });

  it("redirects to the success path after the post-success delay", async () => {
    vi.useFakeTimers();
    try {
      const confirmReset = vi.fn().mockResolvedValue({ message: "ok" });
      renderContent({ confirmReset, successRedirectPath: "/parents/login" });

      fillPasswords(VALID_PASSWORD);
      submit();

      // Flush the awaited confirmReset() + the success state update.
      await act(async () => {
        await Promise.resolve();
        await Promise.resolve();
      });

      expect(screen.getByText(COPY.successTitle)).toBeInTheDocument();
      expect(mocks.push).not.toHaveBeenCalled();

      act(() => {
        vi.advanceTimersByTime(3000);
      });

      expect(mocks.push).toHaveBeenCalledWith("/parents/login");
    } finally {
      vi.useRealTimers();
    }
  });
});

// ---------------------------------------------------------------------------
// Submission failure — status-to-message mapping
// ---------------------------------------------------------------------------

describe("ResetPasswordPageContent — error mapping", () => {
  it.each([
    [410, COPY.expiredLink],
    [404, COPY.notFoundLink],
    [400, COPY.invalidRequest],
    [500, COPY.genericError],
  ])(
    "maps a %s response to its specific copy",
    async (status, expectedMessage) => {
      vi.spyOn(console, "error").mockImplementation(() => undefined);
      vi.spyOn(console, "warn").mockImplementation(() => undefined);

      const confirmReset = vi
        .fn()
        .mockRejectedValue(Object.assign(new Error("boom"), { status }));
      renderContent({ confirmReset });

      fillPasswords(VALID_PASSWORD);
      submit();

      expect(await screen.findByText(expectedMessage)).toBeInTheDocument();
      // The form is shown again (not the success screen) so the user can retry.
      expect(
        screen.getByRole("button", { name: COPY.submit }),
      ).toBeInTheDocument();
    },
  );

  it("falls back to the generic error when the rejection has no status", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);

    const confirmReset = vi.fn().mockRejectedValue(new Error("network down"));
    renderContent({ confirmReset });

    fillPasswords(VALID_PASSWORD);
    submit();

    expect(await screen.findByText(COPY.genericError)).toBeInTheDocument();
  });

  it("re-enables the submit button after a failed attempt", async () => {
    vi.spyOn(console, "error").mockImplementation(() => undefined);

    const confirmReset = vi
      .fn()
      .mockRejectedValue(Object.assign(new Error("boom"), { status: 400 }));
    renderContent({ confirmReset });

    fillPasswords(VALID_PASSWORD);
    submit();

    await screen.findByText(COPY.invalidRequest);
    await waitFor(() =>
      expect(screen.getByRole("button", { name: COPY.submit })).toBeEnabled(),
    );
  });
});

// ---------------------------------------------------------------------------
// Password visibility toggle + tenant branding
// ---------------------------------------------------------------------------

describe("ResetPasswordPageContent — UI affordances", () => {
  it("toggles the password field between hidden and visible", () => {
    renderContent();

    const passwordInput = screen.getByLabelText(COPY.passwordLabel);
    expect(passwordInput).toHaveAttribute("type", "password");

    fireEvent.click(
      screen.getAllByRole("button", { name: COPY.showPassword })[0]!,
    );
    expect(passwordInput).toHaveAttribute("type", "text");

    // The confirm field has its own independent toggle (after the first field
    // flipped to visible, only the confirm toggle still reads "show").
    const confirmInput = screen.getByLabelText(COPY.confirmPasswordLabel);
    expect(confirmInput).toHaveAttribute("type", "password");
    fireEvent.click(
      screen.getAllByRole("button", { name: COPY.showPassword })[0]!,
    );
    expect(confirmInput).toHaveAttribute("type", "text");
  });

  it("renders the tenant login image when the tenant has one configured", () => {
    mocks.tenant = {
      tenant: {
        name: "Musterschule",
        settings: { loginImageUrl: "/uploads/logo.png" },
      },
    };
    renderContent();

    const logo = screen.getByAltText("Musterschule Logo");
    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute("src", "/resolved/uploads/logo.png");
  });
});

// ---------------------------------------------------------------------------
// Default copy — exercises DEFAULT_RESET_PASSWORD_PAGE_COPY + default props so
// those literals are covered when a caller omits the copy prop.
// ---------------------------------------------------------------------------

describe("ResetPasswordPageContent — default copy", () => {
  it("renders the German default chrome when no copy prop is supplied", () => {
    render(<ResetPasswordPageContent confirmReset={vi.fn()} />);

    expect(screen.getByText("Neues Passwort festlegen")).toBeInTheDocument();
    expect(screen.getByText("Passwort-Anforderungen:")).toBeInTheDocument();
  });
});
