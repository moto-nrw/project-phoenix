import type { ReactElement } from "react";
import { fireEvent, render as renderUi, screen } from "@testing-library/react";
import type { TenantSessionSnapshot } from "~/lib/tenant-session-snapshot.server";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { authMock, notFoundMock, readTenantSessionSnapshotMock } = vi.hoisted(
  () => ({
    authMock: vi.fn(async () => null),
    readTenantSessionSnapshotMock: vi.fn<
      () => Promise<TenantSessionSnapshot | null>
    >(async () => null),
    notFoundMock: vi.fn((): never => {
      throw new Error("NEXT_HTTP_ERROR_FALLBACK;404");
    }),
  }),
);

vi.mock("next/navigation", () => ({
  notFound: notFoundMock,
  redirect: vi.fn(),
}));

const { headersMock } = vi.hoisted(() => ({
  headersMock: vi.fn(
    async () => new Headers({ host: "school-a.localhost:3000" }),
  ),
}));
vi.mock("next/headers", () => ({
  headers: headersMock,
}));

// Stands in for Next's data cache: entries are keyed by keyParts and
// arguments only, as in unstable_cache, and a throwing callback stores
// nothing.
const { cacheStore, unstableCacheMock } = vi.hoisted(() => {
  const cacheStore = new Map<string, unknown>();
  const unstableCacheMock = vi.fn(
    (
      cb: (...args: unknown[]) => Promise<unknown>,
      keyParts: string[],
      _options: { revalidate: number; tags: string[] },
    ) =>
      async (...args: unknown[]) => {
        const key = JSON.stringify([keyParts, args]);
        if (cacheStore.has(key)) return cacheStore.get(key);
        const result = await cb(...args);
        cacheStore.set(key, result);
        return result;
      },
  );
  return { cacheStore, unstableCacheMock };
});
vi.mock("next/cache", () => ({
  unstable_cache: unstableCacheMock,
}));

vi.mock("~/server/auth", () => ({
  auth: authMock,
}));

vi.mock("~/lib/shell-bootstrap.server", () => ({
  loadShellBootstrap: vi.fn(),
}));

vi.mock("~/lib/tenant-session-snapshot.server", () => ({
  readTenantSessionSnapshot: readTenantSessionSnapshotMock,
}));

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NODE_ENV: "development",
    TENANT_DOMAIN: "localhost",
  },
}));

const { bareTenantHost, default: TenantLayout } = await import("./layout");
const { setTestClock, TEST_CLOCK_INSTANT } = await import("~/test/clock");
const navigation = await import("next/navigation");

beforeEach(() => {
  authMock.mockClear();
  readTenantSessionSnapshotMock.mockReset();
  readTenantSessionSnapshotMock.mockResolvedValue(null);
  notFoundMock.mockClear();
  headersMock.mockClear();
  headersMock.mockImplementation(
    async () => new Headers({ host: "school-a.localhost:3000" }),
  );
  cacheStore.clear();
  unstableCacheMock.mockClear();
  vi.restoreAllMocks();
});

function tenantResponse() {
  return new Response(
    JSON.stringify({
      status: "success",
      data: {
        tenant_id: 1,
        slug: "school-a",
        name: "School A",
        subdomain: "school-a",
        organization_id: 2,
        organization_name: "Organization",
        settings: {},
        grade_level_max: 4,
      },
    }),
    { status: 200 },
  );
}

function visitorHeaders(ip: string) {
  return new Headers({
    host: "school-a.localhost:3000",
    "user-agent": "Browser/1",
    "x-forwarded-for": ip,
  });
}

describe("bareTenantHost", () => {
  it("strips an invalid local tenant subdomain and keeps the port", () => {
    expect(bareTenantHost("asld.localhost:3000")).toBe("localhost:3000");
  });

  it("keeps the bare tenant domain unchanged", () => {
    expect(bareTenantHost("localhost:3000")).toBe("localhost:3000");
  });
});

describe("TenantLayout", () => {
  it.each(["wp-trackback.php", "__status", "School-a", "a".repeat(64)])(
    "returns 404 for invalid tenant path %s without resolving it",
    async (tenant) => {
      const fetchSpy = vi.spyOn(globalThis, "fetch");

      await expect(
        TenantLayout({ children: null, params: Promise.resolve({ tenant }) }),
      ).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");

      expect(notFoundMock).toHaveBeenCalledOnce();
      expect(fetchSpy).not.toHaveBeenCalled();
    },
  );

  it("returns 404 for a reserved tenant path without resolving it", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch");

    await expect(
      TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "operator" }),
      }),
    ).rejects.toThrow("NEXT_HTTP_ERROR_FALLBACK;404");

    expect(notFoundMock).toHaveBeenCalledOnce();
    expect(fetchSpy).not.toHaveBeenCalled();
  });

  // #5130: path mode on the bare domain shares the session cookie with
  // every school but names none in its host. The page moves to the
  // school's subdomain with its path and query.
  describe("path mode on the bare domain", () => {
    const redirect = vi.mocked(navigation.redirect);

    function requestTo(host: string, originalPath?: string) {
      headersMock.mockImplementation(
        async () =>
          new Headers({
            host: "frontend:3000",
            "x-moto-original-host": host,
            "x-moto-original-proto": "https",
            ...(originalPath && { "x-moto-original-path": originalPath }),
          }),
      );
      vi.spyOn(globalThis, "fetch").mockImplementation(async () =>
        tenantResponse(),
      );
    }

    beforeEach(() => redirect.mockClear());

    it.each([
      [
        "an enrollment link",
        "/school-a/anmeldung/phase-1?late_invite=x",
        "/anmeldung/phase-1?late_invite=x",
      ],
      ["a display link", "/school-a/display", "/display"],
      ["the start page with a query", "/school-a?from=letter", "/?from=letter"],
      ["the start page", "/school-a", "/"],
      ["a request without the proxy's path", undefined, "/"],
    ])(
      "sends %s to the school's subdomain",
      async (_case, originalPath, target) => {
        requestTo("localhost:3000", originalPath);

        await TenantLayout({
          children: null,
          params: Promise.resolve({ tenant: "school-a" }),
        });

        expect(redirect).toHaveBeenCalledWith(
          `https://school-a.localhost:3000${target}`,
        );
      },
    );

    it.each(["school-a.localhost:3000", "frontend-preview.example.org"])(
      "keeps %s, which is no bare tenant domain",
      async (host) => {
        requestTo(host, "/students");

        await expect(
          TenantLayout({
            children: null,
            params: Promise.resolve({ tenant: "school-a" }),
          }),
        ).resolves.toBeTruthy();

        expect(redirect).not.toHaveBeenCalled();
      },
    );
  });

  it("still resolves a syntactically valid tenant path", async () => {
    const fetchSpy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "success",
          data: {
            tenant_id: 1,
            slug: "school-a",
            name: "School A",
            subdomain: "school-a",
            organization_id: 2,
            organization_name: "Organization",
            settings: {},
            grade_level_max: 4,
          },
        }),
        { status: 200 },
      ),
    );

    await expect(
      TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "school-a" }),
      }),
    ).resolves.toBeTruthy();

    expect(fetchSpy).toHaveBeenCalledOnce();
    expect(fetchSpy).toHaveBeenCalledWith(
      "http://server:8080/auth/tenant/resolve?slug=school-a",
      { headers: {}, cache: "no-store" },
    );
    // The lookup stays cached for five minutes under the slug and its
    // revalidation tag, which the settings routes purge.
    expect(unstableCacheMock).toHaveBeenCalledWith(
      expect.any(Function),
      ["tenant-resolve", "school-a"],
      { revalidate: 300, tags: ["tenant-school-a"] },
    );
  });

  it("charges a cache miss to the visitor's IP", async () => {
    // Without the header the backend rate-limits the frontend container,
    // and all visitors share one bucket.
    headersMock.mockImplementation(async () => visitorHeaders("203.0.113.5"));
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(tenantResponse());

    await TenantLayout({
      children: null,
      params: Promise.resolve({ tenant: "school-a" }),
    });

    expect(fetchSpy).toHaveBeenCalledExactlyOnceWith(
      "http://server:8080/auth/tenant/resolve?slug=school-a",
      {
        headers: {
          "X-Forwarded-For": "203.0.113.5",
          "User-Agent": "Browser/1",
        },
        cache: "no-store",
      },
    );
  });

  it("shares the cached lookup across visitors instead of keying it by IP", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => tenantResponse());
    const render = async (ip: string) => {
      headersMock.mockImplementation(async () => visitorHeaders(ip));
      return TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "school-a" }),
      });
    };

    await expect(render("203.0.113.5")).resolves.toBeTruthy();
    await expect(render("198.51.100.9")).resolves.toBeTruthy();

    expect(fetchSpy).toHaveBeenCalledOnce();
    for (const [, keyParts] of unstableCacheMock.mock.calls) {
      expect(keyParts).toEqual(["tenant-resolve", "school-a"]);
    }
  });

  it("asks the backend about an unknown tenant only once per minute", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(async () => new Response(null, { status: 404 }));
    const renderUnknown = () =>
      TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "scanner-probe" }),
      });

    await expect(renderUnknown()).rejects.toThrow(
      "Tenant redirect did not complete",
    );
    await expect(renderUnknown()).rejects.toThrow(
      "Tenant redirect did not complete",
    );
    expect(fetchSpy).toHaveBeenCalledOnce();

    setTestClock(new Date(TEST_CLOCK_INSTANT.getTime() + 61_000));
    await expect(renderUnknown()).rejects.toThrow(
      "Tenant redirect did not complete",
    );
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });

  it("shows a waiting page with a full reload when the lookup is rate limited", async () => {
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(null, { status: 429 }));
    const reloadSpy = vi
      .spyOn(globalThis.location, "reload")
      .mockImplementation(() => undefined);

    const page = await TenantLayout({
      children: <p>Schulinhalt</p>,
      params: Promise.resolve({ tenant: "school-busy" }),
    });
    renderUi(page);

    // Anker: die Warteseite ist gerendert, nicht der Schulinhalt.
    expect(
      screen.getByRole("heading", { name: "Bitte kurz warten" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die Seite lädt gerade nicht. Das liegt nicht an Ihnen. Bitte warten Sie eine Minute. Laden Sie die Seite dann neu.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Schulinhalt")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Neu laden" }));
    expect(reloadSpy).toHaveBeenCalledOnce();

    // The rejection is not cached: the next load asks the backend again.
    await TenantLayout({
      children: null,
      params: Promise.resolve({ tenant: "school-busy" }),
    });
    expect(fetchSpy).toHaveBeenCalledTimes(2);
  });

  it("keeps other lookup failures on the error path", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(null, { status: 503 }),
    );

    await expect(
      TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "school-busy" }),
      }),
    ).rejects.toThrow("Tenant resolution failed with HTTP 503");
  });

  it("does not run refresh-capable auth callbacks from the server layout", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "success",
          data: {
            tenant_id: 1,
            slug: "school-a",
            name: "School A",
            subdomain: "school-a",
            organization_id: 2,
            organization_name: "Organization",
            settings: {},
            grade_level_max: 4,
          },
        }),
        { status: 200 },
      ),
    );

    await TenantLayout({
      children: null,
      params: Promise.resolve({ tenant: "school-a" }),
    });

    expect(authMock).not.toHaveBeenCalled();
  });

  // The provider prop lands in the HTML and RSC payload; only the server
  // side shell preload receives the access token (#5005).
  it("passes the read-only session snapshot to the client provider", async () => {
    const session = {
      expires: "2026-09-04T12:00:00.000Z",
      user: { id: "7", tenantId: 1 },
    };
    readTenantSessionSnapshotMock.mockResolvedValue({
      session,
      accessToken: "backend-access",
    });
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(
        JSON.stringify({
          status: "success",
          data: {
            tenant_id: 1,
            slug: "school-a",
            name: "School A",
            subdomain: "school-a",
            organization_id: 2,
            organization_name: "Organization",
            settings: {},
            grade_level_max: 4,
          },
        }),
        { status: 200 },
      ),
    );

    const element = (await TenantLayout({
      children: null,
      params: Promise.resolve({ tenant: "school-a" }),
    })) as ReactElement<{ session: unknown }>;

    expect(element.props.session).toBe(session);
    expect(JSON.stringify(element.props)).not.toContain("backend-access");
    const { loadShellBootstrap } = await import("~/lib/shell-bootstrap.server");
    expect(loadShellBootstrap).toHaveBeenCalledWith(
      session,
      expect.objectContaining({ tenantId: 1 }),
      "backend-access",
    );
  });

  it.each([
    { timetable_enabled: false, expected: false },
    { timetable_enabled: true, expected: true },
    { timetable_enabled: undefined, expected: true },
  ])(
    "maps timetable_enabled=$timetable_enabled into the server tenant context",
    async ({ timetable_enabled, expected }) => {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(
        new Response(
          JSON.stringify({
            status: "success",
            data: {
              tenant_id: 1,
              slug: "school-a",
              name: "School A",
              subdomain: "school-a",
              organization_id: 2,
              organization_name: "Organization",
              settings: {},
              grade_level_max: 4,
              timetable_enabled,
            },
          }),
          { status: 200 },
        ),
      );

      const element = (await TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "school-a" }),
      })) as ReactElement<{ tenant: { timetableEnabled?: boolean } }>;

      expect(element.props.tenant.timetableEnabled).toBe(expected);
    },
  );

  it.each([
    { analytics_enabled: false, expected: false },
    { analytics_enabled: true, expected: true },
    { analytics_enabled: undefined, expected: false },
  ])(
    "maps analytics_enabled=$analytics_enabled into the server tenant context",
    async ({ analytics_enabled, expected }) => {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(
        new Response(
          JSON.stringify({
            status: "success",
            data: {
              tenant_id: 1,
              slug: "school-a",
              name: "School A",
              subdomain: "school-a",
              organization_id: 2,
              organization_name: "Organization",
              settings: {},
              grade_level_max: 4,
              analytics_enabled,
            },
          }),
          { status: 200 },
        ),
      );

      const element = (await TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "school-a" }),
      })) as ReactElement<{ tenant: { analyticsEnabled?: boolean } }>;

      expect(element.props.tenant.analyticsEnabled).toBe(expected);
    },
  );
  it.each([
    { caldav_enabled: false, expected: false },
    { caldav_enabled: true, expected: true },
    { caldav_enabled: undefined, expected: false },
  ])(
    "maps caldav_enabled=$caldav_enabled into the server tenant context",
    async ({ caldav_enabled, expected }) => {
      vi.spyOn(globalThis, "fetch").mockResolvedValue(
        new Response(
          JSON.stringify({
            status: "success",
            data: {
              tenant_id: 1,
              slug: "school-a",
              name: "School A",
              subdomain: "school-a",
              organization_id: 2,
              organization_name: "Organization",
              settings: {},
              grade_level_max: 4,
              caldav_enabled,
            },
          }),
          { status: 200 },
        ),
      );

      const element = (await TenantLayout({
        children: null,
        params: Promise.resolve({ tenant: "school-a" }),
      })) as ReactElement<{ tenant: { caldavEnabled?: boolean } }>;

      expect(element.props.tenant.caldavEnabled).toBe(expected);
    },
  );
});
