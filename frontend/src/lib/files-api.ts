// Dateiablage (#2596): API client + type mapping for the school file
// storage. The backend decides everything about authority (folder
// visibility, files:manage, files.staff_upload_enabled) and tells the UI what
// the caller may do via can_manage / can_upload / can_delete — the page
// renders exactly that, no client-side authority.

import { getCachedSession, sessionFetch } from "./session-cache";

export type FolderVisibility = "all_staff" | "admins" | "selected";

export const FOLDER_VISIBILITY_LABELS: Record<FolderVisibility, string> = {
  all_staff: "Alle Mitarbeitenden",
  admins: "Nur Leitung",
  selected: "Ausgewählte Rollen und Personen",
};

export interface FileFolder {
  id: string;
  name: string;
  visibility: FolderVisibility;
  fileCount: number;
  roleIds: string[];
  accountIds: string[];
  createdAt: string;
}

export interface FolderOverview {
  folders: FileFolder[];
  canManage: boolean;
  canUpload: boolean;
  staffUploadEnabled: boolean;
  usedBytes: number;
  maxBytes: number;
}

export interface StoredFile {
  id: string;
  folderId: string;
  filename: string;
  sizeBytes: number;
  contentType: string;
  uploadedAt: string;
  uploadedBy: string;
  canDelete: boolean;
}

export interface FolderFiles {
  folder: FileFolder;
  files: StoredFile[];
}

interface AudienceRole {
  id: string;
  name: string;
}

interface AudienceAccount {
  accountId: string;
  firstName: string;
  lastName: string;
}

export interface AudienceOptions {
  roles: AudienceRole[];
  accounts: AudienceAccount[];
}

interface FolderInput {
  name: string;
  visibility: FolderVisibility;
  roleIds: string[];
  accountIds: string[];
}

// Ids arrive as decimal strings: they are PostgreSQL bigints, and JSON.parse
// rounds anything past Number.MAX_SAFE_INTEGER silently — a rounded id
// addresses a different folder or file.
interface BackendFolder {
  id: string;
  name: string;
  visibility: FolderVisibility;
  file_count: number;
  role_ids: string[];
  account_ids: string[];
  created_at: string;
}

interface BackendFolderList {
  folders: BackendFolder[];
  can_manage: boolean;
  can_upload: boolean;
  staff_upload_enabled: boolean;
  used_bytes: number;
  max_bytes: number;
}

interface BackendFile {
  id: string;
  folder_id: string;
  filename: string;
  size_bytes: number;
  content_type: string;
  uploaded_at: string;
  uploaded_by: string;
  can_delete: boolean;
}

interface BackendFolderFiles {
  folder: BackendFolder;
  files: BackendFile[];
}

interface BackendAudience {
  roles: { id: string; name: string }[];
  accounts: { account_id: string; first_name: string; last_name: string }[];
}

function mapFolder(data: BackendFolder): FileFolder {
  return {
    id: data.id,
    name: data.name,
    visibility: data.visibility,
    fileCount: data.file_count,
    roleIds: data.role_ids ?? [],
    accountIds: data.account_ids ?? [],
    createdAt: data.created_at,
  };
}

function mapFile(data: BackendFile): StoredFile {
  return {
    id: data.id,
    folderId: data.folder_id,
    filename: data.filename,
    sizeBytes: data.size_bytes,
    contentType: data.content_type,
    uploadedAt: data.uploaded_at,
    uploadedBy: data.uploaded_by,
    canDelete: data.can_delete,
  };
}

class FilesApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string,
  ) {
    super(message);
    this.name = "FilesApiError";
  }
}

/**
 * The wording of a failed request, decided here rather than taken from the
 * response. Backend validation errors are English sentinels with an English
 * detail appended ("invalid file storage request: name is required") and the
 * page shows this message unchanged, so the raw `error` field is never used.
 * A stable `code` or the status decides; everything else keeps the wording
 * the calling method passes in.
 */
function filesErrorMessage(
  code: string | undefined,
  status: number,
): string | null {
  if (code === "folder_name_taken") {
    return "Es gibt schon einen Ordner mit diesem Namen.";
  }
  if (code === "quota_exceeded") {
    return "Der Speicherplatz der Dateiablage ist voll. Bitte erst Dateien löschen.";
  }
  switch (status) {
    case 401:
      return "Ihre Sitzung ist abgelaufen. Bitte melden Sie sich erneut an.";
    case 403:
      return "Dafür fehlt Ihnen die Berechtigung.";
    case 413:
      return "Die Datei ist zu groß.";
    default:
      return null;
  }
}

async function throwFilesError(
  response: Response,
  fallback: string,
): Promise<never> {
  let code: string | undefined;
  try {
    const body = (await response.json()) as { code?: string };
    code = body.code;
  } catch {
    // Non-JSON body — the fallback carries the wording.
  }
  const message = filesErrorMessage(code, response.status) ?? fallback;
  throw new FilesApiError(message, response.status, code);
}

function toBackendFolderInput(input: FolderInput) {
  return {
    name: input.name,
    visibility: input.visibility,
    role_ids: input.roleIds,
    account_ids: input.accountIds,
  };
}

class FilesService {
  async listFolders(): Promise<FolderOverview> {
    const response = await sessionFetch("/api/files/folders");
    if (!response.ok) {
      await throwFilesError(response, "Ordner konnten nicht geladen werden.");
    }
    const json = (await response.json()) as { data: BackendFolderList };
    return {
      folders: json.data.folders.map(mapFolder),
      canManage: json.data.can_manage,
      canUpload: json.data.can_upload,
      staffUploadEnabled: json.data.staff_upload_enabled,
      usedBytes: json.data.used_bytes,
      maxBytes: json.data.max_bytes,
    };
  }

  async createFolder(input: FolderInput): Promise<FileFolder> {
    const response = await sessionFetch("/api/files/folders", {
      method: "POST",
      body: JSON.stringify(toBackendFolderInput(input)),
    });
    if (!response.ok) {
      await throwFilesError(response, "Ordner konnte nicht angelegt werden.");
    }
    const json = (await response.json()) as { data: BackendFolder };
    return mapFolder(json.data);
  }

  async updateFolder(
    folderId: string,
    input: FolderInput,
  ): Promise<FileFolder> {
    const response = await sessionFetch(`/api/files/folders/${folderId}`, {
      method: "PUT",
      body: JSON.stringify(toBackendFolderInput(input)),
    });
    if (!response.ok) {
      await throwFilesError(
        response,
        "Ordner konnte nicht gespeichert werden.",
      );
    }
    const json = (await response.json()) as { data: BackendFolder };
    return mapFolder(json.data);
  }

  async deleteFolder(folderId: string): Promise<void> {
    const response = await sessionFetch(`/api/files/folders/${folderId}`, {
      method: "DELETE",
    });
    if (!response.ok) {
      await throwFilesError(response, "Ordner konnte nicht gelöscht werden.");
    }
  }

  async listAudience(): Promise<AudienceOptions> {
    const response = await sessionFetch("/api/files/audience");
    if (!response.ok) {
      await throwFilesError(
        response,
        "Rollen und Personen konnten nicht geladen werden.",
      );
    }
    const json = (await response.json()) as { data: BackendAudience };
    return {
      roles: json.data.roles.map((role) => ({
        id: role.id,
        name: role.name,
      })),
      accounts: json.data.accounts.map((account) => ({
        accountId: account.account_id,
        firstName: account.first_name,
        lastName: account.last_name,
      })),
    };
  }

  async listFiles(folderId: string): Promise<FolderFiles> {
    const response = await sessionFetch(`/api/files/folders/${folderId}/files`);
    if (!response.ok) {
      await throwFilesError(response, "Dateien konnten nicht geladen werden.");
    }
    const json = (await response.json()) as { data: BackendFolderFiles };
    return {
      folder: mapFolder(json.data.folder),
      files: json.data.files.map(mapFile),
    };
  }

  async upload(folderId: string, file: File): Promise<void> {
    const formData = new FormData();
    formData.append("file", file);

    const session = await getCachedSession();
    const token = session?.user?.token;
    if (!token) {
      throw new Error("Authentifizierung erforderlich");
    }

    // Raw fetch on purpose — sessionFetch forces Content-Type
    // application/json, which would clobber the multipart boundary.
    const response = await fetch(`/api/files/folders/${folderId}/files`, {
      method: "POST",
      body: formData,
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!response.ok) {
      await throwFilesError(
        response,
        "Die Datei konnte nicht hochgeladen werden. Erlaubt sind PDF, DOCX, XLSX, PPTX, PNG und JPEG bis 25 MB.",
      );
    }
  }

  async deleteFile(folderId: string, fileId: string): Promise<void> {
    const response = await sessionFetch(
      `/api/files/folders/${folderId}/files/${fileId}`,
      { method: "DELETE" },
    );
    if (!response.ok) {
      await throwFilesError(response, "Datei konnte nicht gelöscht werden.");
    }
  }

  /** Same-origin proxy URL for the authenticated download. */
  downloadUrl(folderId: string, fileId: string): string {
    return `/api/files/folders/${folderId}/files/${fileId}/download`;
  }

  /** Same URL, but the browser shows the file (PDF, image) instead of saving it. */
  viewUrl(folderId: string, fileId: string): string {
    return `${this.downloadUrl(folderId, fileId)}?inline=1`;
  }
}

/** Browsers render these themselves; office files always download. */
export function isViewableInBrowser(contentType: string): boolean {
  return (
    contentType === "application/pdf" ||
    contentType === "image/png" ||
    contentType === "image/jpeg" ||
    contentType === "image/jpg"
  );
}

export const filesService = new FilesService();

/** Human-readable size for the quota line and the file table. */
export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)} KB`;
  if (bytes < 1024 * 1024 * 1024) {
    return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  }
  return `${(bytes / (1024 * 1024 * 1024)).toFixed(2)} GB`;
}
