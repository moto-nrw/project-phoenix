import { readFileSync, readdirSync } from "node:fs";
import path from "node:path";
import ts from "typescript";

/** Reviewed targets, never generated from the handlers being tested.
 * Each capability owns a JSON fragment below proxy-routes/. New fragments are
 * discovered independently of the production-route discovery in the table test.
 */
export function readProxyRouteExpectations(directory: string) {
  const routes: Record<string, readonly string[]> = Object.create(
    null,
  ) as Record<string, readonly string[]>;
  const files = readdirSync(directory, {
    recursive: true,
    withFileTypes: true,
  });
  for (const entry of files) {
    if (entry.isDirectory()) continue;
    const filename = path.join(entry.parentPath, entry.name);
    if (!entry.isFile() || !entry.name.endsWith(".json")) {
      throw new Error(`Unexpected proxy expectation file: ${filename}`);
    }
    const text = readFileSync(filename, "utf8");
    const ast = ts.parseJsonText(filename, text);
    assertUniqueProperties(ast, filename);
    const value: unknown = JSON.parse(text);
    if (value === null || Array.isArray(value) || typeof value !== "object") {
      throw new Error(`Invalid proxy expectations: ${filename}`);
    }
    const entries = Object.entries(value);
    if (entries.length === 0)
      throw new Error(`Empty proxy fragment: ${filename}`);
    for (const [route, calls] of entries) {
      if (Object.hasOwn(routes, route)) {
        throw new Error(`Duplicate proxy route: ${route}`);
      }
      if (
        !Array.isArray(calls) ||
        calls.length === 0 ||
        calls.some((call: unknown) => typeof call !== "string") ||
        new Set(calls).size !== calls.length
      ) {
        throw new Error(`Invalid proxy calls: ${route}`);
      }
      const expected = `${proxyResource(route)}.json`;
      if (
        path.relative(directory, filename).split(path.sep).join("/") !==
        expected
      ) {
        throw new Error(`Misplaced proxy route ${route}: expected ${expected}`);
      }
      routes[route] = calls as string[];
    }
  }
  if (Object.keys(routes).length === 0)
    throw new Error("No proxy expectations");
  return routes;
}

function assertUniqueProperties(ast: ts.Node, filename: string) {
  if (ts.isObjectLiteralExpression(ast)) {
    const keys = new Set<string>();
    for (const property of ast.properties) {
      const key =
        property.name && ts.isStringLiteral(property.name)
          ? property.name.text
          : property.name?.getText();
      if (key === undefined || keys.has(key)) {
        throw new Error(
          `Duplicate or invalid JSON property in ${filename}: ${key}`,
        );
      }
      keys.add(key);
    }
  }
  ts.forEachChild(ast, (child) => assertUniqueProperties(child, filename));
}

function proxyResource(route: string) {
  const segments = route.split("/");
  const length = [
    "operator",
    "parent",
    "school",
    "caterer",
    "traeger",
  ].includes(segments[1] ?? "")
    ? 3
    : 2;
  return segments.slice(0, length).join("/");
}

export const PROXY_ROUTES = readProxyRouteExpectations(
  path.join(__dirname, "proxy-routes"),
);
