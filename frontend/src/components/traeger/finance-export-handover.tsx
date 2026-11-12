"use client";

// Handover of a finance export to the recipient of the Träger (#6824, ADR
// 0217) on the export page and on the page of the debit file: the menu
// „Übergeben“ of a run, the result of the last handover above the list, and
// the question before a file goes to the recipient a second time. The file
// is made like a download, with its own number and protocol entry.

import {
  ChevronDown,
  FileSpreadsheet,
  Landmark,
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
  fetchExportRecipient,
  type HandoverFormat,
  type HandoverOptions,
  type HandoverResult,
  handOverRunFile,
} from "~/lib/traeger-export-handovers-api";
import type {
  ExportRunPathKind,
  ProtocolHandover,
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

/**
 * The recipient of the Träger and the handover of a run's file. A recipient
 * that cannot be read (no right „Finanz-Export“) leaves the menu out; the
 * downloads stay.
 */
export function useFinanceHandover(
  onHandedOver: (arrived: HandoverResult | null) => void,
) {
  const [recipient, setRecipient] = useState<ExportRecipient | null>(null);
  const [running, setRunning] = useState<string | null>(null);
  const [notice, setNotice] = useState<HandoverNotice | null>(null);
  const [repeat, setRepeat] = useState<HandoverAttempt | null>(null);
  const errors = useApiErrorDisplay();
  const { success } = useToast();

  useEffect(() => {
    let current = true;
    fetchExportRecipient()
      .then((page) => {
        if (current) setRecipient(page.recipient);
      })
      .catch((err: unknown) => {
        // Without the recipient the page keeps its downloads; the menu needs it.
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
            `${result.fileName} ist bei ${destinationText(result.targetHost, result.targetDirectory)} angekommen.`,
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
        if (code === "parent_billing.export_already_handed_over") {
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
    [errors, onHandedOver, success],
  );

  const handOver = useCallback(
    (format: HandoverFormat, run: HandoverRun, options: HandoverOptions = {}) =>
      void start({ format, run, options }),
    [start],
  );

  return {
    recipient,
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
            ? `${fileLabel(repeat)} ist schon beim Empfänger angekommen, ist noch unterwegs oder ihr Ergebnis ist unklar.`
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
        title="Für die Übergabe fehlt der Empfänger"
        message={`${label} wurde nicht erstellt. Tragen Sie zuerst ein, wohin moto die Dateien übergeben soll.`}
        onDismiss={onDismiss}
        action={
          <ButtonLink
            href={traegerPath(EXPORT_RECIPIENT_PATH)}
            variant="ghost_tone"
            size="compact"
          >
            Empfänger eintragen
          </ButtonLink>
        }
      />
    );
  }
  if (notice.kind === "unknown") {
    return (
      <Alert
        type="warning"
        title={`${notice.result.fileName}: Ergebnis unklar`}
        message={HANDOVER_UNKNOWN_TEXT}
        onDismiss={onDismiss}
      />
    );
  }
  return (
    <Alert
      type="warning"
      title={`${label} ist nicht angekommen`}
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

/** The menu „Übergeben“ of one run: one entry per file the run has. */
export function HandoverMenu({
  run,
  recipient,
  formats,
  busy,
  running,
  onHandOver,
}: Readonly<{
  run: HandoverRun;
  recipient: ExportRecipient | null;
  formats: readonly HandoverFormat[];
  busy: boolean;
  running: boolean;
  onHandOver: (format: HandoverFormat) => void;
}>) {
  if (!recipient || formats.length === 0) return null;
  const target = destinationText(recipient.host, recipient.remoteDirectory);
  const icons: Record<HandoverFormat, ReactNode> = {
    sollstellung: <FileSpreadsheet className="h-4 w-4" aria-hidden />,
    datev: <FileSpreadsheet className="h-4 w-4" aria-hidden />,
    pain008: <Landmark className="h-4 w-4" aria-hidden />,
  };
  const items: OverflowMenuItem[] = recipient.ready
    ? formats.map((format) => ({
        label: `${HANDOVER_FORMAT_LABELS[format]} übergeben`,
        description: `Erstellt die Datei und legt sie bei ${target} ab.`,
        icon: icons[format],
        onClick: () => onHandOver(format),
      }))
    : [
        {
          label: "Empfänger eintragen",
          description:
            recipient.configured && !recipient.enabled
              ? "Die Übergabe ist ausgeschaltet."
              : "Für die Übergabe fehlen noch Angaben.",
          icon: <Settings2 className="h-4 w-4" aria-hidden />,
          href: traegerPath(EXPORT_RECIPIENT_PATH),
          onClick: () => undefined,
        },
      ];
  return (
    <OverflowMenu
      ariaLabel={`Dateien von ${run.label} an den Empfänger übergeben`}
      disabled={busy}
      items={items}
      triggerButton={{ variant: "surface", size: "compact" }}
      triggerClassName="gap-1.5 whitespace-nowrap"
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
  let title = `Ziel: ${target}`;
  if (handover.state === "failed")
    title += `. ${handoverReasonText(handover.reason)}`;
  if (handover.state === "unknown") title += `. ${HANDOVER_UNKNOWN_TEXT}`;
  return (
    <StatusBadge
      wrap
      label={PROTOCOL_LABELS[handover.state]}
      tone={HANDOVER_STATE_TONES[handover.state]}
      icon={Send}
      title={title}
    />
  );
}
