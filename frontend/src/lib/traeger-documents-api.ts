/**
 * Dokumente, die ein Träger mit Eltern und Schulen teilt (#4692).
 *
 * Der Träger lädt eine vorhandene Datei hoch und gibt sie frei: für die Eltern
 * aller Schulen, einer Schule oder einer Gruppe, oder für die Schulen. Eine
 * Freigabe lässt sich zurücknehmen. Die OGS-Leitung liest auf „Dateien“, was
 * für ihre Schule freigegeben ist. Backend-IDs werden hier zu Strings.
 */
import { apiErrorFromBody, transportFetch } from "~/lib/api-error";
import { filenameFromDisposition } from "~/lib/file-download";
import { readJson, traegerRequest } from "~/lib/traeger-api";

/** Größte Datei, die ein Träger teilt; das Backend prüft sie ebenso. */
export const TRAEGER_DOCUMENT_MAX_BYTES = 10 * 1024 * 1024;

/** Die Dateiendungen, die das Backend annimmt (Inhalt wird mitgeprüft). */
export const TRAEGER_DOCUMENT_ACCEPT =
  ".pdf,.docx,.xlsx,.pptx,.png,.jpg,.jpeg,application/pdf,image/png,image/jpeg";

export type TraegerDocumentAudience = "parents" | "schools";

export interface TraegerDocumentRelease {
  readonly id: string;
  readonly audience: TraegerDocumentAudience;
  /** `null`: alle Schulen des Trägers. */
  readonly schoolId: string | null;
  readonly groupId: string | null;
  /** Name der Gruppe bei der Freigabe. */
  readonly groupName: string;
  readonly releasedAt: string;
}

export interface TraegerDocument {
  readonly id: string;
  readonly title: string;
  readonly fileName: string;
  readonly contentType: string;
  readonly sizeBytes: number;
  readonly createdAt: string;
  readonly releases: readonly TraegerDocumentRelease[];
  /** Wie oft Eltern und Schulen das Dokument geöffnet haben. */
  readonly reads: number;
}

/** Ein Dokument, wie die Schule es sieht. */
export interface SharedTraegerDocument {
  readonly id: string;
  readonly title: string;
  readonly fileName: string;
  readonly contentType: string;
  readonly sizeBytes: number;
  readonly releasedAt: string;
  readonly releases: readonly TraegerDocumentRelease[];
}

export interface TraegerDocumentGroup {
  readonly id: string;
  readonly name: string;
}

export interface TraegerDocumentSchool {
  readonly id: string;
  readonly name: string;
  readonly groups: readonly TraegerDocumentGroup[];
}

export interface TraegerDocumentReleaseInput {
  readonly audience: TraegerDocumentAudience;
  readonly schoolId: string | null;
  readonly groupId: string | null;
}

interface BackendRelease {
  id: number;
  audience: TraegerDocumentAudience;
  school_id: number | null;
  group_id: number | null;
  group_name: string;
  released_at: string;
}

interface BackendDocument {
  id: number;
  title: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
  releases: BackendRelease[] | null;
  reads: number;
}

interface BackendSharedDocument {
  id: number;
  title: string;
  file_name: string;
  content_type: string;
  size_bytes: number;
  released_at: string;
  releases: BackendRelease[] | null;
}

interface BackendSchool {
  id: number;
  name: string;
  groups: { id: number; name: string }[] | null;
}

const optionalId = (id: number | null): string | null =>
  id === null ? null : String(id);

export function mapTraegerDocumentRelease(
  release: BackendRelease,
): TraegerDocumentRelease {
  return {
    id: String(release.id),
    audience: release.audience,
    schoolId: optionalId(release.school_id),
    groupId: optionalId(release.group_id),
    groupName: release.group_name ?? "",
    releasedAt: release.released_at,
  };
}

function mapDocument(document: BackendDocument): TraegerDocument {
  return {
    id: String(document.id),
    title: document.title,
    fileName: document.file_name,
    contentType: document.content_type,
    sizeBytes: document.size_bytes,
    createdAt: document.created_at,
    releases: (document.releases ?? []).map(mapTraegerDocumentRelease),
    reads: document.reads,
  };
}

function mapSharedDocument(
  document: BackendSharedDocument,
): SharedTraegerDocument {
  return {
    id: String(document.id),
    title: document.title,
    fileName: document.file_name,
    contentType: document.content_type,
    sizeBytes: document.size_bytes,
    releasedAt: document.released_at,
    releases: (document.releases ?? []).map(mapTraegerDocumentRelease),
  };
}

const traegerDocumentsPath = "/api/traeger/documents";

/** Die Dokumente des Trägers, das neueste zuerst. */
export async function fetchTraegerDocuments(): Promise<TraegerDocument[]> {
  const documents = await traegerRequest<BackendDocument[] | null>(
    traegerDocumentsPath,
  );
  return (documents ?? []).map(mapDocument);
}

/** Ein Dokument des Trägers aus der Liste; `null`, wenn es fehlt. */
export async function fetchTraegerDocument(
  id: string,
): Promise<TraegerDocument | null> {
  const documents = await fetchTraegerDocuments();
  return documents.find((document) => document.id === id) ?? null;
}

/** Die Schulen des Trägers mit ihren Gruppen, für eine Freigabe. */
export async function fetchTraegerDocumentAudiences(): Promise<
  TraegerDocumentSchool[]
> {
  const schools = await traegerRequest<BackendSchool[] | null>(
    `${traegerDocumentsPath}/audiences`,
  );
  return (schools ?? []).map((school) => ({
    id: String(school.id),
    name: school.name,
    groups: (school.groups ?? []).map((group) => ({
      id: String(group.id),
      name: group.name,
    })),
  }));
}

/** Lädt eine Datei mit Titel hoch. Wirft einen ApiError bei Ablehnung. */
export async function uploadTraegerDocument(
  title: string,
  file: File,
): Promise<TraegerDocument> {
  const form = new FormData();
  form.append("title", title);
  form.append("file", file);
  const response = await transportFetch(traegerDocumentsPath, {
    method: "POST",
    credentials: "include",
    cache: "no-store",
    body: form,
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  const document = (data as { data?: BackendDocument } | null)?.data;
  if (!document) {
    throw apiErrorFromBody("Document response without data", 502, data);
  }
  return mapDocument(document);
}

/** Gibt ein Dokument frei. Wirft einen ApiError bei Ablehnung. */
export async function releaseTraegerDocument(
  documentId: string,
  input: TraegerDocumentReleaseInput,
): Promise<TraegerDocumentRelease> {
  const release = await traegerRequest<BackendRelease>(
    `${traegerDocumentsPath}/${encodeURIComponent(documentId)}/releases`,
    {
      method: "POST",
      body: {
        audience: input.audience,
        school_id: input.schoolId === null ? null : Number(input.schoolId),
        group_id: input.groupId === null ? null : Number(input.groupId),
      },
    },
  );
  return mapTraegerDocumentRelease(release);
}

/** Nimmt eine Freigabe zurück. Wirft einen ApiError bei Ablehnung. */
export async function revokeTraegerDocumentRelease(
  documentId: string,
  releaseId: string,
): Promise<void> {
  await traegerRequest(
    `${traegerDocumentsPath}/${encodeURIComponent(documentId)}/releases/${encodeURIComponent(releaseId)}`,
    { method: "DELETE" },
  );
}

async function downloadFrom(
  path: string,
): Promise<{ readonly blob: Blob; readonly filename: string | null }> {
  const response = await transportFetch(path, {
    credentials: "include",
    cache: "no-store",
  });
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      await readJson(response),
    );
  }
  return {
    blob: await response.blob(),
    filename: filenameFromDisposition(response),
  };
}

/** Die Datei eines Dokuments des Trägers. */
export function downloadTraegerDocument(documentId: string) {
  return downloadFrom(
    `${traegerDocumentsPath}/${encodeURIComponent(documentId)}/file`,
  );
}

const schoolDocumentsPath = "/api/traeger-documents";

/** Die Dokumente, die der Träger für die eigene Schule freigegeben hat. */
export async function fetchSchoolTraegerDocuments(): Promise<
  SharedTraegerDocument[]
> {
  const response = await transportFetch(schoolDocumentsPath, {
    credentials: "include",
    cache: "no-store",
  });
  const data = await readJson(response);
  if (!response.ok) {
    throw apiErrorFromBody(
      `Request failed (${response.status})`,
      response.status,
      data,
    );
  }
  const documents =
    (data as { data?: BackendSharedDocument[] | null } | null)?.data ?? [];
  return documents.map(mapSharedDocument);
}

/** Die Datei eines Dokuments, das der Träger für die Schule freigab. */
export function downloadSchoolTraegerDocument(documentId: string) {
  return downloadFrom(
    `${schoolDocumentsPath}/${encodeURIComponent(documentId)}/file`,
  );
}
