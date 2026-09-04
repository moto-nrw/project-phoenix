import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, describe, expect, it } from "vitest";

import { readSeedAccess, SEED_STATE_VERSION } from "./seed-state";

function writeState(path: string, value: unknown): void {
  writeFileSync(path, JSON.stringify(value), { mode: 0o600 });
}

const tempDirs: string[] = [];

function statePath(): string {
  const dir = mkdtempSync(join(tmpdir(), "phoenix-seed-state-"));
  tempDirs.push(dir);
  return join(dir, "state.json");
}

function profile(slug: string, email: string) {
  return {
    school: { tenant_slug: slug },
    credentials: {
      accounts: { admin: [{ email, password: "Test1234%" }] },
    },
  };
}

describe("seed-state profile reader", () => {
  afterEach(() => {
    for (const dir of tempDirs.splice(0)) {
      rmSync(dir, { recursive: true });
    }
  });

  it("selects the default and an explicit profile", () => {
    const path = statePath();
    writeState(path, {
      version: SEED_STATE_VERSION,
      default_profile: "vollbetrieb",
      profiles: {
        vollbetrieb: profile("vollbetrieb", "vollbetrieb@example.test"),
        klein: profile("klein", "klein@example.test"),
      },
    });

    expect(readSeedAccess({ statePath: path })).toEqual({
      slug: "vollbetrieb",
      email: "vollbetrieb@example.test",
      password: "Test1234%",
    });
    expect(readSeedAccess({ statePath: path, profile: "klein" })?.slug).toBe(
      "klein",
    );
  });

  it("rejects unknown contract versions", () => {
    const path = statePath();
    writeState(path, { version: "2" });

    expect(() => readSeedAccess({ statePath: path })).toThrow(
      "Unsupported seed state version",
    );
  });

  it("rejects unknown profiles and lists valid keys", () => {
    const path = statePath();
    writeState(path, {
      version: SEED_STATE_VERSION,
      default_profile: "vollbetrieb",
      profiles: { vollbetrieb: profile("vollbetrieb", "admin@example.test") },
    });

    expect(() => readSeedAccess({ statePath: path, profile: "fehlt" })).toThrow(
      "available profiles: vollbetrieb",
    );
  });
});
