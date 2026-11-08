"use client";

// Die Dokumentart eines Laufs (#4688): Vor der Freigabe die Art, die die
// Freigabe festhält, danach die festgehaltene.

import {
  DOCUMENT_KIND_LABEL,
  type DocumentKind,
} from "~/lib/traeger-document-kinds-api";

export function runDocumentKindLabel(kind: string | undefined): string {
  return kind && kind in DOCUMENT_KIND_LABEL
    ? DOCUMENT_KIND_LABEL[kind as DocumentKind]
    : "–";
}
