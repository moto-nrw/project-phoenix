// Staff documents (#1424): API client + type mapping for the Dokumente tab
// on the staff detail page. The backend decides per-category authority
// (AU-Bescheinigung → staff_documents:health, Lohnabrechnung →
// staff:financial, rest → users:update) and returns only what the caller
// may see plus the caller's visible categories.

import { getCachedSession, sessionFetch } from "./session-cache";

export type StaffDocumentCategory =
  | "arbeitsvertrag"
  | "zeugnis"
  | "lohnabrechnung"
  | "bewerbung"
  | "au_bescheinigung"
  | "sonstiges";

export const staffDocumentCategoryLabels: Record<
  StaffDocumentCategory,
  string
> = {
  arbeitsvertrag: "Arbeitsvertrag",
  zeugnis: "Zeugnis / Qualifikation",
  lohnabrechnung: "Lohnabrechnung",
  bewerbung: "Bewerbungsunterlagen",
  au_bescheinigung: "AU-Bescheinigung",
  sonstiges: "Sonstiges",
};

export interface StaffDocument {
  id: string;
  staffId: string;
  category: StaffDocumentCategory;
  filename: string;
  sizeBytes: number;
  contentType: string;
  uploadedAt: string;
  /** "Aufbewahren bis" (YYYY-MM-DD) — advisory, nothing auto-deletes. */
  retainUntil: string | null;
  /** BDSG review date for Bewerbungsunterlagen (YYYY-MM-DD). */
  reviewDue: string | null;
}

export interface StaffDocumentList {
  documents: StaffDocument[];
  visibleCategories: StaffDocumentCategory[];
}

interface BackendStaffDocument {
  id: number;
  staff_id: number;
  category: StaffDocumentCategory;
  filename: string;
  size_bytes: number;
  content_type: string;
  uploaded_at: string;
  retain_until?: string | null;
  review_due?: string | null;
}

interface BackendStaffDocumentList {
  staff_id: number;
  documents: BackendStaffDocument[];
  visible_categories: StaffDocumentCategory[];
}

function mapDocument(data: BackendStaffDocument): StaffDocument {
  return {
    id: data.id.toString(),
    staffId: data.staff_id.toString(),
    category: data.category,
    filename: data.filename,
    sizeBytes: data.size_bytes,
    contentType: data.content_type,
    uploadedAt: data.uploaded_at,
    retainUntil: data.retain_until ?? null,
    reviewDue: data.review_due ?? null,
  };
}

async function throwDocumentError(
  response: Response,
  fallback: string,
): Promise<never> {
  let message = fallback;
  try {
    const body = (await response.json()) as { error?: string };
    if (body.error) {
      message = body.error;
    }
  } catch {
    // Non-JSON body — keep the fallback.
  }
  if (response.status === 403) {
    message = "Keine Berechtigung für diese Dokument-Kategorie.";
  }
  throw new Error(message);
}

class StaffDocumentsService {
  async list(staffId: string): Promise<StaffDocumentList> {
    const response = await sessionFetch(`/api/staff/${staffId}/documents`);
    if (!response.ok) {
      throw new Error(
        `Failed to fetch staff documents: ${response.statusText}`,
      );
    }
    const json = (await response.json()) as { data: BackendStaffDocumentList };
    return {
      documents: json.data.documents.map(mapDocument),
      visibleCategories: json.data.visible_categories,
    };
  }

  async upload(
    staffId: string,
    file: File,
    category: StaffDocumentCategory,
  ): Promise<void> {
    const formData = new FormData();
    formData.append("file", file);
    formData.append("category", category);

    const session = await getCachedSession();
    const token = session?.user?.token;
    if (!token) {
      throw new Error("Authentifizierung erforderlich");
    }

    // Raw fetch on purpose — sessionFetch forces Content-Type
    // application/json, which would clobber the multipart boundary the
    // browser sets for FormData bodies (same pattern as the student photo
    // upload).
    const response = await fetch(`/api/staff/${staffId}/documents`, {
      method: "POST",
      body: formData,
      headers: { Authorization: `Bearer ${token}` },
    });
    if (!response.ok) {
      await throwDocumentError(
        response,
        "Dokument konnte nicht hochgeladen werden.",
      );
    }
  }

  async delete(staffId: string, documentId: string): Promise<void> {
    const response = await sessionFetch(
      `/api/staff/${staffId}/documents/${documentId}`,
      { method: "DELETE" },
    );
    if (!response.ok) {
      await throwDocumentError(
        response,
        "Dokument konnte nicht gelöscht werden.",
      );
    }
  }

  /** Same-origin proxy URL for the authenticated download. */
  downloadUrl(staffId: string, documentId: string): string {
    return `/api/staff/${staffId}/documents/${documentId}/download`;
  }
}

export const staffDocumentsService = new StaffDocumentsService();

/** Formats a byte count for display: "12 KB", "1,4 MB". */
export function formatFileSize(bytes: number): string {
  if (bytes < 1024) {
    return `${bytes} B`;
  }
  if (bytes < 1024 * 1024) {
    return `${Math.round(bytes / 1024)} KB`;
  }
  return `${(bytes / (1024 * 1024)).toFixed(1).replace(".", ",")} MB`;
}
