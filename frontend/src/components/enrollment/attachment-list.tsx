"use client";

import { useState } from "react";
import { Download, ExternalLink } from "lucide-react";
import { FileTypeIcon } from "~/components/ui/file-type-icon";
import { useTranslations } from "next-intl";
import { Button } from "~/components/ui/button";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  ApiError,
  apiErrorFromResponse,
  transportFetch,
} from "~/lib/api-error";
import { downloadBlob } from "~/lib/file-download";
import type { FormAttachment } from "~/lib/enrollment-form-schema-api";

/** Keep the request on the existing anonymous PDF route, including in saved
 * admin previews whose schema still carries canonical upload URLs. */
function publicDocumentURL(url: string): string {
  const match =
    /^(?:\/uploads\/|\/api\/public\/)(enrollment-form-legal-documents)\/([\w.-]+\.pdf)$/i.exec(
      url,
    );
  if (!match) throw new ApiError("Invalid enrollment PDF URL", 400);
  return `/api/public/${match[1]}/${encodeURIComponent(match[2]!)}`;
}

const NO_ATTACHMENTS: FormAttachment[] = [];

export function AttachmentList({
  attachments = NO_ATTACHMENTS,
}: Readonly<{ attachments?: FormAttachment[] }>) {
  return attachments.length > 0 ? (
    <AttachmentActions attachments={attachments} />
  ) : null;
}

function AttachmentActions({
  attachments,
}: Readonly<{ attachments: FormAttachment[] }>) {
  const t = useTranslations("enrollmentForm.attachments");
  const { show, showMessage } = useApiErrorDisplay();
  const [busyURL, setBusyURL] = useState<string | null>(null);

  async function read(attachment: FormAttachment, open: boolean) {
    const tab = open ? window.open("about:blank", "_blank") : null;
    if (open && !tab) {
      showMessage(t("popupBlocked"));
      return;
    }
    if (tab) {
      tab.opener = null;
      tab.document.title = attachment.label;
    }
    setBusyURL(attachment.document_url);
    try {
      const response = await transportFetch(
        publicDocumentURL(attachment.document_url),
        { cache: "no-store" },
      );
      if (!response.ok)
        throw await apiErrorFromResponse(
          response,
          "Enrollment PDF could not be loaded",
        );
      if (!response.headers.get("content-type")?.includes("application/pdf"))
        throw new ApiError("Expected enrollment PDF", 500);
      const blob = await response.blob();
      if (tab) {
        const url = URL.createObjectURL(blob);
        tab.location.href = url;
        // PDF viewers provide no reliable load event. Release after the viewer
        // has consumed the blob; downloadBlob releases its own URL separately.
        window.setTimeout(() => URL.revokeObjectURL(url), 60_000);
      } else {
        downloadBlob(
          blob,
          `${attachment.label.replace(/[\\/:*?"<>|]/g, "_").replace(/\.pdf$/i, "")}.pdf`,
        );
      }
    } catch (error) {
      tab?.close();
      void show(error, { object: t("errorObject") });
    } finally {
      setBusyURL(null);
    }
  }

  if (attachments.length === 0) return null;
  return (
    <ul className="mt-3 space-y-2">
      {attachments.map((attachment) => (
        <li
          key={attachment.document_url}
          className="moto-content-surface flex flex-wrap items-center gap-x-4 gap-y-2 rounded-xl border px-3 py-2 shadow-sm"
        >
          <div className="flex min-w-0 flex-1 basis-48 items-center gap-3">
            <FileTypeIcon
              contentType="application/pdf"
              className="h-5 w-5 shrink-0 text-gray-500"
            />
            <p className="min-w-0 text-sm font-medium break-words text-gray-900">
              {attachment.label}
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="gap-2"
              disabled={busyURL !== null}
              aria-label={t("openLabel", { label: attachment.label })}
              onClick={() => void read(attachment, true)}
            >
              <ExternalLink className="h-4 w-4" aria-hidden="true" />
              {t("open")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="md"
              className="gap-2"
              disabled={busyURL !== null}
              aria-label={t("downloadLabel", { label: attachment.label })}
              onClick={() => void read(attachment, false)}
            >
              <Download className="h-4 w-4" aria-hidden="true" />
              {t("download")}
            </Button>
          </div>
        </li>
      ))}
    </ul>
  );
}
