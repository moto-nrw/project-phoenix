import { readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  resolveAnalyticsViewId,
  TRACKED_TENANT_ROUTE_TEMPLATES,
} from "./analytics-routes";

function collectPageRoutes(directory: string, prefix = ""): string[] {
  const routes: string[] = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    const relative = prefix ? `${prefix}/${entry.name}` : entry.name;
    if (entry.isDirectory()) {
      routes.push(
        ...collectPageRoutes(path.join(directory, entry.name), relative),
      );
    } else if (entry.name === "page.tsx") {
      const route = relative
        .replace(/\/page\.tsx$/, "")
        .split("/")
        .map((segment) => segment.replace(/^\[(.+)]$/, ":$1"))
        .join("/");
      routes.push(`/${route}`);
    }
  }
  return routes;
}

describe("tenant analytics route allowlist", () => {
  it("covers every authenticated tenant page", () => {
    const protectedRoot = path.resolve(
      path.dirname(fileURLToPath(import.meta.url)),
      "../app/[tenant]/(protected)",
    );
    const pageRoutes = collectPageRoutes(protectedRoot).sort();

    expect([...TRACKED_TENANT_ROUTE_TEMPLATES].sort()).toEqual(pageRoutes);
  });

  it.each(TRACKED_TENANT_ROUTE_TEMPLATES)(
    "resolves %s without exposing dynamic values",
    (template) => {
      const pathname = template.replace(/:[^/]+/g, "sensitive-value-123");
      expect(resolveAnalyticsViewId(pathname)).toBe(template);
    },
  );

  it("rejects unknown, public, operator, and query-bearing paths", () => {
    expect(resolveAnalyticsViewId("/new-unreviewed-page")).toBeNull();
    expect(resolveAnalyticsViewId("/enroll/status/secret-token")).toBeNull();
    expect(resolveAnalyticsViewId("/operator/accounts")).toBeNull();
    expect(resolveAnalyticsViewId("/dashboard?student=42")).toBeNull();
  });
});
