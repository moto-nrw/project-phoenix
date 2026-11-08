"use client";

// Die Dokumentart eines Laufs (#4688): Vor der Freigabe die Art, die die
// Freigabe festhält, danach die festgehaltene. Fehlt dem Gebührenbescheid
// die Rechtsbehelfsbelehrung, sagt der Hinweis, was zu tun ist.

import { Alert } from "~/components/ui/alert";
import { ButtonLink } from "~/components/ui/button";
import {
  DOCUMENT_KIND_LABEL,
  type DocumentKind,
} from "~/lib/traeger-document-kinds-api";
import { traegerPath } from "~/lib/traeger-url";

export function runDocumentKindLabel(kind: string | undefined): string {
  return kind && kind in DOCUMENT_KIND_LABEL
    ? DOCUMENT_KIND_LABEL[kind as DocumentKind]
    : "–";
}

/** Der Hinweis vor der Freigabe, wenn die Belehrung fehlt. */
export function RunLegalRemedyNotice({
  missing,
}: Readonly<{ missing?: boolean }>) {
  if (!missing) return null;
  return (
    <Alert
      type="warning"
      title="Rechtsbehelfsbelehrung fehlt"
      message="Dieser Lauf stellt Gebührenbescheide aus. Tragen Sie zuerst die Rechtsbehelfsbelehrung in der Vorlage „Bescheid“ ein. Danach können Sie freigeben."
      action={
        <ButtonLink
          href={traegerPath(
            "/traeger/einstellungen/briefkopf/vorlagen/bescheid",
          )}
          variant="surface"
          size="compact"
        >
          Vorlage öffnen
        </ButtonLink>
      }
    />
  );
}
