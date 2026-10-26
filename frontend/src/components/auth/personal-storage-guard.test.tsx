import { render } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
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
  localStorage.setItem(LAST_ROOM_NAME, "Raum Sonne");
  localStorage.setItem("messages_unread_count:schule:7", '{"count":3}');
  sessionStorage.setItem("timetable-reopenable-instance", '{"title":"AG"}');
  localStorage.setItem("sidebar-collapsed", "true");
}

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

describe("PersonalStorageGuard (#5000)", () => {
  it("keeps the entries when the same account comes back", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue(signedIn("7", 1));

    render(<PersonalStorageGuard />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBe("Raum Sonne");
    expect(localStorage.getItem("messages_unread_count:schule:7")).not.toBe(
      null,
    );
  });

  it("removes the previous account's entries when another account signs in", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue(signedIn("8", 1));

    render(<PersonalStorageGuard />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBeNull();
    expect(localStorage.getItem("messages_unread_count:schule:7")).toBeNull();
    expect(sessionStorage.getItem("timetable-reopenable-instance")).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:8");
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
  });

  it("removes the entries when the account switches school", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue(signedIn("7", 1));
    const { rerender } = render(<PersonalStorageGuard />);
    expect(localStorage.getItem(LAST_ROOM_NAME)).toBe("Raum Sonne");

    mocks.useSession.mockReturnValue(signedIn("7", 2));
    rerender(<PersonalStorageGuard />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("2:7");
  });

  it("removes the entries once the session has ended", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue({ data: null, status: "unauthenticated" });

    render(<PersonalStorageGuard />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBeNull();
    expect(localStorage.getItem("moto:personal-storage-owner")).toBeNull();
    expect(sessionStorage.getItem("timetable-reopenable-instance")).toBeNull();
    expect(localStorage.getItem("sidebar-collapsed")).toBe("true");
  });

  it("leaves everything in place while the session loads", () => {
    seedPreviousAccount();
    mocks.useSession.mockReturnValue({ data: null, status: "loading" });

    render(<PersonalStorageGuard />);

    expect(localStorage.getItem(LAST_ROOM_NAME)).toBe("Raum Sonne");
    expect(localStorage.getItem("moto:personal-storage-owner")).toBe("1:7");
  });
});
