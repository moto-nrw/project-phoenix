import type { Session } from "next-auth";
import { describe, expect, it } from "vitest";

import {
  canReviewChangeRequests,
  canReviewStudentDataRequests,
} from "./change-request-access";

function session(roles: string[], permissions: string[]): Session {
  return {
    user: { id: "7", roles, permissions },
    expires: "2099-01-01T00:00:00.000Z",
  } as unknown as Session;
}

describe("change request access", () => {
  it("lets an admin review everything", () => {
    const admin = session(["admin"], ["admin:*"]);
    expect(canReviewStudentDataRequests(admin)).toBe(true);
    expect(canReviewChangeRequests(admin)).toBe(true);
  });

  it("lets users:update review every queue", () => {
    const staff = session(["user"], ["users:update"]);
    expect(canReviewStudentDataRequests(staff)).toBe(true);
    expect(canReviewChangeRequests(staff)).toBe(true);
  });

  it("opens the page for users:absence + users:read but not the Stammdaten queues", () => {
    const staff = session(["user"], ["users:read", "users:absence"]);
    expect(canReviewChangeRequests(staff)).toBe(true);
    expect(canReviewStudentDataRequests(staff)).toBe(false);
  });

  // users:absence is a write scope on the children someone may already see; the
  // backend absence gate refuses it without users:read, so the page, its
  // Sidebar-Eintrag and the Zähler-Badge must refuse it too — otherwise the
  // person lands on a queue that answers empty forever.
  it("keeps users:absence without users:read out", () => {
    const staff = session(["user"], ["users:absence"]);
    expect(canReviewChangeRequests(staff)).toBe(false);
    expect(canReviewStudentDataRequests(staff)).toBe(false);
  });

  it("keeps a staffer without either permission out", () => {
    const staff = session(["user"], ["users:read"]);
    expect(canReviewChangeRequests(staff)).toBe(false);
    expect(canReviewStudentDataRequests(staff)).toBe(false);
  });

  it("treats a missing session as no access", () => {
    expect(canReviewChangeRequests(null)).toBe(false);
    expect(canReviewStudentDataRequests(null)).toBe(false);
  });
});
