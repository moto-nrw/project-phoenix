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
      "an aliased hook",
      'import { useSWRAuth as useA } from "~/lib/swr"; export function useProbe() { return useA("literal", async () => []); }',
    ],
    [
      "a hook through a namespace",
      'import * as Lib from "~/lib/swr"; export function useProbe() { return Lib.useSWRAuth("literal", async () => []); }',
    ],
    [
      "a local template",
      "export function useProbe(id: string) { const key = `room-${id}`; return useSWRAuth(id ? key : null, async () => []); }",
    ],
  ])("rejects a free key: %s", async (_name, body) => {
    const result = await lintSource(
      `import { useSWRAuth, useImmutableSWR } from "~/lib/swr"; import { useSWRWithId } from "~/lib/swr/hooks"; void useSWRAuth; void useImmutableSWR; void useSWRWithId;\n${body}`,
    );
    expect(result.status, result.output).toBe(1);
    expect(result.output).toContain("swr(no-free-key)");
    expect(result.output).not.toContain("swr(no-cache-access)");
  });

  it("accepts factory keys directly, through locals and in branches", async () => {
    const result = await lintSource(`
      import { useSWRAuth } from "~/lib/swr";
      import { useSWRWithId } from "~/lib/swr/hooks";
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

  // Every way the reviews found past the generation lock (#5007). The rule
  // judges the resolved module and its actual exports, so each spelling of
  // a path or name lands in one of three checks: internal module, unknown
  // export, opaque namespace member.
  it.each([
    [
      "useSWRConfig from the removed cache module",
      'import { useSWRConfig } from "~/lib/swr/cache"; void useSWRConfig;',
    ],
    [
      "useSWRConfig under an alias from the barrel",
      'import { useSWRConfig as config } from "~/lib/swr"; void config;',
    ],
    [
      "useSWRConfig re-exported",
      'export { useSWRConfig } from "~/lib/swr/cache";',
    ],
    [
      "SWRConfig.defaultValue.mutate from the barrel",
      'import { SWRConfig } from "~/lib/swr"; void SWRConfig.defaultValue.mutate("k", 1);',
    ],
    [
      "SWRConfig.defaultValue.cache.set from the cache module",
      'import { SWRConfig } from "~/lib/swr/cache"; SWRConfig.defaultValue.cache.set("k", {});',
    ],
    [
      "SWRConfig through a namespace member",
      'import * as Lib from "~/lib/swr"; void Lib.SWRConfig;',
    ],
    [
      "a computed namespace member",
      'import * as Lib from "~/lib/swr"; const name = "SWRConfig"; void Lib[name];',
    ],
    [
      "generation internals",
      'import { stampSWRValue } from "~/lib/swr/generation"; void stampSWRValue;',
    ],
    [
      "generation internals under @/",
      'import { stampSWRValue } from "@/lib/swr/generation"; void stampSWRValue;',
    ],
    [
      "generation internals by a relative path",
      'import { stampSWRValue } from "../lib/swr/generation"; void stampSWRValue;',
    ],
    [
      "generation internals with an extension",
      'import { stampSWRValue } from "~/lib/swr/generation.ts"; void stampSWRValue;',
    ],
    [
      "generation internals re-exported wholesale",
      'export * from "~/lib/swr/generation";',
    ],
    [
      "generation internals imported dynamically",
      'void import("~/lib/swr/generation");',
    ],
    [
      "generation internals required",
      'const internals = require("~/lib/swr/generation"); void internals;',
    ],
    [
      "scope controls",
      'import { resumeSWRSession } from "~/lib/swr/scope"; resumeSWRSession();',
    ],
    [
      "the test-only provider",
      'import { SWRConfig } from "~/lib/swr/testing"; void SWRConfig;',
    ],
    [
      "an internal module as an import type",
      'export type Internal = import("~/lib/swr/generation").SWRStart;',
    ],
  ])(
    "rejects unlocked cache access outside ~/lib/swr: %s",
    async (_name, source) => {
      const result = await lintSource(source);
      expect(result.status, result.output).toBe(1);
      expect(result.output).toContain("swr(no-cache-access)");
    },
  );

  it("accepts the public surface under every spelling of its path", async () => {
    const result = await lintSource(`
      import { IsolatedSWRCache, SWRSeedBoundary, useInvalidateMatching, useSWRSeedFrom, stampSWRSeeds, mutate } from "~/lib/swr";
      import { useSWRAuth } from "@/lib/swr/hooks";
      import { roomKeys } from "../lib/swr/rooms-keys";
      import { messageKeys } from "~/lib/swr/portal-keys.ts";
      import * as Lib from "~/lib/swr/index";
      export { SWRScopeBoundary } from "~/lib/swr/scope-boundary";
      void IsolatedSWRCache; void SWRSeedBoundary; void useInvalidateMatching; void useSWRSeedFrom;
      void stampSWRSeeds; void mutate; void useSWRAuth; void roomKeys; void messageKeys;
      void Lib.useTenantMutate;
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
