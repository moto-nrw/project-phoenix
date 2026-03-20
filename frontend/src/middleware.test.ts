import { describe, it, expect, vi } from "vitest";
import { NextRequest } from "next/server";

const OPERATOR_HOSTNAME = "operator.localhost:3000";

vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);

// Import after env is stubbed — middleware reads the env var at module load
const { middleware } = await import("./middleware");

function makeRequest(url: string, host?: string): NextRequest {
  const req = new NextRequest(url);
  if (host) {
    req.headers.set("host", host);
  }
  return req;
}

describe("middleware", () => {
  describe("operator subdomain", () => {
    it("rewrites / to /operator", () => {
      const res = middleware(
        makeRequest(`http://${OPERATOR_HOSTNAME}/`, OPERATOR_HOSTNAME),
      );

      // Rewrites set x-middleware-rewrite header
      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator");
    });

    it("rewrites /login to /operator/login", () => {
      const res = middleware(
        makeRequest(`http://${OPERATOR_HOSTNAME}/login`, OPERATOR_HOSTNAME),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/login");
    });

    it("rewrites /suggestions to /operator/suggestions", () => {
      const res = middleware(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/suggestions`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/suggestions");
    });

    it("rewrites nested operator paths like /suggestions/123", () => {
      const res = middleware(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/suggestions/123`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      expect(rewrite).toContain("/operator/suggestions/123");
    });

    it("passes through /api/* routes without rewriting", () => {
      const res = middleware(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/api/auth/session`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("passes through /_next routes", () => {
      const res = middleware(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/_next/static/chunk.js`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("passes through paths already prefixed with /operator", () => {
      const res = middleware(
        makeRequest(
          `http://${OPERATOR_HOSTNAME}/operator/suggestions`,
          OPERATOR_HOSTNAME,
        ),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("redirects unknown paths like /dashboard to /", () => {
      const res = middleware(
        makeRequest(`http://${OPERATOR_HOSTNAME}/dashboard`, OPERATOR_HOSTNAME),
      );

      const redirect = res.headers.get("location");
      expect(redirect).toContain(OPERATOR_HOSTNAME);
      expect(new URL(redirect!).pathname).toBe("/");
    });
  });

  describe("tenant subdomain", () => {
    const TENANT_HOST = "localhost:3000";

    it("redirects /operator/* to operator subdomain with clean path", () => {
      const res = middleware(
        makeRequest(`http://${TENANT_HOST}/operator/suggestions`, TENANT_HOST),
      );

      const redirect = res.headers.get("location");
      expect(redirect).toContain(OPERATOR_HOSTNAME);
      expect(redirect).toContain("/suggestions");
      expect(redirect).not.toContain("/operator/suggestions");
    });

    it("redirects /operator to operator subdomain root /", () => {
      const res = middleware(
        makeRequest(`http://${TENANT_HOST}/operator`, TENANT_HOST),
      );

      const redirect = res.headers.get("location");
      expect(redirect).toContain(OPERATOR_HOSTNAME);
      expect(new URL(redirect!).pathname).toBe("/");
    });

    it("passes through non-operator paths", () => {
      const res = middleware(
        makeRequest(`http://${TENANT_HOST}/dashboard`, TENANT_HOST),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });

    it("passes through /api/* routes", () => {
      const res = middleware(
        makeRequest(`http://${TENANT_HOST}/api/students`, TENANT_HOST),
      );

      const rewrite = res.headers.get("x-middleware-rewrite");
      const redirect = res.headers.get("location");
      expect(rewrite).toBeNull();
      expect(redirect).toBeNull();
    });
  });
});
