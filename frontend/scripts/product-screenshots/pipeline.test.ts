import {
  mkdtemp,
  mkdir,
  readFile,
  readdir,
  rm,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { afterEach, expect, test } from "vitest";

import { replaceOutput } from "./pipeline";

const directories: string[] = [];

afterEach(async () => {
  await Promise.all(
    directories
      .splice(0)
      .map((dir) => rm(dir, { recursive: true, force: true })),
  );
});

test("ein fehlgeschlagener Austausch erhält die vorige Ausgabe", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(outDir);
  await writeFile(join(outDir, "manifest.json"), "bisherige Ausgabe");

  await expect(
    replaceOutput(join(root, "fehlende-vorbereitung"), outDir),
  ).rejects.toThrow();

  expect(await readFile(join(outDir, "manifest.json"), "utf8")).toBe(
    "bisherige Ausgabe",
  );
  expect(await readdir(root)).toEqual(["ausgabe"]);
});
