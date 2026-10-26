import { afterAll, describe, expect, it } from "vitest";
import { spawnSync } from "node:child_process";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { lintSource, removeProbeDirectories } from "./oxlint-probe";

afterAll(removeProbeDirectories);

const RULE = "swr(no-direct-import)";

describe.concurrent("swr/no-direct-import", () => {
  it.each([
    'import useSWR from "swr"; void useSWR;',
    'import { mutate as update } from "swr"; void update;',
    'import * as SWR from "swr"; void SWR;',
    'import "swr";',
    'import type { SWRConfiguration } from "swr"; export type Config = SWRConfiguration;',
    'export { mutate } from "swr";',
    'export * from "swr";',
    'export type { SWRConfiguration } from "swr";',
    'void import("swr");',
    "void import(`swr`);",
    'const swr = require("swr"); void swr;',
    'import swr = require("swr"); export { swr };',
    'import useSWR from "swr/immutable"; void useSWR;',
    'export type Config = import("swr").SWRConfiguration;',
  ])("rejects a new direct package reference: %s", async (source) => {
    const result = await lintSource(source);
    expect(result.status, result.output).toBe(1);
    expect(result.output).toContain(RULE);
    expect(result.output).toContain("~/lib/swr");
  });

  it("accepts the shared wrapper and unrelated package names", async () => {
    const result = await lintSource(`
      import { useSWRAuth } from "~/lib/swr";
      import other from "swr-other";
      export { mutate } from "~/lib/swr";
      void useSWRAuth; void other;
      // import useSWR from "swr";
      const example = 'import useSWR from "swr"'; void example;
    `);
    expect(result.status, result.output).toBe(0);
  });

  it.each(["hooks.ts", "config.ts", "index.ts", "hooks.test.ts"])(
    "accepts package references inside the shared wrapper: %s",
    async (file) => {
      const result = await lintSource(
        'export { mutate } from "swr";',
        `src/lib/swr/${file}`,
      );
      expect(result.status, result.output).toBe(0);
    },
  );

  it("accepts the existing exception but rejects an additional import", async () => {
    const file = "src/app/operator/accounts/page.tsx";
    const existing = 'import useSWR from "swr"; void useSWR;';
    expect((await lintSource(existing, file)).status).toBe(0);
    const result = await lintSource(
      `${existing}\nexport { mutate } from "swr";`,
      file,
    );
    expect(result.status).toBe(1);
    expect(result.output).toContain(RULE);
  });

  it("requires removing an exception once its import is removed", async () => {
    const result = await lintSource(
      "export const migrated = true;",
      "src/app/operator/accounts/page.tsx",
    );
    expect(result.status).toBe(1);
    expect(result.output).toContain("Shrink");
  });

  it("also requires shrinking a partially migrated file", async () => {
    const result = await lintSource(
      'export { mutate } from "swr";',
      "src/lib/hooks/use-announcements.test.ts",
    );
    expect(result.status).toBe(1);
    expect(result.output).toContain("Shrink");
    expect(result.output).toContain(
      "1 direct SWR references remain (baseline: 2)",
    );
  });

  it("rejects an exception for a deleted file before linting", () => {
    const directory = mkdtempSync(join(tmpdir(), "swr-baseline-"));
    try {
      const scripts = join(directory, "scripts");
      mkdirSync(scripts);
      copyFileSync(
        resolve("scripts/oxlint-plugin-swr.mjs"),
        join(scripts, "plugin.mjs"),
      );
      writeFileSync(
        join(scripts, "swr-import-baseline.json"),
        JSON.stringify({ "src/deleted.ts": 1 }),
      );
      const config = join(directory, "config.json");
      writeFileSync(
        config,
        JSON.stringify({
          jsPlugins: [join(scripts, "plugin.mjs")],
          rules: { "swr/no-direct-import": "error" },
        }),
      );
      const source = join(directory, "probe.ts");
      writeFileSync(source, "export const probe = true;");
      const result = spawnSync(
        resolve("node_modules/.bin/oxlint"),
        ["-c", config, source],
        { encoding: "utf8" },
      );
      expect(result.status).toBe(1);
      expect(`${result.stdout}${result.stderr}`).toContain(
        "Shrink scripts/swr-import-baseline.json: remove deleted file src/deleted.ts.",
      );
    } finally {
      rmSync(directory, { recursive: true, force: true });
    }
  });

  it("also rejects new direct imports in tests and outside src", async () => {
    for (const file of ["src/probe.test.ts", "scripts/probe.ts"]) {
      const result = await lintSource('export { mutate } from "swr";', file);
      expect(result.status, result.output).toBe(1);
      expect(result.output).toContain(RULE);
    }
  });
});
