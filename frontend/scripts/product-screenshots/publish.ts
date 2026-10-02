import { readFile } from "node:fs/promises";
import { join } from "node:path";

import { MANIFEST_FILE, readOutput } from "./pipeline";

// Der Upload-Adapter der Produkt-Screenshots (#3759): "veröffentliche das
// Ausgabeverzeichnis als Version X". Jedes Ziel (Dateisystem, Google Drive)
// beschreibt nur, wie es Ordner und Dateien anlegt, überschreibt und entfernt;
// was im Ziel entsteht, legt storePublisher für alle gleich fest:
//
//   v<version> (<datum>)/   die Ausgabe dieser Version
//   Aktuell/                dieselbe Ausgabe; vorhandene Dateien werden
//                           überschrieben und behalten ihre Identität, damit
//                           Links stabil bleiben; entfernte Shots verschwinden
//
// Andere Versionsordner bleiben unberührt. Ein erneuter Lauf derselben Version
// gleicht deren Ordner an, statt einen zweiten anzulegen. Der Versionsordner
// entsteht vor "Aktuell": bricht der Upload ab, zeigt "Aktuell" weiter den
// vorigen Stand.

const CURRENT_FOLDER = "Aktuell";

interface Release {
  /** Semantische Version ohne führendes `v`, etwa `1.4.0`. */
  readonly version: string;
  /** Tag der Veröffentlichung, `YYYY-MM-DD`. */
  readonly date: string;
}

export interface Publisher {
  publish(outDir: string, release: Release): Promise<void>;
}

export interface StoreEntry {
  readonly id: string;
  readonly name: string;
  readonly folder: boolean;
}

/** Ein Ablageziel: Ordner und Dateien, über eine Kennung adressiert. */
export interface PublishStore {
  readonly rootId: string;
  list(folderId: string): Promise<StoreEntry[]>;
  createFolder(parentId: string, name: string): Promise<string>;
  createFile(parentId: string, name: string, data: Buffer): Promise<void>;
  /** Neuer Inhalt für dieselbe Datei; ihre Kennung bleibt. */
  updateFile(file: StoreEntry, data: Buffer): Promise<void>;
  remove(entry: StoreEntry): Promise<void>;
}

const VERSION_PATTERN = /^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/;
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/;

function versionFolderName(release: Release): string {
  return `v${release.version} (${release.date})`;
}

/** Ordnerbaum der Ausgabe; ein Blatt ist der Pfad relativ zum Ausgabeverzeichnis. */
type Tree = Map<string, Tree | string>;

function outputTree(paths: readonly string[]): Tree {
  const root: Tree = new Map();
  for (const path of paths) {
    const parts = path.split("/");
    let folder = root;
    for (const part of parts.slice(0, -1)) {
      let child = folder.get(part);
      if (typeof child !== "object") {
        child = new Map();
        folder.set(part, child);
      }
      folder = child;
    }
    folder.set(parts.at(-1)!, path);
  }
  return root;
}

async function sync(
  store: PublishStore,
  folderId: string,
  tree: Tree,
  outDir: string,
): Promise<void> {
  const matching = new Map<string, StoreEntry>();
  const obsolete: StoreEntry[] = [];
  for (const entry of await store.list(folderId)) {
    const wanted = tree.get(entry.name);
    const fits =
      wanted !== undefined && (typeof wanted === "object") === entry.folder;
    if (fits && !matching.has(entry.name)) matching.set(entry.name, entry);
    else obsolete.push(entry);
  }
  for (const [name, wanted] of tree) {
    const entry = matching.get(name);
    if (typeof wanted === "string") {
      const data = await readFile(join(outDir, wanted));
      if (entry) await store.updateFile(entry, data);
      else await store.createFile(folderId, name, data);
    } else {
      const id = entry?.id ?? (await store.createFolder(folderId, name));
      await sync(store, id, wanted, outDir);
    }
  }
  for (const entry of obsolete) await store.remove(entry);
}

export function storePublisher(store: PublishStore): Publisher {
  return {
    async publish(outDir, release) {
      if (!VERSION_PATTERN.test(release.version)) {
        throw new Error(
          `Version ${JSON.stringify(release.version)} ist keine semantische Version wie 1.4.0.`,
        );
      }
      if (!DATE_PATTERN.test(release.date)) {
        throw new Error(
          `Datum ${JSON.stringify(release.date)} hat nicht die Form YYYY-MM-DD.`,
        );
      }
      const manifest = await readOutput(outDir);
      if (manifest.version !== release.version) {
        throw new Error(
          `Die Ausgabe in ${outDir} ist Version ${manifest.version}, veröffentlicht werden soll ${release.version}.`,
        );
      }
      // Manifest zuletzt: es beschreibt erst dann den neuen Stand, wenn alle
      // Bilder darin angekommen sind.
      const tree = outputTree([
        ...manifest.shots.flatMap((shot) =>
          shot.dateien.map((file) => file.pfad),
        ),
        MANIFEST_FILE,
      ]);

      const top = await store.list(store.rootId);
      const prefix = `v${release.version} (`;
      const versionFolder =
        top.find((entry) => entry.folder && entry.name.startsWith(prefix))
          ?.id ??
        (await store.createFolder(store.rootId, versionFolderName(release)));
      await sync(store, versionFolder, tree, outDir);

      const currentFolder =
        top.find((entry) => entry.folder && entry.name === CURRENT_FOLDER)
          ?.id ?? (await store.createFolder(store.rootId, CURRENT_FOLDER));
      await sync(store, currentFolder, tree, outDir);
    },
  };
}
