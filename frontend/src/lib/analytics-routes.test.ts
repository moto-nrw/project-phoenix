import { readdirSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  resolveAnalyticsRoute,
  TRACKED_PARENT_ROUTE_TEMPLATES,
  TRACKED_PUBLIC_ROUTE_TEMPLATES,
  TRACKED_SCHOOL_ROUTE_TEMPLATES,
  TRACKED_TENANT_PUBLIC_ROUTE_TEMPLATES,
  TRACKED_TENANT_ROUTE_TEMPLATES,
  type AnalyticsRouteSurface,
} from "./analytics-routes";

const appRoot = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "../app",
);

/** Route of every page.tsx below `directory`, as the browser shows it. */
function collectPageRoutes(
  directory: string,
  skip: ReadonlySet<string> = new Set(),
  prefix: readonly string[] = [],
): string[] {
  const routes: string[] = [];
  for (const entry of readdirSync(directory, { withFileTypes: true })) {
    if (entry.isDirectory()) {
      if (skip.has(entry.name)) continue;
      // Route groups such as (public) do not appear in the URL.
      const segment = /^\(.+\)$/.test(entry.name)
        ? []
        : [
            entry.name
              .replace(/^\[\[\.\.\.(.+)]]$/, ":$1*")
              .replace(/^\[\.\.\.(.+)]$/, ":$1*")
              .replace(/^\[(.+)]$/, ":$1"),
          ];
      const nested = collectPageRoutes(
        path.join(directory, entry.name),
        new Set(),
        [...prefix, ...segment],
      );
      routes.push(...nested);
      // An optional catch-all ([[...topic]]) also serves its parent path.
      if (/^\[\[\.\.\..+]]$/.test(entry.name) && nested.length > 0) {
        routes.push(`/${prefix.join("/")}`);
      }
    } else if (entry.name === "page.tsx") {
      routes.push(`/${prefix.join("/")}`);
    }
  }
  return routes;
}

// Each portal's page tree and its allowlist. A new page.tsx without a
// template fails here: add the template (see .claude/rules/usage-analytics.md).
const PORTAL_PAGE_TREES: ReadonlyArray<{
  readonly name: string;
  readonly directory: string;
  readonly skip?: ReadonlySet<string>;
  readonly templates: readonly string[];
}> = [
  {
    name: "OGS portal (authenticated)",
    directory: "[tenant]/(protected)",
    templates: TRACKED_TENANT_ROUTE_TEMPLATES,
  },
  {
    name: "OGS portal host (login, enrollment, display)",
    directory: "[tenant]",
    // The catch-all 404 page is deliberately sent as /unknown.
    skip: new Set(["(protected)", "[...not-found]"]),
    templates: TRACKED_TENANT_PUBLIC_ROUTE_TEMPLATES,
  },
  {
    name: "parents portal",
    directory: "parents",
    templates: TRACKED_PARENT_ROUTE_TEMPLATES,
  },
  {
    name: "school portal",
    directory: "school",
    templates: TRACKED_SCHOOL_ROUTE_TEMPLATES,
  },
  {
    name: "public pages",
    directory: ".",
    // Portals above; the operator portal has no analytics.
    skip: new Set(["[tenant]", "parents", "school", "operator", "api"]),
    templates: TRACKED_PUBLIC_ROUTE_TEMPLATES,
  },
];

describe("analytics route allowlists", () => {
  it.each(PORTAL_PAGE_TREES)(
    "cover every page of the $name",
    ({ directory, skip, templates }) => {
      const pageRoutes = collectPageRoutes(
        path.join(appRoot, directory),
        skip,
      ).sort();

      expect([...templates].sort()).toEqual(pageRoutes);
    },
  );
});

const SURFACE_TEMPLATES: ReadonlyArray<
  [AnalyticsRouteSurface, readonly string[]]
> = [
  [
    "ogs",
    [
      ...TRACKED_TENANT_ROUTE_TEMPLATES,
      ...TRACKED_TENANT_PUBLIC_ROUTE_TEMPLATES,
    ],
  ],
  ["parents", TRACKED_PARENT_ROUTE_TEMPLATES],
  ["school", TRACKED_SCHOOL_ROUTE_TEMPLATES],
  ["public", TRACKED_PUBLIC_ROUTE_TEMPLATES],
];

describe("resolveAnalyticsRoute", () => {
  it.each(
    SURFACE_TEMPLATES.flatMap(([surface, templates]) =>
      templates.map((template) => [surface, template] as const),
    ),
  )("resolves %s %s without exposing dynamic values", (surface, template) => {
    const pathname = template.replace(/:[^/]+/g, "sensitive-value-123");
    expect(resolveAnalyticsRoute(surface, pathname)).toBe(template);
  });

  it("prefers a static page over a dynamic template", () => {
    expect(resolveAnalyticsRoute("ogs", "/rooms/unterwegs")).toBe(
      "/rooms/unterwegs",
    );
    expect(resolveAnalyticsRoute("ogs", "/rooms/12")).toBe("/rooms/:id");
  });

  it("strips the tenant slug of path routing", () => {
    expect(resolveAnalyticsRoute("ogs", "/school-a/students/42")).toBe(
      "/students/:id",
    );
    expect(resolveAnalyticsRoute("ogs", "/new-unreviewed-page")).toBeNull();
  });

  it("strips the portal prefix outside the portal's own host", () => {
    expect(resolveAnalyticsRoute("parents", "/parents/children/42")).toBe(
      "/children/:id",
    );
    expect(resolveAnalyticsRoute("school", "/school/nachrichten/7")).toBe(
      "/nachrichten/:threadID",
    );
    expect(resolveAnalyticsRoute("public", "/parents/children/42")).toBe(
      "/children/:id",
    );
  });

  it("maps enrollment status tokens to their template", () => {
    expect(resolveAnalyticsRoute("ogs", "/anmeldung/status/secret-token")).toBe(
      "/anmeldung/status/:token",
    );
    expect(
      resolveAnalyticsRoute("parents", "/anmeldung/status/secret-token/edit"),
    ).toBe("/anmeldung/status/:token/edit");
  });

  it("resolves nested help topics to the catch-all template", () => {
    expect(resolveAnalyticsRoute("public", "/help/kindersuche")).toBe(
      "/help/:topic*",
    );
    expect(resolveAnalyticsRoute("public", "/help/gruppe/alltag")).toBe(
      "/help/:topic*",
    );
    expect(resolveAnalyticsRoute("public", "/help/nfc/erste-schritte")).toBe(
      "/help/nfc/erste-schritte",
    );
  });

  it("rejects unknown, operator, and query-bearing paths", () => {
    expect(resolveAnalyticsRoute("ogs", "/new-unreviewed-page/x")).toBeNull();
    expect(resolveAnalyticsRoute("parents", "/dashboard")).toBeNull();
    expect(resolveAnalyticsRoute("public", "/operator/accounts")).toBeNull();
    expect(resolveAnalyticsRoute("ogs", "/dashboard?student=42")).toBeNull();
    expect(resolveAnalyticsRoute("ogs", "https://x/dashboard")).toBeNull();
  });
});
