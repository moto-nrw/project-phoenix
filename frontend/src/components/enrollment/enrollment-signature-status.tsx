"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { Download } from "lucide-react";
import { useLocale, useTranslations } from "next-intl";
import { Button } from "~/components/ui/button";
import { Alert } from "~/components/ui/alert";
import { useApiErrorDisplay } from "~/contexts/ToastContext";
import {
  downloadEnrollmentSignatureProof,
  type EnrollmentSignature,
} from "~/lib/enrollment-submission-api";

/**
 * The parent's signature on the status page (#4786): who signed when, how,
 * and the proof as a PDF. Read-only; a change of the enrollment signs again
 * through the full form.
 */
export function EnrollmentSignatureStatus({
  token,
  signature,
  editable,
}: Readonly<{
  token: string;
  signature: EnrollmentSignature;
  /** The parents may still change the enrollment and then sign again. */
  editable: boolean;
}>) {
  const t = useTranslations("enrollmentStatus");
  const locale = useLocale();
  const { show } = useApiErrorDisplay();
  const [downloading, setDownloading] = useState(false);
  const signedAt = new Date(signature.signed_at).toLocaleString(locale, {
    timeZone: "Europe/Berlin",
    day: "2-digit",
    month: "long",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });

  const download = async () => {
    setDownloading(true);
    try {
      await downloadEnrollmentSignatureProof(token);
    } catch (err: unknown) {
      void show(err, {
        object: t("errorObjectProof"),
        retry: () => void downloadRef.current(),
      });
    } finally {
      setDownloading(false);
    }
  };
  // The retry runs the latest download, not the one of the failed attempt.
  const downloadRef = useRef(download);
  useLayoutEffect(() => {
    downloadRef.current = download;
  });

  return (
    <section className="moto-content-surface space-y-4 rounded-xl border p-5 shadow-sm sm:p-6">
      <div>
        <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
          {t("signatureEyebrow")}
        </p>
        <h2 className="mt-1 text-xl font-semibold text-gray-900">
          {t("signatureTitle")}
        </h2>
      </div>
      <p className="text-sm leading-6 text-gray-700">
        {t("signatureSigned", { name: signature.signer_name, date: signedAt })}{" "}
        {signature.method === "typed"
          ? t("signatureTyped", { name: signature.typed_name ?? "" })
          : t("signatureDrawn")}
      </p>
      {editable ? (
        <p className="text-sm leading-6 text-gray-600">
          {t("signatureEditHint")}
        </p>
      ) : null}
      {signature.integrity_ok ? null : (
        <Alert type="warning" message={t("signatureChanged")} />
      )}
      <Button
        type="button"
        variant="outline"
        size="md"
        className="gap-1.5"
        onClick={() => void download()}
        isLoading={downloading}
        loadingText={t("signatureDownloading")}
        disabled={downloading}
      >
        <Download className="h-4 w-4" aria-hidden="true" />
        {t("signatureDownload")}
      </Button>
    </section>
  );
}
