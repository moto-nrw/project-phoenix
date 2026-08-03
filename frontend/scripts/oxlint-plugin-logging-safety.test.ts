import { afterEach, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";

const temporaryDirectories: string[] = [];

function lintSource(source: string) {
  const directory = mkdtempSync(join(tmpdir(), "logging-safety-"));
  temporaryDirectories.push(directory);
  const sourcePath = join(directory, "probe.ts");
  writeFileSync(sourcePath, source);

  return spawnSync(
    resolve("node_modules/.bin/oxlint"),
    ["-c", resolve(".oxlintrc.json"), sourcePath],
    { encoding: "utf8" },
  );
}

afterEach(() => {
  for (const directory of temporaryDirectories.splice(0)) {
    rmSync(directory, { recursive: true, force: true });
  }
});

describe("logging-safety/no-serialized-context", () => {
  it("reports createLogger aliases, children, and memoized bindings", () => {
    const result = lintSource(`
      import { createLogger as makeLogger } from "~/lib/logger";

      const log = makeLogger({ component: "AliasProbe" });
      const child = log.child({ operation: "probe" });
      const memoized = useMemo(() => makeLogger({ component: "MemoProbe" }), []);

      log.info("alias_probe", { payload: JSON.stringify({ password: "secret" }) });
      child.warn("child_probe", { payload: JSON.stringify({ token: "secret" }) });
      memoized.error("memo_probe", { payload: JSON.stringify({ pin: "1234" }) });
    `);
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(
      output.match(/logging-safety\(no-serialized-context\)/g),
    ).toHaveLength(3);
  });

  it("reports logger parameters with imported Logger types", () => {
    const result = lintSource(`
      import { type Logger as StructuredLogger } from "~/lib/logger";

      function report(log: StructuredLogger) {
        log.error("parameter_probe", {
          payload: JSON.stringify({ password: "secret" }),
        });
      }

      void report;
    `);
    const output = `${result.stdout}${result.stderr}`;

    expect(result.status).toBe(1);
    expect(
      output.match(/logging-safety\(no-serialized-context\)/g),
    ).toHaveLength(1);
  });

  it("ignores JSON.stringify text in logger strings and comments", () => {
    const result = lintSource(`
      import { createLogger } from "~/lib/logger";

      const log = createLogger({ component: "TextProbe" });

      log.info("JSON.stringify(payload) is not called here");
      log.info("comment_probe", {
        /* JSON.stringify({ password: "secret" }) */
        safe: true,
      });
    `);

    expect(result.status).toBe(0);
    expect(`${result.stdout}${result.stderr}`).not.toContain("logging-safety");
  });

  it("ignores unrelated objects and shadowed logger names", () => {
    const result = lintSource(`
      import { createLogger } from "~/lib/logger";

      const log = createLogger({ component: "ScopeProbe" });
      const logger = { info: (_message: string, _context: unknown) => undefined };

      function inspect(log: typeof logger) {
        log.info("shadowed", { payload: JSON.stringify({ safe: true }) });
      }

      logger.info("unrelated", { payload: JSON.stringify({ safe: true }) });
      void log;
      void inspect;
    `);

    expect(result.status).toBe(0);
    expect(`${result.stdout}${result.stderr}`).not.toContain("logging-safety");
  });
});
