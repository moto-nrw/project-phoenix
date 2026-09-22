import { describe, expect, it } from "vitest";
import type { Session } from "next-auth";

import { canReviewGuardianApprovals } from "./guardian-approval-access";

function sessionWith(permissions: readonly string[]): Session {
  return {
    user: { permissions, roles: ["betreuungskraft"] },
    expires: "2099-01-01",
  } as unknown as Session;
}

describe("canReviewGuardianApprovals", () => {
  it("needs the read and the decide permission together", () => {
    expect(
      canReviewGuardianApprovals(sessionWith(["users:manage", "users:update"])),
    ).toBe(true);
  });

  it("keeps the queue hidden with the read permission alone", () => {
    // users:manage liest die Liste, entscheiden lässt das Backend nur mit
    // users:update: sonst stünde dort eine Warteschlange, in der jedes
    // Annehmen mit 403 endet.
    expect(canReviewGuardianApprovals(sessionWith(["users:manage"]))).toBe(
      false,
    );
  });

  it("keeps the queue hidden with the decide permission alone", () => {
    expect(canReviewGuardianApprovals(sessionWith(["users:update"]))).toBe(
      false,
    );
  });

  it("opens for the admin scope", () => {
    expect(canReviewGuardianApprovals(sessionWith(["admin:*"]))).toBe(true);
  });

  it("stays closed without a session", () => {
    expect(canReviewGuardianApprovals(null)).toBe(false);
  });
});
