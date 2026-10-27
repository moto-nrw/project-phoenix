import { readdirSync, readFileSync } from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";
import ts from "typescript";
import type { Session, User } from "next-auth";
import { describe, expect, expectTypeOf, it } from "vitest";

/**
 * The browser never holds an access token, refresh token or recovery proof
 * (#5005). The browser Session and User types carry none, so a plain read
 * fails to compile; this ratchet reads the syntax tree of every module that
 * can run in the browser and also catches what a cast lets through:
 * destructuring with renames, intermediate variables, `as` casts, string
 * keys and session callbacks. Server-only code is exempt: route handlers
 * (app/api), the auth module (server/) and *.server.ts files. Hard zero, no
 * exception list.
 */

const SRC_ROOT = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  "..",
);

/** Credential fields of a session, its user or the session cookie. */
const TOKEN_FIELDS = new Set([
  "token",
  "accessToken",
  "access_token",
  "refreshToken",
  "refresh_token",
  "refreshRecoveryProof",
  "recoveryProof",
  "previewAdminToken",
]);

/** Calls that answer with a session. */
const SESSION_CALLS = new Set([
  "useSession",
  "getSession",
  "getCachedSession",
  "auth",
  "uncachedAuth",
]);

/** Annotations that declare a session, its user or the cookie's content. */
const SESSION_TYPE = /\b(?:Session|ServerSession|User|AuthUser|JWT)\b/;

/** Modules that hand out tokens and are server-only. */
const SERVER_ONLY_MODULE =
  /^(?:(?:~|@)\/server\/auth(?:\/[\w-]+)?|next-auth\/jwt)$/;

interface Finding {
  line: number;
  rule: string;
}

function strip(expression: ts.Expression): ts.Expression {
  let current = expression;
  while (
    ts.isAsExpression(current) ||
    ts.isTypeAssertionExpression(current) ||
    ts.isSatisfiesExpression(current) ||
    ts.isParenthesizedExpression(current) ||
    ts.isNonNullExpression(current) ||
    ts.isAwaitExpression(current)
  ) {
    current = current.expression;
  }
  return current;
}

/** A literal key: `"token"`, `` `token` ``; for names also `token`. */
function stringKey(
  node: ts.Node | undefined,
  allowIdentifier = true,
): string | undefined {
  if (!node) return undefined;
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node))
    return node.text;
  if (allowIdentifier && ts.isIdentifier(node)) return node.text;
  return undefined;
}

function calleeName(call: ts.CallExpression): string | undefined {
  const callee = strip(call.expression);
  if (ts.isIdentifier(callee)) return callee.text;
  if (ts.isPropertyAccessExpression(callee)) return callee.name.text;
  return undefined;
}

/** Every credential read on a session-derived value in one module. */
function findBrowserTokenReads(fileName: string, source: string): Finding[] {
  const file = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    fileName.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  const derived = new Set<string>();
  const findings: Finding[] = [];
  const line = (node: ts.Node) =>
    file.getLineAndCharacterOfPosition(node.getStart(file)).line + 1;

  const isDerived = (expression: ts.Expression): boolean => {
    const node = strip(expression);
    if (ts.isIdentifier(node)) return derived.has(node.text);
    if (ts.isPropertyAccessExpression(node)) {
      // Any `.user` is a user object; anything else inherits its receiver.
      if (node.name.text === "user") return true;
      return !TOKEN_FIELDS.has(node.name.text) && isDerived(node.expression);
    }
    if (ts.isElementAccessExpression(node)) {
      const key = stringKey(node.argumentExpression, false);
      if (key === "user") return true;
      return !(key && TOKEN_FIELDS.has(key)) && isDerived(node.expression);
    }
    if (ts.isCallExpression(node)) {
      const name = calleeName(node);
      if (name && SESSION_CALLS.has(name)) return true;
      const callee = strip(node.expression);
      return ts.isIdentifier(callee) && derived.has(callee.text);
    }
    if (ts.isObjectLiteralExpression(node)) {
      return node.properties.some(
        (property) =>
          ts.isSpreadAssignment(property) && isDerived(property.expression),
      );
    }
    if (ts.isConditionalExpression(node))
      return isDerived(node.whenTrue) || isDerived(node.whenFalse);
    if (
      ts.isBinaryExpression(node) &&
      [
        ts.SyntaxKind.QuestionQuestionToken,
        ts.SyntaxKind.BarBarToken,
        ts.SyntaxKind.AmpersandAmpersandToken,
      ].includes(node.operatorToken.kind)
    )
      return isDerived(node.left) || isDerived(node.right);
    return false;
  };

  const declaresSession = (type: ts.TypeNode | undefined) =>
    type !== undefined && SESSION_TYPE.test(type.getText(file));

  // Binds the names of a destructured session-derived value; reports the
  // credential fields among them when `report` is set.
  const bindPattern = (pattern: ts.BindingName, report: boolean): boolean => {
    let changed = false;
    if (ts.isIdentifier(pattern)) {
      if (!derived.has(pattern.text)) {
        derived.add(pattern.text);
        changed = true;
      }
      return changed;
    }
    for (const element of pattern.elements) {
      if (ts.isOmittedExpression(element)) continue;
      const key = ts.isObjectBindingPattern(pattern)
        ? stringKey(element.propertyName ?? element.name)
        : undefined;
      if (key && TOKEN_FIELDS.has(key)) {
        if (report) findings.push({ line: line(element), rule: key });
        continue;
      }
      if (bindPattern(element.name, report)) changed = true;
    }
    return changed;
  };

  // Source of the first parameter of `derived.then(fn)`.
  const thenCallback = (node: ts.Node): ts.ParameterDeclaration | undefined => {
    if (!ts.isCallExpression(node)) return undefined;
    const callee = strip(node.expression);
    if (!ts.isPropertyAccessExpression(callee) || callee.name.text !== "then")
      return undefined;
    if (!isDerived(callee.expression)) return undefined;
    const callback = node.arguments[0];
    if (
      callback &&
      (ts.isArrowFunction(callback) || ts.isFunctionExpression(callback))
    )
      return callback.parameters[0];
    return undefined;
  };

  const sources = (visit: (name: ts.BindingName) => boolean) => {
    let changed = false;
    const walk = (node: ts.Node) => {
      if (ts.isVariableDeclaration(node)) {
        if (
          (node.initializer && isDerived(node.initializer)) ||
          declaresSession(node.type)
        )
          changed = visit(node.name) || changed;
      } else if (ts.isParameter(node) && declaresSession(node.type)) {
        changed = visit(node.name) || changed;
      } else if (
        ts.isBinaryExpression(node) &&
        node.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
        ts.isIdentifier(node.left) &&
        isDerived(node.right)
      ) {
        changed = visit(node.left) || changed;
      }
      const parameter = thenCallback(node);
      if (parameter) changed = visit(parameter.name) || changed;
      ts.forEachChild(node, walk);
    };
    walk(file);
    return changed;
  };

  // Propagate to a fixed point, then report once.
  while (sources((name) => bindPattern(name, false))) {
    // keep binding until no name is added
  }
  sources((name) => {
    if (!ts.isIdentifier(name)) bindPattern(name, true);
    return false;
  });

  const walkReads = (node: ts.Node) => {
    if (
      ts.isPropertyAccessExpression(node) &&
      TOKEN_FIELDS.has(node.name.text) &&
      isDerived(node.expression)
    ) {
      findings.push({ line: line(node), rule: node.name.text });
    } else if (ts.isElementAccessExpression(node)) {
      const key = stringKey(node.argumentExpression, false);
      if (key && TOKEN_FIELDS.has(key) && isDerived(node.expression))
        findings.push({ line: line(node), rule: key });
    } else if (
      ts.isBinaryExpression(node) &&
      node.operatorToken.kind === ts.SyntaxKind.EqualsToken &&
      ts.isObjectLiteralExpression(node.left) &&
      isDerived(node.right)
    ) {
      for (const property of node.left.properties) {
        const key =
          ts.isShorthandPropertyAssignment(property) ||
          ts.isPropertyAssignment(property)
            ? stringKey(property.name)
            : undefined;
        if (key && TOKEN_FIELDS.has(key))
          findings.push({ line: line(property), rule: key });
      }
    } else if (
      (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) &&
      node.moduleSpecifier &&
      ts.isStringLiteral(node.moduleSpecifier) &&
      SERVER_ONLY_MODULE.test(node.moduleSpecifier.text)
    ) {
      findings.push({ line: line(node), rule: "server-only import" });
    } else if (
      ts.isCallExpression(node) &&
      node.expression.kind === ts.SyntaxKind.ImportKeyword &&
      SERVER_ONLY_MODULE.test(stringKey(node.arguments[0]) ?? "")
    ) {
      findings.push({ line: line(node), rule: "server-only import" });
    }
    ts.forEachChild(node, walkReads);
  };
  walkReads(file);

  return findings.sort((a, b) => a.line - b.line);
}

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
    /\.(?:test|spec|stories)\.tsx?$/.test(relative) ||
    relative.endsWith(".d.ts")
  );
}

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const absolute = path.join(dir, entry.name);
    if (entry.isDirectory()) return sourceFiles(absolute);
    return /\.tsx?$/.test(entry.name) ? [absolute] : [];
  });
}

const clientModules = () =>
  sourceFiles(SRC_ROOT)
    .map((file) => ({
      file,
      relative: path.relative(SRC_ROOT, file).split(path.sep).join("/"),
    }))
    .filter(
      ({ relative }) => !isServerOnly(relative) && !isTestOrStory(relative),
    );

const reads = (source: string) =>
  findBrowserTokenReads("fixture.tsx", source).map(({ rule }) => rule);

describe("browser token ratchet (#5005)", () => {
  const modules = clientModules();

  // Checked by tsc (pnpm run check): a token field on the client-importable
  // types fails the typecheck before this ratchet runs.
  it("keeps token fields off the browser Session and User types", () => {
    expectTypeOf<Session["user"]>().not.toHaveProperty("token");
    expectTypeOf<Session["user"]>().not.toHaveProperty("accessToken");
    expectTypeOf<Session["user"]>().not.toHaveProperty("refreshToken");
    expectTypeOf<Session["user"]>().not.toHaveProperty("refreshRecoveryProof");
    expectTypeOf<Session>().not.toHaveProperty("token");
    expectTypeOf<Session>().not.toHaveProperty("accessToken");
    expectTypeOf<Session>().not.toHaveProperty("refreshToken");
    expectTypeOf<Session>().not.toHaveProperty("refreshRecoveryProof");
    expectTypeOf<User>().not.toHaveProperty("token");
    expectTypeOf<User>().not.toHaveProperty("accessToken");
    expectTypeOf<User>().not.toHaveProperty("refreshToken");
    expectTypeOf<User>().not.toHaveProperty("refreshRecoveryProof");
  });

  it("selects browser modules for the token scan", () => {
    // Guards the selection itself: client code exists and gets scanned.
    expect(modules.length).toBeGreaterThan(500);
    expect(modules.map(({ relative }) => relative)).toContain(
      "lib/session-cache.ts",
    );
  });

  // Give each module its own timeout budget instead of timing the entire
  // source tree as one test on slower CI runners.
  it.each(modules)(
    "keeps every token out of $relative",
    ({ file, relative }) => {
      const violations = findBrowserTokenReads(
        file,
        readFileSync(file, "utf8"),
      ).map(({ line, rule }) => `${relative}:${line} ${rule}`);

      expect(violations).toEqual([]);
    },
  );

  // Each of these compiled and passed the former line-regex ratchet.
  it.each([
    [
      "destructuring with a rename through a cast",
      `const { data: session } = useSession();
       const { token: accessToken } = session.user as unknown as { token: string };`,
      ["token"],
    ],
    [
      "refresh token destructured through a cast",
      `const { data: session } = useSession();
       const { refreshToken: rt } = session!.user as unknown as { refreshToken: string };`,
      ["refreshToken"],
    ],
    [
      "an intermediate variable",
      `const { data: session } = useSession();
       const user = session?.user;
       const t = (user as { token?: string }).token;`,
      ["token"],
    ],
    [
      "an intermediate variable cast to any",
      `const raw = useSession().data as any;
       const t = raw.user.refreshToken;`,
      ["refreshToken"],
    ],
    [
      "a string key",
      `const session = await getCachedSession();
       const t = (session?.user as Record<string, string>)["accessToken"];`,
      ["accessToken"],
    ],
    [
      "an awaited session call",
      `const t = ((await getSession()) as any)?.user?.token;`,
      ["token"],
    ],
    [
      "nested destructuring",
      `const { data: { user: { token } } } = useSession() as any;`,
      ["token"],
    ],
    [
      "a session callback",
      `getSession().then((current: any) => current.user.refreshToken);`,
      ["refreshToken"],
    ],
    [
      "a parameter typed as a session",
      `function read(session: Session) { return (session as any).token; }
       function pick({ user: { refreshToken } }: ServerSession) {}`,
      ["token", "refreshToken"],
    ],
    [
      "a spread copy of the user",
      `const { data: session } = useSession();
       const copy = { ...session?.user };
       const p = (copy as any).refreshRecoveryProof;`,
      ["refreshRecoveryProof"],
    ],
    [
      "a destructuring assignment",
      `let token; ({ token } = (await getSession())!.user as any);`,
      ["token"],
    ],
    [
      "server-only modules",
      `import { auth } from "~/server/auth";
       import type { ServerSession } from "~/server/auth/shared";
       import { getToken } from "next-auth/jwt";
       const mod = await import("~/server/auth/tenant-route");`,
      [
        "server-only import",
        "server-only import",
        "server-only import",
        "server-only import",
      ],
    ],
  ])("reports %s", (_case, source, expected) => {
    expect(reads(source)).toEqual(expected);
  });

  // Tokens that are not the session's: a login answer seeding Auth.js, an
  // invitation or URL token, and plain session fields.
  it("leaves values that are not session credentials alone", () => {
    expect(
      reads(`const tokens = await loginCaterer(email, password);
        await signIn("caterer-credentials", {
          token: tokens.accessToken,
          refreshToken: tokens.refreshToken,
        });
        const { token } = useParams<{ token: string }>();
        const invite = searchParams.get("token") ?? invitation.token;
        const { data: session } = useSession();
        const id = session?.user?.id;
        const allowed = session?.user?.permissions?.includes("x");
        const field = "token";
        const value = record[field];`),
    ).toEqual([]);
  });
});
