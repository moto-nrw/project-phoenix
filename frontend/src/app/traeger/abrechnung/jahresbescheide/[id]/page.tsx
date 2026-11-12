"use client";

// Ein Jahreslauf (#6219): die Vorschau aller Jahresbescheide nach dem
// gültigen Vertrag und der Preisliste, und das Ausstellen, das jedem
// Bescheid seine Nummer gibt. Danach ändern sich die Bescheide nicht mehr;
// eine Vertragsänderung führt im nächsten Lauf zum Änderungsbescheid. Jedes
// Öffnen steht im Protokoll des Laufs.

import { ArchiveX, Inbox } from "lucide-react";
import { useParams, useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import {
  NOTICE_KIND_LABELS,
  NOTICE_KIND_TONES,
  NOTICE_RUN_STATUS_LABELS,
  NOTICE_RUN_STATUS_TONES,
  formatDelta,
  isEmptyNoticeRun,
  noticeCountLabel,
  noticePeriod,
  noticeRateLabel,
  rateCountLabel,
} from "~/components/traeger/care-notice-format";
import { formatEuro } from "~/components/traeger/contribution-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { DataField, DataGrid } from "~/components/ui/detail-modal-components";
import { EmptyState } from "~/components/ui/empty-state";
import { ConfirmationModal } from "~/components/ui/modal";
import { SectionCard } from "~/components/ui/section-card";
import { StatCard } from "~/components/ui/stat-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  type CareNotice,
  type CareNoticeRunDetail,
  discardCareNoticeRun,
  fetchCareNoticeRun,
  releaseCareNoticeRun,
} from "~/lib/traeger-care-notices-api";
import { traegerPath } from "~/lib/traeger-url";
const logger = createLogger({ component: "TraegerCareNoticeRunPage" });

const noticeColumns: DataTableColumn<CareNotice>[] = [
  {
    key: "child",
    header: "Kind",
    render: (notice) => (
      <div className="min-w-0">
        <p className="font-medium text-gray-900">
          {notice.childFirstName} {notice.childLastName}
        </p>
        <p className="text-xs text-gray-600">{notice.schoolName}</p>
      </div>
    ),
    stacked: "title",
  },
  {
    key: "kind",
    header: "Art",
    render: (notice) => (
      <StatusBadge
        label={NOTICE_KIND_LABELS[notice.kind]}
        tone={NOTICE_KIND_TONES[notice.kind]}
      />
    ),
    stacked: "meta",
  },
  {
    key: "number",
    header: "Bescheid-Nr.",
    render: (notice) => (
      <span className="tabular-nums">{notice.number || "–"}</span>
    ),
    stacked: "hidden",
  },
  {
    key: "period",
    header: "Zeitraum",
    render: (notice) => (
      <span className="tabular-nums">
        {noticePeriod(notice.periodFrom, notice.periodUntil)}
      </span>
    ),
  },
  {
    key: "rate",
    header: "Raten",
    render: (notice) => (
      <span className="tabular-nums">
        {rateCountLabel(notice.installments.length)} · {noticeRateLabel(notice)}
      </span>
    ),
  },
  {
    key: "total",
    header: "Gesamtbetrag",
    align: "right",
    className: "tabular-nums font-medium text-gray-900",
    render: (notice) => formatEuro(notice.totalCents),
  },
  {
    key: "delta",
    header: "Zu-/Abgang",
    align: "right",
    className: "tabular-nums",
    render: (notice) =>
      notice.kind === "change" ? formatDelta(notice.deltaCents) : "–",
    stacked: "hidden",
  },
];

/** "1 Kind", "3 Kinder". */
function childrenLabel(count: number): string {
  return count === 1 ? "1 Kind" : `${count} Kinder`;
}

export default function TraegerCareNoticeRunPage() {
  const router = useRouter();
  const params = useParams<{ id: string }>();
  const runId = params.id;
  const [run, setRun] = useState<CareNoticeRunDetail | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [releasing, setReleasing] = useState(false);
  const [confirmingDiscard, setConfirmingDiscard] = useState(false);
  const [discarding, setDiscarding] = useState(false);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const releaseErrors = useApiErrorDisplay();
  const { success } = useToast();

  const load = useCallback(async () => {
    try {
      setRun(await fetchCareNoticeRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_notice_run_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      await showLoadError(err, {
        object: "der Jahreslauf",
        retry: () => void load(),
      });
    }
  }, [runId, clearLoadError, showLoadError]);

  useEffect(() => {
    void load();
  }, [load]);

  const release = async () => {
    if (!run) return;
    setReleasing(true);
    try {
      setRun(await releaseCareNoticeRun(run));
      setConfirming(false);
      success(
        run.previewCount === 0
          ? "Der Jahreslauf ist abgeschlossen."
          : "Die Bescheide sind ausgestellt.",
      );
    } catch (err) {
      logger.error("traeger_care_notice_run_release_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirming(false);
      void releaseErrors.show(err, { object: "der Jahreslauf" });
      void load();
    } finally {
      setReleasing(false);
    }
  };

  const discard = async () => {
    if (!run) return;
    setDiscarding(true);
    try {
      setRun(await discardCareNoticeRun(run.id));
      setConfirmingDiscard(false);
      success(
        `Der Lauf ist verworfen. Für ${run.year} können Sie einen neuen Jahreslauf starten.`,
      );
    } catch (err) {
      logger.error("traeger_care_notice_run_discard_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setConfirmingDiscard(false);
      void releaseErrors.show(err, { object: "der Jahreslauf" });
      void load();
    } finally {
      setDiscarding(false);
    }
  };

  const openNotice = (notice: CareNotice) =>
    router.push(
      traegerPath(
        `/traeger/abrechnung/jahresbescheide/${runId}/bescheid/${notice.contractId}`,
      ),
    );

  const preview = run?.status === "calculated";
  const discarded = run?.status === "discarded";
  const empty = run ? isEmptyNoticeRun(run) : false;
  const changes =
    run?.notices.filter((notice) => notice.kind === "change") ?? [];
  const canRelease =
    preview &&
    (run.previewCount === 0 || (run.numberRangeSet && !run.legalRemedyMissing));

  return (
    <TenantPage
      title={run ? `Jahresbescheide ${run.year}` : "Jahreslauf"}
      back
      backHref={traegerPath("/traeger/abrechnung/jahresbescheide")}
      backLabel="Zurück zu den Jahresbescheiden"
      stats={
        run ? (
          <HolidayHeadFacts>
            <StatusBadge
              label={
                run.status === "released" && run.previewCount === 0
                  ? "Abgeschlossen"
                  : NOTICE_RUN_STATUS_LABELS[run.status]
              }
              tone={NOTICE_RUN_STATUS_TONES[run.status]}
            />
            <span className="tabular-nums">
              {noticePeriod(run.periodFrom, run.periodUntil)}
            </span>
            {discarded ? (
              <span>
                Verworfen am{" "}
                {run.discardedAt ? formatDate(run.discardedAt) : "–"}
              </span>
            ) : run.issuedOn ? (
              <span>
                {run.previewCount === 0 ? "Abgeschlossen" : "Ausgestellt"} am{" "}
                {formatDate(run.issuedOn)}
              </span>
            ) : run.noticeDate ? (
              <span>Bescheiddatum {formatDate(run.noticeDate)}</span>
            ) : null}
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      actions={
        empty ? (
          <Button
            type="button"
            variant="outline_danger"
            size="md"
            onClick={() => setConfirmingDiscard(true)}
          >
            Leeren Lauf verwerfen
          </Button>
        ) : preview ? (
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={!canRelease}
            onClick={() => setConfirming(true)}
          >
            {run.previewCount === 0
              ? "Jahreslauf abschließen"
              : "Bescheide ausstellen"}
          </Button>
        ) : run?.status === "released" && run.previewCount > 0 ? (
          // The PAIN.008 file of the installments (#4697) and the delivery of
          // the stored originals (#6347) have pages of their own.
          <div className="flex flex-wrap gap-2">
            <ButtonLink
              href={traegerPath(
                `/traeger/abrechnung/jahresbescheide/${run.id}/lastschrift`,
              )}
              variant="outline"
              size="md"
            >
              Lastschrift der Raten
            </ButtonLink>
            <ButtonLink
              href={traegerPath(
                `/traeger/abrechnung/jahresbescheide/${run.id}/versand`,
              )}
              variant="primary"
              size="md"
            >
              An Familien senden
            </ButtonLink>
          </div>
        ) : undefined
      }
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
      overlays={
        run && empty ? (
          <ConfirmationModal
            isOpen={confirmingDiscard}
            title="Leeren Lauf verwerfen?"
            confirmText="Verwerfen"
            confirmVariant="danger"
            isConfirmLoading={discarding}
            loadingText="Wird verworfen …"
            onClose={() => setConfirmingDiscard(false)}
            onConfirm={() => void discard()}
          >
            <div className="space-y-3 text-sm text-gray-700">
              <p>
                Der Jahreslauf für das Jahr {run.year} stellt nichts aus. Er
                bleibt als „Verworfen“ in der Liste.
              </p>
              <p>
                Danach ist das Jahr frei. Sie können sofort einen neuen Lauf
                starten.
              </p>
            </div>
          </ConfirmationModal>
        ) : run ? (
          <ConfirmationModal
            isOpen={confirming}
            title={
              run.previewCount === 0
                ? "Jahreslauf abschließen?"
                : "Bescheide ausstellen?"
            }
            confirmText={run.previewCount === 0 ? "Abschließen" : "Ausstellen"}
            isConfirmLoading={releasing}
            loadingText={
              run.previewCount === 0
                ? "Wird abgeschlossen …"
                : "Wird ausgestellt …"
            }
            onClose={() => setConfirming(false)}
            onConfirm={() => void release()}
          >
            {run.previewCount === 0 ? (
              <p className="text-sm text-gray-700">
                Dieser Jahreslauf stellt keine Bescheide aus. Danach können Sie
                einen neuen Jahreslauf starten.
              </p>
            ) : (
              <p className="text-sm text-gray-700">
                {noticeCountLabel(run.previewCount)} über{" "}
                {formatEuro(run.previewCents)} bekommen ihre Nummer und das
                Bescheiddatum{" "}
                {run.noticeDate ? formatDate(run.noticeDate) : "von heute"}.
                Danach lassen sie sich nicht mehr ändern.
              </p>
            )}
            {changes.length > 0 ? (
              <p className="mt-3 text-sm text-gray-700">
                {changes.length === 1
                  ? "1 Änderungsbescheid ersetzt den bisherigen Bescheid für seine Monate."
                  : `${changes.length} Änderungsbescheide ersetzen die bisherigen Bescheide für ihre Monate.`}
              </p>
            ) : null}
          </ConfirmationModal>
        ) : null
      }
    >
      {run && (discarded || empty) ? (
        <SectionCard title="Jahreslauf">
          <EmptyState
            tone="neutral"
            icon={
              discarded ? (
                <ArchiveX className="h-7 w-7" aria-hidden="true" />
              ) : (
                <Inbox className="h-7 w-7" aria-hidden="true" />
              )
            }
            title={
              discarded
                ? "Dieser Lauf ist verworfen"
                : "Dieser Lauf hat nichts abzurechnen"
            }
            description={
              discarded
                ? `Er hatte nichts abzurechnen und hat nichts ausgestellt. Für das Jahr ${run.year} können Sie einen neuen Lauf starten.`
                : `Für das Jahr ${run.year} gibt es keinen Bescheid. Es fehlt auch kein Preis. Sie können den Lauf verwerfen. Danach ist das Jahr frei für einen neuen Lauf.`
            }
            className="py-6"
          />
          <div className="mt-2 border-t border-gray-100 pt-5">
            <DataGrid columns={2}>
              <DataField label="Zeitraum">
                {noticePeriod(run.periodFrom, run.periodUntil)}
              </DataField>
              <DataField label="Schulen">
                {run.schoolIds.length === 1
                  ? "1 Schule"
                  : `${run.schoolIds.length} Schulen`}
              </DataField>
              <DataField label="Gestartet am">
                {formatDate(run.createdAt)}
              </DataField>
              {discarded ? (
                <DataField label="Verworfen am">
                  {run.discardedAt ? formatDate(run.discardedAt) : "–"}
                </DataField>
              ) : null}
            </DataGrid>
          </div>
        </SectionCard>
      ) : run ? (
        <>
          {preview && run.previewCount > 0 && run.legalRemedyMissing ? (
            <Alert
              type="warning"
              title="Rechtsbehelfsbelehrung fehlt"
              message="In der Vorlage „Bescheid“ fehlt die Rechtsbehelfsbelehrung. Ohne sie stellt moto keinen Bescheid aus. Wer das Recht „Einstellungen“ hat, trägt sie ein."
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
          ) : null}
          {preview && run.previewCount > 0 && !run.numberRangeSet ? (
            <Alert
              type="warning"
              message="Bevor Sie ausstellen: Legen Sie das Muster der Rechnungsnummern für Monatsbeiträge fest. Der Bescheid nimmt seine Nummer daraus."
              action={
                <ButtonLink
                  href={traegerPath("/traeger/einstellungen/abrechnung")}
                  variant="surface"
                  size="compact"
                >
                  Muster festlegen
                </ButtonLink>
              }
            />
          ) : null}
          {preview && (run.unpricedCount > 0 || run.duplicateCount > 0) ? (
            <Alert
              type="warning"
              message="Für einige Kinder fehlt ein Preis oder es gibt einen doppelten Vertrag. Für diese Monate bleibt ein bisheriger Bescheid gültig. Ohne bisherigen Bescheid bleiben sie offen. Der Monatslauf zeigt, was fehlt."
            />
          ) : null}
          <div className="grid gap-4 sm:grid-cols-3">
            <StatCard
              label={preview ? "Bescheide in der Vorschau" : "Bescheide"}
              value={noticeCountLabel(run.previewCount)}
            />
            <StatCard
              label="Gesamtbetrag aller Bescheide"
              value={formatEuro(run.previewCents)}
              // A released notice is a claim before any payment: moto sees
              // no incoming money, the bookkeeping does (#6348).
              hint={
                preview
                  ? undefined
                  : "Forderung vor Zahlungseingang. Zahlungen sieht moto nicht."
              }
            />
            <StatCard
              label="Zu-/Abgang gegenüber bisher"
              value={formatDelta(run.previewDeltaCents)}
            />
          </div>
          {run.unchanged > 0 || run.billed > 0 ? (
            <Alert
              type="info"
              message={[
                run.unchanged > 0
                  ? `${childrenLabel(run.unchanged)} mit gültigem Bescheid: nichts geändert, kein neuer Bescheid.`
                  : "",
                run.billed > 0
                  ? `${childrenLabel(run.billed)} mit Monatsrechnung in diesem Zeitraum: diese Monate stehen in keinem Bescheid.`
                  : "",
              ]
                .filter(Boolean)
                .join(" ")}
            />
          ) : null}
          <DataTable
            columns={noticeColumns}
            rows={[...run.notices]}
            getRowKey={(notice) => notice.contractId}
            stackedOnMobile
            stackedLayout="row"
            stackedDetailKey="rate"
            onRowClick={openNotice}
            caption={
              preview
                ? "Wählen Sie eine Zeile. Sie sehen Seite 1 des Bescheids."
                : "Wählen Sie eine Zeile, um Seite 1 des Bescheids anzusehen. Das Original zum Versand finden Sie unter „An Familien senden“."
            }
            emptyState={
              <EmptyState
                title={
                  preview ? "Kein Bescheid auszustellen." : "Keine Bescheide."
                }
                description={
                  run.unchanged > 0
                    ? "Alle Bescheide stimmen noch mit den Verträgen überein."
                    : "Für diesen Zeitraum hat kein Kind einen abrechenbaren Vertrag."
                }
              />
            }
          />
        </>
      ) : null}
    </TenantPage>
  );
}
