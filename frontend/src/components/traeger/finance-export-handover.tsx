"use client";

// Handover of a finance export to the recipient of its format (#6824, ADR
// 0218; #6911, ADR 0222) on the export page and on the page of the debit
// file: the menu „Übergeben“ of a run, the result of the last handover above
// the list, and the question before a file goes to the same recipient a
// second time. The file is made like a download, with its own number and
// protocol entry.

import {
  ChevronDown,
  FileSpreadsheet,
  Landmark,
  RotateCcw,
  Send,
  Settings2,
} from "lucide-react";
import { type ReactNode, useCallback, useEffect, useState } from "react";
import {
  destinationText,
  EXPORT_RECIPIENT_PATH,
  HANDOVER_FORMAT_LABELS,
  HANDOVER_STATE_TONES,
  HANDOVER_UNKNOWN_TEXT,
  handoverReasonText,
} from "~/components/traeger/export-handover-format";
import { Alert } from "~/components/ui/alert";
import { Button, ButtonLink } from "~/components/ui/button";
import { ConfirmationModal } from "~/components/ui/modal";
import { StatusBadge } from "~/components/ui/status-badge";
import {
  OverflowMenu,
  type OverflowMenuItem,
} from "~/components/ui/page-header/OverflowMenu";
import { useApiErrorDisplay, useToast } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { createLogger } from "~/lib/logger";
import {
  type ExportRecipient,
  type ExportRecipientsPage,
  fetchExportRecipients,
  type HandoverFormat,
  type HandoverOptions,
  type HandoverResult,
  handOverRunFile,
} from "~/lib/traeger-export-handovers-api";
import {
  type DatevBookingProblem,
  datevBookingProblems,
  type ExportRunPathKind,
  type ProtocolHandover,
} from "~/lib/traeger-finance-exports-api";
import { traegerPath } from "~/lib/traeger-url";

const logger = createLogger({ component: "FinanceExportHandover" });

/** The run a file is made of, as the export page and the debit page name it. */
export interface HandoverRun {
  readonly pathKind: ExportRunPathKind;
  readonly runId: string;
  readonly label: string;
}

/** One handover a person started: which file of which run. */
interface HandoverAttempt {
  readonly format: HandoverFormat;
  readonly run: HandoverRun;
  readonly options: HandoverOptions;
}

/** What the notice above the list shows after a handover. */
type HandoverNotice =
  | {
      readonly kind: "failed";
      readonly attempt: HandoverAttempt;
      readonly result: HandoverResult;
    }
  | {
      readonly kind: "unknown";
      readonly attempt: HandoverAttempt;
      readonly result: HandoverResult;
    }
  | { readonly kind: "not_ready"; readonly attempt: HandoverAttempt };

function attemptKey(attempt: Pick<HandoverAttempt, "format" | "run">): string {
  return `${attempt.format}/${attempt.run.pathKind}/${attempt.run.runId}`;
}

function fileLabel(attempt: HandoverAttempt): string {
  return `${HANDOVER_FORMAT_LABELS[attempt.format]} von ${attempt.run.label}`;
}

/** The recipient a format goes to, or null for the download. */
export function recipientOfFormat(
  page: ExportRecipientsPage | null,
  format: HandoverFormat,
): ExportRecipient | null {
  const id = page?.assignments.find(
    (assignment) => assignment.format === format,
  )?.recipientId;
  if (!page || !id) return null;
  return page.recipients.find((recipient) => recipient.id === id) ?? null;
}

/** What the export page shows itself for a DATEV handover, as for the download. */
export interface DatevHandoverNotices {
  /** No new bookings, or another file took them first. */
  readonly onNothingNew: (run: HandoverRun, taken: boolean) => void;
  /** Invoices whose bookings DATEV would refuse, each with its problems. */
  readonly onRefused: (
    run: HandoverRun,
    problems: readonly DatevBookingProblem[],
  ) => void;
}

/**
 * The recipients of the Träger and the handover of a run's file. Without the
 * right „Finanz-Export“ the recipients stay hidden (recipientHidden): the
 * menu still offers the PAIN.008 handover, which needs only „Lastschriften
 * verwalten“, and the server names a missing recipient.
 */
export function useFinanceHandover(
  onHandedOver: (arrived: HandoverResult | null) => void,
  datev?: DatevHandoverNotices,
) {
  const [recipients, setRecipients] = useState<ExportRecipientsPage | null>(
    null,
  );
  const [recipientHidden, setRecipientHidden] = useState(false);
  const [running, setRunning] = useState<string | null>(null);
  const [notice, setNotice] = useState<HandoverNotice | null>(null);
  const [repeat, setRepeat] = useState<HandoverAttempt | null>(null);
  const errors = useApiErrorDisplay();
  const { success } = useToast();

  useEffect(() => {
    let current = true;
    fetchExportRecipients()
      .then((page) => {
        if (current) setRecipients(page);
      })
      .catch((err: unknown) => {
        if (current && err instanceof ApiError && err.status === 403)
          setRecipientHidden(true);
        // Without the recipient the page keeps its downloads.
        logger.warn("traeger_export_recipient_unavailable", {
          error: err instanceof Error ? err.message : String(err),
        });
      });
    return () => {
      current = false;
    };
  }, []);

  const start = useCallback(
    async (attempt: HandoverAttempt) => {
      setRunning(attemptKey(attempt));
      setNotice(null);
      let arrived: HandoverResult | null = null;
      try {
        const result = await handOverRunFile(
          attempt.format,
          attempt.run.pathKind,
          attempt.run.runId,
          attempt.options,
        );
        setRepeat(null);
        if (result.state === "transferred") {
          arrived = result;
          success(
            `${result.fileName} ist bei ${result.recipientName || destinationText(result.targetHost, result.targetDirectory)} angekommen.`,
          );
        } else {
          setNotice({
            kind: result.state === "unknown" ? "unknown" : "failed",
            attempt,
            result,
          });
        }
      } catch (err) {
        const code = err instanceof ApiError ? err.code : undefined;
        const problems =
          attempt.format === "datev" && err instanceof ApiError
            ? datevBookingProblems(err.details)
            : [];
        if (
          datev &&
          (code === "parent_billing.datev_no_new_bookings" ||
            code === "parent_billing.datev_bookings_taken")
        ) {
          // Nothing new to book, as for the download: the page says so.
          setRepeat(null);
          datev.onNothingNew(
            attempt.run,
            code === "parent_billing.datev_bookings_taken",
          );
        } else if (datev && problems.length > 0) {
          // A refusal of the content: the page names every invoice and what
          // to do, as for the download; a toast would name none.
          setRepeat(null);
          datev.onRefused(attempt.run, problems);
        } else if (code === "parent_billing.export_already_handed_over") {
          // A content that arrived or may have arrived: ask first.
          setRepeat(attempt);
        } else if (code === "parent_billing.export_recipient_not_ready") {
          setRepeat(null);
          setNotice({ kind: "not_ready", attempt });
        } else {
          setRepeat(null);
          logger.error("traeger_finance_handover_failed", {
            format: attempt.format,
            error: err instanceof Error ? err.message : String(err),
          });
          void errors.show(err, { object: fileLabel(attempt) });
        }
      } finally {
        setRunning(null);
        onHandedOver(arrived);
      }
    },
    [datev, errors, onHandedOver, success],
  );

  const handOver = useCallback(
    (format: HandoverFormat, run: HandoverRun, options: HandoverOptions = {}) =>
      void start({ format, run, options }),
    [start],
  );

  return {
    recipients,
    recipientHidden,
    running,
    handOver,
    isRunning: (format: HandoverFormat, run: HandoverRun) =>
      running === attemptKey({ format, run }),
    notice: notice ? (
      <HandoverNoticeAlert
        notice={notice}
        onRetry={() => void start(notice.attempt)}
        onDismiss={() => setNotice(null)}
      />
    ) : null,
    dialog: (
      <ConfirmationModal
        isOpen={repeat !== null}
        title="Diese Datei hat der Empfänger schon"
        confirmText="Noch einmal übergeben"
        confirmVariant="warning"
        isConfirmLoading={repeat !== null && running === attemptKey(repeat)}
        loadingText="Wird übergeben…"
        onClose={() => setRepeat(null)}
        onConfirm={() => {
          if (repeat)
            void start({
              ...repeat,
              options: { ...repeat.options, repeat: true },
            });
        }}
      >
        <p className="text-sm text-gray-700">
          {repeat
            ? `${fileLabel(repeat)} ist schon bei ${recipientOfFormat(recipients, repeat.format)?.name ?? "diesem Empfänger"} angekommen, ist noch unterwegs oder ihr Ergebnis ist unklar.`
            : null}{" "}
          Übergeben Sie sie nur noch einmal, wenn der Empfänger sie nicht hat.
          Sonst bucht er sie vielleicht doppelt.
        </p>
      </ConfirmationModal>
    ),
  };
}

/** The result of the last handover that did not simply arrive. */
function HandoverNoticeAlert({
  notice,
  onRetry,
  onDismiss,
}: Readonly<{
  notice: HandoverNotice;
  onRetry: () => void;
  onDismiss: () => void;
}>) {
  const label = fileLabel(notice.attempt);
  if (notice.kind === "not_ready") {
    return (
      <Alert
        type="warning"
        title={`Für die ${HANDOVER_FORMAT_LABELS[notice.attempt.format]} fehlt der Empfänger`}
        message={`${label} wurde nicht erstellt. Ordnen Sie der Datei zuerst einen bereiten Empfänger zu.`}
        onDismiss={onDismiss}
        action={
          <ButtonLink
            href={traegerPath(EXPORT_RECIPIENT_PATH)}
            variant="ghost_tone"
            size="compact"
          >
            Empfänger zuordnen
          </ButtonLink>
        }
      />
    );
  }
  const at = notice.result.recipientName
    ? ` bei ${notice.result.recipientName}`
    : "";
  if (notice.kind === "unknown") {
    return (
      <Alert
        type="warning"
        title={`${notice.result.fileName}: Ergebnis${at} unklar`}
        message={HANDOVER_UNKNOWN_TEXT}
        onDismiss={onDismiss}
      />
    );
  }
  return (
    <Alert
      type="warning"
      title={`${label} ist${at} nicht angekommen`}
      message={`${handoverReasonText(notice.result.reason)} Die Datei ${notice.result.fileName} steht trotzdem im Exportprotokoll.`}
      onDismiss={onDismiss}
      actionLayout="responsive"
      action={
        <span className="flex flex-wrap gap-2">
          <Button
            type="button"
            variant="ghost_tone"
            size="compact"
            tooltip="Erstellt die Datei neu und übergibt sie noch einmal."
            onClick={onRetry}
          >
            Noch einmal übergeben
          </Button>
          <ButtonLink
            href={traegerPath(EXPORT_RECIPIENT_PATH)}
            variant="ghost_tone"
            size="compact"
          >
            Empfänger prüfen
          </ButtonLink>
        </span>
      }
    />
  );
}

/** Why a format cannot go out yet, in a few words for the menu. */
function blockedText(
  page: ExportRecipientsPage,
  format: HandoverFormat,
): string {
  const recipient = recipientOfFormat(page, format);
  const file = HANDOVER_FORMAT_LABELS[format];
  if (page.recipients.length === 0) return "Noch kein Empfänger eingetragen.";
  if (!recipient) return `${file}: noch kein Empfänger zugeordnet.`;
  if (!recipient.enabled) return `${recipient.name}: Übergabe ausgeschaltet.`;
  return `${recipient.name}: Es fehlen noch Angaben.`;
}

/** The menu „Übergeben“ of one run: one entry per file the run has. */
export function HandoverMenu({
  run,
  recipients,
  formats,
  busy,
  running,
  onHandOver,
  onHandOverAll,
  recipientHidden = false,
  size = "compact",
}: Readonly<{
  /** The size of the actions beside it: compact in a table row, md in a card head. */
  size?: "compact" | "md";
  run: HandoverRun;
  recipients: ExportRecipientsPage | null;
  formats: readonly HandoverFormat[];
  busy: boolean;
  running: boolean;
  onHandOver: (format: HandoverFormat) => void;
  /** DATEV: every booking again, for a file the recipient lost. */
  onHandOverAll?: () => void;
  /** The recipients exist for the Träger but this account may not read them. */
  recipientHidden?: boolean;
}>) {
  if ((!recipients && !recipientHidden) || formats.length === 0) return null;
  const icons: Record<HandoverFormat, ReactNode> = {
    sollstellung: <FileSpreadsheet className="h-4 w-4" aria-hidden />,
    datev: <FileSpreadsheet className="h-4 w-4" aria-hidden />,
    pain008: <Landmark className="h-4 w-4" aria-hidden />,
  };
  // Without the right to read the recipients every file is offered; the
  // server names a missing recipient.
  const ready = recipients
    ? formats.filter((format) => recipientOfFormat(recipients, format)?.ready)
    : formats;
  const blocked = recipients
    ? formats.filter((format) => !ready.includes(format))
    : [];
  const target = (format: HandoverFormat) => {
    const recipient = recipientOfFormat(recipients, format);
    return recipient
      ? `${recipient.name} (${destinationText(recipient.host, recipient.remoteDirectory)})`
      : "dem Empfänger Ihres Trägers";
  };
  const items: OverflowMenuItem[] = [
    ...ready.map((format) => ({
      label: `${HANDOVER_FORMAT_LABELS[format]} übergeben`,
      description: `Erstellt die Datei und legt sie bei ${target(format)} ab.`,
      icon: icons[format],
      onClick: () => onHandOver(format),
    })),
    ...(onHandOverAll && ready.includes("datev")
      ? [
          {
            label: "Alle DATEV-Buchungen erneut übergeben",
            description: "Nur für eine verlorene Datei",
            icon: <RotateCcw className="h-4 w-4" aria-hidden />,
            onClick: onHandOverAll,
          },
        ]
      : []),
    ...(recipients && blocked.length > 0
      ? [
          {
            label:
              ready.length === 0
                ? "Empfänger einrichten"
                : "Empfänger zuordnen",
            description: blocked
              .map((format) => blockedText(recipients, format))
              .filter((text, index, all) => all.indexOf(text) === index)
              .join(" "),
            icon: <Settings2 className="h-4 w-4" aria-hidden />,
            href: traegerPath(EXPORT_RECIPIENT_PATH),
            onClick: () => undefined,
          },
        ]
      : []),
  ];
  return (
    <OverflowMenu
      ariaLabel={`Dateien von ${run.label} an die Empfänger übergeben`}
      disabled={busy}
      items={items}
      triggerButton={{ variant: size === "md" ? "outline" : "surface", size }}
      // In a card head it stands beside full-width actions on a phone.
      wrapperClassName={size === "md" ? "w-full min-[480px]:w-auto" : undefined}
      triggerClassName={
        size === "md"
          ? "w-full justify-center gap-1.5 whitespace-nowrap"
          : "gap-1.5 whitespace-nowrap"
      }
      triggerContent={
        <>
          <Send className="h-4 w-4" aria-hidden />
          {running ? "Wird übergeben…" : "Übergeben"}
          <ChevronDown className="h-4 w-4" aria-hidden />
        </>
      }
    />
  );
}

const PROTOCOL_LABELS: Record<ProtocolHandover["state"], string> = {
  transferred: "An Empfänger übergeben",
  failed: "Übergabe nicht angekommen",
  in_progress: "Wird übergeben",
  unknown: "Übergabe unklar",
};

/** The handover of a protocol entry: where it went and whether it arrived. */
export function HandoverBadge({
  handover,
}: Readonly<{ handover: ProtocolHandover }>) {
  const target = destinationText(handover.targetHost, handover.targetDirectory);
  let title = handover.recipientName
    ? `Empfänger: ${handover.recipientName}, ${target}`
    : `Ziel: ${target}`;
  if (handover.state === "failed")
    title += `. ${handoverReasonText(handover.reason)}`;
  if (handover.state === "unknown") title += `. ${HANDOVER_UNKNOWN_TEXT}`;
  return (
    <StatusBadge
      wrap
      label={
        handover.state === "transferred" && handover.recipientName
          ? `An ${handover.recipientName} übergeben`
          : PROTOCOL_LABELS[handover.state]
      }
      tone={HANDOVER_STATE_TONES[handover.state]}
      icon={Send}
      title={title}
    />
  );
}
