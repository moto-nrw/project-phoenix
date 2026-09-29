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

async function seedState(createdAt: string): Promise<string> {
  const root = await mkdtemp(join(tmpdir(), "marketing-seed-"));
  directories.push(root);
  const statePath = join(root, "seed-state.json");
  await writeFile(
    statePath,
    JSON.stringify({
      version: "3",
      created_at: createdAt,
      profiles: {
        marketing: {
          school: { tenant_slug: "marketing" },
          credentials: {
            school_admin: { email: "admin@example.test", password: "test" },
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
  expect(
    loadStackAccess({
      statePath,
      env,
      now: new Date("2026-09-10T08:15:00Z"),
    }).tenantOrigin,
  ).toBe("http://marketing.localhost:3000");
});
