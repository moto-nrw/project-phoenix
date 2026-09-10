import { describe, expect, it, vi } from "vitest";
import { renderHook } from "@testing-library/react";
import type { Session } from "next-auth";

const session = vi.hoisted(() => ({ current: null as Session | null }));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: session.current }),
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

import { useDayPlanHref, useDayPlanLabel } from "./use-day-plan-href";

function withRoles(roles: string[], permissions: string[] = []): Session {
  return {
    user: { id: "1", roles, permissions },
    expires: "2099-12-31",
  } as unknown as Session;
}

/**
 * Die Seitenleiste blendet den Tagesplan für reine Adminkonten aus
 * (`hideForAdmin`) — sie haben den Betreuungsplan im Planungsbereich. Ein
 * Weiterlink der Startseite darf nicht auf eine Seite zeigen, die diese
 * Person in ihrer Navigation gar nicht hat.
 */
describe("useDayPlanHref (#2180)", () => {
  it("führt eine Betreuungskraft in den Tagesplan", () => {
    session.current = withRoles(["user"]);

    expect(renderHook(() => useDayPlanHref()).result.current).toBe(
      "/test-tenant/tagesplan",
    );
    expect(renderHook(() => useDayPlanLabel()).result.current).toBe(
      "Zum Tagesplan",
    );
  });

  it("führt ein reines Adminkonto in den Betreuungsplan", () => {
    session.current = withRoles(["admin"], ["admin:*"]);

    expect(renderHook(() => useDayPlanHref()).result.current).toBe(
      "/test-tenant/betreuungsplan",
    );
    expect(renderHook(() => useDayPlanLabel()).result.current).toBe(
      "Zum Betreuungsplan",
    );
  });

  // Wer beides ist, sieht den Tagesplan auch in der Seitenleiste.
  it("führt eine Leitung, die auch betreut, in den Tagesplan", () => {
    session.current = withRoles(["admin", "user"], ["admin:*"]);

    expect(renderHook(() => useDayPlanHref()).result.current).toBe(
      "/test-tenant/tagesplan",
    );
  });
});
