"use client";

// Versand der Jahresbescheide eines ausgestellten Laufs (#6347): moto
// speichert jeden Bescheid einmal als PDF. Dieses Original geht per E-Mail an
// die Familie und liegt danach in der Eltern-App unter „Dokumente“, aber erst,
// wenn das Büro sendet. Die Seite des Laufs lädt diesen Code nicht.

import { useParams } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { CareLetterPanel } from "~/components/traeger/care-letter-panel";
import {
  NOTICE_KIND_LABELS,
  NOTICE_KIND_TONES,
  noticeCountLabel,
} from "~/components/traeger/care-notice-format";
import { canSendMail } from "~/components/traeger/billing-mail-status";
import { formatEuro } from "~/components/traeger/contribution-format";
import { HolidayHeadFacts } from "~/components/traeger/holiday-period-head";
import { LetterSendCell } from "~/components/traeger/letter-send-cell";
import {
  LetterSendState,
  lettersToSend,
} from "~/components/traeger/letter-send-state";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import {
  type OverflowMenuItem,
  OverflowMenu,
} from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { TenantPage } from "~/components/ui/tenant-page";
import {
  useApiErrorDisplay,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { formatBerlinDate } from "~/lib/date-helpers";
import { downloadBlob } from "~/lib/file-download";
import { createLogger } from "~/lib/logger";
import {
  type CareNoticeLetterSummary,
  type CareNoticeLetters,
  type CareNoticeMailing,
  type CareNoticeSeries,
  createCareNoticeLetters,
  downloadCareNoticePDF,
  fetchCareNoticeLetters,
  mailCareNoticeLetter,
  mailCareNoticeLetters,
  printCareNoticeLetters,
} from "~/lib/traeger-care-notice-letters-api";
import {
  type CareNoticeRunDetail,
  fetchCareNoticeRun,
} from "~/lib/traeger-care-notices-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "TraegerCareNoticeDeliveryPage" });

// While a mail waits, the page looks again after this many milliseconds.
const REFRESH_WHILE_QUEUED_MS = 15_000;

function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err);
}

/** „1 Familie“ / „3 Familien“. */
function familyCount(count: number): string {
  return count === 1 ? "1 Familie" : `${count} Familien`;
}

/** The notices „An alle senden“ sends: stored, unsent, with an address. */
function noticesToSend(summary: CareNoticeLetterSummary): number {
  return lettersToSend(summary);
}

/** The success toast of „An alle senden“. */
function seriesMessage(series: CareNoticeSeries): string {
  const parts = [
    series.queued === 0
      ? "Es gab keinen neuen Bescheid zu senden."
      : series.queued === 1
        ? "1 Bescheid wird jetzt gesendet."
        : `${series.queued} Bescheide werden jetzt gesendet.`,
  ];
  if (series.withoutEmail > 0) {
    parts.push(
      `${familyCount(series.withoutEmail)} ohne E-Mail-Adresse: Bitte drucken Sie deren Bescheid.`,
    );
  }
  if (series.recipientChanged > 0) {
    parts.push(
      `${series.recipientChanged === 1 ? "1 Bescheid ging" : `${series.recipientChanged} Bescheide gingen`} nicht raus: Die Person zahlt nicht mehr für diesen Vertrag. Drucken Sie diesen Bescheid und geben Sie ihn der Familie auf Papier.`,
    );
  }
  return parts.join(" ");
}

/** „An die Familie“: what reached the family. */
function SendCell({ notice }: Readonly<{ notice: CareNoticeMailing }>) {
  if (!notice.hasLetter) {
    return <p className="text-sm text-gray-600">Noch kein Original</p>;
  }
  return (
    <LetterSendCell
      letter={notice}
      printInstruction="Bitte drucken Sie den Bescheid."
    />
  );
}

interface RowActions {
  readonly sendingContract: string | null;
  readonly onShow: (notice: CareNoticeMailing) => void;
  readonly onSend: (notice: CareNoticeMailing) => void;
}

function rowMenuItems(
  notice: CareNoticeMailing,
  actions: RowActions,
): OverflowMenuItem[] {
  if (!notice.hasLetter) return [];
  const items: OverflowMenuItem[] = [
    {
      label: "Original ansehen",
      description: "Öffnet das gespeicherte PDF des Bescheids",
      onClick: () => actions.onShow(notice),
    },
  ];
  if (canSendMail(notice) && actions.sendingContract === null) {
    items.push({
      label: notice.sends === 0 ? "Senden" : "Erneut senden",
      description:
        notice.sends === 0
          ? notice.inInbox
            ? "Per E-Mail und in die Eltern-App"
            : "Nur per E-Mail, nicht in die Eltern-App"
          : "Noch einmal an dieselbe Adresse",
      onClick: () => actions.onSend(notice),
    });
  }
  return items;
}

function noticeColumns(
  actions: RowActions,
): DataTableColumn<CareNoticeMailing>[] {
  return [
    {
      key: "child",
      header: "Kind",
      stacked: "title",
      render: (notice) => (
        <div className="min-w-0">
          <p className="font-medium text-gray-900">
            {notice.childFirstName} {notice.childLastName}
          </p>
          <p className="text-xs text-gray-600">{notice.schoolName}</p>
        </div>
      ),
    },
    {
      key: "notice",
      header: "Bescheid",
      stacked: "field",
      render: (notice) => (
        <div className="flex min-w-0 flex-col items-start gap-1">
          <div className="flex flex-wrap items-center gap-2">
            <StatusBadge
              label={NOTICE_KIND_LABELS[notice.kind]}
              tone={NOTICE_KIND_TONES[notice.kind]}
            />
            <span className="text-sm text-gray-900 tabular-nums">
              {notice.number}
            </span>
          </div>
          {notice.amendsNumber ? (
            <p className="text-xs text-gray-600">
              Ändert Bescheid {notice.amendsNumber}
            </p>
          ) : null}
          {notice.amendedByNumber ? (
            <p className="text-xs text-gray-600">
              Geändert durch {notice.amendedByNumber}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      key: "recipient",
      header: "Empfänger",
      stacked: "field",
      render: (notice) => (
        <div className="min-w-0">
          <p className="text-sm text-gray-900">
            {notice.recipientFirstName} {notice.recipientLastName}
          </p>
          <p className="text-xs break-all text-gray-600">
            {notice.recipientEmail || "Keine E-Mail-Adresse"}
          </p>
        </div>
      ),
    },
    {
      key: "total",
      header: "Gesamtbetrag",
      align: "right",
      stacked: "field",
      className: "tabular-nums font-medium text-gray-900",
      render: (notice) => formatEuro(notice.totalCents),
    },
    {
      key: "send",
      header: "An die Familie",
      stacked: "field",
      className: "w-56 align-top",
      render: (notice) => <SendCell notice={notice} />,
    },
    {
      key: "actions",
      header: "",
      align: "right",
      stacked: "meta",
      stackedLabel: false,
      className: "align-top",
      render: (notice) => {
        const items = rowMenuItems(notice, actions);
        return items.length > 0 ? (
          <OverflowMenu
            ariaLabel={`Aktionen für den Bescheid von ${notice.childFirstName} ${notice.childLastName}`}
            triggerSize="sm"
            items={items}
          />
        ) : null;
      },
    },
  ];
}

export default function TraegerCareNoticeDeliveryPage() {
  const params = useParams<{ id: string }>();
  const runId = params?.id ?? "";
  const runPath = traegerPath(`/traeger/abrechnung/jahresbescheide/${runId}`);
  const [run, setRun] = useState<CareNoticeRunDetail | null>(null);
  const [letters, setLetters] = useState<CareNoticeLetters | null>(null);
  const pageLoad = useApiLoadError();
  const { show: showLoadError, clear: clearLoadError } = pageLoad;
  const lettersLoad = useApiLoadError();
  const { show: showLettersError, clear: clearLettersError } = lettersLoad;
  const actionErrors = useApiErrorDisplay();
  const { success } = useToast();
  const [creating, setCreating] = useState(false);
  const [confirming, setConfirming] = useState(false);
  const [sendingAll, setSendingAll] = useState(false);
  const [sendingContract, setSendingContract] = useState<string | null>(null);
  const [printing, setPrinting] = useState(false);
  const [shown, setShown] = useState<CareNoticeMailing | null>(null);

  const load = useCallback(async () => {
    try {
      setRun(await fetchCareNoticeRun(runId));
      clearLoadError();
    } catch (err) {
      logger.error("traeger_care_notice_run_load_failed", {
        error: errorMessage(err),
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

  const released = run?.status === "released" && run.previewCount > 0;
  const loadLetters = useCallback(async () => {
    try {
      setLetters(await fetchCareNoticeLetters(runId));
      clearLettersError();
    } catch (err) {
      logger.error("traeger_care_notice_letters_load_failed", {
        error: errorMessage(err),
      });
      await showLettersError(err, {
        object: "die Bescheide",
        retry: () => void loadLetters(),
      });
    }
  }, [runId, clearLettersError, showLettersError]);

  useEffect(() => {
    if (released) void loadLetters();
  }, [released, loadLetters]);

  // A mail on its way: look again until it is sent or failed.
  const waiting = (letters?.summary.queued ?? 0) > 0;
  useEffect(() => {
    if (!waiting) return;
    const timer = setTimeout(() => void loadLetters(), REFRESH_WHILE_QUEUED_MS);
    return () => clearTimeout(timer);
  }, [waiting, letters, loadLetters]);

  const create = async () => {
    setCreating(true);
    try {
      setLetters(await createCareNoticeLetters(runId));
      success("Die Originale sind gespeichert. Gesendet ist noch nichts.");
    } catch (err) {
      logger.error("traeger_care_notice_letters_create_failed", {
        error: errorMessage(err),
      });
      void actionErrors.show(err, { object: "die Originale" });
      void loadLetters();
    } finally {
      setCreating(false);
    }
  };

  const sendAll = async () => {
    setSendingAll(true);
    try {
      success(seriesMessage(await mailCareNoticeLetters(runId)));
    } catch (err) {
      logger.error("traeger_care_notice_letters_mail_failed", {
        error: errorMessage(err),
      });
      void actionErrors.show(err, { object: "die Bescheide" });
    } finally {
      setSendingAll(false);
      setConfirming(false);
      void loadLetters();
    }
  };

  const sendOne = async (notice: CareNoticeMailing) => {
    setSendingContract(notice.contractId);
    try {
      await mailCareNoticeLetter(runId, notice.contractId);
      success(
        `Der Bescheid an ${notice.recipientFirstName} ${notice.recipientLastName} wird jetzt gesendet.`,
      );
    } catch (err) {
      logger.error("traeger_care_notice_letter_mail_failed", {
        error: errorMessage(err),
      });
      void actionErrors.show(err, { object: "der Bescheid" });
    } finally {
      setSendingContract(null);
      void loadLetters();
    }
  };

  const print = async (withoutEmailOnly: boolean) => {
    setPrinting(true);
    try {
      const file = await printCareNoticeLetters(runId, withoutEmailOnly);
      downloadBlob(file.blob, file.filename);
    } catch (err) {
      logger.error("traeger_care_notice_letters_print_failed", {
        error: errorMessage(err),
      });
      void actionErrors.show(err, { object: "die Bescheide" });
    } finally {
      setPrinting(false);
    }
  };

  const summary = letters?.summary;
  const missing = summary ? summary.notices - summary.letters : 0;
  const toSend = summary ? noticesToSend(summary) : 0;
  const printItems = [
    {
      label: "Alle Bescheide drucken",
      description: "Alle gespeicherten Originale in einer Datei",
      onClick: () => void print(false),
    },
    ...(summary && summary.withoutEmail > 0
      ? [
          {
            label: "Nur Familien ohne E-Mail",
            description: `${familyCount(summary.withoutEmail)} ohne E-Mail-Adresse`,
            onClick: () => void print(true),
          },
        ]
      : []),
  ];

  return (
    <TenantPage
      title={run ? `Versand der Jahresbescheide ${run.year}` : "Versand"}
      back
      backHref={runPath}
      backLabel="Zurück zum Jahreslauf"
      stats={
        run ? (
          <HolidayHeadFacts>
            <span className="tabular-nums">
              {noticeCountLabel(run.previewCount)} ·{" "}
              {formatEuro(run.previewCents)}
            </span>
            {run.issuedOn ? (
              <span>Ausgestellt am {formatBerlinDate(run.issuedOn)}</span>
            ) : null}
          </HolidayHeadFacts>
        ) : null
      }
      statsLoading={!run && !pageLoad.error}
      error={pageLoad.error}
      loading={!pageLoad.error && !run}
      overlays={
        run && summary ? (
          <>
            <CareLetterPanel
              letter={shown}
              title={
                shown ? `${NOTICE_KIND_LABELS[shown.kind]} ${shown.number}` : ""
              }
              fileKey={shown ? `notice-${shown.noticeId}` : ""}
              load={(notice) =>
                downloadCareNoticePDF(run.id, notice.contractId)
              }
              onClose={() => setShown(null)}
            />
            <ConfirmationModal
              isOpen={confirming}
              title="An alle senden?"
              confirmText={
                toSend === 1
                  ? "1 Bescheid senden"
                  : `${toSend} Bescheide senden`
              }
              isConfirmLoading={sendingAll}
              loadingText="Wird gesendet …"
              onClose={() => setConfirming(false)}
              onConfirm={() => void sendAll()}
            >
              <div className="space-y-3 text-sm text-gray-700">
                <p>
                  {toSend === 1
                    ? "1 Bescheid geht jetzt per E-Mail an die Familie."
                    : `${toSend} Bescheide gehen jetzt per E-Mail an die Familien.`}{" "}
                  Mit einem Konto in der Eltern-App liegt er auch dort unter
                  „Dokumente“.
                </p>
                {summary.withoutEmail > 0 ? (
                  <p>
                    {familyCount(summary.withoutEmail)} ohne E-Mail-Adresse{" "}
                    {summary.withoutEmail === 1 ? "bekommt" : "bekommen"}{" "}
                    nichts. Drucken Sie deren Bescheid.
                  </p>
                ) : null}
              </div>
            </ConfirmationModal>
          </>
        ) : null
      }
    >
      {run && !released ? (
        <SectionCard
          title="Noch nicht ausgestellt"
          description="Versenden können Sie erst ausgestellte Bescheide. Stellen Sie sie auf der Seite des Jahreslaufs aus."
        />
      ) : null}
      {released && lettersLoad.error ? (
        <SectionCard title="Versand an die Familien">
          <LoadErrorAlert error={lettersLoad.error} actionLayout="responsive" />
        </SectionCard>
      ) : null}
      {released && !lettersLoad.error && !summary ? (
        <SectionCard title="Versand an die Familien" loading />
      ) : null}
      {released && summary && missing > 0 ? (
        <SectionCard
          title="Originale speichern"
          description="moto speichert jeden Bescheid einmal als PDF. Dieses Original bleibt gleich, auch wenn sich später Briefkopf, Vorlage, Adresse oder Vertrag ändern. Gesendet wird dabei noch nichts."
          action={
            <Button
              type="button"
              variant="primary"
              size="md"
              isLoading={creating}
              loadingText="Wird gespeichert …"
              tooltip="Speichert jeden Bescheid als PDF, gesendet wird noch nichts."
              onClick={() => void create()}
            >
              {missing === 1
                ? "1 Original speichern"
                : `${missing} Originale speichern`}
            </Button>
          }
        />
      ) : null}
      {released && summary && summary.letters > 0 ? (
        <SectionCard
          title="Versand an die Familien"
          description="Jede Familie bekommt das gespeicherte Original per E-Mail. Hat sie mit dieser Adresse ein Konto in der Eltern-App, liegt es dort unter „Dokumente“. Familien ohne E-Mail geben Sie den gedruckten Bescheid."
          action={
            <div className="flex flex-wrap gap-2">
              <OverflowMenu
                ariaLabel="Bescheide drucken"
                triggerButton={{ variant: "outline", size: "md" }}
                triggerContent={printing ? "Wird erstellt …" : "Drucken"}
                disabled={printing}
                items={printItems}
              />
              <Button
                type="button"
                variant="primary"
                size="md"
                isLoading={sendingAll}
                loadingText="Wird gesendet …"
                disabled={toSend === 0}
                disabledReason="Alle Bescheide mit E-Mail-Adresse sind schon gesendet."
                tooltip="moto fragt vorher nach, dann gehen die Bescheide per E-Mail an die Familien."
                onClick={() => setConfirming(true)}
              >
                An alle senden
              </Button>
            </div>
          }
        >
          <div className="space-y-5">
            <LetterSendState
              summary={summary}
              noun={{ one: "Bescheid", many: "Bescheiden" }}
            />
            <Alert
              type="info"
              title="Keine förmliche Zustellung"
              message="E-Mail und Eltern-App sind kein Nachweis, dass die Familie den Bescheid bekommen hat. Brauchen Sie einen Nachweis? Dann stellen Sie den Bescheid zusätzlich auf anderem Weg zu."
            />
            {summary.queued > 0 ? (
              <p className="text-sm text-gray-600" role="status">
                {summary.queued === 1
                  ? "1 Bescheid wird noch gesendet."
                  : `${summary.queued} Bescheide werden noch gesendet.`}{" "}
                Die Seite zeigt den neuen Stand von selbst.
              </p>
            ) : null}
            {summary.failed > 0 ? (
              <p className="text-sm text-gray-600">
                Einige Bescheide sind nicht angekommen. Senden Sie diese über
                das Menü ihrer Zeile erneut oder drucken Sie sie.
              </p>
            ) : null}
            {toSend === 0 &&
            summary.sent > 0 &&
            summary.queued === 0 &&
            summary.failed === 0 &&
            missing === 0 ? (
              <p className="text-sm text-gray-600">
                Alle Bescheide mit E-Mail-Adresse sind gesendet. Einzelne
                Bescheide senden Sie erneut über das Menü ihrer Zeile.
              </p>
            ) : null}
          </div>
        </SectionCard>
      ) : null}
      {released && letters ? (
        <DataTable
          columns={noticeColumns({
            sendingContract,
            onShow: setShown,
            onSend: (notice) => void sendOne(notice),
          })}
          rows={[...letters.notices]}
          getRowKey={(notice) => notice.noticeId}
          stackedOnMobile
          caption="Jede Zeile ist ein ausgestellter Bescheid mit seinem Original."
          emptyState={
            <EmptyState
              title="Keine Bescheide."
              description="Dieser Jahreslauf hat keinen Bescheid ausgestellt."
            />
          }
        />
      ) : null}
    </TenantPage>
  );
}
