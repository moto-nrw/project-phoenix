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

import { assertReplaceable, replaceOutput } from "./pipeline";

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

test("ein fremdes Manifest erlaubt keinen Austausch", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(outDir);
  await writeFile(
    join(outDir, "manifest.json"),
    JSON.stringify({ version: "fremd" }),
  );
  await writeFile(join(outDir, "notizen.txt"), "behalten");

  await expect(assertReplaceable(outDir)).rejects.toThrow(/Pipeline/);
  expect(await readFile(join(outDir, "notizen.txt"), "utf8")).toBe("behalten");
});

test("nicht gelistete Dateien in einer Pipeline-Ausgabe bleiben geschützt", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(outDir);
  await writeFile(
    join(outDir, "manifest.json"),
    JSON.stringify({ format: "moto-product-screenshots/v1", shots: [] }),
  );
  await writeFile(join(outDir, "notizen.txt"), "behalten");

  await expect(assertReplaceable(outDir)).rejects.toThrow(/notizen\.txt/);
  expect(await readFile(join(outDir, "notizen.txt"), "utf8")).toBe("behalten");
});

test("akzeptiert nur vollständig gelistete Pipeline-Dateien", async () => {
  const root = await mkdtemp(join(tmpdir(), "product-screenshots-"));
  directories.push(root);
  const outDir = join(root, "ausgabe");
  await mkdir(join(outDir, "shot"), { recursive: true });
  await writeFile(join(outDir, "shot", "macbook.png"), "bild");
  await writeFile(
    join(outDir, "manifest.json"),
    JSON.stringify({
      format: "moto-product-screenshots/v1",
      shots: [{ id: "shot", dateien: [{ pfad: "shot/macbook.png" }] }],
    }),
  );

  await expect(assertReplaceable(outDir)).resolves.toBeUndefined();
});
