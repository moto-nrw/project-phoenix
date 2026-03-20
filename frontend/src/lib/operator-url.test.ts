import { describe, it, expect, vi, beforeEach } from "vitest";

const OPERATOR_HOSTNAME = "operator.localhost:3000";

// Must mock before importing — operator-url reads process.env at call time
vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);

// Re-import fresh for each test to reset the memoized _isOperator cache
async function importFresh() {
  vi.resetModules();
  return await import("./operator-url");
}

describe("operator-url", () => {
  beforeEach(() => {
    vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", OPERATOR_HOSTNAME);
  });

  describe("isOperatorSubdomain", () => {
    it("returns true when window.location.host matches operator hostname", async () => {
      Object.defineProperty(window, "location", {
        value: {
          host: OPERATOR_HOSTNAME,
          origin: `http://${OPERATOR_HOSTNAME}`,
        },
        writable: true,
      });

      const { isOperatorSubdomain } = await importFresh();
      expect(isOperatorSubdomain()).toBe(true);
    });

    it("returns false when window.location.host does not match", async () => {
      Object.defineProperty(window, "location", {
        value: { host: "localhost:3000", origin: "http://localhost:3000" },
        writable: true,
      });

      const { isOperatorSubdomain } = await importFresh();
      expect(isOperatorSubdomain()).toBe(false);
    });

    it("throws when NEXT_PUBLIC_OPERATOR_HOSTNAME is not set", async () => {
      vi.stubEnv("NEXT_PUBLIC_OPERATOR_HOSTNAME", "");

      Object.defineProperty(window, "location", {
        value: { host: "localhost:3000", origin: "http://localhost:3000" },
        writable: true,
      });

      const { isOperatorSubdomain } = await importFresh();
      expect(() => isOperatorSubdomain()).toThrow(
        "NEXT_PUBLIC_OPERATOR_HOSTNAME is not set",
      );
    });
  });

  describe("operatorPath", () => {
    it("strips /operator prefix on operator subdomain", async () => {
      Object.defineProperty(window, "location", {
        value: {
          host: OPERATOR_HOSTNAME,
          origin: `http://${OPERATOR_HOSTNAME}`,
        },
        writable: true,
      });

      const { operatorPath } = await importFresh();
      expect(operatorPath("/operator/suggestions")).toBe("/suggestions");
    });

    it("returns / when stripping /operator leaves empty string", async () => {
      Object.defineProperty(window, "location", {
        value: {
          host: OPERATOR_HOSTNAME,
          origin: `http://${OPERATOR_HOSTNAME}`,
        },
        writable: true,
      });

      const { operatorPath } = await importFresh();
      expect(operatorPath("/operator")).toBe("/");
    });

    it("adds /operator prefix on tenant subdomain", async () => {
      Object.defineProperty(window, "location", {
        value: { host: "localhost:3000", origin: "http://localhost:3000" },
        writable: true,
      });

      const { operatorPath } = await importFresh();
      expect(operatorPath("/suggestions")).toBe("/operator/suggestions");
    });

    it("does not double-prefix paths already starting with /operator on tenant", async () => {
      Object.defineProperty(window, "location", {
        value: { host: "localhost:3000", origin: "http://localhost:3000" },
        writable: true,
      });

      const { operatorPath } = await importFresh();
      expect(operatorPath("/operator/suggestions")).toBe(
        "/operator/suggestions",
      );
    });
  });

  describe("operatorAbsoluteUrl", () => {
    it("returns full URL on operator subdomain", async () => {
      Object.defineProperty(window, "location", {
        value: {
          host: OPERATOR_HOSTNAME,
          origin: `http://${OPERATOR_HOSTNAME}`,
        },
        writable: true,
      });

      const { operatorAbsoluteUrl } = await importFresh();
      expect(operatorAbsoluteUrl("/operator/suggestions")).toBe(
        `http://${OPERATOR_HOSTNAME}/suggestions`,
      );
    });

    it("returns full URL on tenant subdomain", async () => {
      Object.defineProperty(window, "location", {
        value: { host: "localhost:3000", origin: "http://localhost:3000" },
        writable: true,
      });

      const { operatorAbsoluteUrl } = await importFresh();
      expect(operatorAbsoluteUrl("/suggestions")).toBe(
        "http://localhost:3000/operator/suggestions",
      );
    });
  });
});
