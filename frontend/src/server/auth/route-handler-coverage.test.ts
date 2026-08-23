import { describe, expect, it } from "vitest";
import { readFileSync, readdirSync, statSync } from "node:fs";
import path from "node:path";

const API_ROOT = path.join(process.cwd(), "src/app/api");
const AUTH_IMPORT = /(?:~\/server\/auth|server\/auth)/;
const RESPONSE_AWARE_ROUTE =
  /with(?:Tenant|Operator|Parent|School)Auth|create(?:FileUpload|Get|Post|Put|Patch|Delete)Handler/;

function routeFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((entry) => {
    const absolute = path.join(dir, entry);
    if (statSync(absolute).isDirectory()) return routeFiles(absolute);
    return entry === "route.ts" ? [absolute] : [];
  });
}

describe("authenticated route response coverage", () => {
  it("keeps every BFF auth read inside a response-aware Auth.js handler", () => {
    const bypasses = routeFiles(API_ROOT).flatMap((file) => {
      const source = readFileSync(file, "utf8");
      if (!AUTH_IMPORT.test(source)) return [];
      // These are Auth.js' own protocol endpoints; their exported handlers are
      // already the raw Auth.js response handlers and must not wrap themselves.
      if (file.includes(`${path.sep}[...nextauth]${path.sep}`)) return [];
      return RESPONSE_AWARE_ROUTE.test(source)
        ? []
        : [path.relative(process.cwd(), file)];
    });

    expect(
      bypasses,
      "A bare auth() call can rotate the backend refresh token while dropping Auth.js Set-Cookie, causing #1938 again",
    ).toEqual([]);
  });

  it("keeps all shared authenticated factories response-aware", () => {
    const wrappers = [
      ["src/lib/route-wrapper.server.ts", "withTenantAuth"],
      ["src/lib/file-upload-wrapper.server.ts", "withTenantAuth"],
      ["src/lib/operator/route-wrapper.server.ts", "withOperatorAuth"],
      ["src/lib/parent/route-wrapper.server.ts", "withParentAuth"],
      ["src/lib/school/route-wrapper.server.ts", "withSchoolAuth"],
    ] as const;

    for (const [file, expectedWrapper] of wrappers) {
      expect(readFileSync(path.join(process.cwd(), file), "utf8")).toContain(
        expectedWrapper,
      );
    }
  });
});
