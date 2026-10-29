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
  it("rejects new free hook keys but accepts factory keys", async () => {
    const bad = await lintSource(
      'import { useSWRAuth } from "~/lib/swr"; export function useProbe() { return useSWRAuth("new-key", async () => []); }',
    );
    expect(bad.status, bad.output).toBe(1);
    expect(bad.output).toContain("swr(no-free-key)");
    const good = await lintSource(
      'import { useSWRAuth } from "~/lib/swr"; import { roomKeys } from "~/lib/swr/rooms-keys"; export function useProbe() { return useSWRAuth(roomKeys.key(), async () => []); }',
    );
    expect(good.status, good.output).toBe(0);
  });
  it.each([
    [
      "a module constant",
      'const FREE_KEY = "rooms"; export function useProbe() { return useSWRAuth(FREE_KEY, async () => []); }',
    ],
    [
      "a join",
      'export function useProbe(id: string) { return useSWRAuth(["a", id].join("-"), async () => []); }',
    ],
    [
      "a concatenation",
      'export function useProbe(id: string) { return useSWRAuth("a-" + id, async () => []); }',
    ],
    [
      "a parameter",
      "export function useProbe(key: string) { return useSWRAuth(key, async () => []); }",
    ],
    [
      "another module's constant",
      'import { USER_KEY } from "~/lib/user"; export function useProbe() { return useImmutableSWR(USER_KEY, async () => []); }',
    ],
    [
      "a useSWRWithId base key",
      'export function useProbe(id: string) { return useSWRWithId("student", id, async () => []); }',
    ],
    [
      "a local template",
      "export function useProbe(id: string) { const key = `room-${id}`; return useSWRAuth(id ? key : null, async () => []); }",
    ],
  ])("rejects a free key: %s", async (_name, body) => {
    const result = await lintSource(
      `import { useSWRAuth, useImmutableSWR, useSWRWithId } from "~/lib/swr"; void useSWRAuth; void useImmutableSWR; void useSWRWithId;\n${body}`,
    );
    expect(result.status, result.output).toBe(1);
    expect(result.output).toContain("swr(no-free-key)");
  });

  it("accepts factory keys directly, through locals and in branches", async () => {
    const result = await lintSource(`
      import { useSWRAuth, useSWRWithId } from "~/lib/swr";
      import { roomKeys } from "~/lib/swr/rooms-keys";
      import { meKeys } from "~/lib/swr/me-keys";
      export function useProbe(id: string | null) {
        const key = id ? roomKeys.detailKey(id) : null;
        const later = useSWRAuth(listKey, async () => []);
        const listKey = roomKeys.key({ page: 1 });
        void later;
        void useSWRWithId(meKeys.family, id, async () => []);
        return useSWRAuth(key, async () => []);
      }
    `);
    expect(result.status, result.output).toBe(0);
  });

  it.each([
    'import { useSWRConfig } from "~/lib/swr/cache"; void useSWRConfig;',
    'import { useSWRConfig as config } from "~/lib/swr"; void config;',
    'export { useSWRConfig } from "~/lib/swr/cache";',
    'import { stampSWRValue } from "~/lib/swr/generation"; void stampSWRValue;',
    'export * from "~/lib/swr/generation";',
    'void import("~/lib/swr/generation");',
  ])("rejects unlocked cache access outside ~/lib/swr: %s", async (source) => {
    const result = await lintSource(source);
    expect(result.status, result.output).toBe(1);
    expect(result.output).toContain("swr(no-cache-access)");
  });

  it("accepts the provider and the locked helpers", async () => {
    const result = await lintSource(`
      import { SWRConfig } from "~/lib/swr/cache";
      import { useInvalidateMatching, useSWRSeedFrom, stampSWRSeeds } from "~/lib/swr";
      void SWRConfig; void useInvalidateMatching; void useSWRSeedFrom; void stampSWRSeeds;
    `);
    expect(result.status, result.output).toBe(0);
  });

  it.each(["src/lib/swr/seed.ts", "src/components/probe.test.tsx"])(
    "leaves cache access to the boundary and its tests: %s",
    async (file) => {
      const result = await lintSource(
        'import { stampSWRValue } from "~/lib/swr/generation"; void stampSWRValue;',
        file,
      );
      expect(result.output).not.toContain("swr(no-cache-access)");
    },
  );

  it.each([
    'import useSWR from "swr"; void useSWR;',
    'import useSWRInfinite from "swr/infinite"; void useSWRInfinite;',
    'import { preload } from "swr"; void preload;',
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
    const file = "src/components/ui/database/database-form.test.tsx";
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
      "src/components/ui/database/database-form.test.tsx",
    );
    expect(result.status).toBe(1);
    expect(result.output).toContain("Shrink");
  });

  it("also requires shrinking a partially migrated file", async () => {
    const result = await lintSource(
      'export { mutate } from "swr";',
      "src/app/[tenant]/(protected)/students/[id]/page.test.tsx",
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
      writeFileSync(join(scripts, "swr-key-baseline.json"), "{}");
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
