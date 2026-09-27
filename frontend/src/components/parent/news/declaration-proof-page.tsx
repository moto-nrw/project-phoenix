"use client";

/**
 * Nachweis einer Erklärung (#3430): was dieses Konto für ein Kind erklärt hat,
 * mit dem vollständigen Text der Fassung, den Prüfsummen und dem eigenen
 * Verlauf. Zum Lesen auf dem Handy und zum Drucken; „als PDF speichern“ ist
 * der Druckdialog des Browsers.
 */

import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useLocale, useTranslations } from "next-intl";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { PrintButton, PrintDocument } from "~/components/ui/print-document";
import {
  ParentPage,
  ParentPageHeader,
  ParentPageSkeleton,
} from "~/components/parent/parent-page";
import { formatChatDateTime } from "~/lib/date-helpers";
import { formatBytes } from "~/lib/files-api";
import { createLogger } from "~/lib/logger";
import {
  ParentApiError,
  fetchDeclarationProof,
  type ParentDeclarationProof,
} from "~/lib/parent-api";
import { parentPath } from "~/lib/parent-url";

const logger = createLogger({ component: "ParentDeclarationProof" });

const SIGNER_ROLES = new Set([
  "primary_guardian",
  "legal_guardian",
  "co_guardian",
]);

const HASH = "font-mono text-xs break-all text-gray-800";

export function DeclarationProofPage({
  announcementId,
}: Readonly<{ announcementId: string }>) {
  const t = useTranslations("parentDeclarationProof");
  const studentId = useSearchParams().get("student") ?? "";
  const [proof, setProof] = useState<ParentDeclarationProof | null>(null);
  const [failure, setFailure] = useState<"notFound" | "error" | null>(null);
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let active = true;
    setProof(null);
    setFailure(null);
    if (!studentId) {
      setFailure("notFound");
      return;
    }
    fetchDeclarationProof(announcementId, studentId)
      .then((data) => {
        if (active) setProof(data);
      })
      .catch((err: unknown) => {
        if (!active) return;
        logger.warn("parent_declaration_proof_load_failed", {
          status: err instanceof ParentApiError ? err.status : undefined,
        });
        setFailure(
          err instanceof ParentApiError && err.status === 404
            ? "notFound"
            : "error",
        );
      });
    return () => {
      active = false;
    };
  }, [announcementId, studentId, reload]);

  const retry = useCallback(() => setReload((n) => n + 1), []);

  return (
    <ParentPage>
      <ParentPageHeader
        title={t("title")}
        description={proof ? proof.title : undefined}
        backHref={parentPath(
          `/parents/news?brief=${encodeURIComponent(announcementId)}`,
        )}
        backLabel={t("back")}
        actions={
          proof ? <PrintButton label={t("print")} size="touch" /> : undefined
        }
      />

      {failure === "notFound" && <Alert type="info" message={t("notFound")} />}
      {failure === "error" && (
        <Alert
          type="error"
          message={t("loadError")}
          action={
            <Button type="button" variant="outline" size="md" onClick={retry}>
              {t("retry")}
            </Button>
          }
        />
      )}
      {!proof && !failure && <ParentPageSkeleton rows={2} />}

      {proof && (
        <div className="moto-content-surface rounded-2xl border p-5 shadow-sm">
          <PrintDocument>
            <ProofDocument proof={proof} />
          </PrintDocument>
        </div>
      )}
    </ParentPage>
  );
}

function ProofDocument({ proof }: Readonly<{ proof: ParentDeclarationProof }>) {
  const t = useTranslations("parentDeclarationProof");
  const td = useTranslations("parentDeclaration");
  const tRoles = useTranslations("parentChildDetail.guardians.roles");
  const locale = useLocale();
  const at = (iso: string) => formatChatDateTime(iso, locale);
  // Only guardians with custody can declare; other roles have no label here.
  const role = (value: string) =>
    SIGNER_ROLES.has(value) ? tRoles(value) : null;

  return (
    <article className="space-y-6 text-base leading-7 text-gray-900 print:text-[11pt] print:leading-6">
      <header className="space-y-1">
        <h2 className="hidden text-xl font-semibold print:block">
          {t("title")}
        </h2>
        <p className="text-sm text-gray-600">
          {t("generated", { date: at(proof.generated_at) })}
        </p>
      </header>

      <dl className="grid grid-cols-1 gap-x-4 gap-y-1 text-sm sm:grid-cols-[max-content_1fr]">
        <dt className="text-gray-600">{t("child")}</dt>
        <dd className="font-semibold">{proof.child_name}</dd>
        <dt className="text-gray-600">{t("school")}</dt>
        <dd>{proof.school_name}</dd>
        <dt className="text-gray-600">{t("declaration")}</dt>
        <dd>{proof.title}</dd>
        <dt className="text-gray-600">{t("kind")}</dt>
        <dd>
          {proof.kind === "consent" ? t("kindConsent") : t("kindAcknowledge")}
        </dd>
        <dt className="text-gray-600">{t("method")}</dt>
        <dd>{proof.method_label}</dd>
      </dl>

      <section className="space-y-3">
        <h3 className="text-lg font-semibold">{t("historyTitle")}</h3>
        <ol className="space-y-3">
          {proof.submissions.map((entry) => (
            <li
              key={entry.id}
              className="break-inside-avoid rounded-xl bg-gray-50 px-4 py-3 text-sm print:border print:border-gray-300 print:bg-white"
            >
              <p className="text-base font-semibold">
                {t("entry", {
                  action: td(`done.${entry.action}`),
                  date: at(entry.submitted_at),
                })}
              </p>
              <p>
                {t("entryBy", { name: entry.signer_name })}
                {role(entry.guardian_role)
                  ? ` (${role(entry.guardian_role)})`
                  : ""}
              </p>
              <p>
                {t("entryVersion", { version: entry.version_no })} ·{" "}
                {entry.password_confirmed ? t("passwordYes") : t("passwordNo")}
              </p>
              <p className="mt-1 text-gray-600">{t("versionChecksum")}</p>
              <p className={HASH}>{entry.content_hash}</p>
              <p className="mt-1 text-gray-600">{t("recordChecksum")}</p>
              <p className={HASH}>{entry.record_hash}</p>
            </li>
          ))}
        </ol>
      </section>

      <section className="space-y-4">
        <h3 className="text-lg font-semibold">{t("versionsTitle")}</h3>
        {proof.versions.map((version) => (
          <div
            key={version.id}
            className="space-y-2 border-t border-gray-200 pt-3"
          >
            <p className="text-sm font-semibold text-gray-700">
              {t("versionHeading", {
                version: version.version_no,
                date: at(version.published_at),
              })}
            </p>
            <p className="font-semibold">{version.title}</p>
            <p className="whitespace-pre-line">{version.body}</p>
            <p className="text-sm text-gray-600">{t("checksum")}</p>
            <p className={HASH}>{version.content_hash}</p>
            {version.attachments.length > 0 && (
              <div className="text-sm">
                <p className="text-gray-600">{t("attachments")}</p>
                <ul className="mt-1 space-y-2">
                  {version.attachments.map((file) => (
                    <li key={`${file.filename}-${file.sha256}`}>
                      <span className="block">
                        {file.filename} ({formatBytes(file.size_bytes)})
                      </span>
                      <span className={`block ${HASH}`}>
                        SHA-256: {file.sha256}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        ))}
      </section>

      <footer className="border-t border-gray-200 pt-3 text-sm text-gray-600">
        {t("note")}
      </footer>
    </article>
  );
}
