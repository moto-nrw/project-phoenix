import { renderHook } from "@testing-library/react";
import { expect, it, vi } from "vitest";

const { session, useSSE } = vi.hoisted(() => ({
  session: {
    user: { id: "7", tenantId: 1, roles: ["user"] },
  },
  useSSE: vi.fn(() => ({
    status: "idle",
    isConnected: false,
    error: null,
    reconnectAttempts: 0,
  })),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: session, status: "authenticated" }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => ({ tenant: { tenantId: 1 } }),
  // The family invalidation scopes its keys to the school of the URL.
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/hooks/use-sse", () => ({ useSSE }));

import { useGlobalSSE } from "./use-global-sse";

it("suspends the school stream while the provider still names another school", () => {
  const view = renderHook(() => useGlobalSSE());
  expect(useSSE).toHaveBeenLastCalledWith(
    "/api/sse/events",
    expect.objectContaining({ enabled: true }),
  );

  session.user = { ...session.user, tenantId: 2 };
  view.rerender();
  expect(useSSE).toHaveBeenLastCalledWith(
    "/api/sse/events",
    expect.objectContaining({ enabled: false }),
  );

  // A fresh provider answer for the same wrong school must not restart the
  // refused stream and trigger another session lookup during explicit logout.
  session.user = { ...session.user };
  view.rerender();
  expect(useSSE).toHaveBeenLastCalledWith(
    "/api/sse/events",
    expect.objectContaining({ enabled: false }),
  );

  session.user = { ...session.user, tenantId: 1 };
  view.rerender();
  expect(useSSE).toHaveBeenLastCalledWith(
    "/api/sse/events",
    expect.objectContaining({ enabled: true }),
  );
});
