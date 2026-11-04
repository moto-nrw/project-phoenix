import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it, vi } from "vitest";
import { PROXY_ROUTES } from "~/test/proxy-routes.table";

/**
 * Table test for every pure proxy route: a `route.ts` whose only top-level
 * code is `export const METHOD = proxyX(endpoint, options?)`. The proxy
 * factories, auth and error handling are tested once in
 * `src/lib/route-proxy.server.test.ts`, `src/lib/route-wrapper.test.ts` and the
 * portal wrapper tests; per route only the portal, the backend path (with the
 * route params encoded) and the `raw` flag can differ. Those live in
 * capability-local JSON files under `src/test/proxy-routes/` fragments. The loader
 * rejects duplicate identities; a new pure proxy route without an expectation fails.
 */

type Portal =
  "tenant" | "operator" | "parent" | "school" | "caterer" | "traeger";
type Factory = "get" | "post" | "put" | "patch" | "delete";

interface ProxyDescriptor {
  portal: Portal;
  factory: Factory;
  endpoint: string | ((params: Record<string, unknown>) => string);
  options?: { raw?: boolean };
}

const proxyFactories = (portal: Portal) => {
  const make =
    (factory: Factory) =>
    (endpoint: ProxyDescriptor["endpoint"], options?: { raw?: boolean }) =>
      ({ portal, factory, endpoint, options }) satisfies ProxyDescriptor;
  return {
    proxyGet: make("get"),
    proxyPost: make("post"),
    proxyPut: make("put"),
    proxyPatch: make("patch"),
    proxyDelete: make("delete"),
  };
};

vi.mock("~/lib/route-proxy.server", () => proxyFactories("tenant"));
vi.mock("~/lib/operator/route-wrapper.server", () =>
  proxyFactories("operator"),
);
vi.mock("~/lib/parent/route-wrapper.server", () => proxyFactories("parent"));
vi.mock("~/lib/school/route-wrapper.server", () => proxyFactories("school"));
vi.mock("~/lib/caterer/route-wrapper.server", () => proxyFactories("caterer"));
vi.mock("~/lib/traeger/route-wrapper.server", () => proxyFactories("traeger"));

const appDir = path.resolve(__dirname, "..");
const PROXY_CALL = /^proxy(Get|Post|Put|Patch|Delete)$/;

/** A literal path or a one-expression builder; builders with own logic keep their own tests. */
function isPlainEndpoint(endpoint: ts.Expression | undefined): boolean {
  if (!endpoint) return false;
  if (ts.isArrowFunction(endpoint)) return !ts.isBlock(endpoint.body);
  return (
    ts.isStringLiteral(endpoint) || ts.isNoSubstitutionTemplateLiteral(endpoint)
  );
}

/** True when every top-level statement is an import, a type, or an exported proxy call. */
function isPureProxyRoute(file: string): boolean {
  const source = ts.createSourceFile(
    file,
    readFileSync(file, "utf8"),
    ts.ScriptTarget.Latest,
    true,
  );
  let proxies = 0;
  for (const statement of source.statements) {
    if (
      ts.isImportDeclaration(statement) ||
      ts.isInterfaceDeclaration(statement) ||
      ts.isTypeAliasDeclaration(statement)
    ) {
      continue;
    }
    const exported =
      ts.isVariableStatement(statement) &&
      statement.modifiers?.some((m) => m.kind === ts.SyntaxKind.ExportKeyword);
    if (!exported) return false;
    for (const declaration of statement.declarationList.declarations) {
      const init = declaration.initializer;
      if (
        !init ||
        !ts.isCallExpression(init) ||
        !PROXY_CALL.test(init.expression.getText(source)) ||
        !isPlainEndpoint(init.arguments[0])
      ) {
        return false;
      }
      proxies++;
    }
  }
  return proxies > 0;
}

/** Each dynamic segment `[name]` gets `name/1`, so the path shows its encoding. */
function sampleParams(route: string): Record<string, string> {
  const params: Record<string, string> = {};
  for (const [, name] of route.matchAll(/\[([^\]]+)\]/g)) {
    if (name) params[name] = `${name}/1`;
  }
  return params;
}

function describeExport(route: string, method: string, value: unknown) {
  const proxy = value as ProxyDescriptor;
  const endpoint =
    typeof proxy.endpoint === "function"
      ? proxy.endpoint(sampleParams(route))
      : proxy.endpoint;
  const raw = proxy.options?.raw ? " raw" : "";
  return `${method} ${proxy.portal} ${endpoint}${raw}`;
}

const pureRoutes = readdirSync(appDir, { recursive: true, encoding: "utf8" })
  .filter((file) => path.basename(file) === "route.ts")
  .filter((file) => isPureProxyRoute(path.join(appDir, file)))
  .map((file) => path.dirname(file))
  .sort();

describe("pure proxy routes", () => {
  it("has exactly one table row per pure proxy route", () => {
    expect(Object.keys(PROXY_ROUTES).sort()).toEqual(pureRoutes);
  });

  it.each(pureRoutes)("%s", async (route) => {
    const routeModule = (await import(
      path.join(appDir, route, "route.ts")
    )) as Record<string, unknown>;

    const actual = Object.entries(routeModule).map(([method, value]) => {
      expect((value as ProxyDescriptor).factory).toBe(
        method === "DELETE" ? "delete" : method.toLowerCase(),
      );
      return describeExport(route, method, value);
    });

    expect(actual.sort()).toEqual([...(PROXY_ROUTES[route] ?? [])].sort());
  });
});
