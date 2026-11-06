"use client";

/**
 * Dokumente (#4671): the documents the Träger of the family's OGS sent to
 * this account: the confirmation letters of holiday registrations and the
 * letters of their Storno and Gutschrift (#4690), and the care contracts
 * (#4678).
 * Each one downloads as the stored PDF. The list only reads; the Träger
 * decides what is sent.
 */

import { useCallback, useEffect, useState } from "react";
import { useLocale, useTranslations } from "next-intl";

import { ParentPage, ParentPageHeader } from "~/components/parent/parent-page";
import { ParentSection } from "~/components/parent/shell/parent-section";
import { Button, ButtonLink } from "~/components/ui/button";
import { StatusBadge } from "~/components/ui/status-badge";
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
import { careContractsConcept } from "~/lib/concepts/careContracts";
import { parentPath } from "~/lib/parent-url";
import { documentsConcept } from "~/lib/concepts/documents";

/** The title of each kind of letter in the inbox (#4690). */
const DOCUMENT_TITLES = {
  confirmation_letter: "confirmationLetter",
  cancellation_letter: "cancellationLetter",
  credit_letter: "creditLetter",
  care_contract: "careContract",
} as const satisfies Record<ParentDocument["kind"], string>;

export function ParentDocumentsPage() {
  const t = useTranslations("parentDocuments");
  const [documents, setDocuments] = useState<ParentDocument[] | null>(null);
  const { error, show: showLoadError, clear } = useApiLoadError();

  const load = useCallback(async () => {
    clear();
    try {
      const result = await listMyDocuments();
      setDocuments(result.documents);
      if (result.failedSource) {
        void showLoadError(result.error, {
          object: t(
            result.failedSource === "letters"
              ? "lettersErrorObject"
              : "contractsErrorObject",
          ),
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
          {documents.map((document) => (
            <li
              key={`${document.kind}-${document.organization_id}-${document.id}`}
            >
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
  const awaitingSignature =
    document.kind === "care_contract" &&
    document.contract_status === "versendet";
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
      concept={
        document.kind === "care_contract"
          ? careContractsConcept
          : documentsConcept
      }
      title={t(DOCUMENT_TITLES[document.kind], {
        period: document.period_name,
      })}
      description={t("sender", { traeger: document.traeger_name })}
      actions={
        <div className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant={awaitingSignature ? "outline" : "primary"}
            size="md"
            onClick={() => void download()}
            disabled={downloading}
          >
            {downloading ? t("downloading") : t("download")}
          </Button>
          {awaitingSignature ? (
            // Ein versendeter Vertrag wartet auf die Unterschrift (#4679).
            <ButtonLink
              href={parentPath(
                `/documents/vertrag/${encodeURIComponent(document.organization_id)}/${encodeURIComponent(document.id)}`,
              )}
              variant="success"
              size="md"
            >
              {t("signNow")}
            </ButtonLink>
          ) : null}
        </div>
      }
    >
      {document.kind === "care_contract" ? (
        <DataGrid>
          {document.contract_status ? (
            <DataField label={t("status")}>
              {awaitingSignature ? (
                <StatusBadge label={t("awaitingSignature")} tone="orange" />
              ) : document.signed_at ? (
                <StatusBadge
                  label={`${t("signed")} · ${formatDate(document.signed_at, false, locale)}`}
                  tone="green"
                />
              ) : (
                <StatusBadge label={t("signed")} tone="green" />
              )}
            </DataField>
          ) : null}
          <DataField label={t("children")}>
            {document.child_names.join(", ")}
          </DataField>
          <DataField label={t("bookedCare")}>{document.period_name}</DataField>
          <DataField label={t("startsOn")}>
            {formatDate(document.service_from, false, locale)}
          </DataField>
          <DataField label={t("sentAt")}>
            {formatDate(document.sent_at, false, locale)}
          </DataField>
        </DataGrid>
      ) : (
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
      )}
    </ParentSection>
  );
}
