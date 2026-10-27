import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

/**
 * The browser never holds an access token, refresh token or recovery proof
 * (#5005). The browser Session type carries none, so a read of one fails to
 * compile; this ratchet also catches casts and string-indexed reads. Code
 * that runs only on the server is exempt: route handlers (app/api), the auth
 * module (server/) and *.server.ts files. Hard zero, no allowlist.
 */

const SRC_ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);

const FORBIDDEN: ReadonlyArray<[string, RegExp]> = [
  ["session token read", /\buser\??\.(?:token|refreshToken)\b/],
  [
    "string-indexed session token read",
    /\[\s*["'](?:token|refreshToken)["']\s*\]/,
  ],
  ["refresh recovery proof", /\brefreshRecoveryProof\b/],
  ["server auth import", /["'](?:~|@)\/server\/auth(?:\/[\w-]+)?["']/],
  ["session cookie decoder", /["']next-auth\/jwt["']/],
];

function isServerOnly(relative: string): boolean {
  return (
    relative.startsWith("server/") ||
    relative.startsWith("app/api/") ||
    /\.server\.tsx?$/.test(relative)
  );
}

function isTestOrStory(relative: string): boolean {
  return (
    relative.startsWith("test/") ||
    /\.(?:test|spec|stories)\.tsx?$/.test(relative)
  );
}

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const absolute = path.join(dir, entry.name);
    if (entry.isDirectory()) return sourceFiles(absolute);
    return /\.tsx?$/.test(entry.name) ? [absolute] : [];
  });
}

describe("browser token ratchet (#5005)", () => {
  it("keeps every token out of code that can run in the browser", () => {
    const violations: string[] = [];
    for (const file of sourceFiles(SRC_ROOT)) {
      const relative = path.relative(SRC_ROOT, file).split(path.sep).join("/");
      if (isServerOnly(relative) || isTestOrStory(relative)) continue;
      const lines = readFileSync(file, "utf8").split("\n");
      lines.forEach((line, index) => {
        for (const [rule, pattern] of FORBIDDEN) {
          if (pattern.test(line)) {
            violations.push(`${relative}:${index + 1} ${rule}`);
          }
        }
      });
    }

    expect(violations).toEqual([]);
  });

  it("recognizes the patterns it forbids", () => {
    const samples = [
      "const t = session?.user?.token;",
      "headers.Authorization = `Bearer ${session.user.refreshToken}`;",
      'const t = (session.user as Record<string, string>)["token"];',
      "token.refreshRecoveryProof",
      'import { auth } from "~/server/auth";',
      'import type { ServerSession } from "~/server/auth/shared";',
      'import { getToken } from "next-auth/jwt";',
    ];
    for (const sample of samples) {
      expect(FORBIDDEN.some(([, pattern]) => pattern.test(sample))).toBe(true);
    }
  });
});
