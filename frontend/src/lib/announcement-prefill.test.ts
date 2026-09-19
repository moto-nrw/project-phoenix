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
    expect(
      stashAnnouncementStudents([
        { id: "7", name: "Mia Arslan" },
        { id: "8", name: "Ben Yilmaz" },
      ]),
    ).toBe(true);

    expect(takeAnnouncementStudents()).toEqual([
      { id: "7", name: "Mia Arslan" },
      { id: "8", name: "Ben Yilmaz" },
    ]);
    // Reloading the composer must not address the same families again.
    expect(takeAnnouncementStudents()).toEqual([]);
  });

  it("returns nothing when no selection was stashed", () => {
    expect(takeAnnouncementStudents()).toEqual([]);
  });

  it("drops entries that are not a student", () => {
    window.sessionStorage.setItem(
      "moto:announcement-prefill-students",
      JSON.stringify([
        { id: "7", name: "Mia Arslan" },
        { id: "", name: "Ohne Nummer" },
        { id: 9, name: "Zahl statt Text" },
        "kein Objekt",
        null,
      ]),
    );
    expect(takeAnnouncementStudents()).toEqual([
      { id: "7", name: "Mia Arslan" },
    ]);
  });

  it("survives unreadable content and clears it", () => {
    window.sessionStorage.setItem(
      "moto:announcement-prefill-students",
      "{kaputt",
    );
    expect(takeAnnouncementStudents()).toEqual([]);
    expect(
      window.sessionStorage.getItem("moto:announcement-prefill-students"),
    ).toBeNull();
  });

  it("reports a blocked storage instead of throwing", () => {
    // The test setup installs its own storage object, so the spy goes on that
    // object and not on Storage.prototype.
    vi.spyOn(window.sessionStorage, "setItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(stashAnnouncementStudents([{ id: "7", name: "Mia" }])).toBe(false);

    vi.spyOn(window.sessionStorage, "getItem").mockImplementation(() => {
      throw new Error("blocked");
    });
    expect(takeAnnouncementStudents()).toEqual([]);
  });
});
