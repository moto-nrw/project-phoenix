/**
 * Fotos und Dateien an Nachrichten (#6685), für alle drei Chats und beide
 * Portale. Eine Datei wird zuerst hochgeladen; die Nachricht nennt dann die
 * IDs ihrer Uploads. Die Grenzen hier spiegeln das Backend
 * (modules/communication/message_attachments.go), das sie erneut prüft.
 */
import {
  ApiError,
  apiErrorFromResponse,
  transportFetch,
} from "~/lib/api-error";

/** Ein Anhang, wie ihn jede Unterhaltung liefert (IDs sind Strings). */
export interface MessageAttachment {
  readonly id: string;
  readonly filename: string;
  readonly size_bytes: number;
  readonly content_type: string;
  /** Pixelgröße eines Bildes, damit sein Platz vor dem Laden feststeht. */
  readonly width?: number;
  readonly height?: number;
}

export const MAX_MESSAGE_ATTACHMENTS = 5;
export const MAX_MESSAGE_ATTACHMENT_MB = 10;
const MAX_MESSAGE_ATTACHMENT_BYTES = MAX_MESSAGE_ATTACHMENT_MB * 1024 * 1024;

/** Endungen, die der Dateidialog anbietet und der Upload annimmt. */
export const MESSAGE_ATTACHMENT_ACCEPT =
  ".png,.jpg,.jpeg,.pdf,.docx,.xlsx,.pptx";
const ACCEPTED_EXTENSIONS = MESSAGE_ATTACHMENT_ACCEPT.split(",");

export function isImageAttachment(contentType: string): boolean {
  return contentType === "image/png" || contentType === "image/jpeg";
}

/** Warum eine gewählte Datei nicht angehängt wird. */
export type AttachmentRefusal =
  | { readonly kind: "too_many" }
  | { readonly kind: "too_large"; readonly filename: string }
  | { readonly kind: "type"; readonly filename: string };

/**
 * Teilt die gewählten Dateien in die, die angehängt werden, und die Gründe
 * für den Rest. `held` ist die Zahl der Anhänge, die die Nachricht schon hat.
 */
export function checkAttachmentFiles(
  held: number,
  files: readonly File[],
): { accepted: File[]; refusals: AttachmentRefusal[] } {
  const accepted: File[] = [];
  const refusals: AttachmentRefusal[] = [];
  for (const file of files) {
    const name = file.name.toLowerCase();
    if (!ACCEPTED_EXTENSIONS.some((extension) => name.endsWith(extension))) {
      refusals.push({ kind: "type", filename: file.name });
    } else if (file.size > MAX_MESSAGE_ATTACHMENT_BYTES) {
      refusals.push({ kind: "too_large", filename: file.name });
    } else if (held + accepted.length >= MAX_MESSAGE_ATTACHMENTS) {
      if (!refusals.some((refusal) => refusal.kind === "too_many")) {
        refusals.push({ kind: "too_many" });
      }
    } else {
      accepted.push(file);
    }
  }
  return { accepted, refusals };
}

/** Wo ein Chat seine Anhänge hochlädt und abruft. */
export interface MessageAttachmentRoutes {
  readonly uploadUrl: string;
  readonly downloadUrl: (attachmentId: string) => string;
}

/** Routen unter einer Basis mit `/attachments` und `/…/download`. */
export function messageAttachmentRoutes(base: string): MessageAttachmentRoutes {
  return {
    uploadUrl: `${base}/attachments`,
    downloadUrl: (attachmentId) =>
      `${base}/attachments/${encodeURIComponent(attachmentId)}/download`,
  };
}

export const PARENT_MESSAGE_ATTACHMENTS = messageAttachmentRoutes("/api/messages");
export const STAFF_MESSAGE_ATTACHMENTS = messageAttachmentRoutes(
  "/api/staff-messages",
);
export const SCHOOL_STAFF_MESSAGE_ATTACHMENTS = messageAttachmentRoutes(
  "/api/school/staff-messages",
);

/** Die Anhänge der Eltern hängen am Kind, über das sie schreiben. */
export function guardianMessageAttachments(
  studentId: string,
): MessageAttachmentRoutes {
  return messageAttachmentRoutes(
    `/api/parent/me/messages/children/${encodeURIComponent(studentId)}`,
  );
}

/** Adresse, unter der der Browser ein Bild oder PDF anzeigt statt speichert. */
export function inlineAttachmentUrl(downloadUrl: string): string {
  return `${downloadUrl}?inline=1`;
}

/** Lädt eine Datei hoch und liefert den Anhang für die nächste Nachricht. */
export async function uploadMessageAttachment(
  uploadUrl: string,
  file: File,
  signal?: AbortSignal,
): Promise<MessageAttachment> {
  const form = new FormData();
  form.append("file", file, file.name);
  const response = await transportFetch(uploadUrl, {
    method: "POST",
    body: form,
    signal,
  });
  if (!response.ok) {
    throw await apiErrorFromResponse(
      response,
      "Die Datei konnte nicht hochgeladen werden",
    );
  }
  const body = (await response.json()) as { data?: MessageAttachment };
  if (!body.data?.id) {
    throw new ApiError("Die Datei konnte nicht hochgeladen werden", 500);
  }
  return body.data;
}

/**
 * Die IDs als letztes Argument eines Sende-Aufrufs: ohne Anhänge entfällt
 * es, damit eine Nachricht nur mit Text genau wie bisher gesendet wird.
 */
export function attachmentIdArgs(
  attachments: readonly MessageAttachment[],
): [] | [string[]] {
  return attachments.length === 0
    ? []
    : [attachments.map((attachment) => attachment.id)];
}
