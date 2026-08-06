import { fireEvent, render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  redirect: vi.fn(),
  signIn: vi.fn(),
  signOut: vi.fn(),
  useSession: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  redirect: mocks.redirect,
  useRouter: () => ({ push: mocks.push, refresh: vi.fn() }),
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

describe("ParentLoginPage i18n", () => {
  beforeEach(() => {
    vi.clearAllMocks();
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
});
