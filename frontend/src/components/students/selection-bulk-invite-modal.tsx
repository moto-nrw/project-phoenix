"use client";

import { useEffect, useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Checkbox } from "~/components/ui/checkbox";
import { useFormError } from "~/components/ui/form-error";
import { FormModal } from "~/components/ui/form-modal";
import { Skeleton } from "~/components/ui/skeleton";
import { useToast } from "~/contexts/ToastContext";
import {
  bulkInviteGuardians,
  type BulkInviteProblem,
  type BulkInviteResult,
} from "~/lib/guardian-bulk-invite-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SelectionBulkInviteModal" });

const FAILED_MESSAGE =
  "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";

const PROBLEM_LABEL: Record<BulkInviteProblem["reason"], string> = {
  missing_email: "E-Mail-Adresse fehlt",
  invalid_email: "E-Mail-Adresse ist falsch geschrieben",
  duplicate_email: "Gleiche E-Mail-Adresse wie eine andere Person",
};

function parents(count: number): string {
  return count === 1 ? "1 Elternteil" : `${count} Eltern`;
}

interface SelectionBulkInviteModalProps {
  isOpen: boolean;
  onClose: () => void;
  studentIds: string[];
}

/**
 * Sammeleinladung ins Elternportal für die ausgewählten Kinder (#3378). Der
 * Dialog zählt zuerst nur (dry run) und verschickt erst nach dem Klick.
 * Gezählt werden Eltern, nicht Kinder: ein Elternteil mit drei Kindern
 * bekommt eine E-Mail.
 */
export function SelectionBulkInviteModal({
  isOpen,
  onClose,
  studentIds,
}: SelectionBulkInviteModalProps) {
  const { success } = useToast();
  const [preview, setPreview] = useState<BulkInviteResult | null>(null);
  const [sent, setSent] = useState<BulkInviteResult | null>(null);
  const [resendOpen, setResendOpen] = useState(false);
  const [sending, setSending] = useState(false);
  const [error, setError] = useFormError();
  const selectionKey = studentIds.join(",");

  useEffect(() => {
    if (!isOpen) return;
    let cancelled = false;
    bulkInviteGuardians(selectionKey.split(","), { dryRun: true, resendOpen })
      .then((result) => {
        if (!cancelled) setPreview(result);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("bulk_invite_preview_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setError(FAILED_MESSAGE);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, selectionKey, resendOpen, setError]);

  const toMail = preview ? preview.invited + preview.resent : 0;

  const handleResendOpenChange = (value: boolean) => {
    setPreview(null);
    setError(null);
    setResendOpen(value);
  };

  const handleSend = async () => {
    setSending(true);
    setError(null);
    try {
      const result = await bulkInviteGuardians(studentIds, {
        dryRun: false,
        resendOpen,
      });
      setSent(result);
      success(
        `${parents(result.invited + result.resent + result.linkedExistingAccount)} eingeladen`,
      );
    } catch (err) {
      logger.error("bulk_invite_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setError(FAILED_MESSAGE);
    } finally {
      setSending(false);
    }
  };

  return (
    <FormModal
      isOpen={isOpen}
      onClose={onClose}
      title="Eltern ins Elternportal einladen"
      size="md"
      mobilePosition="bottom"
      closeDisabled={sending}
      error={error}
      footer={
        <div className="flex justify-end gap-2 p-4">
          {sent ? (
            <Button type="button" variant="outline" size="md" onClick={onClose}>
              Schließen
            </Button>
          ) : (
            <>
              <Button
                type="button"
                variant="outline"
                size="md"
                onClick={onClose}
                disabled={sending}
              >
                Abbrechen
              </Button>
              <Button
                type="button"
                variant="success"
                size="md"
                onClick={handleSend}
                disabled={sending || toMail === 0}
              >
                {sending
                  ? "Wird gesendet..."
                  : toMail === 0
                    ? "Einladen"
                    : `${parents(toMail)} einladen`}
              </Button>
            </>
          )}
        </div>
      }
    >
      <div className="space-y-4 p-4">
        {sent ? (
          <SentSummary result={sent} />
        ) : preview ? (
          <PreviewSummary
            preview={preview}
            childCount={studentIds.length}
            resendOpen={resendOpen}
            onResendOpenChange={handleResendOpenChange}
            disabled={sending}
          />
        ) : error ? null : (
          <div className="space-y-2" aria-busy="true">
            <Skeleton className="h-5 w-3/4" />
            <Skeleton className="h-5 w-1/2" />
          </div>
        )}
        {(sent ?? preview)?.problems.length ? (
          <ProblemList problems={(sent ?? preview)?.problems ?? []} />
        ) : null}
      </div>
    </FormModal>
  );
}

function PreviewSummary({
  preview,
  childCount,
  resendOpen,
  onResendOpenChange,
  disabled,
}: {
  preview: BulkInviteResult;
  childCount: number;
  resendOpen: boolean;
  onResendOpenChange: (value: boolean) => void;
  disabled: boolean;
}) {
  const toMail = preview.invited + preview.resent;
  const openCount = preview.skippedOpen + preview.resent;
  return (
    <>
      <p className="text-sm text-gray-900">
        {toMail === 0 ? (
          "Für diese Auswahl gibt es niemanden einzuladen."
        ) : (
          <>
            <strong>{parents(toMail)}</strong>{" "}
            {toMail === 1 ? "bekommt" : "bekommen"} eine E-Mail mit der
            Einladung. Ausgewählt:{" "}
            {childCount === 1 ? "1 Kind" : `${childCount} Kinder`}.
          </>
        )}
      </p>
      {toMail > 0 ? (
        <p className="text-sm text-gray-600">
          Sagen Sie den Eltern vorher Bescheid, dass eine E-Mail von moto kommt.
        </p>
      ) : null}
      <ul className="space-y-1 text-sm text-gray-700">
        <CountRow
          label="Nutzen das Elternportal schon"
          count={preview.skippedActive}
        />
        <CountRow label="Einladung ist noch offen" count={openCount} />
        <CountRow
          label="Nur Kontakt, zum Beispiel abholberechtigt. Wird nicht eingeladen"
          count={preview.skippedRestricted}
        />
      </ul>
      {openCount > 0 ? (
        <label
          htmlFor="bulk-invite-resend-open"
          className="flex cursor-pointer items-start gap-3 text-sm text-gray-800"
        >
          <Checkbox
            id="bulk-invite-resend-open"
            checked={resendOpen}
            disabled={disabled}
            onChange={(event) => onResendOpenChange(event.target.checked)}
          />
          <span>Offene Einladungen noch einmal senden</span>
        </label>
      ) : null}
    </>
  );
}

function CountRow({ label, count }: { label: string; count: number }) {
  if (count === 0) return null;
  return (
    <li className="flex items-baseline justify-between gap-4">
      <span>{label}</span>
      <span className="font-semibold text-gray-900 tabular-nums">{count}</span>
    </li>
  );
}

function SentSummary({ result }: { result: BulkInviteResult }) {
  const messages = [
    result.invited > 0
      ? `Die Einladung an ${parents(result.invited)} wird jetzt verschickt.`
      : null,
    result.resent > 0
      ? `Die Einladung an ${parents(result.resent)} wird noch einmal verschickt.`
      : null,
    result.linkedExistingAccount > 0
      ? `Der Hinweis an ${parents(result.linkedExistingAccount)} mit moto-Konto wird jetzt verschickt.`
      : null,
  ].filter((message): message is string => message !== null);
  const summary =
    messages.length > 0
      ? `${messages.join(" ")} Das dauert ein paar Minuten.`
      : "Es wurde keine E-Mail eingeplant.";
  return (
    <div className="space-y-2">
      <Alert type="success" message={summary} />
      {result.linkedExistingAccount > 0 ? (
        <p className="text-sm text-gray-700">
          {parents(result.linkedExistingAccount)} mit vorhandenem moto-Konto:
          Zugang ist freigeschaltet.
        </p>
      ) : null}
    </div>
  );
}

function ProblemList({ problems }: { problems: BulkInviteProblem[] }) {
  return (
    <div className="space-y-2">
      <Alert
        type="warning"
        message={`${parents(problems.length)} ${problems.length === 1 ? "kann" : "können"} nicht eingeladen werden. Tragen Sie die E-Mail-Adresse beim Kind ein. Laden Sie danach noch einmal ein.`}
      />
      <ul className="max-h-48 space-y-1 overflow-y-auto text-sm text-gray-700">
        {problems.map((problem) => (
          <li key={problem.guardianProfileId}>
            <span className="font-medium text-gray-900">
              {problem.guardianName || "Ohne Namen"}
            </span>
            {problem.studentNames.length > 0
              ? ` (${problem.studentNames.join(", ")})`
              : ""}
            : {PROBLEM_LABEL[problem.reason]}
          </li>
        ))}
      </ul>
    </div>
  );
}
