import { act, render, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useReopenBanner } from "~/components/active-supervisions/use-reopen-banner";
import {
  _resetPersonalStorageLeave,
  mayWritePersonalStorage,
} from "~/lib/personal-storage";
import { sidebarMemoryKey } from "~/lib/sidebar-memory";

const mocks = vi.hoisted(() => ({ useSession: vi.fn() }));

vi.mock("next-auth/react", () => ({ useSession: mocks.useSession }));

import { PersonalStorageGuard } from "./personal-storage-guard";

const LAST_ROOM_NAME = sidebarMemoryKey("sidebar-last-room-name", "1:7");

function signedIn(id: string, tenantId: number) {
  return { data: { user: { id, tenantId } }, status: "authenticated" };
}

function seedPreviousAccount() {
  localStorage.setItem("moto:personal-storage-owner", "1:7");
  sessionStorage.setItem("moto:personal-storage-owner", "1:7");
  localStorage.setItem(LAST_ROOM_NAME, "Raum Sonne");
  localStorage.setItem("messages_unread_count:schule:7", '{"count":3}');
  sessionStorage.setItem("timetable-reopenable-instance", '{"title":"AG"}');
  localStorage.setItem("sidebar-collapsed", "true");
}

const home = () => "/start";
let replace: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  _resetPersonalStorageLeave();
  replace = vi
    .spyOn(window.location, "replace")
    .mockImplementation(() => undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("PersonalStorageGuard (#5000)", () => {
  it.each(["1:7", null])(
    "clears a previous tab's undo offer when the shared owner already matches (tab owner: %s)",
    (tabOwner) => {
      localStorage.setItem("moto:personal-storage-owner", "1:8");
      localStorage.setItem("messages_unread_count:schule:8", '{"count":2}');
      if (tabOwner !== null) {
        sessionStorage.setItem("moto:personal-storage-owner", tabOwner);
      }
      sessionStorage.setItem(
        "timetable-reopenable-instance",
        JSON.stringify({
          instanceId: "9",
          title: "Fußball",
          roomId: "4",
          expiresAt: Date.now() + 5 * 60 * 1000,
        }),
      );
      mocks.useSession.mockReturnValue(signedIn("8", 1));

      const { result } = renderHook(() => useReopenBanner(), {
        wrapper: ({ children }) => (
          <>
            <PersonalStorageGuard homeHref={home} />
            {children}
          </>
        ),
      });

      expect(result.current.reopenable).toBeNull();
      expect(
        sessionStorage.getItem("timetable-reopenable-instance"),
      ).toBeNull();
      expect(sessionStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
      expect(localStorage.getItem("messages_unread_count:schule:8")).toBe(
        '{"count":2}',
      );
    },
  );

  it("keeps the entries when the same account comes back", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue(signedIn("7", 1));

    render(<PersonalStorageGuard homeHref={home} />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBe("Raum Sonne");
    expect(localStorage.getItem("messages_unread_count:schule:7")).not.toBe(
      null,
    );
  });

  it("removes the previous account's entries when another account signs in", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue(signedIn("8", 1));

    render(<PersonalStorageGuard homeHref={home} />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBeNull();
    expect(localStorage.getItem("messages_unread_count:schule:7")).toBeNull();
    expect(sessionStorage.getItem("timetable-reopenable-instance")).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
  });

  it("removes the entries when the account switches school", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue(signedIn("7", 1));
    const { rerender } = render(<PersonalStorageGuard homeHref={home} />);
    expect(localStorage.getItem(LAST_ROOM_NAME)).toBe("Raum Sonne");

    mocks.useSession.mockReturnValue(signedIn("7", 2));
    rerender(<PersonalStorageGuard homeHref={home} />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("2:7");
  });

  it("removes the entries once the session has ended", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue({ data: null, status: "unauthenticated" });

    render(<PersonalStorageGuard homeHref={home} />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBeNull();
    expect(sessionStorage.getItem("timetable-reopenable-instance")).toBeNull();
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
  });

  it("leaves everything in place while the session loads", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue({ data: null, status: "loading" });

    render(<PersonalStorageGuard homeHref={home} />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBe("Raum Sonne");
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:7");
  });

  it("never reloads a page that was loaded for the account it shows", () => {
    // The page the guard sent to the start page loads with the new account
    // already bound: nothing to leave, no endless reload.
    localStorage.setItem("moto:personal-storage-owner", "1:8");
    mocks.useSession.mockReturnValue(signedIn("8", 1));

    const { rerender } = render(<PersonalStorageGuard homeHref={home} />);
    rerender(<PersonalStorageGuard homeHref={home} />);

    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
    expect(replace).not.toHaveBeenCalled();
  });
});

// A page that showed one account never shows another (#5000): it reloads at
// the portal's start page, so neither loaded state nor URL filters of the
// previous account survive.
describe("PersonalStorageGuard leaves a page of another account", () => {
  function renderPage(id: string, tenantId = 1) {
    mocks.useSession.mockReturnValue(signedIn(id, tenantId));
    const view = render(<PersonalStorageGuard homeHref={home} />);
    return (nextId: string | null, nextTenantId = 1) => {
      mocks.useSession.mockReturnValue(
        nextId === null
          ? { data: null, status: "unauthenticated" }
          : signedIn(nextId, nextTenantId),
      );
      view.rerender(<PersonalStorageGuard homeHref={home} />);
    };
  }

  it("when the session of this tab switches to another account", () => {
    const switchTo = renderPage("7");
    sessionStorage.setItem("timetable-reopenable-instance", '{"title":"AG"}');

    switchTo("8");

    expect(replace).toHaveBeenCalledWith("/start");
    expect(sessionStorage.getItem("timetable-reopenable-instance")).toBeNull();
    // Until the page is gone, nothing writes for anybody.
    expect(mayWritePersonalStorage("1:8")).toBe(false);
  });

  it("when another tab binds the entries to another account", () => {
    renderPage("7");

    act(() => {
      localStorage.setItem("moto:personal-storage-owner", "1:8");
      window.dispatchEvent(
        new StorageEvent("storage", {
          key: "moto:personal-storage-owner",
          oldValue: "1:7",
          newValue: "1:8",
          storageArea: localStorage,
        }),
      );
    });

    expect(replace).toHaveBeenCalledWith("/start");
    expect(mayWritePersonalStorage("1:8")).toBe(false);
  });

  it.each([
    ["the account only switches school", "7", 2],
    ["the same account signs in again", "7", 1],
  ] as const)("not when %s", (_case, nextId, nextTenantId) => {
    const switchTo = renderPage("7");
    localStorage.setItem(LAST_ROOM_NAME, "Raum Sonne");

    switchTo(null);
    switchTo(nextId, nextTenantId);

    // Anker: the entries were cleared at the logout.
    expect(localStorage.getItem(LAST_ROOM_NAME)).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe(
      `${nextTenantId}:7`,
    );
    expect(replace).not.toHaveBeenCalled();
  });

  it("not when another tab binds the same account at another school or logs out", () => {
    renderPage("7");

    act(() => {
      for (const newValue of ["2:7", null]) {
        window.dispatchEvent(
          new StorageEvent("storage", {
            key: "moto:personal-storage-owner",
            newValue,
            storageArea: localStorage,
          }),
        );
      }
    });

    // Anker: the page still writes for its account.
    localStorage.setItem("moto:personal-storage-owner", "1:7");
    expect(mayWritePersonalStorage("1:7")).toBe(true);
    expect(replace).not.toHaveBeenCalled();
  });

  it("not when somebody signs in on a page loaded signed out", () => {
    mocks.useSession.mockReturnValue({ data: null, status: "unauthenticated" });
    const { rerender } = render(<PersonalStorageGuard homeHref={home} />);

    mocks.useSession.mockReturnValue(signedIn("8", 1));
    rerender(<PersonalStorageGuard homeHref={home} />);

    // Anker: the new account's entries are bound.
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
    expect(replace).not.toHaveBeenCalled();
  });
});

// Account 7 ends an activity, then the session of this tab switches to
// account 8 without a reload. The undo banner names 7's activity; it must
// not stay in front of 8 (#5000).
describe("undo banner after an account switch without a reload", () => {
  function UndoBanner() {
    const { reopenable, rememberReopenable } = useReopenBanner();
    return (
      <>
        <button
          type="button"
          onClick={() =>
            rememberReopenable(
              { instanceId: "9", title: "Fußball", roomId: "4" },
              null,
            )
          }
        >
          Beenden
        </button>
        {reopenable ? <p>{`„${reopenable.title}“ wurde beendet.`}</p> : null}
      </>
    );
  }

  function Page() {
    return (
      <>
        <PersonalStorageGuard homeHref={home} />
        <UndoBanner />
      </>
    );
  }

  it("leaves for the start page, which opens without the banner", () => {
    mocks.useSession.mockReturnValue(signedIn("7", 1));
    const page = render(<Page />);
    act(() => {
      page.getByRole("button", { name: "Beenden" }).click();
    });
    // Anker: account 7 sees its own undo.
    expect(page.getByText("„Fußball“ wurde beendet.")).toBeInTheDocument();

    mocks.useSession.mockReturnValue(signedIn("8", 1));
    page.rerender(<Page />);

    expect(replace).toHaveBeenCalledWith("/start");
    expect(sessionStorage.getItem("timetable-reopenable-instance")).toBeNull();

    // The start page loads for account 8.
    page.unmount();
    _resetPersonalStorageLeave();
    const startPage = render(<Page />);
    expect(
      startPage.getByRole("button", { name: "Beenden" }),
    ).toBeInTheDocument();
    expect(startPage.queryByText("„Fußball“ wurde beendet.")).toBeNull();
  });
});
