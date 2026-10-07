import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { releaseFakeTimers } from "~/test/clock";
import "@testing-library/jest-dom/vitest";
import enMessages from "~/i18n/messages/en.json";
import type { ErrorCode } from "~/lib/error-codes.generated";
import { catalogText } from "~/test/error-catalog-text";

/** The English sentence the shared error path shows (#2518). */
const enCatalogText = (code: ErrorCode, object: string) =>
  catalogText(code, object, "en");

const LOGIN_OBJECT = enMessages.parentLogin.errorObject;

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  redirect: vi.fn(),
  signIn: vi.fn(),
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
  signIn: mocks.signIn,
  signOut: mocks.signOut,
  useSession: mocks.useSession,
}));

// parentPath() reads NEXT_PUBLIC_PARENTS_HOSTNAME and throws when it is unset,
// which no unit-test env provides. The path mapping itself is covered by
// parent-url.test.ts.
vi.mock("~/lib/parent-url", () => ({
  parentPath: (path: string) => path,
}));

vi.mock("next-intl", async () => {
  const en = (await import("~/i18n/messages/en.json")).default as Record<
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
      screen.getByText(/I can find the most important OGS information/),
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
        user: { scope: "parent", token: "stale-token" },
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
        user: { scope: "parent", token: "stale-token" },
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
      data: { user: { scope: "parent", token: "valid-token" } },
    });

    render(<ParentLoginPage />);

    expect(mocks.redirect).toHaveBeenCalledWith("/parents");
  });

  it("leaves the redirect to the session and never pushes a second navigation", async () => {
    // Two concurrent navigations to the same target left the App Router state
    // as a pending thenable, and Next's conditional `use(state)` then rendered
    // a different number of hooks ("Rendered more hooks than during the
    // previous render") — the portal died right after login until a reload.
    mocks.signIn.mockResolvedValue({ error: null });

    render(<ParentLoginPage />);

    fireEvent.change(screen.getByLabelText("Email address"), {
      target: { value: "parent@example.com" },
    });
    fireEvent.change(screen.getByLabelText("Password"), {
      target: { value: "password123" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

    await vi.waitFor(() => {
      expect(mocks.signIn).toHaveBeenCalledWith("parent-credentials", {
        redirect: false,
        email: "parent@example.com",
        password: "password123",
      });
    });

    expect(mocks.push).not.toHaveBeenCalled();
    // The form stays locked until the session-driven redirect navigates away.
    expect(
      screen.getByRole("button", { name: "Signing in..." }),
    ).toBeDisabled();
  });

  it("releases the form when the session never arrives after a successful login", async () => {
    // NextAuth holt die Session per fetchData(), das jeden Fetch-Fehler
    // schluckt und null liefert. signIn meldet dann ok, status bleibt aber
    // "unauthenticated" und der Redirect feuert nie — ohne Watchdog bliebe das
    // Formular dauerhaft gesperrt und ohne Fehlermeldung zurück.
    vi.useFakeTimers();
    mocks.signIn.mockResolvedValue({ error: null });

    try {
      render(<ParentLoginPage />);

      fireEvent.change(screen.getByLabelText("Email address"), {
        target: { value: "parent@example.com" },
      });
      fireEvent.change(screen.getByLabelText("Password"), {
        target: { value: "password123" },
      });
      fireEvent.click(screen.getByRole("button", { name: "Sign in" }));

      await act(async () => {
        await vi.advanceTimersByTimeAsync(0);
      });

      expect(
        screen.getByRole("button", { name: "Signing in..." }),
      ).toBeDisabled();

      await act(async () => {
        await vi.advanceTimersByTimeAsync(10_000);
      });

      expect(screen.getByRole("button", { name: "Sign in" })).toBeEnabled();
      expect(
        screen.getByRole("button", { name: "Forgot your password?" }),
      ).toBeEnabled();
      await vi.waitFor(() =>
        expect(
          screen.getByText(enCatalogText("general.unavailable", LOGIN_OBJECT)),
        ).toBeVisible(),
      );
      expect(screen.getByRole("button", { name: "Try again" })).toBeVisible();
      expect(mocks.push).not.toHaveBeenCalled();
    } finally {
      releaseFakeTimers();
    }
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
      mocks.signIn.mockResolvedValue({
        error: "CredentialsSignin",
        code: "not_a_guardian",
      });

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

    it("keeps a wrong password generic and offers no staff link", async () => {
      mocks.signIn.mockResolvedValue({
        error: "CredentialsSignin",
        code: "invalid_credentials",
      });

      await submitLogin();

      expect(
        await screen.findByText(
          enCatalogText("identity.invalid_credentials", LOGIN_OBJECT),
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
      mocks.signIn.mockResolvedValue({
        error: "CredentialsSignin",
        code: "account_inactive",
      });

      await submitLogin();

      expect(
        await screen.findByText(
          enCatalogText("identity.session_account_inactive", LOGIN_OBJECT),
        ),
      ).toBeInTheDocument();
    });

    it("keeps its own sentence for too many attempts", async () => {
      mocks.signIn.mockResolvedValue({
        error: "CredentialsSignin",
        code: "rate_limited",
      });

      await submitLogin();

      expect(
        screen.getByText(enMessages.parentLogin.errors.rateLimited),
      ).toBeInTheDocument();
    });

    it("never shows the text of a thrown sign-in error", async () => {
      mocks.signIn
        .mockRejectedValueOnce(new Error("Failed to fetch secret detail"))
        .mockResolvedValue({ error: null });

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
      expect(mocks.signIn).toHaveBeenCalledTimes(2);
      expect(mocks.signIn).toHaveBeenLastCalledWith("parent-credentials", {
        redirect: false,
        email: "parent@example.com",
        password: "password123",
      });
    });

    it("keeps the session-expired notice until an error replaces it", async () => {
      mocks.searchParamsGet.mockImplementation((key: string) =>
        key === "error" ? "SessionExpired" : null,
      );
      mocks.signIn.mockResolvedValue({
        error: "CredentialsSignin",
        code: "invalid_credentials",
      });

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
