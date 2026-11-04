import * as apiError from "~/lib/api-error";
import type { Session } from "next-auth";
import { afterEach, describe, expect, it, vi } from "vitest";
import { tenantPrefetchResources } from "./tenant-prefetch-resources";
import { studentKeys } from "~/lib/swr/portal-keys";
import { groups } from "~/lib/swr/groups";
import { dashboardKeys } from "~/lib/swr/shell-keys";
import { berlinTodayISO } from "~/lib/date-helpers";

function context(permissions: string[], scope = "") {
  const session: Session = {
    expires: "2099-01-01",
    user: { id: "42", tenantId: 7, permissions, scope },
  };
  return {
    session,
    slug: "school-a",
    groupId: "7",
    groups: ["7"],
    hasSupervision: true,
    detailedMode: true,
    photoState: "photos-disabled" as const,
  };
}

afterEach(() => vi.restoreAllMocks());

describe("tenant destination resources", () => {
  it.each(["", "tenant"])(
    "uses the destination's exact unfiltered search key for scope %j",
    async (scope) => {
      const resources = await tenantPrefetchResources(
        "/school-a/students/search",
        context(["users:read", "groups:read", "rooms:read"], scope),
      );
      expect(resources.map((resource) => resource.key)).toContain(
        studentKeys.search({
          groupIds: [],
          search: "",
          schoolClasses: [],
          roomId: "",
          dayStatus: "all",
          date: berlinTodayISO(),
          isToday: true,
          photoState: "photos-disabled",
          bus: "all",
          photoConsent: "all",
          companions: false,
          years: [],
          pickupStatus: "all",
        }),
      );
      expect(resources).toHaveLength(4);
    },
  );
  it("does not guess a main-list key for filtered search", async () => {
    const resources = await tenantPrefetchResources(
      "/students/search?group_id=7",
      context(["students:read"]),
    );
    expect(resources.map((resource) => resource.key)).toEqual([
      studentKeys.schoolClasses(),
    ]);
  });
  it("preloads only a shell-confirmed OGS group", async () => {
    const allowed = context(["users:read"]);
    expect(
      (await tenantPrefetchResources("/ogs-groups?group=7", allowed)).map(
        (resource) => resource.key,
      ),
    ).toEqual([groups.live("7")]);
    expect(
      await tenantPrefetchResources("/ogs-groups?group=8", allowed),
    ).toEqual([]);
    expect(
      (
        await tenantPrefetchResources("/ogs-groups", {
          ...allowed,
          groupId: null,
        })
      ).map((resource) => resource.key),
    ).toEqual([groups.live(null)]);
    expect(
      await tenantPrefetchResources("/ogs-groups", {
        ...allowed,
        groupId: null,
        groups: [],
      }),
    ).toEqual([]);
  });
  it("shares the exact complete child-detail key, never an inferred permission flag", async () => {
    expect(
      (
        await tenantPrefetchResources(
          "/students/123?from=/students/search",
          context(["students:read"]),
        )
      ).map((resource) => resource.key),
    ).toEqual([studentKeys.detail("123")]);
    expect(await tenantPrefetchResources("/students/123", context([]))).toEqual(
      [],
    );
  });
  it("shares the supervision aggregate key only for confirmed supervision", async () => {
    const allowed = context(["groups:read"]);
    expect(
      (await tenantPrefetchResources("/active-supervisions", allowed)).map(
        (resource) => resource.key,
      ),
    ).toEqual([dashboardKeys.supervision("42")]);
    expect(
      await tenantPrefetchResources("/active-supervisions", {
        ...allowed,
        hasSupervision: false,
      }),
    ).toEqual([]);
    expect(
      (
        await tenantPrefetchResources("/active-supervisions?session=9", allowed)
      ).map((resource) => resource.key),
    ).toEqual([dashboardKeys.supervision("42")]);
    expect(
      (
        await tenantPrefetchResources("/active-supervisions?room=5", allowed)
      ).map((resource) => resource.key),
    ).toEqual([dashboardKeys.supervision("42")]);
    expect(
      await tenantPrefetchResources("/active-supervisions", {
        ...allowed,
        detailedMode: false,
      }),
    ).toEqual([]);
  });
  it("keeps the wave's other measured links outside the first destination catalog", async () => {
    const allowed = context(["users:read", "users:update"]);
    expect(
      (await tenantPrefetchResources("/students/123", allowed)).map(
        (resource) => resource.key,
      ),
    ).toEqual([studentKeys.detail("123")]);
    expect(await tenantPrefetchResources("/home", allowed)).toEqual([]);
    expect(await tenantPrefetchResources("/anfragen", allowed)).toEqual([]);
  });
  it.each([
    "/traeger/access-log",
    "/parents/holiday-preallocation",
    "https://example.invalid/home",
    "//example.invalid/home",
    "/another-school/students/search",
  ])("does not speculate about %s", async (href) => {
    expect(
      await tenantPrefetchResources(
        href,
        context(["users:read", "groups:read"]),
      ),
    ).toEqual([]);
  });
  it.each(["parent", "school", "org", "platform", "caterer"])(
    "rejects the %s portal even with matching permission claims",
    async (scope) => {
      expect(
        await tenantPrefetchResources(
          "/students/search",
          context(["users:read"], scope),
        ),
      ).toEqual([]);
    },
  );
});

it("uses the cold destination's exact entry fetcher for a parameterized supervision link", async () => {
  const transport = vi
    .spyOn(apiError, "transportFetch")
    .mockResolvedValueOnce(
      new Response(JSON.stringify({ data: { selectedGroupId: "first" } })),
    );
  const resources = await tenantPrefetchResources(
    "/active-supervisions?session=9",
    context(["groups:read"]),
  );
  expect(resources).toHaveLength(1);
  await expect(resources[0]!.fetcher()).resolves.toEqual({
    selectedGroupId: "first",
  });
  expect(transport).toHaveBeenCalledExactlyOnceWith(
    "/api/active-supervision-dashboard",
    {
      headers: { "Content-Type": "application/json" },
    },
  );
});
