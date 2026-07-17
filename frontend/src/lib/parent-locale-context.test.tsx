import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";

import { ParentLocaleProvider, useParentLocale } from "./parent-locale-context";
import { fetchParentProfile, updateParentPortalLocale } from "./parent-api";
import { useSession } from "next-auth/react";
import { useLocale } from "next-intl";
import { writeLocaleCookie } from "~/i18n/locales";

// next/navigation: a STABLE router object so the sync effect (which depends on
// `router`) doesn't re-run every render and re-fetch the profile.
const refreshMock = vi.fn();
const routerMock = { refresh: refreshMock };
vi.mock("next/navigation", () => ({
  useRouter: () => routerMock,
}));

// Control the session status per test.
vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));

// Override the globally-mocked next-intl so we can drive the server-resolved
// locale (useLocale) the provider starts from. useTranslations returns the key
// verbatim so failure-toast assertions can match on "saveError".
vi.mock("next-intl", () => ({
  useLocale: vi.fn(() => "de"),
  useTranslations: () => (key: string) => key,
}));

// The provider surfaces a toast when an authenticated locale save fails.
const toastErrorMock = vi.fn();
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => ({
    error: toastErrorMock,
    success: vi.fn(),
    info: vi.fn(),
    warning: vi.fn(),
    remove: vi.fn(),
  }),
}));

// The profile API is exercised through the provider, never hit for real.
vi.mock("./parent-api", () => ({
  fetchParentProfile: vi.fn(),
  updateParentPortalLocale: vi.fn(),
}));

// Keep normalizeLocale / DEFAULT_LOCALE real; spy on the cookie writer.
vi.mock("~/i18n/locales", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/i18n/locales")>();
  return { ...actual, writeLocaleCookie: vi.fn() };
});

const mockedUseSession = vi.mocked(useSession);
const mockedUseLocale = vi.mocked(useLocale);
const mockedFetchProfile = vi.mocked(fetchParentProfile);
const mockedUpdateLocale = vi.mocked(updateParentPortalLocale);
const mockedWriteCookie = vi.mocked(writeLocaleCookie);

function setSession(status: "authenticated" | "unauthenticated" | "loading") {
  // The provider only reads `status`; data/update are unused.
  mockedUseSession.mockReturnValue({
    status,
    data: null,
    update: vi.fn(),
  } as unknown as ReturnType<typeof useSession>);
}

function Consumer() {
  const ctx = useParentLocale();
  return (
    <div>
      <span data-testid="locale">{ctx?.locale}</span>
      <span data-testid="auth">{String(ctx?.authenticated)}</span>
      <button type="button" onClick={() => void ctx?.setLocale("ru")}>
        set-ru
      </button>
      <button type="button" onClick={() => void ctx?.setLocale("en")}>
        set-en
      </button>
    </div>
  );
}

function renderProvider() {
  return render(
    <ParentLocaleProvider>
      <Consumer />
    </ParentLocaleProvider>,
  );
}

function createDeferred<T>() {
  let resolve!: (value: T | PromiseLike<T>) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((innerResolve, innerReject) => {
    resolve = innerResolve;
    reject = innerReject;
  });
  return { promise, resolve, reject };
}

beforeEach(() => {
  vi.clearAllMocks();
  window.localStorage.clear();
  mockedUseLocale.mockReturnValue("de");
  mockedUpdateLocale.mockResolvedValue({ portal_locale: "ru" });
});

describe("ParentLocaleProvider — anonymous", () => {
  it("does not touch the profile API and seeds locale from useLocale", async () => {
    setSession("unauthenticated");
    mockedUseLocale.mockReturnValue("en");
    renderProvider();

    expect(screen.getByTestId("auth").textContent).toBe("false");
    expect(screen.getByTestId("locale").textContent).toBe("en");
    // Anonymous surfaces have no profile to sync.
    expect(mockedFetchProfile).not.toHaveBeenCalled();
  });

  it("setLocale writes the cookie and soft-refreshes without persisting", async () => {
    setSession("unauthenticated");
    renderProvider();

    fireEvent.click(screen.getByText("set-ru"));

    await waitFor(() => expect(mockedWriteCookie).toHaveBeenCalledWith("ru"));
    expect(refreshMock).toHaveBeenCalled();
    // No profile write when anonymous.
    expect(mockedUpdateLocale).not.toHaveBeenCalled();
  });
});

describe("ParentLocaleProvider — authenticated profile sync", () => {
  it("adopts the anonymous locale and persists it when none is stored", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("en"); // arrived as English
    mockedFetchProfile.mockResolvedValue({ portal_locale: null });

    renderProvider();

    // The arrived-with locale is persisted so it sticks, ...
    await waitFor(() => expect(mockedUpdateLocale).toHaveBeenCalledWith("en"));
    expect(screen.getByTestId("locale").textContent).toBe("en");
    // ... but no refresh: the tree was already server-rendered in "en".
    expect(refreshMock).not.toHaveBeenCalled();
  });

  it("persists German when the adopted locale is already the default", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de");
    mockedFetchProfile.mockResolvedValue({ portal_locale: null });

    renderProvider();

    await waitFor(() => expect(mockedUpdateLocale).toHaveBeenCalledWith("de"));
    // de is the default, but it may still be an explicit pre-login choice.
    // Persisting it prevents the next non-German browser from overwriting it.
    expect(refreshMock).not.toHaveBeenCalled();
  });

  it("applies an explicit stored locale and soft-refreshes when it differs", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de"); // rendered in de
    mockedFetchProfile.mockResolvedValue({ portal_locale: "ru" });

    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId("locale").textContent).toBe("ru"),
    );
    expect(mockedWriteCookie).toHaveBeenCalledWith("ru");
    // Stored ru ≠ rendered de → refresh the server tree in ru.
    expect(refreshMock).toHaveBeenCalled();
  });

  it("does not refresh when the stored locale already matches the render", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("ru");
    mockedFetchProfile.mockResolvedValue({ portal_locale: "ru" });

    renderProvider();

    await waitFor(() => expect(mockedFetchProfile).toHaveBeenCalled());
    expect(refreshMock).not.toHaveBeenCalled();
  });

  it("swallows a profile fetch failure (graceful degradation)", async () => {
    setSession("authenticated");
    mockedFetchProfile.mockRejectedValue(new Error("network"));

    renderProvider();

    await waitFor(() => expect(mockedFetchProfile).toHaveBeenCalled());
    // No crash, no refresh, stays on the server-resolved locale.
    expect(screen.getByTestId("locale").textContent).toBe("de");
    expect(refreshMock).not.toHaveBeenCalled();
  });
});

describe("ParentLocaleProvider — setLocale when authenticated", () => {
  it("persists the chosen locale to the profile and soft-refreshes", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de");
    // Stored de so the mount effect is a no-op (no refresh from sync).
    mockedFetchProfile.mockResolvedValue({ portal_locale: "de" });

    renderProvider();
    await waitFor(() => expect(mockedFetchProfile).toHaveBeenCalled());
    refreshMock.mockClear();

    fireEvent.click(screen.getByText("set-ru"));

    await waitFor(() => expect(mockedUpdateLocale).toHaveBeenCalledWith("ru"));
    expect(mockedWriteCookie).toHaveBeenCalledWith("ru");
    expect(refreshMock).toHaveBeenCalled();
  });

  it("toasts but still switches the UI when the persist fails", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de");
    mockedFetchProfile.mockResolvedValue({ portal_locale: "de" });
    mockedUpdateLocale.mockRejectedValue(new Error("network"));

    const { rerender } = renderProvider();
    await waitFor(() => expect(mockedFetchProfile).toHaveBeenCalled());
    refreshMock.mockClear();
    mockedWriteCookie.mockClear();

    fireEvent.click(screen.getByText("set-ru"));

    // The cookie + soft refresh still apply the choice locally, ...
    await waitFor(() => expect(mockedWriteCookie).toHaveBeenCalledWith("ru"));
    expect(refreshMock).toHaveBeenCalled();
    // ... but the failed account save is surfaced, not swallowed.
    await waitFor(() =>
      expect(toastErrorMock).toHaveBeenCalledWith("saveError"),
    );

    mockedUseLocale.mockReturnValue("ru");
    rerender(
      <ParentLocaleProvider>
        <Consumer />
      </ParentLocaleProvider>,
    );

    await waitFor(() =>
      expect(mockedFetchProfile.mock.calls.length).toBeGreaterThanOrEqual(2),
    );
    expect(screen.getByTestId("locale").textContent).toBe("ru");
    expect(mockedWriteCookie).not.toHaveBeenCalledWith("de");
  });

  it("serializes profile saves so the last locale choice wins", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de");
    mockedFetchProfile.mockResolvedValue({ portal_locale: "de" });

    const firstSave = createDeferred<{ portal_locale: "ru" }>();
    mockedUpdateLocale
      .mockReturnValueOnce(firstSave.promise)
      .mockResolvedValueOnce({ portal_locale: "en" });

    renderProvider();
    await waitFor(() => expect(mockedFetchProfile).toHaveBeenCalled());
    refreshMock.mockClear();

    fireEvent.click(screen.getByText("set-ru"));
    await waitFor(() => expect(mockedUpdateLocale).toHaveBeenCalledWith("ru"));

    fireEvent.click(screen.getByText("set-en"));
    expect(screen.getByTestId("locale").textContent).toBe("en");
    expect(mockedWriteCookie).toHaveBeenCalledWith("en");
    expect(mockedUpdateLocale).toHaveBeenCalledTimes(1);

    firstSave.resolve({ portal_locale: "ru" });

    await waitFor(() => expect(mockedUpdateLocale).toHaveBeenCalledWith("en"));
    expect(mockedUpdateLocale.mock.calls.map(([locale]) => locale)).toEqual([
      "ru",
      "en",
    ]);
    expect(refreshMock).toHaveBeenCalledTimes(1);
    expect(window.localStorage.getItem("phoenix_parent_unsynced_locale")).toBe(
      null,
    );
  });
});
