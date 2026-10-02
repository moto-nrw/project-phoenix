import { randomUUID } from "node:crypto";
import { extname } from "node:path";

import { storePublisher, type Publisher, type StoreEntry } from "./publish";

// Upload-Adapter auf Google Drive (#3759), angemeldet per OAuth-Refresh-Token
// eines Benutzerkontos. Scope `drive.file`: der Adapter sieht nur Dateien, die
// diese App angelegt hat, also den Zielordner aus dem Einrichtungs-Wizard
// (scripts/product-screenshots-drive-wizard.sh) und alles darunter.
//
// Eine Datei in "Aktuell" bekommt bei jedem Release eine neue Dateiversion
// unter derselben Datei-ID; Links darauf bleiben gültig. Entfernte Shots
// landen im Papierkorb, nicht endgültig gelöscht.

const TOKEN_URL = "https://oauth2.googleapis.com/token";
const API = "https://www.googleapis.com/drive/v3/files";
const UPLOAD = "https://www.googleapis.com/upload/drive/v3/files";
const FOLDER_MIME = "application/vnd.google-apps.folder";
const DRIVE_ID = /^[A-Za-z0-9_-]+$/;
const MAX_ATTEMPTS = 5;

const MIME_TYPES: Record<string, string> = {
  ".png": "image/png",
  ".webp": "image/webp",
  ".json": "application/json",
};

interface DriveCredentials {
  readonly clientId: string;
  readonly clientSecret: string;
  readonly refreshToken: string;
}

export interface DrivePublisherOptions {
  readonly credentials: DriveCredentials;
  /** Zielordner, in dem "Aktuell" und die Versionsordner liegen. */
  readonly folderId: string;
  readonly log?: (message: string) => void;
  /** Für Tests: eine nachgebildete Drive-API. */
  readonly fetch?: typeof fetch;
}

interface DriveFile {
  readonly id: string;
  readonly name: string;
  readonly mimeType: string;
}

function mimeType(name: string): string {
  return MIME_TYPES[extname(name).toLowerCase()] ?? "application/octet-stream";
}

function driveId(id: string): string {
  if (!DRIVE_ID.test(id)) {
    throw new Error(`${JSON.stringify(id)} ist keine Drive-Datei-ID.`);
  }
  return id;
}

/** Ratenlimits und Serverfehler sind vorübergehend; alles andere nicht. */
function retryable(status: number, body: string): boolean {
  return (
    status === 429 ||
    status >= 500 ||
    (status === 403 && /rateLimitExceeded/.test(body))
  );
}

export function drivePublisher(options: DrivePublisherOptions): Publisher {
  const log = options.log ?? (() => undefined);
  const fetch = options.fetch ?? globalThis.fetch;
  let token: { value: string; expiresAt: number } | null = null;

  async function accessToken(): Promise<string> {
    if (token && Date.now() < token.expiresAt) return token.value;
    const response = await fetch(TOKEN_URL, {
      method: "POST",
      body: new URLSearchParams({
        client_id: options.credentials.clientId,
        client_secret: options.credentials.clientSecret,
        refresh_token: options.credentials.refreshToken,
        grant_type: "refresh_token",
      }),
    });
    const body = (await response.json().catch(() => ({}))) as {
      access_token?: string;
      expires_in?: number;
      error?: string;
      error_description?: string;
    };
    if (!response.ok || !body.access_token) {
      // invalid_grant: Token widerrufen oder abgelaufen (Wizard erneut ausführen).
      throw new Error(
        `Google-Anmeldung fehlgeschlagen (${response.status} ${body.error ?? ""}: ${body.error_description ?? ""}). Den Wizard scripts/product-screenshots-drive-wizard.sh erneut ausführen.`,
      );
    }
    token = {
      value: body.access_token,
      // Eine Minute Puffer vor dem Ablauf.
      expiresAt: Date.now() + ((body.expires_in ?? 3600) - 60) * 1000,
    };
    return token.value;
  }

  async function request(
    method: string,
    url: string,
    body?: { readonly type: string; readonly data: BodyInit },
  ): Promise<unknown> {
    for (let attempt = 1; ; attempt++) {
      const response = await fetch(url, {
        method,
        headers: {
          Authorization: `Bearer ${await accessToken()}`,
          ...(body ? { "Content-Type": body.type } : {}),
        },
        ...(body ? { body: body.data } : {}),
      });
      if (response.ok) return response.json();
      const text = await response.text();
      if (response.status === 401 && attempt < MAX_ATTEMPTS) {
        token = null;
        continue;
      }
      if (retryable(response.status, text) && attempt < MAX_ATTEMPTS) {
        const delay = 2 ** attempt * 500;
        log(`Drive ${response.status}, neuer Versuch in ${delay} ms`);
        await new Promise((resolve) => setTimeout(resolve, delay));
        continue;
      }
      throw new Error(
        `Drive ${method} ${new URL(url).pathname} fehlgeschlagen: ${response.status} ${text.slice(0, 500)}`,
      );
    }
  }

  function json(value: unknown) {
    return { type: "application/json", data: JSON.stringify(value) };
  }

  return storePublisher({
    rootId: driveId(options.folderId),
    async list(folderId) {
      const files: StoreEntry[] = [];
      let pageToken = "";
      do {
        const params = new URLSearchParams({
          q: `'${driveId(folderId)}' in parents and trashed = false`,
          fields: "nextPageToken, files(id, name, mimeType)",
          pageSize: "1000",
          supportsAllDrives: "true",
          includeItemsFromAllDrives: "true",
          ...(pageToken ? { pageToken } : {}),
        });
        const page = (await request("GET", `${API}?${params}`)) as {
          files?: DriveFile[];
          nextPageToken?: string;
        };
        for (const file of page.files ?? []) {
          files.push({
            id: file.id,
            name: file.name,
            folder: file.mimeType === FOLDER_MIME,
          });
        }
        pageToken = page.nextPageToken ?? "";
      } while (pageToken);
      return files;
    },
    async createFolder(parentId, name) {
      log(`Drive: Ordner ${name}`);
      const folder = (await request(
        "POST",
        `${API}?supportsAllDrives=true&fields=id`,
        json({ name, mimeType: FOLDER_MIME, parents: [driveId(parentId)] }),
      )) as { id: string };
      return driveId(folder.id);
    },
    async createFile(parentId, name, data) {
      const boundary = `moto-${randomUUID()}`;
      const metadata = JSON.stringify({
        name,
        mimeType: mimeType(name),
        parents: [driveId(parentId)],
      });
      const multipart = Buffer.concat([
        Buffer.from(
          `--${boundary}\r\nContent-Type: application/json; charset=UTF-8\r\n\r\n${metadata}\r\n` +
            `--${boundary}\r\nContent-Type: ${mimeType(name)}\r\n\r\n`,
        ),
        data,
        Buffer.from(`\r\n--${boundary}--\r\n`),
      ]);
      await request(
        "POST",
        `${UPLOAD}?uploadType=multipart&supportsAllDrives=true&fields=id`,
        { type: `multipart/related; boundary=${boundary}`, data: multipart },
      );
    },
    async updateFile(file, data) {
      await request(
        "PATCH",
        `${UPLOAD}/${driveId(file.id)}?uploadType=media&supportsAllDrives=true&fields=id`,
        // Kopie: fetch verlangt einen Puffer auf eigenem ArrayBuffer.
        { type: mimeType(file.name), data: new Uint8Array(data) },
      );
    },
    async remove(entry) {
      log(`Drive: ${entry.name} in den Papierkorb`);
      await request(
        "PATCH",
        `${API}/${driveId(entry.id)}?supportsAllDrives=true&fields=id`,
        json({ trashed: true }),
      );
    },
  });
}
