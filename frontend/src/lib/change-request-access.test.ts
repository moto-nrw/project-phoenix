import type { Session } from "next-auth";
import { describe, expect, it } from "vitest";

import {
  canOpenParentRequestsTab,
  canOpenRequestsPage,
  canReviewChangeRequests,
  canReviewEnrollmentChangeRequests,
  canReviewStudentDataRequests,
  resolveChangeRequestAccess,
} from "./change-request-access";

function session(roles: string[], permissions: string[]): Session {
  return {
    user: { id: "7", roles, permissions },
    expires: "2099-01-01T00:00:00.000Z",
  } as unknown as Session;
}

describe("change request access", () => {
  it("hält Betreuer ohne effektive Elternanfragen-Freigabe aus dem Modul", () => {
    const staff = session(["user"], ["users:read", "users:update"]);

    expect(resolveChangeRequestAccess(staff, "none")).toMatchObject({
      canReviewParentRequests: false,
      canOpenParentRequestsTab: false,
      canOpenRequestsPage: false,
    });
  });

  it("öffnet das Modul für eine serverseitig bestätigte Gruppenleitung", () => {
    const groupLeader = session(["user"], ["users:read", "users:update"]);

    expect(
      resolveChangeRequestAccess(groupLeader, "group_leader"),
    ).toMatchObject({
      canReviewParentRequests: true,
      canReviewStudentDataRequests: true,
      canOpenParentRequestsTab: true,
      canOpenRequestsPage: true,
    });
  });

  it.each([
    {
      permission: "config:manage",
      capability: "canReviewEnrollmentChangeRequests" as const,
    },
    {
      permission: "users:delete",
      capability: "canReviewCareWithdrawals" as const,
    },
    {
      permission: "vacation:approve",
      capability: "canReviewStaffAbsenceRequests" as const,
    },
  ])(
    "öffnet mit $permission unabhängig von der Elternanfragen-Freigabe",
    ({ permission, capability }) => {
      const staff = session(["user"], [permission]);
      const access = resolveChangeRequestAccess(staff, "none");

      expect(access.canReviewParentRequests).toBe(false);
      expect(access[capability]).toBe(true);
      expect(access.canOpenRequestsPage).toBe(true);
    },
  );

  it("lässt OGS-Admins schulweite Elternanfragen sehen", () => {
    const admin = session(["admin"], ["admin:*"]);

    expect(resolveChangeRequestAccess(admin, "admin")).toMatchObject({
      canReviewParentRequests: true,
      canReviewStudentDataRequests: true,
      canOpenRequestsPage: true,
    });
  });

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

  // Anmeldungsänderungen hängen an config:manage — der Eltern-Reiter öffnet
  // damit, aber nur mit dieser einen Art (#2435).
  it("öffnet den Eltern-Reiter auch für config:manage allein", () => {
    const configManager = session(["user"], ["config:manage"]);
    expect(canReviewEnrollmentChangeRequests(configManager)).toBe(true);
    expect(canOpenParentRequestsTab(configManager)).toBe(true);
    expect(canOpenRequestsPage(configManager)).toBe(true);
    expect(canReviewChangeRequests(configManager)).toBe(false);
    expect(canReviewStudentDataRequests(configManager)).toBe(false);
  });

  it("hält Anmeldungsänderungen von users:update fern", () => {
    const staff = session(["user"], ["users:update"]);
    expect(canReviewEnrollmentChangeRequests(staff)).toBe(false);
    expect(canOpenParentRequestsTab(staff)).toBe(true);
  });

  it("treats a missing session as no access", () => {
    expect(canReviewChangeRequests(null)).toBe(false);
    expect(canReviewStudentDataRequests(null)).toBe(false);
    expect(canReviewEnrollmentChangeRequests(null)).toBe(false);
    expect(canOpenParentRequestsTab(null)).toBe(false);
  });
});
