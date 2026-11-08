"use client";

/**
 * Dokumente (#4671): the documents the Träger of the family's OGS sent to
 * this account: the confirmation letters of holiday registrations and the
 * letters of their Storno and Gutschrift (#4690), and the care contracts
 * (#4678), and the documents the Träger released for the parents, such as a
 * house rule (#4692).
 * Each one downloads as the stored file. The Träger decides what is sent or
 * released; a care contract that waits for the signature links to the page
 * where the parents sign it (#4679).
 */

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { StatusBadge } from "~/components/ui/status-badge";
import { useApiErrorDisplay, useApiLoadError } from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { formatBytes } from "~/lib/files-api";
import {
  downloadMyDocument,
  downloadMySharedDocument,
  listMyDocuments,
  type ParentDocument,
  type ParentSharedDocument,
} from "~/lib/parent-api";
import { careContractsConcept } from "~/lib/concepts/careContracts";
import { documentsConcept } from "~/lib/concepts/documents";
import { parentPath } from "~/lib/parent-url";
import type { MotoConcept } from "~/lib/moto-concepts";

/** The title of each kind of letter in the inbox (#4690). */
const DOCUMENT_TITLES = {
  confirmation_letter: "confirmationLetter",
  cancellation_letter: "cancellationLetter",
  credit_letter: "creditLetter",
  care_contract: "careContract",
} as const satisfies Record<ParentDocument["kind"], string>;

/** A row of the inbox: a letter or contract, or a released document. */
type InboxEntry =
  | {
      readonly type: "sent";
      readonly at: string;
      readonly document: ParentDocument;
    }
  | {
      readonly type: "shared";
      readonly at: string;
      readonly document: ParentSharedDocument;
    };

/** The failed source of the inbox, as the catalog object names it. */
const FAILED_OBJECTS = {
  letters: "lettersErrorObject",
  contracts: "contractsErrorObject",
  shared: "sharedErrorObject",
  several: "errorObject",
} as const;

function inboxEntries(
  documents: readonly ParentDocument[],
  shared: readonly ParentSharedDocument[],
): InboxEntry[] {
  return [
    ...documents.map((document): InboxEntry => ({
      type: "sent",
      at: document.sent_at,
      document,
    })),
    ...shared.map((document): InboxEntry => ({
      type: "shared",
      at: document.released_at,
      document,
    })),
  ].sort((a, b) => Date.parse(b.at) - Date.parse(a.at));
}

export function ParentDocumentsPage() {
  const t = useTranslations("parentDocuments");
  const [documents, setDocuments] = useState<InboxEntry[] | null>(null);
  const { error, show: showLoadError, clear } = useApiLoadError();

  const load = useCallback(async () => {
    clear();
    try {
      const result = await listMyDocuments();
      setDocuments(inboxEntries(result.documents, result.shared));
      if (result.failedSource) {
        void showLoadError(result.error, {
          object: t(FAILED_OBJECTS[result.failedSource]),
          retry: () => void load(),
        });
      }
    } catch (err) {
      setDocuments([]);
      void showLoadError(err, { object: t("errorObject") });
    }
  }, [clear, showLoadError, t]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <ParentPage
      header={
        <ParentPageHeader title={t("title")} description={t("description")} />
      }
      loading={documents === null}
      empty={
        documents?.length === 0 && !error
          ? {
              icon: <MotoConceptIcon concept={documentsConcept} />,
              title: t("emptyTitle"),
              description: t("emptyDescription"),
            }
          : null
      }
    >
      {error ? <LoadErrorAlert error={error} /> : null}
      {documents && documents.length > 0 ? (
        <ul className="space-y-4">
          {documents.map((entry) =>
            entry.type === "shared" ? (
              <li
                key={`shared-${entry.document.organization_id}-${entry.document.id}`}
              >
                <ParentSharedDocumentCard document={entry.document} />
              </li>
            ) : (
              <li
                key={`${entry.document.kind}-${entry.document.organization_id}-${entry.document.id}`}
              >
                <ParentDocumentCard document={entry.document} />
              </li>
            ),
          )}
        </ul>
      ) : null}
    </ParentPage>
  );
}

/**
 * One document of the inbox with its download: a letter, a care contract or
 * a document a Träger released. The kinds differ only in title and facts.
 */
function InboxDocumentCard({
  concept,
  title,
  description,
  downloadLabel,
  onDownload,
  signHref,
  children,
}: Readonly<{
  concept: MotoConcept;
  title: string;
  description: string;
  downloadLabel: string;
  onDownload: () => Promise<void>;
  /** A care contract waiting for the signature links here (#4679). */
  signHref?: string;
  children: ReactNode;
}>) {
  const t = useTranslations("parentDocuments");
  const { show } = useApiErrorDisplay();
  const [downloading, setDownloading] = useState(false);

  const download = async () => {
    setDownloading(true);
    try {
      await onDownload();
    } catch (err) {
      void show(err, { object: t("downloadErrorObject") });
    } finally {
      setDownloading(false);
    }
  };

  return (
    <ParentSection
      level={2}
      concept={concept}
      title={title}
      description={description}
      actions={
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant={signHref ? "outline" : "primary"}
            size="md"
            onClick={() => void download()}
            disabled={downloading}
          >
            {downloading ? t("downloading") : downloadLabel}
          </Button>
          {signHref ? (
            <ButtonLink href={signHref} variant="success" size="md">
              {t("signNow")}
            </ButtonLink>
          ) : null}
        </div>
      }
    >
      {children}
    </ParentSection>
  );
}

function ParentDocumentCard({
  document,
}: Readonly<{ document: ParentDocument }>) {
  const t = useTranslations("parentDocuments");
  const locale = useLocale();
  const contract = document.kind === "care_contract";
  const awaitingSignature =
    contract && document.contract_status === "versendet";
  const canSign = awaitingSignature && document.can_sign === true;
  const amount = new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "EUR",
  }).format(document.total_cents / 100);

  return (
    <InboxDocumentCard
      concept={contract ? careContractsConcept : documentsConcept}
      title={t(DOCUMENT_TITLES[document.kind], {
        period: document.period_name,
      })}
      description={t("sender", { traeger: document.traeger_name })}
      downloadLabel={t("download")}
      onDownload={() => downloadMyDocument(document)}
      signHref={
        canSign
          ? parentPath(
              `/documents/vertrag/${encodeURIComponent(document.organization_id)}/${encodeURIComponent(document.id)}`,
            )
          : undefined
      }
    >
      {contract && document.replaced ? (
        <div className="mb-4 space-y-2">
          <StatusBadge label={t("replaced")} tone="outline" />
          <Alert
            type="info"
            message={t("replacedNote", {
              date: formatDate(document.service_from, false, locale),
            })}
          />
        </div>
      ) : null}
      <DataGrid>
        {contract && document.contract_status ? (
          <DataField label={t("status")}>
            {awaitingSignature ? (
              <StatusBadge label={t("awaitingSignature")} tone="orange" />
            ) : document.signed_at ? (
              <StatusBadge
                label={`${t("signed")} · ${formatDate(document.signed_at, false, locale)}`}
                tone="green"
              />
            ) : (
              <StatusBadge label={t("concluded")} tone="green" />
            )}
          </DataField>
        ) : null}
        <DataField label={t("children")}>
          {document.child_names.join(", ")}
        </DataField>
        {contract ? (
          <>
            <DataField label={t("bookedCare")}>
              {document.period_name}
            </DataField>
            <DataField label={t("startsOn")}>
              {formatDate(document.service_from, false, locale)}
            </DataField>
          </>
        ) : (
          <>
            <DataField label={t("amount")}>{amount}</DataField>
            <DataField label={t("number")}>{document.number}</DataField>
          </>
        )}
        <DataField label={t("sentAt")}>
          {formatDate(document.sent_at, false, locale)}
        </DataField>
      </DataGrid>
      {contract ? <ContractNoticeRow document={document} /> : null}
    </InboxDocumentCard>
  );
}

/**
 * The way to end a care contract (#4682, § 312k BGB): the button „Verträge
 * hier kündigen“ on an active contract, or the end and the confirmation of
 * one the parents or the Träger ended.
 */
function ContractNoticeRow({
  document,
}: Readonly<{ document: ParentDocument }>) {
  const t = useTranslations("parentDocuments");
  const locale = useLocale();
  const href = `/documents/vertraege/${encodeURIComponent(document.organization_id)}/${encodeURIComponent(document.id)}`;
  const status = document.contract_status;
  if (status === "aktiv") {
    // Only where the declaration would pass: without the relationship right
    // the button would open a page that refuses (#4682).
    return (
      <div className="mt-4 border-t border-gray-100 pt-4">
        {document.can_end_contract ? (
          <div className="flex flex-wrap gap-3">
            <ButtonLink
              href={parentPath(
                `/documents/vertrag/${encodeURIComponent(document.organization_id)}/${encodeURIComponent(document.id)}`,
              )}
              variant="outline"
              size="md"
            >
              {t("familyChildren")}
            </ButtonLink>
            <ButtonLink href={href} variant="outline" size="md">
              {t("cancelContract")}
            </ButtonLink>
          </div>
        ) : (
          <p className="text-sm text-gray-500">{t("cancelNotPermitted")}</p>
        )}
      </div>
    );
  }
  if (status !== "gekuendigt" && status !== "widerrufen") return null;
  return (
    <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-t border-gray-100 pt-4">
      <StatusBadge
        label={
          status === "widerrufen"
            ? t("withdrawn")
            : document.ends_on
              ? t("terminatedOn", {
                  date: formatDate(document.ends_on, false, locale),
                })
              : t("terminationInReview")
        }
        tone={status === "widerrufen" ? "red" : "orange"}
      />
      <ButtonLink href={href} variant="ghost" size="md">
        {t("showConfirmation")}
      </ButtonLink>
    </div>
  );
}

/** The kind of a released file in one word (#4692). */
function fileKind(contentType: string): string {
  if (contentType === "application/pdf") return "PDF";
  if (contentType.includes("wordprocessingml")) return "Word";
  if (contentType.includes("spreadsheetml")) return "Excel";
  if (contentType.includes("presentationml")) return "PowerPoint";
  if (contentType.startsWith("image/")) return "image";
  return "";
}

/** A document a Träger released for the parents (#4692). */
function ParentSharedDocumentCard({
  document,
}: Readonly<{ document: ParentSharedDocument }>) {
  const t = useTranslations("parentDocuments");
  const locale = useLocale();
  const kind = fileKind(document.content_type);

  return (
    <InboxDocumentCard
      concept={documentsConcept}
      title={document.title}
      description={t("sharedSender", { traeger: document.traeger_name })}
      downloadLabel={t("downloadFile")}
      onDownload={() => downloadMySharedDocument(document)}
    >
      <DataGrid>
        <DataField label={t("file")}>
          {[
            kind === "image" ? t("image") : kind,
            formatBytes(document.size_bytes),
          ]
            .filter(Boolean)
            .join(" · ")}
        </DataField>
        <DataField label={t("releasedAt")}>
          {formatDate(document.released_at, false, locale)}
        </DataField>
      </DataGrid>
    </InboxDocumentCard>
  );
}
