import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, expect, test } from "vitest";

import { loadStackAccess } from "./access";

const directories: string[] = [];

afterEach(async () => {
  await Promise.all(
    directories
      .splice(0)
      .map((dir) => rm(dir, { recursive: true, force: true })),
  );
});

async function seedState(
  createdAt: string,
  tenantSlug = "vollbetrieb",
): Promise<string> {
  const root = await mkdtemp(join(tmpdir(), "screenshot-seed-"));
  directories.push(root);
  const statePath = join(root, "seed-state.json");
  await writeFile(
    statePath,
    JSON.stringify({
      version: "3",
      created_at: createdAt,
      profiles: {
        vollbetrieb: {
          school: { tenant_slug: tenantSlug },
          credentials: {
            accounts: {
              admin: [
                {
                  key: "katarzyna-schmidt",
                  email: "admin@example.test",
                  password: "test",
                },
              ],
            },
          },
        },
      },
    }),
  );
  return statePath;
}

const env = {
  NODE_ENV: "test" as const,
  NEXT_PUBLIC_PARENTS_HOSTNAME: "eltern.localhost:3000",
  TENANT_DOMAIN: "localhost",
};

test("weist Anwesenheit vom Vortag vor dem Capture zurück", async () => {
  const statePath = await seedState("2026-09-09T21:59:00Z");
  expect(() =>
    loadStackAccess({
      statePath,
      env,
      now: new Date("2026-09-09T22:30:00Z"),
    }),
  ).toThrow(/neu seeden/);
});

test("akzeptiert den am selben Berliner Tag erstellten Seed", async () => {
  const statePath = await seedState("2026-09-09T22:05:00Z");
  const access = loadStackAccess({
    statePath,
    env,
    now: new Date("2026-09-10T08:15:00Z"),
  });
  expect(access.tenantOrigin).toBe("http://vollbetrieb.localhost:3000");
  expect(access.parentsOrigin).toBe("http://eltern.localhost:3000");
});

test.each([
  {
    name: "Eltern-Host",
    overrides: {
      NEXT_PUBLIC_PARENTS_HOSTNAME: "eltern.localhost:3000@remote.example",
    },
  },
  {
    name: "Tenant-Domain",
    overrides: { TENANT_DOMAIN: "localhost@remote.example" },
  },
  {
    name: "Eltern-Host mit Pfad",
    overrides: { NEXT_PUBLIC_PARENTS_HOSTNAME: "eltern.localhost:3000/remote" },
  },
])("weist $name vor der Anmeldung zurück", async ({ overrides }) => {
  const statePath = await seedState("2026-09-09T22:05:00Z");
  expect(() =>
    loadStackAccess({
      statePath,
      env: { ...env, ...overrides },
      now: new Date("2026-09-10T08:15:00Z"),
    }),
  ).toThrow(/lokale Origin/);
});

test("weist einen URL-Benutzeranteil im Tenant-Slug zurück", async () => {
  const statePath = await seedState(
    "2026-09-09T22:05:00Z",
    "vollbetrieb.localhost@remote.example",
  );
  expect(() =>
    loadStackAccess({
      statePath,
      env,
      now: new Date("2026-09-10T08:15:00Z"),
    }),
  ).toThrow(/Screenshot-Tenant/);
});
