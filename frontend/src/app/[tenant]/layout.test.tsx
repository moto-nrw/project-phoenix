import type { ReactElement } from "react";
import type { Session } from "next-auth";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { authMock, notFoundMock, readTenantSessionSnapshotMock } = vi.hoisted(
  () => ({
    authMock: vi.fn(async () => null),
    readTenantSessionSnapshotMock: vi.fn<() => Promise<Session | null>>(
      async () => null,
    ),
    notFoundMock: vi.fn((): never => {
      throw new Error("NEXT_HTTP_ERROR_FALLBACK;404");
    }),
  }),
);

vi.mock("next/navigation", () => ({
  notFound: notFoundMock,
  redirect: vi.fn(),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ host: "school-a.localhost:3000" })),
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

beforeEach(() => {
  authMock.mockClear();
  readTenantSessionSnapshotMock.mockReset();
  readTenantSessionSnapshotMock.mockResolvedValue(null);
  notFoundMock.mockClear();
  vi.restoreAllMocks();
});

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
      { next: { revalidate: 300, tags: ["tenant-school-a"] } },
    );
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

  it("passes the read-only session snapshot to the client provider", async () => {
    const session = {
      expires: "2026-09-04T12:00:00.000Z",
      user: { id: "7", token: "backend-access", tenantId: 1 },
    };
    readTenantSessionSnapshotMock.mockResolvedValue(session);
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
