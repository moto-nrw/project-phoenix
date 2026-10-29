import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";
import ts from "typescript";
import { describe, expect, it } from "vitest";
import { PRIVATE_DOWNLOAD_ROUTES } from "~/test/private-download-routes";

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
  "~/lib/tenant-login-route.server": ["createTenantLoginRoute"],
  "~/lib/private-no-store": [
    "withPrivateNoStore",
    "withPrivateDownloadNoStore",
  ],
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
  "~/lib/caterer/route-wrapper.server": [
    "proxyGet",
    "proxyPost",
    "createCatererJsonProxy",
  ],
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

const DOWNLOAD_POLICY_FACTORIES: Record<string, string[]> = {
  "~/lib/private-no-store": ["withPrivateDownloadNoStore"],
};

// Non-JSON resources deliberately outside the private-download rule. Image,
// legal-document and subscription caching is not changed by #5119.
const NON_DOWNLOAD_RESOURCES: Record<string, string> = {
  "parent/me/news/[announcementId]/declaration/proof":
    "JSON proof metadata; the actual file is under proof/pdf",
  "traeger/master-data/logo":
    "Authenticated brand image, existing no-store policy",
  "public/enrollment-form-legal-documents/[filename]":
    "Public form PDF, public cache policy",
  "public/enrollment-legal-documents/[filename]":
    "Public legal PDF, public cache policy",
  "public/login-image/[filename]": "Public login image, public cache policy",
  "me/profile/avatar/[filename]":
    "Session avatar, existing private image cache",
  "staff/[id]/avatar": "Staff avatar, existing private image cache",
  "students/[id]/photo/[filename]":
    "Photo with existing backend revalidation policy",
  "calendar-feed/[token]": "Subscription feed, private max-age=3600",
  "request-feed/[token]": "Subscription feed, already private no-store",
  "sse/events": "Live stream, existing SSE cache/abort contract",
  "parent/sse/events": "Live stream, existing SSE cache/abort contract",
  "school/sse/events": "Live stream, existing SSE cache/abort contract",
};

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
  factories = POLICY_FACTORIES,
): boolean {
  if (!expression) return false;
  // `export const PATCH = PUT` reuses a handler defined in the same file.
  if (ts.isIdentifier(expression)) {
    const name = expression.text;
    if (seen.has(name) || !locals.has(name)) return false;
    seen.add(name);
    return isPolicyFactoryCall(
      locals.get(name),
      bindings,
      locals,
      seen,
      factories,
    );
  }
  if (!ts.isCallExpression(expression)) return false;
  if (!ts.isIdentifier(expression.expression)) return false;
  const binding = bindings.get(expression.expression.text);
  if (!binding) return false;
  const [module, name] = binding.split("#") as [string, string];
  return factories[module]?.includes(name) ?? false;
}

function hasExportModifier(node: ts.Node): boolean {
  return (
    ts.canHaveModifiers(node) &&
    (ts.getModifiers(node) ?? []).some(
      (modifier) => modifier.kind === ts.SyntaxKind.ExportKeyword,
    )
  );
}

/**
 * `METHOD` → covered? for every HTTP handler the route source exports. Star
 * re-exports appear as `export * from "<module>"` and never count as covered:
 * the check rejects what it cannot see instead of resolving other modules.
 */
function analyzeRouteSource(
  file: string,
  text: string,
  factories = POLICY_FACTORIES,
): Map<string, boolean> {
  const source = ts.createSourceFile(
    file,
    text,
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
              isPolicyFactoryCall(
                declaration.initializer,
                bindings,
                locals,
                new Set(),
                factories,
              ),
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
    } else if (ts.isExportDeclaration(statement)) {
      const clause = statement.exportClause;
      if (!clause || ts.isNamespaceExport(clause)) {
        // `export * from` / `export * as x from` can carry any handler.
        const specifier = statement.moduleSpecifier;
        const target =
          specifier && ts.isStringLiteral(specifier) ? specifier.text : "?";
        handlers.set(`export * from "${target}"`, false);
        continue;
      }
      // `export { GET }`, `export { handler as POST } from "…"`: the
      // exported binding is not built here, so it cannot prove the policy.
      for (const element of clause.elements) {
        if (HTTP_METHODS.has(element.name.text)) {
          handlers.set(element.name.text, false);
        }
      }
    }
  }
  return handlers;
}

function exportedHandlers(file: string): Map<string, boolean> {
  return analyzeRouteSource(file, readFileSync(file, "utf8"));
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

describe("private download cache policy (#5119)", () => {
  it.each(PRIVATE_DOWNLOAD_ROUTES)(
    "%s:%s enforces all MIME types, not just JSON",
    (route, method) => {
      const file = path.join(API_ROOT, route, "route.ts");
      expect(
        analyzeRouteSource(
          file,
          readFileSync(file, "utf8"),
          DOWNLOAD_POLICY_FACTORIES,
        ).get(method),
      ).toBe(true);
    },
  );

  it("inventories file constructors, file factories and download paths", () => {
    const inventoried = new Set<string>(
      PRIVATE_DOWNLOAD_ROUTES.map(([route]) => route),
    );
    const missing = routeFiles(API_ROOT).filter((file) => {
      const route = path.relative(API_ROOT, path.dirname(file));
      const text = readFileSync(file, "utf8");
      const fileFactory =
        /\b(?:createFileExportRoute|createFileDownloadRoute|pdfDownloadResponse|proxyFile|streamIcsDownload)\s*[(<]/.test(
          text,
        );
      const binaryResponse =
        /new (?:Next)?Response\(/.test(text) &&
        /content-disposition|application\/octet-stream|text\/calendar|text\/event-stream|image\//i.test(
          text,
        );
      const downloadPath =
        /(?:^|\/)(?:export(?:\/\[format\])?|download|template|ics|signature-proof|proof(?:\/pdf)?|pdf)$|-export$/.test(
          route,
        );
      return (
        (fileFactory || binaryResponse || downloadPath) &&
        !inventoried.has(route) &&
        !(route in NON_DOWNLOAD_RESOURCES)
      );
    });
    expect(missing.map((file) => path.relative(API_ROOT, file))).toEqual([]);
  });

  it("keeps the non-download inventory pointed at real routes", () => {
    const routes = new Set(
      routeFiles(API_ROOT).map((file) =>
        path.relative(API_ROOT, path.dirname(file)),
      ),
    );
    expect(
      Object.keys(NON_DOWNLOAD_RESOURCES).filter((route) => !routes.has(route)),
    ).toEqual([]);
  });

  it("does not mistake a JSON or portal wrapper for a download policy", () => {
    const file = path.join(API_ROOT, "fixture/route.ts");
    for (const [module, name] of [
      ["~/lib/private-no-store", "withPrivateNoStore"],
      ["~/server/auth/tenant-route", "withTenantAuth"],
    ]) {
      const text = `import { ${name} } from "${module}"; export const GET = ${name}(handler);`;
      expect(
        analyzeRouteSource(file, text, DOWNLOAD_POLICY_FACTORIES).get("GET"),
      ).toBe(false);
    }
  });
});

describe("route cache policy check rejects loopholes", () => {
  const file = path.join(API_ROOT, "fixture/route.ts");
  const uncovered = (text: string) =>
    [...analyzeRouteSource(file, text)]
      .filter(([, covered]) => !covered)
      .map(([name]) => name);

  it.each([
    [
      "a star re-export",
      'export * from "./handlers";',
      'export * from "./handlers"',
    ],
    [
      "a namespace re-export",
      'export * as handlers from "./handlers";',
      'export * from "./handlers"',
    ],
    ["a named re-export", 'export { GET } from "./handlers";', "GET"],
    [
      "a renamed re-export",
      'export { handler as POST } from "./handlers";',
      "POST",
    ],
    [
      "a local named export",
      'import { withPrivateNoStore } from "~/lib/private-no-store";\nconst POST = withPrivateNoStore(async () => Response.json({}));\nexport { POST };',
      "POST",
    ],
    [
      "an imported handler",
      'import { handler } from "./handlers";\nexport const POST = handler;',
      "POST",
    ],
    [
      "an imported handler behind a local alias",
      'import { POST as imported } from "./handlers";\nconst local = imported;\nexport const POST = local;',
      "POST",
    ],
    [
      "a property of an imported module",
      'import * as handlers from "./handlers";\nexport const POST = handlers.POST;',
      "POST",
    ],
    [
      "a call to an unknown factory",
      'import { makeHandler } from "./handlers";\nexport const POST = makeHandler();',
      "POST",
    ],
    [
      "a known factory name from another module",
      'import { withPrivateNoStore } from "./handlers";\nexport const POST = withPrivateNoStore(async () => Response.json({}));',
      "POST",
    ],
    [
      "an Auth.js destructuring export",
      'import { handlers } from "~/server/auth";\nexport const { GET } = handlers;',
      "GET",
    ],
    [
      "a plain function export",
      "export async function POST() {\n  return Response.json({ token: 'a' });\n}",
      "POST",
    ],
  ])("rejects %s", (_label, text, expected) => {
    expect(uncovered(text)).toEqual([expected]);
  });

  it("accepts a wrapped handler and an alias of it", () => {
    const text = [
      'import { withPrivateNoStore } from "~/lib/private-no-store";',
      "const handler = withPrivateNoStore(async () => Response.json({}));",
      "export const PUT = handler;",
      "export const PATCH = PUT;",
    ].join("\n");

    const handlers = analyzeRouteSource(file, text);

    expect([...handlers]).toEqual([
      ["PUT", true],
      ["PATCH", true],
    ]);
  });
});
