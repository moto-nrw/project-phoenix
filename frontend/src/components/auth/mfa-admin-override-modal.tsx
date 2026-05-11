"use client";

import { useEffect, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  adminRegenerateRecoveryCodes,
  adminResetMFA,
  germanMFAErrorMessage,
} from "~/lib/mfa-api";
import { RecoveryCodesDisplay } from "./recovery-codes-display";

const logger = createLogger({ component: "MFAAdminOverrideModal" });

const PRIMARY_BLUE = "#5080D8";
const DANGER_RED = "#FF3130";
const MIN_REASON_LENGTH = 3;

type View =
  | { kind: "menu" }
  | { kind: "reset" }
  | { kind: "regenerate" }
  | { kind: "regenerate-codes"; codes: string[] }
  | { kind: "reset-done" };

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly bearerToken: string;
  readonly accountId: string;
  readonly accountLabel: string;
}

export function MFAAdminOverrideModal({
  isOpen,
  onClose,
  bearerToken,
  accountId,
  accountLabel,
}: Props) {
  const toast = useToast();
  const [view, setView] = useState<View>({ kind: "menu" });
  const [reason, setReason] = useState("");
  const [isWorking, setIsWorking] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!isOpen) {
      setView({ kind: "menu" });
      setReason("");
      setError("");
      setIsWorking(false);
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const handleReset = async () => {
    const trimmed = reason.trim();
    if (trimmed.length < MIN_REASON_LENGTH) {
      setError(
        `Bitte geben Sie einen Grund mit mindestens ${MIN_REASON_LENGTH} Zeichen an.`,
      );
      return;
    }
    setIsWorking(true);
    setError("");
    try {
      await adminResetMFA(bearerToken, accountId, trimmed);
      toast.success(`2FA für ${accountLabel} wurde zurückgesetzt.`);
      setView({ kind: "reset-done" });
    } catch (err) {
      setError(germanMFAErrorMessage(err));
      logger.warn("admin_reset_failed", {
        account_id: accountId,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsWorking(false);
    }
  };

  const handleRegenerate = async () => {
    const trimmed = reason.trim();
    if (trimmed.length < MIN_REASON_LENGTH) {
      setError(
        `Bitte geben Sie einen Grund mit mindestens ${MIN_REASON_LENGTH} Zeichen an.`,
      );
      return;
    }
    setIsWorking(true);
    setError("");
    try {
      const result = await adminRegenerateRecoveryCodes(
        bearerToken,
        accountId,
        trimmed,
      );
      toast.success(
        `Neue Wiederherstellungscodes für ${accountLabel} erzeugt.`,
      );
      setView({ kind: "regenerate-codes", codes: result.recovery_codes });
    } catch (err) {
      setError(germanMFAErrorMessage(err));
      logger.warn("admin_regenerate_failed", {
        account_id: accountId,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setIsWorking(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl">
        <header className="mb-4 flex items-start justify-between">
          <div>
            <h2 className="text-lg font-semibold text-gray-900">
              2FA-Verwaltung
            </h2>
            <p className="text-sm text-gray-600">{accountLabel}</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-700"
            aria-label="Schließen"
          >
            ✕
          </button>
        </header>

        {view.kind === "menu" && (
          <div className="space-y-3">
            <p className="text-sm text-gray-600">
              Wählen Sie eine Aktion. Beide Aktionen werden im Audit-Log mit
              Ihrem Konto und der angegebenen Begründung erfasst.
            </p>
            <button
              type="button"
              onClick={() => {
                setView({ kind: "regenerate" });
                setReason("");
                setError("");
              }}
              className="w-full rounded-lg px-4 py-3 text-left text-sm font-medium text-white transition-all hover:opacity-90 focus:outline-none active:scale-[0.99]"
              style={{ backgroundColor: PRIMARY_BLUE }}
            >
              Wiederherstellungscodes neu generieren
              <span className="block text-xs font-normal opacity-90">
                Alte Codes werden ungültig. Neue Codes werden einmalig
                angezeigt.
              </span>
            </button>
            <button
              type="button"
              onClick={() => {
                setView({ kind: "reset" });
                setReason("");
                setError("");
              }}
              className="w-full rounded-lg border px-4 py-3 text-left text-sm font-medium transition-all hover:bg-red-50 focus:outline-none active:scale-[0.99]"
              style={{ borderColor: DANGER_RED, color: DANGER_RED }}
            >
              2FA komplett zurücksetzen
              <span className="block text-xs font-normal opacity-90">
                Enrollment, Recovery-Codes und alle vertrauenswürdigen Geräte
                werden entfernt.
              </span>
            </button>
          </div>
        )}

        {(view.kind === "reset" || view.kind === "regenerate") && (
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (view.kind === "reset") void handleReset();
              else void handleRegenerate();
            }}
            className="space-y-3"
          >
            <label
              htmlFor="admin-mfa-reason"
              className="block text-sm font-medium text-gray-700"
            >
              Grund (Pflichtfeld, wird im Audit-Log gespeichert)
            </label>
            <textarea
              id="admin-mfa-reason"
              rows={3}
              required
              autoFocus
              value={reason}
              onChange={(e) => setReason(e.target.value)}
              maxLength={500}
              placeholder="z. B. „Mailbox gesperrt, kein Recovery-Code mehr verfügbar"
              disabled={isWorking}
              className="block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-[#5080D8] disabled:bg-gray-50"
            />
            {error && (
              <div
                role="alert"
                className="rounded-lg bg-red-50 p-3 text-sm text-red-700"
              >
                {error}
              </div>
            )}
            <div className="flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setView({ kind: "menu" })}
                disabled={isWorking}
                className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50 disabled:opacity-50"
              >
                Zurück
              </button>
              <button
                type="submit"
                disabled={isWorking}
                className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition-all hover:opacity-90 disabled:opacity-50"
                style={{
                  backgroundColor:
                    view.kind === "reset" ? DANGER_RED : PRIMARY_BLUE,
                }}
              >
                {isWorking
                  ? "Wird ausgeführt…"
                  : view.kind === "reset"
                    ? "2FA endgültig zurücksetzen"
                    : "Codes neu generieren"}
              </button>
            </div>
          </form>
        )}

        {view.kind === "regenerate-codes" && (
          <RecoveryCodesDisplay
            codes={view.codes}
            onConfirm={() => {
              onClose();
            }}
            title="Neue Wiederherstellungscodes"
            intro="Übergeben Sie diese Codes an den/die Betroffene auf einem sicheren Weg. Sie werden nur jetzt einmalig angezeigt."
          />
        )}

        {view.kind === "reset-done" && (
          <div className="space-y-4">
            <p className="text-sm text-gray-700">
              Das 2FA-Setup für{" "}
              <span className="font-semibold">{accountLabel}</span> wurde
              vollständig entfernt. Beim nächsten Login kann sich der Account
              ohne 2FA-Code anmelden — oder wird, wenn 2FA für die Schule
              Pflicht ist, durch den Enrollment-Flow geführt.
            </p>
            <div className="flex justify-end">
              <button
                type="button"
                onClick={onClose}
                className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white hover:bg-gray-800"
              >
                Schließen
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
