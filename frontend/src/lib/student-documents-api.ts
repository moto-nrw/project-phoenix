// Child documents (#777): API client + type mapping for the Dokumente tab on
// the child detail page. The backend decides per-category authority (Attest,
// Impfnachweis, Medikamentenplan → student_documents:health, Sorgerecht →
// student_documents:legal, rest → users:update) and returns only what the
// caller may see plus the caller's visible categories.

import { ApiError, apiErrorFromResponse, transportFetch } from "./api-error";
import { getCachedSession, sessionFetch } from "./session-cache";

interface StudentDocumentCategoryOption {
  value: string;
  label: string;
  /** Health or custody paperwork: every download is logged. */
  sensitive: boolean;
}

export interface StudentDocument {
  id: string;
  studentId: string;
  category: string;
  categoryLabel: string;
  filename: string;
  sizeBytes: number;
  contentType: string;
  uploadedAt: string;
  sensitive: boolean;
}

export interface StudentDocumentList {
  documents: StudentDocument[];
  visibleCategories: StudentDocumentCategoryOption[];
}

interface BackendStudentDocument {
  id: number;
  student_id: number;
  category: string;
  category_label: string;
  filename: string;
  size_bytes: number;
  content_type: string;
  uploaded_at: string;
  sensitive: boolean;
}

interface BackendStudentDocumentCategory {
  value: string;
  label: string;
  sensitive: boolean;
}

interface BackendStudentDocumentList {
  student_id: number;
  documents: BackendStudentDocument[];
  visible_categories: BackendStudentDocumentCategory[];
}

function mapDocument(data: BackendStudentDocument): StudentDocument {
  return {
    id: data.id.toString(),
    studentId: data.student_id.toString(),
    category: data.category,
    categoryLabel: data.category_label,
    filename: data.filename,
    sizeBytes: data.size_bytes,
    contentType: data.content_type,
    uploadedAt: data.uploaded_at,
    sensitive: data.sensitive,
  };
}

async function throwDocumentError(
  response: Response,
  fallback: string,
): Promise<never> {
  throw await apiErrorFromResponse(response, fallback);
}

class StudentDocumentsService {
  async list(studentId: string): Promise<StudentDocumentList> {
    const response = await sessionFetch(`/api/students/${studentId}/documents`);
    if (!response.ok) {
      await throwDocumentError(response, "Failed to fetch student documents");
    }
    const json = (await response.json()) as {
      data: BackendStudentDocumentList;
    };
    return {
      documents: json.data.documents.map(mapDocument),
      visibleCategories: json.data.visible_categories,
    };
  }

  async upload(studentId: string, file: File, category: string): Promise<void> {
    const formData = new FormData();
    formData.append("file", file);
    formData.append("category", category);

    const session = await getCachedSession();
    const token = session?.user?.token;
    if (!token) {
      // Ohne Sitzung führt der gemeinsame Fehlerweg zur Anmeldung.
      throw new ApiError("Authentifizierung erforderlich", 401);
    }

    // Raw fetch on purpose — sessionFetch forces Content-Type
    // application/json, which would clobber the multipart boundary the
    // browser sets for FormData bodies (same pattern as the staff document
    // and student photo uploads).
    const response = await transportFetch(
      `/api/students/${studentId}/documents`,
      {
        method: "POST",
        body: formData,
        headers: { Authorization: `Bearer ${token}` },
      },
    );
    if (!response.ok) {
      await throwDocumentError(
        response,
        "Dokument konnte nicht hochgeladen werden.",
      );
    }
  }

  async delete(studentId: string, documentId: string): Promise<void> {
    const response = await sessionFetch(
      `/api/students/${studentId}/documents/${documentId}`,
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
  downloadUrl(studentId: string, documentId: string): string {
    return `/api/students/${studentId}/documents/${documentId}/download`;
  }
}

export const studentDocumentsService = new StudentDocumentsService();
