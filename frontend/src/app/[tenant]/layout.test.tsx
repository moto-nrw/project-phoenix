import { beforeEach, describe, expect, it, vi } from "vitest";

const { notFoundMock } = vi.hoisted(() => ({
  notFoundMock: vi.fn((): never => {
    throw new Error("NEXT_HTTP_ERROR_FALLBACK;404");
  }),
}));

vi.mock("next/navigation", () => ({
  notFound: notFoundMock,
  redirect: vi.fn(),
}));

vi.mock("next/headers", () => ({
  headers: vi.fn(async () => new Headers({ host: "school-a.localhost:3000" })),
}));

vi.mock("~/env", () => ({
  env: {
    API_URL: "http://server:8080",
    NODE_ENV: "development",
    TENANT_DOMAIN: "localhost",
  },
}));

const { default: TenantLayout } = await import("./layout");

beforeEach(() => {
  notFoundMock.mockClear();
  vi.restoreAllMocks();
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

  it("renders the Schule-nicht-gefunden screen when the subdomain resolves to no school (#2624)", async () => {
    // The mocked host is school-a.localhost:3000, so tenant "school-a" is
    // subdomain-routed. An unknown school on its own subdomain must show the
    // dedicated screen — not redirect, not the generic 404.
    const fetchSpy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue(new Response(null, { status: 404 }));

    const result = await TenantLayout({
      children: null,
      params: Promise.resolve({ tenant: "school-a" }),
    });

    expect(fetchSpy).toHaveBeenCalledOnce();
    expect(notFoundMock).not.toHaveBeenCalled();
    const { render, screen } = await import("@testing-library/react");
    render(result);
    expect(
      screen.getByRole("heading", { name: "Schule nicht gefunden" }),
    ).toBeInTheDocument();
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
});
