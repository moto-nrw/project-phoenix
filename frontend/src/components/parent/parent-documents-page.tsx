"use client";

/**
 * Dokumente (#4671): the documents the Träger of the family's OGS sent to
 * this account, today the confirmation letters of holiday registrations.
 * Each one downloads as the stored PDF. The list only reads; the Träger
 * decides what is sent.
 */

import { useCallback, useEffect, useState } from "react";
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
import {
  downloadMyDocument,
  listMyDocuments,
  type ParentDocument,
} from "~/lib/parent-api";

export function ParentDocumentsPage() {
  const t = useTranslations("parentDocuments");
  const [documents, setDocuments] = useState<ParentDocument[] | null>(null);
  const { error, show: showLoadError, clear } = useApiLoadError();

  const load = useCallback(async () => {
    clear();
    try {
      setDocuments(await listMyDocuments());
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
      {documents === null ? (
        <div className="space-y-3" aria-busy="true">
          <Skeleton className="h-36 w-full rounded-2xl" />
          <Skeleton className="h-36 w-full rounded-2xl" />
        </div>
      ) : error ? (
        <LoadErrorAlert error={error} />
      ) : documents.length === 0 ? (
        <EmptyState
          icon={<MotoConceptIcon concept="documents" />}
          title={t("emptyTitle")}
          description={t("emptyDescription")}
        />
      ) : (
        <ul className="space-y-4">
          {documents.map((document) => (
            <li key={`${document.organization_id}-${document.id}`}>
              <ParentDocumentCard document={document} />
            </li>
          ))}
        </ul>
      )}
    </ParentPage>
  );
}

function ParentDocumentCard({
  document,
}: Readonly<{ document: ParentDocument }>) {
  const t = useTranslations("parentDocuments");
  const locale = useLocale();
  const { show } = useApiErrorDisplay();
  const [downloading, setDownloading] = useState(false);
  const amount = new Intl.NumberFormat(locale, {
    style: "currency",
    currency: "EUR",
  }).format(document.total_cents / 100);

  const download = async () => {
    setDownloading(true);
    try {
      await downloadMyDocument(document);
    } catch (err) {
      void show(err, { object: t("downloadErrorObject") });
    } finally {
      setDownloading(false);
    }
  };

  return (
    <ParentSection
      level={2}
      concept="documents"
      title={t("confirmationLetter", { period: document.period_name })}
      description={t("sender", { traeger: document.traeger_name })}
      actions={
        <Button
          type="button"
          variant="secondary"
          size="md"
          onClick={() => void download()}
          disabled={downloading}
        >
          {downloading ? t("downloading") : t("download")}
        </Button>
      }
    >
      <DataGrid>
        <DataField label={t("children")}>
          {document.child_names.join(", ")}
        </DataField>
        <DataField label={t("amount")}>{amount}</DataField>
        <DataField label={t("number")}>{document.number}</DataField>
        <DataField label={t("sentAt")}>
          {formatDate(document.sent_at, false, locale)}
        </DataField>
      </DataGrid>
    </ParentSection>
  );
}
