"use client";

import { useState } from "react";
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
    <ul className="mt-3 space-y-3">
      {attachments.map((attachment) => (
        <li key={attachment.document_url} className="space-y-1">
          <p className="text-sm font-medium text-gray-900">
            {attachment.label}
          </p>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={busyURL !== null}
              aria-label={t("openLabel", { label: attachment.label })}
              onClick={() => void read(attachment, true)}
            >
              {t("open")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={busyURL !== null}
              aria-label={t("downloadLabel", { label: attachment.label })}
              onClick={() => void read(attachment, false)}
            >
              {t("download")}
            </Button>
          </div>
        </li>
      ))}
    </ul>
  );
}
