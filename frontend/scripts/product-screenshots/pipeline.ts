import { randomUUID } from "node:crypto";
import {
  mkdir,
  readFile,
  readdir,
  rename,
  rm,
  writeFile,
} from "node:fs/promises";
import { basename, dirname, join } from "node:path";
import sharp from "sharp";

import type { StackAccess } from "./access";
import { captureShots } from "./capture";
import { DEVICES, type DeviceId } from "./devices";
import { BEZEL_DIR, frameMockup, webpVariants } from "./frame";
import { referenceInstant } from "./reference-time";
import type { Shot } from "./shot-list";

// Die Pipeline (#3759): fotografieren, prüfen, rahmen, ausgeben. Ein kaputter
// Shot wirft in captureShots, bevor irgendetwas geschrieben wird; das
// Ausgabeverzeichnis entsteht erst, wenn alle Bilder fertig sind.

export const MANIFEST_FILE = "manifest.json";
const MANIFEST_FORMAT = "moto-product-screenshots/v1";

interface ManifestFile {
  /** Pfad relativ zum Ausgabeverzeichnis. */
  readonly pfad: string;
  readonly art: "roh" | "mockup";
  readonly geraet: DeviceId;
  readonly format: "png" | "webp";
  readonly breite: number;
  readonly hoehe: number;
  readonly bytes: number;
  /** Nur bei WebP: die Zielbreite ist größer als das Original. */
  readonly hochskaliert?: boolean;
}

interface ManifestShot {
  readonly id: string;
  readonly titel: string;
  readonly portal: Shot["portal"];
  readonly rolle: Shot["rolle"];
  readonly pfad: string;
  readonly dateien: readonly ManifestFile[];
}

export interface Manifest {
  readonly format: typeof MANIFEST_FORMAT;
  readonly version: string;
  /** Wann die Bilder entstanden sind. */
  readonly zeitpunkt: string;
  /** Die feste Uhrzeit der Browser-Uhr in allen Bildern. */
  readonly referenzzeit: string;
  readonly shots: readonly ManifestShot[];
}

export interface PipelineOptions {
  readonly access: StackAccess;
  readonly shots: readonly Shot[];
  readonly outDir: string;
  /** Release-Version; lokal ein frei gewählter Name. */
  readonly version: string;
  readonly now?: Date;
  readonly bezelDir?: string;
  readonly loadTimeoutMs?: number;
  readonly log?: (message: string) => void;
}

interface StagedFile {
  readonly entry: ManifestFile;
  readonly data: Buffer;
}

async function stageImage(
  base: string,
  art: ManifestFile["art"],
  geraet: DeviceId,
  png: Buffer,
): Promise<StagedFile[]> {
  const meta = await sharp(png).metadata();
  const prefix = art === "roh" ? "roh-" : "";
  const files: StagedFile[] = [
    {
      entry: {
        pfad: `${base}/${prefix}${geraet}.png`,
        art,
        geraet,
        format: "png",
        breite: meta.width ?? 0,
        hoehe: meta.height ?? 0,
        bytes: png.length,
      },
      data: png,
    },
  ];
  for (const variant of await webpVariants(png)) {
    files.push({
      entry: {
        pfad: `${base}/${prefix}${geraet}-${variant.width}.webp`,
        art,
        geraet,
        format: "webp",
        breite: variant.width,
        hoehe: variant.height,
        bytes: variant.data.length,
        hochskaliert: variant.upscaled,
      },
      data: variant.data,
    });
  }
  return files;
}

/** Ein Verzeichnis, das nicht von der Pipeline stammt, wird nie ersetzt. */
export async function assertReplaceable(outDir: string): Promise<void> {
  let entries: string[];
  try {
    entries = await readdir(outDir);
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") return;
    throw error;
  }
  if (entries.length === 0) return;
  if (!entries.includes(MANIFEST_FILE)) {
    throw new Error(
      `${outDir} ist nicht leer und enthält kein ${MANIFEST_FILE}. Die Pipeline ersetzt nur ihre eigene Ausgabe.`,
    );
  }
  let manifest: Partial<Manifest>;
  try {
    manifest = JSON.parse(
      await readFile(join(outDir, MANIFEST_FILE), "utf8"),
    ) as Partial<Manifest>;
  } catch (error) {
    throw new Error(`${outDir} enthält kein gültiges Pipeline-Manifest.`, {
      cause: error,
    });
  }
  if (manifest?.format !== MANIFEST_FORMAT || !Array.isArray(manifest.shots)) {
    throw new Error(
      `${outDir} enthält kein Manifest dieser Pipeline und wird nicht ersetzt.`,
    );
  }

  const expectedFiles = new Set<string>([MANIFEST_FILE]);
  const expectedDirs = new Set<string>();
  for (const shot of manifest.shots) {
    if (!shot || typeof shot.id !== "string" || !Array.isArray(shot.dateien)) {
      throw new Error(
        `${outDir} enthält ein unvollständiges Pipeline-Manifest.`,
      );
    }
    expectedDirs.add(shot.id);
    for (const file of shot.dateien) {
      if (
        !file ||
        typeof file.pfad !== "string" ||
        file.pfad.split("/").length !== 2 ||
        !file.pfad.startsWith(`${shot.id}/`)
      ) {
        throw new Error(
          `${outDir} enthält einen ungültigen Dateipfad im Pipeline-Manifest.`,
        );
      }
      expectedFiles.add(file.pfad);
    }
  }

  async function checkEntries(dir: string, prefix = ""): Promise<void> {
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const path = prefix ? `${prefix}/${entry.name}` : entry.name;
      if (entry.isDirectory() && expectedDirs.delete(path)) {
        await checkEntries(join(dir, entry.name), path);
      } else if (!entry.isFile() || !expectedFiles.delete(path)) {
        throw new Error(
          `${outDir} enthält den nicht gelisteten Eintrag ${path} und wird nicht ersetzt.`,
        );
      }
    }
  }
  await checkEntries(outDir);
  if (expectedFiles.size > 0 || expectedDirs.size > 0) {
    throw new Error(
      `${outDir} enthält nicht alle im Pipeline-Manifest gelisteten Dateien und wird nicht ersetzt.`,
    );
  }
}

/** Behält die vorige Ausgabe bis zum erfolgreichen Austausch als Rückfall. */
export async function replaceOutput(
  tmpDir: string,
  outDir: string,
): Promise<void> {
  const backupDir = join(
    dirname(outDir),
    `.${basename(outDir)}.backup-${randomUUID()}`,
  );
  let hasBackup = false;
  try {
    await rename(outDir, backupDir);
    hasBackup = true;
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code !== "ENOENT") throw error;
  }
  try {
    await rename(tmpDir, outDir);
  } catch (error) {
    if (hasBackup) {
      try {
        await rename(backupDir, outDir);
      } catch (restoreError) {
        throw new Error(
          `Ausgabe konnte nicht ersetzt werden (${String(error)}); vorige Ausgabe liegt in ${backupDir}`,
          { cause: restoreError },
        );
      }
    }
    throw error;
  }
  if (hasBackup) await rm(backupDir, { recursive: true });
}

export async function runPipeline(options: PipelineOptions): Promise<Manifest> {
  const log = options.log ?? (() => undefined);
  const now = options.now ?? new Date();
  await assertReplaceable(options.outDir);

  const raws = await captureShots({
    access: options.access,
    shots: options.shots,
    now,
    ...(options.loadTimeoutMs === undefined
      ? {}
      : { loadTimeoutMs: options.loadTimeoutMs }),
    log,
  });

  const staged = new Map<string, StagedFile[]>();
  for (const raw of raws) {
    log(`Rahme ${raw.shotId} auf ${raw.device}`);
    const mockup = await frameMockup(
      raw.png,
      DEVICES[raw.device],
      options.bezelDir ?? BEZEL_DIR,
    );
    const files = staged.get(raw.shotId) ?? [];
    files.push(
      ...(await stageImage(raw.shotId, "roh", raw.device, raw.png)),
      ...(await stageImage(raw.shotId, "mockup", raw.device, mockup)),
    );
    staged.set(raw.shotId, files);
  }

  const manifest: Manifest = {
    format: MANIFEST_FORMAT,
    version: options.version,
    zeitpunkt: now.toISOString(),
    referenzzeit: referenceInstant(now).toISOString(),
    shots: options.shots.map((shot) => ({
      id: shot.id,
      titel: shot.titel,
      portal: shot.portal,
      rolle: shot.rolle,
      pfad: shot.pfad,
      dateien: (staged.get(shot.id) ?? []).map((file) => file.entry),
    })),
  };

  // Erst in ein Nachbarverzeichnis schreiben, dann austauschen: ein Abbruch
  // beim Schreiben lässt die vorige Ausgabe unberührt.
  const tmpDir = join(
    dirname(options.outDir),
    `.${basename(options.outDir)}.tmp-${process.pid}`,
  );
  await rm(tmpDir, { recursive: true, force: true });
  try {
    for (const files of staged.values()) {
      for (const file of files) {
        const target = join(tmpDir, file.entry.pfad);
        await mkdir(dirname(target), { recursive: true });
        await writeFile(target, file.data);
      }
    }
    await writeFile(
      join(tmpDir, MANIFEST_FILE),
      `${JSON.stringify(manifest, null, 2)}\n`,
    );
    await assertReplaceable(options.outDir);
    await replaceOutput(tmpDir, options.outDir);
  } catch (error) {
    await rm(tmpDir, { recursive: true, force: true });
    throw error;
  }
  log(`Ausgabe: ${options.outDir}`);
  return manifest;
}
