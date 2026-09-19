import { afterEach, describe, expect, it, vi } from "vitest";
import {
  stashAnnouncementStudents,
  takeAnnouncementStudents,
} from "./announcement-prefill";

afterEach(() => {
  vi.restoreAllMocks();
  window.sessionStorage.clear();
});

describe("announcement prefill handoff", () => {
  it("hands the students over exactly once", () => {
    const token = stashAnnouncementStudents("testschule", [
      { id: "7", name: "Mia Arslan" },
      { id: "8", name: "Ben Yilmaz" },
    ]);
    expect(token).not.toBeNull();

    expect(takeAnnouncementStudents("testschule", token!)).toEqual([
      { id: "7", name: "Mia Arslan" },
      { id: "8", name: "Ben Yilmaz" },
    ]);
    // Reloading the composer must not address the same families again.
    expect(takeAnnouncementStudents("testschule", token!)).toEqual([]);
  });

  it("returns nothing when no selection was stashed", () => {
    expect(takeAnnouncementStudents("testschule", "missing")).toEqual([]);
  });

  it("drops entries that are not a student", () => {
    const token = "test-token";
    window.sessionStorage.setItem(
      `moto:announcement-prefill:${token}`,
      JSON.stringify(["not a valid payload"]),
    );
    expect(takeAnnouncementStudents("testschule", token)).toEqual([]);
  });

  it("rejects a selection from another tenant", () => {
    const token = stashAnnouncementStudents("andere-schule", [
      { id: "7", name: "Mia Arslan" },
    ]);
    expect(token).not.toBeNull();
    expect(takeAnnouncementStudents("testschule", token!)).toEqual([]);
  });

  it("drops malformed student entries", () => {
    const token = "test-token";
    window.sessionStorage.setItem(
      `moto:announcement-prefill:${token}`,
      JSON.stringify({
        tenantSlug: "testschule",
        students: [
          { id: "7", name: "Mia Arslan" },
          { id: "", name: "Ohne Nummer" },
          { id: 9, name: "Zahl statt Text" },
          "kein Objekt",
          null,
        ],
      }),
    );
    expect(takeAnnouncementStudents("testschule", token)).toEqual([
      { id: "7", name: "Mia Arslan" },
    ]);
  });

  it("survives unreadable content and clears it", () => {
    const token = "test-token";
    window.sessionStorage.setItem(
      `moto:announcement-prefill:${token}`,
      "{kaputt",
    );
    expect(takeAnnouncementStudents("testschule", token)).toEqual([]);
    expect(
      window.sessionStorage.getItem(`moto:announcement-prefill:${token}`),
    ).toBeNull();
  });

  it("reports a blocked storage instead of throwing", () => {
    // The test setup installs its own storage object, so the spy goes on that
    // object and not on Storage.prototype.
    vi.spyOn(window.sessionStorage, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(
      stashAnnouncementStudents("testschule", [{ id: "7", name: "Mia" }]),
    ).toBeNull();

    vi.spyOn(window.sessionStorage, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(takeAnnouncementStudents("testschule", "test-token")).toEqual([]);
  });
});
