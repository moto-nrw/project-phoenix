import { mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";

import { storePublisher, type Publisher } from "./publish";

// Upload-Adapter auf ein lokales Verzeichnis. Die Kennung eines Eintrags ist
// sein Pfad; überschriebene Dateien behalten ihren Inode, so wie Drive-Dateien
// ihre Datei-ID. Grundlage der Vertragstests (publish.test.ts).

export function filesystemPublisher(rootDir: string): Publisher {
  return storePublisher({
    rootId: rootDir,
    async list(folderId) {
      try {
        return (await readdir(folderId, { withFileTypes: true })).map(
          (entry) => ({
            id: join(folderId, entry.name),
            name: entry.name,
            folder: entry.isDirectory(),
          }),
        );
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code === "ENOENT") return [];
        throw error;
      }
    },
    async readJson(file) {
      return JSON.parse(await readFile(file.id, "utf8")) as unknown;
    },
    async createFolder(parentId, name) {
      const path = join(parentId, name);
      await mkdir(path, { recursive: true });
      return path;
    },
    async createFile(parentId, name, data) {
      await writeFile(join(parentId, name), data, { flag: "wx" });
    },
    async updateFile(file, data) {
      await writeFile(file.id, data);
    },
    async remove(entry) {
      await rm(entry.id, { recursive: true });
    },
  });
}
