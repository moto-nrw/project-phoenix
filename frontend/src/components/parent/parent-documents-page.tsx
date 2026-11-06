"use client";

/**
 * Dokumente (#4671): the documents the Träger of the family's OGS sent to
 * this account: the confirmation letters of holiday registrations and the
 * letters of their Storno and Gutschrift (#4690), and the care contracts
 * (#4678), and the documents the Träger released for the parents, such as a
 * house rule (#4692).
 * Each one downloads as the stored file. The list only reads; the Träger
 * decides what is sent or released.
 */

import { type ReactNode, useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { Button } from "~/components/ui/button";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { Skeleton } from "~/components/ui/skeleton";
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
    <ParentPage>
      <ParentPageHeader title={t("title")} description={t("description")} />
      {error ? <LoadErrorAlert error={error} /> : null}
      {documents === null ? (
        <div className="space-y-3" aria-busy="true">
          <Skeleton className="h-36 w-full rounded-2xl" />
          <Skeleton className="h-36 w-full rounded-2xl" />
        </div>
      ) : documents.length === 0 ? (
        error ? null : (
          <EmptyState
            icon={<MotoConceptIcon concept={documentsConcept} />}
            title={t("emptyTitle")}
            description={t("emptyDescription")}
          />
        )
      ) : (
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
      )}
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
  children,
}: Readonly<{
  concept: MotoConcept;
  title: string;
  description: string;
  downloadLabel: string;
  onDownload: () => Promise<void>;
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
        <Button
          type="button"
          variant="primary"
          size="md"
          onClick={() => void download()}
          disabled={downloading}
        >
          {downloading ? t("downloading") : downloadLabel}
        </Button>
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
    >
      <DataGrid>
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
    </InboxDocumentCard>
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
