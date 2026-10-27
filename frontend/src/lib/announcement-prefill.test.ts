import { afterEach, describe, expect, it, vi } from "vitest";
import {
  announcementPrefillPath,
  clearAnnouncementStudents,
  stashAnnouncementStudents,
  takeAnnouncementStudents,
} from "./announcement-prefill";

afterEach(() => {
  vi.restoreAllMocks();
  clearAnnouncementStudents();
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

  it("rejects a selection from another tenant", () => {
    const token = stashAnnouncementStudents("andere-schule", [
      { id: "7", name: "Mia Arslan" },
    ]);
    expect(token).not.toBeNull();
    expect(takeAnnouncementStudents("testschule", token!)).toEqual([]);
  });

  it("keeps the names out of the browser storage (#5000)", () => {
    const token = stashAnnouncementStudents("testschule", [
      { id: "7", name: "Mia Arslan" },
    ]);
    expect(token).not.toBeNull();

    expect(window.sessionStorage.length).toBe(0);
    expect(window.localStorage.length).toBe(0);
    expect(takeAnnouncementStudents("testschule", token!)).toEqual([
      { id: "7", name: "Mia Arslan" },
    ]);
  });

  it("drops an unclaimed selection at logout or account switch", () => {
    const token = stashAnnouncementStudents("testschule", [
      { id: "7", name: "Mia Arslan" },
    ]);
    expect(token).not.toBeNull();

    clearAnnouncementStudents();

    expect(takeAnnouncementStudents("testschule", token!)).toEqual([]);
  });

  it("keeps only the latest selection", () => {
    const first = stashAnnouncementStudents("testschule", [
      { id: "7", name: "Mia Arslan" },
    ]);
    const second = stashAnnouncementStudents("testschule", [
      { id: "8", name: "Ben Yilmaz" },
    ]);
    expect(first).not.toBeNull();
    expect(second).not.toBeNull();

    expect(takeAnnouncementStudents("testschule", first!)).toEqual([]);
    expect(takeAnnouncementStudents("testschule", second!)).toEqual([
      { id: "8", name: "Ben Yilmaz" },
    ]);
  });

  it("hands nothing over without a random token", () => {
    vi.spyOn(globalThis.crypto, "randomUUID").mockReturnValue(
      undefined as unknown as ReturnType<Crypto["randomUUID"]>,
    );
    expect(
      stashAnnouncementStudents("testschule", [{ id: "7", name: "Mia" }]),
    ).toBeNull();
  });
});

describe("announcementPrefillPath", () => {
  it("points at the composer with the token and the chosen tab", () => {
    expect(announcementPrefillPath("abc", "elternbriefe")).toBe(
      "/parent-announcements?art=elternbriefe&neu=kinder&vorbelegung=abc",
    );
    expect(announcementPrefillPath("abc")).toBe(
      "/parent-announcements?neu=kinder&vorbelegung=abc",
    );
  });

  it("falls back to the list when nothing could be stashed", () => {
    expect(announcementPrefillPath(null)).toBe("/parent-announcements");
    expect(announcementPrefillPath(null, "elternbriefe")).toBe(
      "/parent-announcements?art=elternbriefe",
    );
  });
});
