import { expect, test } from "@playwright/test";
import { access, readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
import sharp from "sharp";

import { loadStackAccess, type StackAccess } from "./access";
import { DEVICES, nativeSize, type DeviceId } from "./devices";
import { WEBP_WIDTHS } from "./frame";
import { MANIFEST_FILE, runPipeline, type Manifest } from "./pipeline";
import { BrokenShotError } from "./quality";
import { parseShotList } from "./shot-list";

// Seam 1 der Produkt-Screenshots (#3759): die ganze Pipeline gegen den lokalen
// Stack. Geprüft wird nur, was ein Nutzer der Ausgabe sieht: die erwarteten
// Dateien, exakte Pixelmaße, transparente Ecken im Mockup und das Manifest. Der
// Negativfall zeigt, dass ein kaputter Shot den Lauf abbricht und nichts
// ausgibt.
//
// Voraussetzung: der Stack läuft (scripts/dev-native.sh up) und ist mit dem
// Profil `marketing` geseedet. Ohne ihn überspringt der Test.
//   pnpm run test:screenshots

const MINI_LIST = `
shots:
  - id: pipeline-tenant
    titel: Pipeline-Test Tenant
    portal: tenant
    rolle: admin
    pfad: /home
  - id: pipeline-eltern
    titel: Pipeline-Test Eltern
    portal: eltern
    rolle: eltern
    pfad: /children
`;

let stack: StackAccess | null = null;
let skipReason = "";
try {
  stack = loadStackAccess();
} catch (error) {
  skipReason = error instanceof Error ? error.message : String(error);
}

test.describe.configure({ mode: "serial" });
test.skip(stack === null, `Kein geseedeter Stack: ${skipReason}`);

const NOW = new Date();

async function exists(path: string): Promise<boolean> {
  try {
    await access(path);
    return true;
  } catch {
    return false;
  }
}

async function pixel(file: string, x: number, y: number) {
  const { data, info } = await sharp(file)
    .ensureAlpha()
    .raw()
    .toBuffer({ resolveWithObject: true });
  const offset = (y * info.width + x) * info.channels;
  return {
    r: data[offset],
    g: data[offset + 1],
    b: data[offset + 2],
    a: data[offset + 3],
  };
}

test("erzeugt Rohbilder, Mockups und Manifest für zwei Shots", async () => {
  const outDir = test.info().outputPath("ausgabe");
  const shots = parseShotList(MINI_LIST);
  const manifest = await runPipeline({
    access: stack!,
    shots,
    outDir,
    version: "test",
    now: NOW,
  });

  // Manifest: Version, Zeitpunkt, beide Shots mit ihren Dateien.
  const onDisk = JSON.parse(
    await readFile(join(outDir, MANIFEST_FILE), "utf8"),
  ) as Manifest;
  expect(onDisk).toEqual(manifest);
  expect(onDisk.version).toBe("test");
  expect(Number.isNaN(Date.parse(onDisk.zeitpunkt))).toBe(false);
  expect(onDisk.shots.map((shot) => shot.id)).toEqual([
    "pipeline-tenant",
    "pipeline-eltern",
  ]);

  const expectedDevices: Record<string, DeviceId[]> = {
    "pipeline-tenant": ["macbook", "ipad"],
    "pipeline-eltern": ["iphone", "ipad"],
  };
  for (const shot of onDisk.shots) {
    const devices = expectedDevices[shot.id]!;
    // Pro Gerät: Rohbild + Mockup, je als PNG und in sechs WebP-Größen.
    expect(shot.dateien).toHaveLength(
      devices.length * 2 * (1 + WEBP_WIDTHS.length),
    );
    for (const device of devices) {
      const spec = DEVICES[device];
      const raw = join(outDir, shot.id, `roh-${device}.png`);
      const mockup = join(outDir, shot.id, `${device}.png`);

      // Exakte Pixelmaße: Rohbild nativ, Mockup so groß wie der Bezel.
      const rawMeta = await sharp(raw).metadata();
      expect({ width: rawMeta.width, height: rawMeta.height }).toEqual(
        nativeSize(spec),
      );
      const mockupMeta = await sharp(mockup).metadata();
      expect({ width: mockupMeta.width, height: mockupMeta.height }).toEqual({
        width: spec.bezel.width,
        height: spec.bezel.height,
      });

      // Transparente Ecken; der Bildschirm in der Mitte ist opak.
      const last = { x: spec.bezel.width - 1, y: spec.bezel.height - 1 };
      for (const [x, y] of [
        [0, 0],
        [last.x, 0],
        [0, last.y],
        [last.x, last.y],
      ] as const) {
        expect((await pixel(mockup, x, y)).a).toBe(0);
      }
      expect(
        (await pixel(mockup, spec.bezel.width >> 1, spec.bezel.height >> 1)).a,
      ).toBe(255);

      // WebP in allen Größen, mit der versprochenen Breite.
      for (const [base, art] of [
        [`roh-${device}`, "roh"],
        [device, "mockup"],
      ] as const) {
        for (const width of WEBP_WIDTHS) {
          const file = join(outDir, shot.id, `${base}-${width}.webp`);
          expect((await sharp(file).metadata()).width).toBe(width);
          const entry = shot.dateien.find(
            (candidate) =>
              candidate.pfad === `${shot.id}/${base}-${width}.webp`,
          );
          expect(entry, `Manifest kennt ${base}-${width}.webp`).toMatchObject({
            art,
            geraet: device,
            format: "webp",
            breite: width,
          });
        }
      }
    }
    // Jede Zeile des Manifests zeigt auf eine vorhandene Datei.
    for (const entry of shot.dateien) {
      expect(await exists(join(outDir, entry.pfad))).toBe(true);
    }
  }
  // Und es liegt nichts im Verzeichnis, das das Manifest nicht kennt.
  const listed = new Set(
    onDisk.shots.flatMap((shot) => shot.dateien.map((entry) => entry.pfad)),
  );
  for (const shot of onDisk.shots) {
    for (const name of await readdir(join(outDir, shot.id))) {
      expect(listed.has(`${shot.id}/${name}`)).toBe(true);
    }
  }
});

test("ein Klick der Vorbereitung darf auf eine Detailseite wechseln", async () => {
  // Detailseiten tragen IDs aus dem Seed; der Shot erreicht sie über die
  // Liste. Die Kindersuche hat außerdem animierte Schaltflächen und Räume
  // pulsierende Status-Punkte: beides ist kein Fehler und kein Ladezustand.
  const outDir = test.info().outputPath("ausgabe-klick");
  const shots = parseShotList(`
shots:
  - id: kinderdetail
    titel: Kinderdetail über die Suche
    portal: tenant
    rolle: admin
    pfad: /students/search
    geraete: [macbook]
    vorbereitung:
      - klicken: 'role=button[name="Emir Yilmaz - Details öffnen"]'
      - warten_auf: 'role=heading[name="Emir Yilmaz"]'
  - id: raeume
    titel: Räume mit belegten Räumen
    portal: tenant
    rolle: admin
    pfad: /rooms
    geraete: [macbook]
`);
  const manifest = await runPipeline({
    access: stack!,
    shots,
    outDir,
    version: "test",
    now: NOW,
  });
  expect(manifest.shots.map((shot) => shot.id)).toEqual([
    "kinderdetail",
    "raeume",
  ]);
  expect(await exists(join(outDir, "kinderdetail", "roh-macbook.png"))).toBe(
    true,
  );
});

test("ein Shot mit nicht existierender Route bricht den Lauf ab und gibt nichts aus", async () => {
  const outDir = test.info().outputPath("ausgabe-abbruch");
  const shots = parseShotList(`
shots:
  - id: gibt-es-nicht
    titel: Route, die es nicht gibt
    portal: tenant
    rolle: admin
    pfad: /diese-route-gibt-es-nicht
    geraete: [macbook]
`);
  await expect(
    runPipeline({ access: stack!, shots, outDir, version: "test", now: NOW }),
  ).rejects.toThrow(BrokenShotError);
  expect(await exists(outDir)).toBe(false);
});
