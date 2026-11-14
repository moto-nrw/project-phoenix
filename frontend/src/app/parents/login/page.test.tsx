import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import enMessages from "~/i18n/messages/en.generated";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { catalogText } from "~/test/error-catalog-text";

/** The English sentence the shared error path shows (#2518). */
const enCatalogText = (code: ErrorCode, object: string) =>
  catalogText(code, object, "en");

const LOGIN_OBJECT = enMessages.parentLogin.errorObject;

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  redirect: vi.fn(),
  login: vi.fn(),
  signOut: vi.fn(),
  useSession: vi.fn(),
  searchParamsGet: vi.fn((_key: string) => null as string | null),
  requestParentPasswordReset: vi.fn(),
}));

vi.mock("~/lib/auth-api", () => ({
  requestParentPasswordReset: mocks.requestParentPasswordReset,
}));

vi.mock("next/navigation", () => ({
  redirect: mocks.redirect,
  useRouter: () => ({ push: mocks.push, refresh: vi.fn() }),
  // Stable object: the page reads it during render, a fresh identity each
  // call would churn dependent effects.
  useSearchParams: () => ({ get: mocks.searchParamsGet }),
}));

vi.mock("next-auth/react", () => ({
  signOut: mocks.signOut,
  useSession: mocks.useSession,
}));

// The page signs in through the parents BFF (#7051); the transport has its
// own tests.
vi.mock("~/lib/portal-password-login", () => ({
  portalPasswordLogin: mocks.login,
}));

/** A refusal of the parents BFF, as the transport throws it. */
async function refusal(status: number, code: ErrorCode) {
  const { ApiError } = await import("~/lib/api-error");
  return new ApiError("refused", status, { code });
}

// parentPath() reads NEXT_PUBLIC_PARENTS_HOSTNAME and throws when it is unset,
// which no unit-test env provides. The path mapping itself is covered by
// parent-url.test.ts.
vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("next-intl", async () => {
  const en = (await import("~/i18n/messages/en.generated")).default as Record<
    string,
    unknown
  >;

  const resolve = (path: string): unknown =>
    path.split(".").reduce<unknown>((acc, part) => {
      if (acc && typeof acc === "object") {
        return (acc as Record<string, unknown>)[part];
      }
      return undefined;
    }, en);

  const interpolate = (
    value: string,
    values?: Record<string, unknown>,
  ): string =>
    values
      ? Object.entries(values).reduce(
          (str, [key, next]) => str.replaceAll(`{${key}}`, String(next)),
          value,
        )
      : value;

  const makeT = (namespace?: string) => {
    const prefix = namespace ? `${namespace}.` : "";
    const t = (key: string, values?: Record<string, unknown>) => {
      const value = resolve(`${prefix}${key}`);
      return typeof value === "string"
        ? interpolate(value, values)
        : `${prefix}${key}`;
    };
    // Mirror next-intl's `t.raw`: returns the message verbatim with no ICU
    // formatting (the auth shell fills `{number}` itself via String.replace).
    t.raw = (key: string) => {
      const value = resolve(`${prefix}${key}`);
      return typeof value === "string" ? value : `${prefix}${key}`;
    };
    // Mirror next-intl's `t.rich`: a tag hands its chunk to its function.
    t.rich = (key: string, values: Record<string, unknown> = {}) => {
      const plain = Object.fromEntries(
        Object.entries(values).filter(([, v]) => typeof v !== "function"),
      );
      const text = t(key, plain);
      return text
        .split(/(<\w+>[^<]*<\/\w+>)/)
        .filter(Boolean)
        .map((part) => {
          const match = /^<(\w+)>([^<]*)<\/\w+>$/.exec(part);
          const render = match ? values[match[1] ?? ""] : undefined;
          return typeof render === "function"
            ? (render as (chunk: string) => unknown)(match?.[2] ?? "")
            : part;
        });
    };
    return t;
  };

  const cache = new Map<string, ReturnType<typeof makeT>>();

  return {
    useLocale: () => "en",
    useTranslations: (namespace?: string) => {
      const key = namespace ?? "";
      const existing = cache.get(key);
      if (existing) return existing;
      const t = makeT(namespace);
      cache.set(key, t);
      return t;
    },
  };
});

import ParentLoginPage from "./page";

afterEach(() => {
  document.documentElement.lang = "";
});

describe("ParentLoginPage i18n", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    // The shared error path reads the page language from <html lang>, which
    // the parents layout sets to the parent's locale.
    document.documentElement.lang = "en";
    mocks.searchParamsGet.mockReturnValue(null);
    mocks.useSession.mockReturnValue({
      status: "unauthenticated",
      data: null,
    });
  });

  it("renders a pre-login language switcher and localized parent auth chrome", () => {
    render(<ParentLoginPage />);

    expect(screen.getByRole("button", { name: "Language" })).toBeVisible();
    expect(screen.getByText("Language")).toBeInTheDocument();
    expect(screen.getByText("English")).toBeInTheDocument();

    expect(screen.getByText("Made for after-school care")).toBeInTheDocument();
    expect(screen.getByText("Hosted in Germany")).toBeInTheDocument();
    expect(
      screen.getByText(
        /I can find the most important information about childcare/,
      ),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Parent")).toHaveLength(4);
    expect(screen.getByText("Everything in one place")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Show testimonial 1" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Forgot your password?" }),
    ).toBeEnabled();

    expect(
      screen.queryByText("Für den Ganztag gemacht"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("In Deutschland gehostet"),
    ).not.toBeInTheDocument();
    expect(
      within(screen.getByRole("main")).queryByText("Elternteil"),
    ).not.toBeInTheDocument();
  });

  it("keeps the auth shell and language switcher stable while the session loads", () => {
    mocks.useSession.mockReturnValue({
      status: "loading",
      data: null,
    });

    render(<ParentLoginPage />);

    expect(screen.getByRole("button", { name: "Language" })).toBeVisible();
    expect(screen.getByText("Welcome to the parent portal")).toBeVisible();
    expect(screen.queryByText("Loading...")).not.toBeInTheDocument();

    expect(screen.getByLabelText("Email address")).toBeDisabled();
    expect(screen.getByLabelText("Password")).toBeDisabled();
    expect(screen.getByRole("button", { name: "Sign in" })).toBeDisabled();
  });

  it("clears an errored parent session before redirecting", async () => {
    mocks.signOut.mockResolvedValue(undefined);
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: {
        error: "RefreshTokenExpired",
        user: { scope: "parent" },
      },
    });

    render(<ParentLoginPage />);

    expect(mocks.redirect).not.toHaveBeenCalled();
    await vi.waitFor(() => {
      expect(mocks.signOut).toHaveBeenCalledWith({ redirect: false });
    });
  });

  it("reports a failed stale-session cleanup and runs it again on retry", async () => {
    mocks.signOut
      .mockRejectedValueOnce(new TypeError("Failed to fetch"))
      .mockResolvedValueOnce(undefined);
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: {
        error: "RefreshTokenExpired",
        user: { scope: "parent" },
      },
    });

    render(<ParentLoginPage />);

    // The locked form says why instead of spinning without an end.
    expect(
      await screen.findByText(
        enCatalogText("general.unavailable", LOGIN_OBJECT),
      ),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    await vi.waitFor(() => {
      expect(mocks.signOut).toHaveBeenCalledTimes(2);
    });
  });

  it("redirects to the parents portal once the session is published", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { scope: "parent" } },
    });

    render(<ParentLoginPage />);

    expect(mocks.redirect).toHaveBeenCalledWith("/parents");
  });

  it("returns a parent to the holiday registration the login was opened from (#6671)", () => {
    mocks.searchParamsGet.mockImplementation((key: string) =>
      key === "next" ? "/ferienanmeldung/link-key" : null,
    );
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { scope: "parent" } },
    });

    render(<ParentLoginPage />);

    expect(mocks.redirect).toHaveBeenCalledWith("/ferienanmeldung/link-key");
  });

  it("ignores a return target that is no holiday registration", () => {
    mocks.searchParamsGet.mockImplementation((key: string) =>
      key === "next" ? "https://example.org/ferienanmeldung/link-key" : null,
    );
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { scope: "parent" } },
    });

    render(<ParentLoginPage />);

    expect(mocks.redirect).toHaveBeenCalledWith("/parents");
  });

  // The layout's server snapshot carries no access token (#4990, #5005).
  it("redirects a signed-in parent whose session came from the server snapshot", () => {
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: { user: { id: "41", scope: "parent" } },
    });

    render(<ParentLoginPage />);

    expect(mocks.redirect).toHaveBeenCalledWith("/parents");
  });

  // The browser session carries no token (#5005): a session without a
  // usable access token is marked with an error and never enters the portal.
  it("does not redirect a session without a usable access token", async () => {
    mocks.signOut.mockResolvedValue(undefined);
    mocks.useSession.mockReturnValue({
      status: "authenticated",
      data: {
        error: "RefreshTokenError",
        user: { id: "41", scope: "parent" },
      },
    });

    render(<ParentLoginPage />);

    await vi.waitFor(() => {
      expect(mocks.signOut).toHaveBeenCalled();
    });
    expect(mocks.redirect).not.toHaveBeenCalled();
  });

  async function fillAndSubmit(email = "parent@example.com") {
    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: email },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "password123" },
    });
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
    });
  }

  it("enters the portal once the BFF installed the session, without a router push", async () => {
    // Two concurrent navigations to the same target left the App Router state
    // as a pending thenable; the page leaves by a full load instead.
    mocks.login.mockResolvedValue({ status: "authenticated" });

    render(<ParentLoginPage />);
    await fillAndSubmit();

    expect(mocks.login).toHaveBeenCalledWith(
      "parent",
      "parent@example.com",
      "password123",
    );
    await vi.waitFor(() => expect(window.location.pathname).toBe("/parents"));
    expect(mocks.push).not.toHaveBeenCalled();
    // The form stays locked until the page has left.
    expect(
      screen.getByRole("button", { name: "Signing in..." }),
    ).toBeDisabled();
  });

  // The sign-in with a code (#7051): the password leads to the code step in
  // the parent's language, with the two answers of every portal.
  it("asks for the mailed code when the account has the second factor", async () => {
    mocks.login.mockResolvedValue({
      status: "mfa_required",
      challengeToken: "challenge",
      maskedEmail: "p***@example.com",
      trustedDeviceEnabled: true,
      trustedDeviceDays: 90,
    });

    render(<ParentLoginPage />);
    await fillAndSubmit();

    expect(
      await screen.findByRole("heading", { name: "Enter code" }),
    ).toBeVisible();
    expect(screen.getByText("p***@example.com")).toBeVisible();
    expect(screen.getByText("Should moto remember this device?")).toBeVisible();
    expect(screen.getByText("Yes, remember and sign in")).toBeVisible();
    expect(screen.getByText("No, just sign in")).toBeVisible();
    expect(
      screen.getByText(
        "No code on this device for 90 days. Only for your own device.",
      ),
    ).toBeVisible();
    expect(screen.queryByText("Code-Eingabe")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Back to sign-in" }));
    expect(screen.getByLabelText("Password")).toHaveValue("");
  });

  it("opens the password reset modal with localized parent copy", () => {
    render(<ParentLoginPage />);

    fireEvent.click(
      screen.getByRole("button", { name: "Forgot your password?" }),
    );

    const resetDialog = within(screen.getByRole("dialog"));

    expect(resetDialog.getByText("Reset password")).toBeVisible();
    expect(
      resetDialog.getByText(
        "Enter your email address and we will send you a link to reset your password.",
      ),
    ).toBeVisible();
    expect(resetDialog.getByLabelText("Email address")).toBeVisible();
    expect(
      resetDialog.getByRole("button", { name: "Send link" }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Passwort zurücksetzen")).not.toBeInTheDocument();
  });

  it("shows a failed reset request in the dialog in the parent's language", async () => {
    const { ApiError } = await import("~/lib/api-error");
    mocks.requestParentPasswordReset.mockRejectedValue(
      new ApiError("diag", 503, { code: "general.unavailable" }),
    );
    render(<ParentLoginPage />);

    fireEvent.click(
      screen.getByRole("button", { name: "Forgot your password?" }),
    );
    const resetDialog = within(screen.getByRole("dialog"));
    fireEvent.change(resetDialog.getByLabelText("Email address"), {
      target: { value: "parent@example.com" },
    });
    fireEvent.click(resetDialog.getByRole("button", { name: "Send link" }));

    expect(
      await resetDialog.findByText(
        enCatalogText(
          "general.unavailable",
          enMessages.parentPasswordResetModal.errorObject,
        ),
      ),
    ).toBeVisible();
    expect(
      resetDialog.getByRole("button", { name: "Try again" }),
    ).toBeVisible();
  });

  describe("portal mix-up", () => {
    it("greets guardians handed over by the staff login", () => {
      mocks.searchParamsGet.mockImplementation((key: string) =>
        key === "from" ? "staff" : null,
      );

      render(<ParentLoginPage />);

      expect(
        screen.getByText(/Parent accounts sign in here/),
      ).toBeInTheDocument();
    });

    it("shows no hand-over banner on a plain visit", () => {
      render(<ParentLoginPage />);

      // Anker: das Anmeldeformular des Elternportals ist gerendert.
      expect(screen.getByLabelText("Email address")).toBeInTheDocument();
      expect(
        screen.queryByText(/Parent accounts sign in here/),
      ).not.toBeInTheDocument();
    });

    async function submitLogin() {
      render(<ParentLoginPage />);

      await act(async () => {
        fireEvent.change(screen.getByLabelText("Email address"), {
          target: { value: "staff@example.com" },
        });
        fireEvent.change(screen.getByLabelText("Password"), {
          target: { value: "correct-password" },
        });
      });

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
      });
    }

    // Mirror of the tenant-side fix: the backend refuses a staff account
    // here only AFTER accepting the password, so naming the reason leaks
    // nothing and saves the user a pointless password reset.
    it("names the reason when a staff account lands here", async () => {
      mocks.login.mockRejectedValue(await refusal(403, "care.not_a_guardian"));

      await submitLogin();

      expect(
        screen.getByText(/This account belongs to school staff/),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("link", { name: "Go to the school login" }),
      ).toBeInTheDocument();
      // The masked copy it replaced — must not come back.
      expect(
        screen.queryByText(/Please check your credentials/),
      ).not.toBeInTheDocument();
    });

    // None of the guardian's schools booked Eltern-App (#4895): the page
    // says who decides, in the parent's language, instead of an error.
    it("explains a parents portal the school does not use", async () => {
      mocks.login.mockRejectedValue(await refusal(403, "module.not_booked"));

      await submitLogin();

      expect(
        screen.getByText(
          /Your school is not using the parent portal at the moment/,
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(/Please check your credentials/),
      ).not.toBeInTheDocument();
    });

    it("keeps a wrong password generic and offers no staff link", async () => {
      mocks.login.mockRejectedValue(
        await refusal(401, "care.invalid_credentials"),
      );

      await submitLogin();

      expect(
        await screen.findByText(
          enCatalogText("care.invalid_credentials", LOGIN_OBJECT),
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByRole("link", { name: "Go to the school login" }),
      ).not.toBeInTheDocument();
    });
  });

  describe("login errors from codes", () => {
    async function submitLogin() {
      render(<ParentLoginPage />);
      await act(async () => {
        fireEvent.change(screen.getByLabelText("Email address"), {
          target: { value: "parent@example.com" },
        });
        fireEvent.change(screen.getByLabelText("Password"), {
          target: { value: "password123" },
        });
      });
      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
      });
    }

    it("names a switched-off account by its code", async () => {
      mocks.login.mockRejectedValue(
        await refusal(401, "identity.session_account_inactive"),
      );

      await submitLogin();

      expect(
        await screen.findByText(
          enCatalogText("identity.session_account_inactive", LOGIN_OBJECT),
        ),
      ).toBeInTheDocument();
    });

    it("keeps its own sentence for too many attempts", async () => {
      mocks.login.mockRejectedValue(await refusal(429, "general.unavailable"));

      await submitLogin();

      expect(
        screen.getByText(enMessages.parentLogin.errors.rateLimited),
      ).toBeInTheDocument();
    });

    it("never shows the text of a thrown sign-in error", async () => {
      mocks.login
        .mockRejectedValueOnce(new Error("Failed to fetch secret detail"))
        .mockResolvedValue({ status: "authenticated" });

      await submitLogin();

      expect(
        await screen.findByText(
          enCatalogText("general.unavailable", LOGIN_OBJECT),
        ),
      ).toBeInTheDocument();
      expect(screen.queryByText(/secret detail/)).not.toBeInTheDocument();

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Try again" }));
      });
      expect(mocks.login).toHaveBeenCalledTimes(2);
      expect(mocks.login).toHaveBeenLastCalledWith(
        "parent",
        "parent@example.com",
        "password123",
      );
    });

    it("keeps the session-expired notice until an error replaces it", async () => {
      mocks.searchParamsGet.mockImplementation((key: string) =>
        key === "error" ? "SessionExpired" : null,
      );
      mocks.login.mockRejectedValue(
        await refusal(401, "identity.invalid_credentials"),
      );

      render(<ParentLoginPage />);
      expect(
        screen.getByText(enMessages.errorCatalog.loginNotice),
      ).toBeInTheDocument();

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Sign in" }));
      });

      expect(
        await screen.findByText(
          enCatalogText("identity.invalid_credentials", LOGIN_OBJECT),
        ),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(enMessages.errorCatalog.loginNotice),
      ).not.toBeInTheDocument();
    });
  });
});
