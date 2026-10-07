"use client";

import { useEffect, useState } from "react";

import { Button } from "~/components/ui/button";
import { formErrorMessage } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { Loading } from "~/components/ui/loading";
import { Modal } from "~/components/ui/modal";
import { useApiLoadError } from "~/contexts/ToastContext";
import { formatChatDateTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import { fetchNoticeAcknowledgements } from "~/lib/staff-notices-api";
import type {
  StaffNotice,
  StaffNoticeAcknowledger,
} from "~/lib/staff-notices-api";

const logger = createLogger({ component: "StaffNoticeAcknowledgementsModal" });

/**
 * Die Bestätigungsliste einer Tagesinformation (#2208): wer wann zur Kenntnis
 * genommen hat. Für die Leitung — die Zahl in der Liste sagt, dass jemand
 * bestätigt hat, diese Liste sagt, wer. Geladen beim Öffnen, nicht mit der
 * Seite: die Namen braucht man selten, die Zahl immer.
 */
export function StaffNoticeAcknowledgementsModal({
  notice,
  onClose,
}: {
  readonly notice: StaffNotice | null;
  readonly onClose: () => void;
}) {
  const [rows, setRows] = useState<StaffNoticeAcknowledger[] | null>(null);
  const [loadFailed, setLoadFailed] = useState(false);
  // Erhöht sich bei „Wiederholen“ und lädt die Liste neu.
  const [loadAttempt, setLoadAttempt] = useState(0);
  // Der Ladefehler steht im Dialog, mit Wiederholen (#2517).
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;

  useEffect(() => {
    if (!notice) return;
    let cancelled = false;
    setRows(null);
    setLoadFailed(false);
    clearLoadError();
    fetchNoticeAcknowledgements(notice.id)
      .then((data) => {
        if (!cancelled) setRows(data);
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        logger.error("staff_notice_acknowledgements_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        setLoadFailed(true);
        void showLoadError(err, {
          object: "die Liste der Bestätigungen",
          retry: () => setLoadAttempt((attempt) => attempt + 1),
        });
      });
    return () => {
      cancelled = true;
    };
  }, [notice, loadAttempt, showLoadError, clearLoadError]);

  return (
    <Modal
      isOpen={notice !== null}
      onClose={onClose}
      title="Bestätigungen"
      footer={
        <div className="flex justify-end">
          <Button type="button" variant="outline" size="md" onClick={onClose}>
            Schließen
          </Button>
        </div>
      }
    >
      {notice && (
        <div className="space-y-4">
          <p className="text-sm leading-6 text-gray-600">
            Diese Personen haben „{notice.title}“ zur Kenntnis genommen.
          </p>

          {loadFailed && formErrorMessage(loadError.error) !== null ? (
            <LoadErrorAlert error={loadError.error} />
          ) : rows === null || loadFailed ? (
            <Loading fullPage={false} />
          ) : rows.length === 0 ? (
            <p className="text-sm leading-6 text-gray-600">
              Noch niemand hat bestätigt.
            </p>
          ) : (
            <ul className="divide-y divide-gray-100">
              {rows.map((row) => (
                <li
                  key={row.account_id}
                  className="flex items-center justify-between gap-3 py-2 first:pt-0 last:pb-0"
                >
                  <span className="text-sm font-medium text-gray-900">
                    {row.name}
                  </span>
                  <span className="text-sm text-gray-500">
                    {formatChatDateTime(row.acknowledged_at)}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}
    </Modal>
  );
}
