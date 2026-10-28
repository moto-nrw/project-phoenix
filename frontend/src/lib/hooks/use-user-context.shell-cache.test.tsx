import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  ShellCacheSeeder,
  shellOwnedEntries,
  shellSwrConfig,
} from "~/app/[tenant]/providers";
import type { ShellBootstrap } from "~/lib/shell-seed";
import { mutate, SWRConfig } from "~/lib/swr";
import type { UserContextResponse } from "~/lib/user-context-types";
import { USER_CONTEXT_SWR_KEY, useUserContext } from "./use-user-context";

// Der Nutzerkontext ist unveränderliches SWR mit Server-Seed. Ohne Eintrag im
// Cache beantwortete die erste Invalidierung (Einstellungen, SSE-Ereignis)
// den Abruf mit dem Seed statt mit Serverdaten (Review von #5116).

// Echtes SWR: gezählt wird, was beim Server ankommt.
vi.unmock("swr");

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "7" }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
// SWRs gemeinsamer Cache wie in der App; jeder Test nimmt eine eigene Schule.
const tenantState = vi.hoisted(() => ({ slug: "schule-0" }));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => tenantState.slug,
}));

const SEEDED: UserContextResponse = {
  educationalGroups: [],
  supervisedGroups: [],
  currentStaff: null,
  educationalGroupIds: ["1"],
  educationalGroupRoomNames: [],
  supervisedRoomNames: [],
  incomplete: false,
  unavailableSections: [],
  schoolSwitches: null,
};

const fetchMock = vi.fn<typeof fetch>();

function contextRequests(): number {
  return fetchMock.mock.calls.filter(
    ([url]) => String(url) === "/api/user-context",
  ).length;
}

function Navigation() {
  const { userContext } = useUserContext();
  return (
    <span data-testid="groups">
      {userContext?.educationalGroupIds.join(",") ?? "-"}
    </span>
  );
}

function Shell() {
  // Wie TenantProviders: Seed in fallback und cacheData, die Einträge der
  // Hülle vor dem ersten Leser im Cache.
  const shell: ShellBootstrap = {
    accountId: "7",
    counts: {},
    userContext: SEEDED,
  };
  return (
    <SWRConfig value={shellSwrConfig(shell, tenantState.slug)}>
      <ShellCacheSeeder entries={shellOwnedEntries(shell, tenantState.slug)} />
      <Navigation />
    </SWRConfig>
  );
}

let nextSchool = 0;
beforeEach(() => {
  nextSchool += 1;
  tenantState.slug = `schule-${nextSchool}`;
  fetchMock.mockReset();
  fetchMock.mockImplementation(async () =>
    Response.json({ data: { ...SEEDED, educationalGroupIds: ["1", "2"] } }),
  );
  vi.stubGlobal("fetch", fetchMock);
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("useUserContext with the shell seed", () => {
  it("shows the seed without asking the server", async () => {
    render(<Shell />);

    expect(screen.getByTestId("groups")).toHaveTextContent("1");
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(contextRequests()).toBe(0);
  });

  it("asks the server on the first invalidation after the seed", async () => {
    render(<Shell />);
    expect(screen.getByTestId("groups")).toHaveTextContent("1");

    // Wie useTenantMutate(USER_CONTEXT_SWR_KEY) nach einer Einstellung.
    await act(async () => {
      await mutate(`${tenantState.slug}:${USER_CONTEXT_SWR_KEY}`);
    });

    await waitFor(() =>
      expect(screen.getByTestId("groups")).toHaveTextContent("1,2"),
    );
    expect(contextRequests()).toBe(1);
  });
});
