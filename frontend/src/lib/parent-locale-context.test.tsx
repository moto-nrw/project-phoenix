import { describe, it, expect, beforeEach, vi } from "vitest";
import { render, screen, waitFor, fireEvent } from "@testing-library/react";

// The provider now shares the real SWR cache with the parent shell.
vi.unmock("swr");

import { SWRConfig } from "swr";

import { ParentLocaleProvider, useParentLocale } from "./parent-locale-context";
import { fetchParentProfile, updateParentPortalLocale } from "./parent-api";
import { useSession } from "next-auth/react";
import { useLocale } from "next-intl";
import { writeLocaleCookie } from "~/i18n/locales";
import { ApiError, unavailableApiError } from "~/lib/api-error";

// A language change refreshes the server tree instead of loading the
// document a second time, so client state such as a half-filled form
// survives it (#5001, #5327).
const refreshMock = vi.hoisted(() => vi.fn());
const routerMock = vi.hoisted(() => ({ refresh: refreshMock }));
vi.mock("next/navigation", () => ({ useRouter: () => routerMock }));

// Control the session status per test.
vi.mock("next-auth/react", () => ({ useSession: vi.fn() }));

// Override the globally-mocked next-intl so we can drive the server-resolved
// locale (useLocale) the provider starts from. useTranslations returns the key
// verbatim so failure assertions can match on "errorObject".
vi.mock("next-intl", () => ({
  useLocale: vi.fn(() => "de"),
  useTranslations: () => (key: string) => key,
}));

// The provider reports a failed authenticated locale save on the shared
// error path. A stable mock keeps the provider's callbacks stable.
const showErrorMock = vi.hoisted(() => vi.fn());
const errorDisplay = vi.hoisted(() => ({ show: showErrorMock }));
vi.mock("~/contexts/ToastContext", () => ({
  useApiErrorDisplay: () => errorDisplay,
}));

// The profile API is exercised through the provider, never hit for real.
vi.mock("./parent-api", () => ({
  fetchParentProfile: vi.fn(),
  parentProfileCacheKey: (accountID: string) => ["parent-profile", accountID],
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
  // The provider keys the profile cache by the authenticated account.
  mockedUseSession.mockReturnValue({
    status,
    data: status === "authenticated" ? { user: { id: "parent-1" } } : null,
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
    <SWRConfig value={{ provider: () => new Map() }}>
      <ParentLocaleProvider>
        <Consumer />
      </ParentLocaleProvider>
    </SWRConfig>,
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
  window.sessionStorage.clear();
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
    expect(refreshMock).toHaveBeenCalledTimes(1);
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

  it("applies an explicit stored locale and refreshes without a second document load", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de"); // rendered in de
    mockedFetchProfile.mockResolvedValue({ portal_locale: "ru" });

    renderProvider();

    await waitFor(() =>
      expect(screen.getByTestId("locale").textContent).toBe("ru"),
    );
    expect(mockedWriteCookie).toHaveBeenCalledWith("ru");
    // Stored ru ≠ rendered de → refresh the server tree in ru, without
    // loading the document again.
    await waitFor(() => expect(refreshMock).toHaveBeenCalledTimes(1));
  });

  it("refreshes for an unsynced local choice the profile does not know yet", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de");
    mockedFetchProfile.mockResolvedValue({ portal_locale: "de" });
    window.localStorage.setItem(
      "phoenix_parent_unsynced_locale:parent-1",
      "ru",
    );

    const { rerender } = renderProvider();

    await waitFor(() => expect(refreshMock).toHaveBeenCalledTimes(1));
    expect(screen.getByTestId("locale").textContent).toBe("ru");
    expect(mockedWriteCookie).toHaveBeenCalledWith("ru");

    // The refreshed server tree renders in ru; nothing refreshes again.
    mockedUseLocale.mockReturnValue("ru");
    rerender(
      <SWRConfig value={{ provider: () => new Map() }}>
        <ParentLocaleProvider>
          <Consumer />
        </ParentLocaleProvider>
      </SWRConfig>,
    );
    expect(screen.getByTestId("locale").textContent).toBe("ru");
    expect(refreshMock).toHaveBeenCalledTimes(1);
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
    await waitFor(() => expect(refreshMock).toHaveBeenCalledTimes(1));
  });

  it("shows the failed persist at once but still switches the UI", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de");
    mockedFetchProfile.mockResolvedValue({ portal_locale: "de" });
    const failure = new ApiError("diag", 503, {
      code: "general.unavailable",
      instance: "req-locale",
    });
    mockedUpdateLocale.mockRejectedValue(failure);

    renderProvider();
    await waitFor(() => expect(mockedFetchProfile).toHaveBeenCalled());
    refreshMock.mockClear();
    mockedWriteCookie.mockClear();

    fireEvent.click(screen.getByText("set-ru"));

    // The cookie + soft refresh still apply the choice locally, ...
    await waitFor(() => expect(mockedWriteCookie).toHaveBeenCalledWith("ru"));
    await waitFor(() => expect(refreshMock).toHaveBeenCalledTimes(1));
    // ... and the failed account save is shown, not swallowed: the soft
    // refresh keeps the toast.
    expect(showErrorMock).toHaveBeenCalledTimes(1);
    const [shownError, options] = showErrorMock.mock.calls[0]!;
    expect(shownError).toBe(failure);
    expect(options).toMatchObject({
      object: "errorObject",
      messageSuffix: "savedOnDevice",
    });
    expect(screen.getByTestId("locale").textContent).toBe("ru");

    // Wiederholen saves the chosen language again.
    mockedUpdateLocale.mockResolvedValue({ portal_locale: "ru" });
    (options as { retry: () => void }).retry();
    await waitFor(() =>
      expect(mockedUpdateLocale).toHaveBeenLastCalledWith("ru"),
    );
    expect(mockedWriteCookie).not.toHaveBeenCalledWith("de");
  });

  it("shows a save that got no answer marked as such (#2512)", async () => {
    setSession("authenticated");
    mockedUseLocale.mockReturnValue("de");
    mockedFetchProfile.mockResolvedValue({ portal_locale: "de" });
    mockedUpdateLocale.mockRejectedValue(
      unavailableApiError(new TypeError("Failed to fetch")),
    );

    renderProvider();
    await waitFor(() => expect(mockedFetchProfile).toHaveBeenCalled());
    fireEvent.click(screen.getByText("set-ru"));

    await waitFor(() => expect(showErrorMock).toHaveBeenCalledTimes(1));
    expect(showErrorMock.mock.calls[0]![0]).toMatchObject({
      code: "general.unavailable",
      noResponse: true,
    });
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
    await waitFor(() => expect(refreshMock).toHaveBeenCalledTimes(1));
    expect(window.localStorage.getItem("phoenix_parent_unsynced_locale")).toBe(
      null,
    );
  });
});
