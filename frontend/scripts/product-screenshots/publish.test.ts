import {
  mkdir,
  mkdtemp,
  readFile,
  readdir,
  rm,
  stat,
  writeFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { afterEach, describe, expect, test } from "vitest";

import type { Publisher } from "./publish";
import { drivePublisher } from "./publish-drive";
import { filesystemPublisher } from "./publish-filesystem";

// Seam 2 der Produkt-Screenshots (#3759): der Upload-Adapter "veröffentliche
// das Ausgabeverzeichnis als Version X". Derselbe Vertrag läuft gegen den
// Dateisystem-Adapter und gegen den Drive-Adapter mit einer nachgebildeten
// Drive-REST-API. Geprüft wird nur der sichtbare Zustand im Ziel: welche
// Ordner und Dateien es gibt, was darin steht und ob eine Datei in "Aktuell"
// dieselbe bleibt (Inode bzw. Drive-Datei-ID).

const directories: string[] = [];

afterEach(async () => {
  await Promise.all(
    directories
      .splice(0)
      .map((dir) => rm(dir, { recursive: true, force: true })),
  );
});

async function tempDir(): Promise<string> {
  const dir = await mkdtemp(join(tmpdir(), "product-screenshots-publish-"));
  directories.push(dir);
  return dir;
}

type Shots = Record<string, Record<string, string>>;

/** Schreibt eine Ausgabe, wie runPipeline sie hinterlässt. */
async function writeOutput(version: string, shots: Shots): Promise<string> {
  const outDir = join(await tempDir(), "ausgabe");
  const manifestShots = [];
  for (const [id, files] of Object.entries(shots)) {
    const dateien = [];
    for (const [name, content] of Object.entries(files)) {
      const path = join(outDir, id, name);
      await mkdir(dirname(path), { recursive: true });
      await writeFile(path, content);
      dateien.push({ pfad: `${id}/${name}` });
    }
    manifestShots.push({ id, dateien });
  }
  await mkdir(outDir, { recursive: true });
  await writeFile(
    join(outDir, "manifest.json"),
    JSON.stringify({
      format: "moto-product-screenshots/v1",
      version,
      shots: manifestShots,
    }),
  );
  return outDir;
}

interface Snapshot {
  [path: string]: { readonly identity: string; readonly content: string };
}

interface Target {
  readonly publisher: Publisher;
  /** Alle Dateien im Ziel, mit Inhalt und Identität. */
  snapshot(): Promise<Snapshot>;
}

async function filesystemTarget(): Promise<Target> {
  const root = await tempDir();
  async function walk(dir: string, prefix: string): Promise<Snapshot> {
    const result: Snapshot = {};
    for (const entry of await readdir(dir, { withFileTypes: true })) {
      const path = prefix ? `${prefix}/${entry.name}` : entry.name;
      const full = join(dir, entry.name);
      if (entry.isDirectory()) {
        Object.assign(result, await walk(full, path));
      } else {
        result[path] = {
          identity: String((await stat(full)).ino),
          content: await readFile(full, "utf8"),
        };
      }
    }
    return result;
  }
  return {
    publisher: filesystemPublisher(root),
    snapshot: () => walk(root, ""),
  };
}

interface FakeDriveFile {
  name: string;
  mimeType: string;
  parent: string;
  data: string;
  trashed: boolean;
}

const FOLDER_MIME = "application/vnd.google-apps.folder";

/**
 * Nachbildung der Drive-v3-Endpunkte, die der Adapter benutzt.
 * `createdButFailed`: so viele Datei-Uploads legen die Datei an, antworten
 * aber mit 500, wie Drive es bei einem Serverfehler nach dem Schreiben tut.
 */
function driveTarget({
  createdButFailed = 0,
  rejectedRequest = undefined,
}: {
  createdButFailed?: number;
  rejectedRequest?: { method: string; status: number; reason: string };
} = {}): Target {
  const files = new Map<string, FakeDriveFile>();
  let nextId = 0;
  const root = "zielordner";

  function reply(status: number, body: unknown): Response {
    return new Response(JSON.stringify(body), { status });
  }

  const fakeFetch = async (
    input: string | URL | Request,
    init?: RequestInit,
  ): Promise<Response> => {
    const url = new URL(String(input));
    const method = init?.method ?? "GET";
    if (url.href === "https://oauth2.googleapis.com/token") {
      const form = new URLSearchParams(String(init?.body));
      return form.get("refresh_token") === "refresh"
        ? reply(200, { access_token: "zugang", expires_in: 3600 })
        : reply(400, { error: "invalid_grant" });
    }
    const headers = new Headers(init?.headers);
    if (headers.get("Authorization") !== "Bearer zugang") {
      return reply(401, { error: "unauthenticated" });
    }
    if (rejectedRequest?.method === method) {
      const { status, reason } = rejectedRequest;
      rejectedRequest = undefined;
      return reply(status, { error: { errors: [{ reason }] } });
    }
    const id = /\/files\/([^/]+)$/.exec(url.pathname)?.[1];

    if (method === "GET" && url.pathname === "/drive/v3/files") {
      const parent = /^'([^']+)' in parents and trashed = false$/.exec(
        url.searchParams.get("q") ?? "",
      )?.[1];
      return reply(200, {
        files: [...files]
          .filter(([, file]) => file.parent === parent && !file.trashed)
          .map(([fileId, file]) => ({
            id: fileId,
            name: file.name,
            mimeType: file.mimeType,
          })),
      });
    }
    if (method === "POST" && url.pathname === "/drive/v3/files") {
      const meta = JSON.parse(String(init?.body)) as {
        name: string;
        mimeType: string;
        parents: string[];
      };
      const fileId = `ordner-${nextId++}`;
      files.set(fileId, {
        name: meta.name,
        mimeType: meta.mimeType,
        parent: meta.parents[0]!,
        data: "",
        trashed: false,
      });
      return reply(200, { id: fileId });
    }
    if (method === "POST" && url.pathname === "/upload/drive/v3/files") {
      const boundary = /boundary=(.+)$/.exec(
        headers.get("Content-Type") ?? "",
      )?.[1];
      const body = Buffer.from(init?.body as Uint8Array).toString("utf8");
      const [metaPart, dataPart] = body
        .split(`--${boundary}`)
        .slice(1, 3)
        .map((part) => part.slice(part.indexOf("\r\n\r\n") + 4, -2));
      const meta = JSON.parse(metaPart!) as {
        name: string;
        mimeType: string;
        parents: string[];
      };
      const fileId = `datei-${nextId++}`;
      files.set(fileId, {
        name: meta.name,
        mimeType: meta.mimeType,
        parent: meta.parents[0]!,
        data: dataPart!,
        trashed: false,
      });
      if (createdButFailed > 0) {
        createdButFailed--;
        return reply(500, { error: "backendError" });
      }
      return reply(200, { id: fileId });
    }
    const file = id ? files.get(id) : undefined;
    if (!file) return reply(404, { error: "notFound" });
    if (method === "GET" && url.searchParams.get("alt") === "media") {
      return new Response(file.data, { status: 200 });
    }
    if (method === "PATCH" && url.pathname.startsWith("/upload/")) {
      file.data = Buffer.from(init?.body as Uint8Array).toString("utf8");
      return reply(200, { id });
    }
    if (method === "PATCH") {
      Object.assign(file, JSON.parse(String(init?.body)));
      return reply(200, { id });
    }
    return reply(400, { error: `${method} ${url.pathname}` });
  };

  function walk(parent: string, prefix: string): Snapshot {
    const result: Snapshot = {};
    for (const [fileId, file] of files) {
      if (file.parent !== parent || file.trashed) continue;
      const path = prefix ? `${prefix}/${file.name}` : file.name;
      if (file.mimeType === FOLDER_MIME) {
        Object.assign(result, walk(fileId, path));
      } else {
        if (path in result) throw new Error(`Doppelte Datei ${path}`);
        result[path] = { identity: fileId, content: file.data };
      }
    }
    return result;
  }

  return {
    publisher: drivePublisher({
      credentials: {
        clientId: "client",
        clientSecret: "secret",
        refreshToken: "refresh",
      },
      folderId: root,
      fetch: fakeFetch as typeof fetch,
    }),
    snapshot: async () => walk(root, ""),
  };
}

function contents(snap: Snapshot): Record<string, string> {
  return Object.fromEntries(
    Object.entries(snap).map(([path, file]) => [path, file.content]),
  );
}

function under(snap: Snapshot, folder: string): Snapshot {
  return Object.fromEntries(
    Object.entries(snap)
      .filter(([path]) => path.startsWith(`${folder}/`))
      .map(([path, file]) => [path.slice(folder.length + 1), file]),
  );
}

function topFolders(snap: Snapshot): string[] {
  return [
    ...new Set(Object.keys(snap).map((path) => path.split("/")[0]!)),
  ].sort();
}

describe.each([
  ["Dateisystem", filesystemTarget],
  ["Google Drive", async () => driveTarget()],
] as const)("Upload-Adapter %s", (_name, createTarget) => {
  test("legt den Versionsordner und Aktuell mit der Ausgabe an", async () => {
    const target = await createTarget();
    const outDir = await writeOutput("1.0.0", {
      anwesenheit: { "macbook.png": "anwesenheit 1" },
      "eltern-kinder": { "iphone.png": "eltern 1" },
    });

    await target.publisher.publish(outDir, {
      version: "1.0.0",
      date: "2026-10-01",
    });

    const snap = await target.snapshot();
    const expected = {
      "anwesenheit/macbook.png": "anwesenheit 1",
      "eltern-kinder/iphone.png": "eltern 1",
      "manifest.json": await readFile(join(outDir, "manifest.json"), "utf8"),
    };
    expect(topFolders(snap)).toEqual(["Aktuell", "v1.0.0 (2026-10-01)"]);
    expect(contents(under(snap, "v1.0.0 (2026-10-01)"))).toEqual(expected);
    expect(contents(under(snap, "Aktuell"))).toEqual(expected);
  });

  test("überschreibt Aktuell mit derselben Datei-Identität", async () => {
    const target = await createTarget();
    await target.publisher.publish(
      await writeOutput("1.0.0", { anwesenheit: { "macbook.png": "alt" } }),
      { version: "1.0.0", date: "2026-10-01" },
    );
    const before = under(await target.snapshot(), "Aktuell");

    await target.publisher.publish(
      await writeOutput("1.1.0", { anwesenheit: { "macbook.png": "neu" } }),
      { version: "1.1.0", date: "2026-10-08" },
    );

    const after = under(await target.snapshot(), "Aktuell");
    expect(after["anwesenheit/macbook.png"]).toEqual({
      identity: before["anwesenheit/macbook.png"]!.identity,
      content: "neu",
    });
    expect(after["manifest.json"]!.identity).toBe(
      before["manifest.json"]!.identity,
    );
  });

  test("ein entfernter Shot fehlt in Aktuell, alte Versionsordner bleiben unverändert", async () => {
    const target = await createTarget();
    await target.publisher.publish(
      await writeOutput("1.0.0", {
        anwesenheit: { "macbook.png": "anwesenheit 1" },
        raeume: { "ipad.png": "raeume 1" },
      }),
      { version: "1.0.0", date: "2026-10-01" },
    );
    const oldVersion = under(await target.snapshot(), "v1.0.0 (2026-10-01)");

    const outDir = await writeOutput("1.1.0", {
      anwesenheit: { "macbook.png": "anwesenheit 2" },
      "eltern-kinder": { "iphone.png": "eltern 2" },
    });
    await target.publisher.publish(outDir, {
      version: "1.1.0",
      date: "2026-10-08",
    });

    const snap = await target.snapshot();
    const expected = {
      "anwesenheit/macbook.png": "anwesenheit 2",
      "eltern-kinder/iphone.png": "eltern 2",
      "manifest.json": await readFile(join(outDir, "manifest.json"), "utf8"),
    };
    expect(contents(under(snap, "Aktuell"))).toEqual(expected);
    expect(contents(under(snap, "v1.1.0 (2026-10-08)"))).toEqual(expected);
    expect(under(snap, "v1.0.0 (2026-10-01)")).toEqual(oldVersion);
  });

  test("ein erneuter Lauf derselben Version gleicht ihren Ordner an", async () => {
    const target = await createTarget();
    await target.publisher.publish(
      await writeOutput("1.0.0", { anwesenheit: { "macbook.png": "erster" } }),
      { version: "1.0.0", date: "2026-10-01" },
    );

    await target.publisher.publish(
      await writeOutput("1.0.0", {
        anwesenheit: { "macbook.png": "zweiter" },
      }),
      { version: "1.0.0", date: "2026-10-02" },
    );

    const snap = await target.snapshot();
    expect(topFolders(snap)).toEqual(["Aktuell", "v1.0.0 (2026-10-01)"]);
    expect(contents(snap)["v1.0.0 (2026-10-01)/anwesenheit/macbook.png"]).toBe(
      "zweiter",
    );
    expect(contents(snap)["Aktuell/anwesenheit/macbook.png"]).toBe("zweiter");
  });

  test.each([
    ["1.10.0", "1.9.0"],
    ["2.0.0", "1.99.99"],
    ["1.1.10", "1.1.9"],
    ["1.0.0", "1.0.0-rc.1"],
    ["1.0.0-rc.10", "1.0.0-rc.9"],
    ["1.0.0-beta", "1.0.0-alpha"],
    ["1.0.0-alpha.beta", "1.0.0-alpha.1"],
    ["1.0.0-alpha.1", "1.0.0-alpha"],
  ])(
    "Version %s bleibt aktuell beim Nachholen von %s",
    async (newer, older) => {
      const target = await createTarget();
      await target.publisher.publish(
        await writeOutput(newer, {
          anwesenheit: { "macbook.png": "neu" },
          raeume: { "ipad.png": "neuer Shot" },
        }),
        { version: newer, date: "2026-10-01" },
      );
      const before = await target.snapshot();
      const oldOutput = await writeOutput(older, {
        anwesenheit: { "macbook.png": "alt" },
      });
      await target.publisher.publish(oldOutput, {
        version: older,
        date: "2026-10-02",
      });
      const after = await target.snapshot();
      expect(under(after, "Aktuell")).toEqual(under(before, "Aktuell"));
      expect(under(after, `v${newer} (2026-10-01)`)).toEqual(
        under(before, `v${newer} (2026-10-01)`),
      );
      expect(contents(under(after, `v${older} (2026-10-02)`))).toEqual({
        "anwesenheit/macbook.png": "alt",
        "manifest.json": await readFile(
          join(oldOutput, "manifest.json"),
          "utf8",
        ),
      });
    },
  );

  test("eine unvollständige oder fremde Ausgabe lässt das Ziel unverändert", async () => {
    const target = await createTarget();
    await target.publisher.publish(
      await writeOutput("1.0.0", { anwesenheit: { "macbook.png": "gut" } }),
      { version: "1.0.0", date: "2026-10-01" },
    );
    const before = await target.snapshot();

    const missingFile = await writeOutput("1.1.0", {
      anwesenheit: { "macbook.png": "neu" },
    });
    await rm(join(missingFile, "anwesenheit", "macbook.png"));
    const unlisted = await writeOutput("1.1.0", {
      anwesenheit: { "macbook.png": "neu" },
    });
    await writeFile(join(unlisted, "notizen.txt"), "fremd");
    const otherVersion = await writeOutput("1.2.0", {
      anwesenheit: { "macbook.png": "neu" },
    });
    const release = { version: "1.1.0", date: "2026-10-08" };

    await expect(
      target.publisher.publish(missingFile, release),
    ).rejects.toThrow(/gelisteten Dateien/);
    await expect(target.publisher.publish(unlisted, release)).rejects.toThrow(
      /notizen\.txt/,
    );
    await expect(
      target.publisher.publish(otherVersion, release),
    ).rejects.toThrow(/1\.2\.0/);
    await expect(
      target.publisher.publish(join(missingFile, "fehlt"), release),
    ).rejects.toThrow();
    expect(await target.snapshot()).toEqual(before);
  });
});

test("ein ungültiges Refresh-Token nennt den Wizard", async () => {
  const publisher = drivePublisher({
    credentials: {
      clientId: "client",
      clientSecret: "secret",
      refreshToken: "widerrufen",
    },
    folderId: "zielordner",
    fetch: (async () =>
      new Response(JSON.stringify({ error: "invalid_grant" }), {
        status: 400,
      })) as typeof fetch,
  });

  await expect(
    publisher.publish(
      await writeOutput("1.0.0", { anwesenheit: { "macbook.png": "bild" } }),
      { version: "1.0.0", date: "2026-10-01" },
    ),
  ).rejects.toThrow(/invalid_grant.*product-screenshots-drive-wizard\.sh/);
});

test("Drive: ein Serverfehler nach dem Anlegen erzeugt beim erneuten Lauf kein Duplikat", async () => {
  const target = driveTarget({ createdButFailed: 1 });
  const outDir = await writeOutput("1.0.0", {
    anwesenheit: { "macbook.png": "bild" },
  });
  const release = { version: "1.0.0", date: "2026-10-01" };

  await expect(target.publisher.publish(outDir, release)).rejects.toThrow(
    /500/,
  );
  await target.publisher.publish(outDir, release);

  const snap = await target.snapshot();
  expect(contents(under(snap, "Aktuell"))["anwesenheit/macbook.png"]).toBe(
    "bild",
  );
  expect(
    contents(under(snap, "v1.0.0 (2026-10-01)"))["anwesenheit/macbook.png"],
  ).toBe("bild");
});

test.each(["rateLimitExceeded", "userRateLimitExceeded"])(
  "Drive: %s wiederholt abgewiesene Lese-, Anlege- und Update-Anfragen",
  async (reason) => {
    for (const method of ["GET", "POST", "PATCH"]) {
      const target = driveTarget({
        rejectedRequest: { method, status: 403, reason },
      });
      const release = { version: "1.0.0", date: "2026-10-01" };
      await target.publisher.publish(
        await writeOutput("1.0.0", {
          anwesenheit: { "macbook.png": "erster" },
        }),
        release,
      );
      await target.publisher.publish(
        await writeOutput("1.0.0", {
          anwesenheit: { "macbook.png": "zweiter" },
        }),
        release,
      );
      const snap = await target.snapshot();
      for (const folder of ["Aktuell", "v1.0.0 (2026-10-01)"]) {
        expect(contents(under(snap, folder))["anwesenheit/macbook.png"]).toBe(
          "zweiter",
        );
      }
    }
  },
);

test("Drive: andere 403-Fehler werden nicht wiederholt", async () => {
  const target = driveTarget({
    rejectedRequest: {
      method: "POST",
      status: 403,
      reason: "insufficientFilePermissions",
    },
  });
  await expect(
    target.publisher.publish(
      await writeOutput("1.0.0", { anwesenheit: { "macbook.png": "bild" } }),
      { version: "1.0.0", date: "2026-10-01" },
    ),
  ).rejects.toThrow(/403.*insufficientFilePermissions/);
  expect(await target.snapshot()).toEqual({});
});

test.each(["kein JSON", "null", '{"version":"fremd"}'])(
  "ein ungültiges aktuelles Manifest schützt Aktuell vor Änderungen: %s",
  async (manifest) => {
    const root = await tempDir();
    const publisher = filesystemPublisher(root);
    await publisher.publish(
      await writeOutput("1.0.0", { anwesenheit: { "macbook.png": "alt" } }),
      { version: "1.0.0", date: "2026-10-01" },
    );
    await writeFile(join(root, "Aktuell", "manifest.json"), manifest);
    await expect(
      publisher.publish(
        await writeOutput("1.1.0", { anwesenheit: { "macbook.png": "neu" } }),
        { version: "1.1.0", date: "2026-10-02" },
      ),
    ).rejects.toThrow();
    expect(await readFile(join(root, "Aktuell", "manifest.json"), "utf8")).toBe(
      manifest,
    );
    expect(
      await readFile(
        join(root, "Aktuell", "anwesenheit", "macbook.png"),
        "utf8",
      ),
    ).toBe("alt");
  },
);
