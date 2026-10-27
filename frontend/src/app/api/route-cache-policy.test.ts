import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";

/**
 * Every exported HTTP handler under `src/app/api` must be built by a factory
 * that applies the `private, no-store` cache policy (#4999), or carry a
 * reasoned entry in ROUTE_CACHE_POLICY_EXCEPTIONS. The check reads the
 * handler definitions, not the imports: a route that answers through a plain
 * `export async function GET` fails until it is wrapped.
 */

const SRC_ROOT = path.join(process.cwd(), "src");
const API_ROOT = path.join(SRC_ROOT, "app/api");
const HTTP_METHODS = new Set([
  "GET",
  "HEAD",
  "POST",
  "PUT",
  "PATCH",
  "DELETE",
  "OPTIONS",
]);

const PORTAL_PROXIES = ["proxyGet", "proxyPost", "proxyPut", "proxyPatch"];

/**
 * Factories that apply the cache policy, keyed by module. Authenticated
 * factories run through `createResponseAwareAuth().withAuthResponse`; the
 * public ones wrap themselves in `withPrivateNoStore`. Tests:
 * `src/server/auth/portal-cache-control.test.ts`.
 */
const POLICY_FACTORIES: Record<string, readonly string[]> = {
  "~/lib/private-no-store": ["withPrivateNoStore"],
  "~/server/auth": ["withTenantAuth"],
  "~/server/auth/tenant-route": ["withTenantAuth"],
  "~/server/auth/operator": ["withOperatorAuth"],
  "~/server/auth/parent": ["withParentAuth"],
  "~/server/auth/school": ["withSchoolAuth"],
  "~/server/auth/caterer": ["withCatererAuth"],
  "~/server/auth/traeger": ["withTraegerAuth"],
  "~/server/auth/operator-route": ["withOperatorAuth"],
  "~/server/auth/parent-route": ["withParentAuth"],
  "~/server/auth/school-route": ["withSchoolAuth"],
  "~/server/auth/caterer-route": ["withCatererAuth"],
  "~/server/auth/traeger-route": ["withTraegerAuth"],
  "~/lib/route-wrapper.server": [
    "createGetHandler",
    "createPostHandler",
    "createPutHandler",
    "createPatchHandler",
    "createDeleteHandler",
    "createDeleteWithBodyHandler",
  ],
  "~/lib/route-proxy.server": [...PORTAL_PROXIES, "proxyDelete"],
  "~/lib/operator/route-wrapper.server": [
    ...PORTAL_PROXIES,
    "proxyDelete",
    "createOperatorGetHandler",
    "createOperatorPostHandler",
    "createOperatorPutHandler",
    "createOperatorDeleteHandler",
    "createOperatorProxyPostHandler",
    "createOperatorPublicProxyPostHandler",
    "createOperatorProxyMethodHandler",
  ],
  "~/lib/parent/route-wrapper.server": [
    ...PORTAL_PROXIES,
    "proxyDelete",
    "createParentDeleteHandler",
    "createParentPutHandler",
    "createParentPatchHandler",
  ],
  "~/lib/school/route-wrapper.server": [
    ...PORTAL_PROXIES,
    "proxyDelete",
    "createSchoolDeleteHandler",
  ],
  "~/lib/caterer/route-wrapper.server": ["proxyGet", "proxyPost"],
  "~/lib/traeger/route-wrapper.server": [
    ...PORTAL_PROXIES,
    "proxyDelete",
    "proxyFile",
    "proxyUpload",
  ],
  "~/lib/backend-proxy-route.server": [
    "createTenantJsonProxy",
    "createOptionalTenantJsonProxy",
    "createPublicJsonProxy",
    "createOperatorJsonProxy",
    "createTenantApiAdapter",
  ],
  "~/lib/file-upload-wrapper.server": ["createFileUploadHandler"],
};

/**
 * Handlers that may answer without the policy, as `<route dir>:<METHOD>`
 * with the reason. Keep this list short: wrapping a file, image or stream
 * route costs nothing, because the policy leaves non-JSON answers alone.
 */
const ROUTE_CACHE_POLICY_EXCEPTIONS: Record<string, string> = {};

function routeFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const absolute = path.join(dir, entry);
    if (statSync(absolute).isDirectory()) return routeFiles(absolute);
    return /^route\.tsx?$/.test(entry) ? [absolute] : [];
  });
}

/** Normalizes an import specifier to its `~/…` form. */
function moduleKey(file: string, specifier: string): string {
  if (!specifier.startsWith(".")) return specifier.replace(/^@\//, "~/");
  const resolved = path.resolve(path.dirname(file), specifier);
  return `~/${path.relative(SRC_ROOT, resolved).split(path.sep).join("/")}`;
}

/** Local name → `module#exportedName` for every named import. */
function importedBindings(file: string, source: ts.SourceFile) {
  const bindings = new Map<string, string>();
  for (const statement of source.statements) {
    if (
      !ts.isImportDeclaration(statement) ||
      !ts.isStringLiteral(statement.moduleSpecifier)
    ) {
      continue;
    }
    const key = moduleKey(file, statement.moduleSpecifier.text);
    const named = statement.importClause?.namedBindings;
    if (!named || !ts.isNamedImports(named)) continue;
    for (const element of named.elements) {
      const imported = (element.propertyName ?? element.name).text;
      bindings.set(element.name.text, `${key}#${imported}`);
    }
  }
  return bindings;
}

function isPolicyFactoryCall(
  expression: ts.Expression | undefined,
  bindings: Map<string, string>,
  locals: Map<string, ts.Expression | undefined>,
  seen = new Set<string>(),
): boolean {
  if (!expression) return false;
  // `export const PATCH = PUT` reuses a handler defined in the same file.
  if (ts.isIdentifier(expression)) {
    const name = expression.text;
    if (seen.has(name) || !locals.has(name)) return false;
    seen.add(name);
    return isPolicyFactoryCall(locals.get(name), bindings, locals, seen);
  }
  if (!ts.isCallExpression(expression)) return false;
  if (!ts.isIdentifier(expression.expression)) return false;
  const binding = bindings.get(expression.expression.text);
  if (!binding) return false;
  const [module, name] = binding.split("#") as [string, string];
  return POLICY_FACTORIES[module]?.includes(name) ?? false;
}

function hasExportModifier(node: ts.Node): boolean {
  return (
    ts.canHaveModifiers(node) &&
    (ts.getModifiers(node) ?? []).some(
      (modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword,
    )
  );
}

/** `METHOD` → covered? for every HTTP handler the route file exports. */
function exportedHandlers(file: string): Map<string, boolean> {
  const source = ts.createSourceFile(
    file,
    readFileSync(file, "utf8"),
    ts.ScriptTarget.Latest,
    true,
    ts.ScriptKind.TS,
  );
  const bindings = importedBindings(file, source);
  const locals = new Map<string, ts.Expression | undefined>();
  for (const statement of source.statements) {
    if (!ts.isVariableStatement(statement)) continue;
    for (const declaration of statement.declarationList.declarations) {
      if (ts.isIdentifier(declaration.name)) {
        locals.set(declaration.name.text, declaration.initializer);
      }
    }
  }
  const handlers = new Map<string, boolean>();

  for (const statement of source.statements) {
    if (ts.isVariableStatement(statement) && hasExportModifier(statement)) {
      for (const declaration of statement.declarationList.declarations) {
        if (ts.isIdentifier(declaration.name)) {
          if (HTTP_METHODS.has(declaration.name.text)) {
            handlers.set(
              declaration.name.text,
              isPolicyFactoryCall(declaration.initializer, bindings, locals),
            );
          }
          continue;
        }
        // `export const { GET, POST } = handlers` cannot carry a wrapper.
        for (const element of declaration.name.elements) {
          if (ts.isBindingElement(element) && ts.isIdentifier(element.name)) {
            if (HTTP_METHODS.has(element.name.text)) {
              handlers.set(element.name.text, false);
            }
          }
        }
      }
    } else if (
      ts.isFunctionDeclaration(statement) &&
      hasExportModifier(statement) &&
      statement.name &&
      HTTP_METHODS.has(statement.name.text)
    ) {
      handlers.set(statement.name.text, false);
    } else if (
      ts.isExportDeclaration(statement) &&
      statement.exportClause &&
      ts.isNamedExports(statement.exportClause)
    ) {
      for (const element of statement.exportClause.elements) {
        if (HTTP_METHODS.has(element.name.text)) {
          handlers.set(element.name.text, false);
        }
      }
    }
  }
  return handlers;
}

describe("BFF route cache policy (#4999)", () => {
  const files = routeFiles(API_ROOT);

  it("finds the route handlers", () => {
    expect(files.length).toBeGreaterThan(500);
  });

  it("builds every exported handler with a factory that sends private, no-store", () => {
    const uncovered = files.flatMap((file) => {
      const route = path.relative(API_ROOT, path.dirname(file));
      return [...exportedHandlers(file)]
        .filter(([method, covered]) => {
          if (covered) return false;
          return !(`${route}:${method}` in ROUTE_CACHE_POLICY_EXCEPTIONS);
        })
        .map(([method]) => `${route}:${method}`);
    });

    expect(
      uncovered,
      "Wrap the handler in withPrivateNoStore (~/lib/private-no-store) or a portal wrapper, or add a reasoned exception",
    ).toEqual([]);
  });

  it("keeps every exception pointed at a real, uncovered handler", () => {
    const stale = Object.keys(ROUTE_CACHE_POLICY_EXCEPTIONS).filter((key) => {
      const [route, method] = key.split(":") as [string, string];
      const file = files.find(
        (candidate) =>
          path.relative(API_ROOT, path.dirname(candidate)) === route,
      );
      return !file || exportedHandlers(file).get(method) !== false;
    });

    expect(stale).toEqual([]);
  });
});
