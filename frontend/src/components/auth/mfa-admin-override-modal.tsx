"use client";

import { useCallback, useEffect, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  adminGetMFAState,
  adminResetMFA,
  adminSetMFAOverride,
  germanMFAErrorMessage,
  operatorAdminGetMFAState,
  operatorAdminResetMFA,
  operatorAdminSetMFAOverride,
  type MFAAdminOverride,
  type MFAAdminState,
} from "~/lib/mfa-api";

const logger = createLogger({ component: "MFAAdminOverrideModal" });

const DANGER_RED = "#FF3130";
const MIN_REASON_LENGTH = 3;

type View = { kind: "form" } | { kind: "reset-done" };

interface Props {
  readonly isOpen: boolean;
  readonly onClose: () => void;
  readonly bearerToken: string;
  readonly accountId: string;
  readonly accountLabel: string;
  /**
   * Which surface is hosting the modal — drives which set of API
   * helpers we call. Defaults to "tenant" so existing usage (and tests)
   * keep working without changes. The operator dashboard passes
   * "operator" + the relevant schoolId so the proxy hits
   * /api/operator/provisioning/schools/{schoolId}/accounts/{accountId}/mfa
   * instead of the tenant /auth/accounts/{accountId}/mfa path.
   */
  readonly scope?: "tenant" | "operator";
  readonly schoolId?: string;
}

function overrideLabel(override: MFAAdminOverride): string {
  switch (override) {
    case "force_off":
      return "Deaktiv";
    case "force_on":
      return "Aktiv";
    default:
      return "Standard";
  }
}

export function MFAAdminOverrideModal({
  isOpen,
  onClose,
  bearerToken,
  accountId,
  accountLabel,
  scope = "tenant",
  schoolId,
}: Props) {
  const toast = useToast();
  const [view, setView] = useState<View>({ kind: "form" });
  const [reason, setReason] = useState("");
  const [isWorking, setIsWorking] = useState(false);
  const [error, setError] = useState("");

  // Override-section state. The current override drives which toggle
  // buttons are offered. We fetch it lazily once the modal opens.
  const [override, setOverride] = useState<MFAAdminOverride>("none");
  // selectedOverride is the radio-button selection (what the admin
  // *wants* to apply). It defaults to the currently-persisted override
  // so the form opens in a no-op state — saving is only meaningful when
  // the admin changes the value.
  const [selectedOverride, setSelectedOverride] =
    useState<MFAAdminOverride>("none");
  const [enrolled, setEnrolled] = useState<boolean>(false);
  const [stateLoaded, setStateLoaded] = useState(false);
  const [overrideReason, setOverrideReason] = useState("");
  const [overrideError, setOverrideError] = useState("");
  const [overrideWorking, setOverrideWorking] = useState(false);

  useEffect(() => {
    if (!isOpen) {
      setView({ kind: "form" });
      setReason("");
      setError("");
      setIsWorking(false);
      setOverride("none");
      setSelectedOverride("none");
      setEnrolled(false);
      setStateLoaded(false);
      setOverrideReason("");
      setOverrideError("");
      setOverrideWorking(false);
    }
  }, [isOpen]);

  const loadState = useCallback(async () => {
    try {
      let state: MFAAdminState;
      if (scope === "operator") {
        if (!schoolId) {
          throw new Error("schoolId is required for operator scope");
        }
        state = await operatorAdminGetMFAState(
          bearerToken,
          schoolId,
          accountId,
        );
      } else {
        state = await adminGetMFAState(bearerToken, accountId);
      }
      setOverride(state.override);
      setSelectedOverride(state.override);
      setEnrolled(state.enrolled);
    } catch (err) {
      // State load failures don't block the reset path — log and continue
      // with safe defaults. The override section will still render with
      // "none" + an inline notice.
      logger.warn("admin_mfa_state_load_failed", {
        account_id: accountId,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setStateLoaded(true);
    }
  }, [bearerToken, accountId, scope, schoolId]);

  useEffect(() => {
    if (isOpen) {
      // Fire-and-forget pattern is the project convention for async work in
      // useEffect; errors are surfaced via the function's own try/catch.
      void loadState(); //NOSONAR(typescript:S3735)
    }
  }, [isOpen, loadState]);

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
      if (scope === "operator") {
        if (!schoolId) {
          throw new Error("schoolId is required for operator scope");
        }
        await operatorAdminResetMFA(bearerToken, schoolId, accountId, trimmed);
      } else {
        await adminResetMFA(bearerToken, accountId, trimmed);
      }
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

  const handleOverrideSubmit = async () => {
    if (selectedOverride === override) {
      // No-op: the radio matches what's already stored.
      return;
    }
    const trimmed = overrideReason.trim();
    if (trimmed.length < MIN_REASON_LENGTH) {
      setOverrideError(
        `Bitte geben Sie eine Begründung mit mindestens ${MIN_REASON_LENGTH} Zeichen an.`,
      );
      return;
    }
    setOverrideWorking(true);
    setOverrideError("");
    try {
      if (scope === "operator") {
        if (!schoolId) {
          throw new Error("schoolId is required for operator scope");
        }
        await operatorAdminSetMFAOverride(
          bearerToken,
          schoolId,
          accountId,
          selectedOverride,
          trimmed,
        );
      } else {
        await adminSetMFAOverride(
          bearerToken,
          accountId,
          selectedOverride,
          trimmed,
        );
      }
      setOverride(selectedOverride);
      let msg: string;
      if (selectedOverride === "force_off") {
        msg = `2FA für ${accountLabel} wurde deaktiviert.`;
      } else if (selectedOverride === "force_on") {
        msg = `2FA für ${accountLabel} wurde aktiviert.`;
      } else {
        msg = `Für ${accountLabel} gilt wieder die Standard-Einstellung.`;
      }
      toast.success(msg);
      setOverrideReason("");
    } catch (err) {
      setOverrideError(germanMFAErrorMessage(err));
      logger.warn("admin_override_failed", {
        account_id: accountId,
        target: selectedOverride,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setOverrideWorking(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl bg-white p-6 shadow-xl">
        <header className="mb-4 flex items-start justify-between">
          <div>
            <h2 className="text-lg font-semibold text-gray-900">
              Zwei-Faktor-Authentifizierung verwalten
            </h2>
            <p className="text-sm text-gray-600">{accountLabel}</p>
            {stateLoaded && (
              <p className="mt-1 text-xs text-gray-500">
                Aktueller Status: {overrideLabel(override)}
                {enrolled
                  ? " · 2FA bereits eingerichtet"
                  : " · 2FA noch nicht eingerichtet"}
              </p>
            )}
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

        {view.kind === "form" && (
          <>
            <section className="mb-6">
              <h3 className="mb-2 text-sm font-semibold text-gray-900">
                Bisherige 2FA zurücksetzen
              </h3>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  void handleReset();
                }}
                className="space-y-4"
              >
                <p className="text-sm text-gray-600">
                  Die bestehende 2FA-Einrichtung und alle vertrauenswürdigen
                  Geräte werden entfernt. Beim nächsten Login wird der Account
                  neu durch das 2FA-Setup geführt. Die Aktion wird im Audit-Log
                  mit Ihrem Konto und dem angegebenen Grund erfasst.
                </p>
                <div className="space-y-2">
                  <label
                    htmlFor="admin-mfa-reason"
                    className="block text-sm font-medium text-gray-700"
                  >
                    Grund (Pflichtfeld)
                  </label>
                  <textarea
                    id="admin-mfa-reason"
                    rows={3}
                    required
                    autoFocus
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    maxLength={500}
                    placeholder="z. B. „Mitarbeiter hat keinen Zugriff mehr auf sein E-Mail-Postfach"
                    disabled={isWorking}
                    className="block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-[#5080D8] disabled:bg-gray-50"
                  />
                </div>
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
                    type="submit"
                    disabled={isWorking}
                    className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition-all hover:opacity-90 disabled:opacity-50"
                    style={{ backgroundColor: DANGER_RED }}
                  >
                    {isWorking
                      ? "Wird ausgeführt…"
                      : "2FA endgültig zurücksetzen"}
                  </button>
                </div>
              </form>
            </section>

            <section className="border-t border-gray-100 pt-6">
              <h3 className="mb-2 text-sm font-semibold text-gray-900">
                2FA für diesen Mitarbeiter aktivieren oder deaktivieren
              </h3>
              <p className="mb-3 text-sm text-gray-600">
                Wählen Sie, ob für diesen Account die Schul-Einstellung gelten
                soll oder ob 2FA für diesen Mitarbeiter individuell aktiviert
                oder deaktiviert wird — z. B. wenn der Mitarbeiter keinen
                Zugriff auf sein E-Mail-Postfach hat.
              </p>

              <fieldset
                className="space-y-2"
                disabled={overrideWorking || !stateLoaded}
              >
                <legend className="sr-only">2FA-Einstellung</legend>
                {(
                  [
                    {
                      value: "none",
                      title: "Standard",
                      desc: "Die in den Schul-Einstellungen festgelegte 2FA-Pflicht gilt.",
                    },
                    {
                      value: "force_on",
                      title: "Aktiv",
                      desc: "2FA ist für diesen Mitarbeiter immer erforderlich, unabhängig von der Schul-Einstellung.",
                    },
                    {
                      value: "force_off",
                      title: "Deaktiv",
                      desc: "2FA ist für diesen Mitarbeiter abgeschaltet. Vertraute Geräte werden beim Speichern entfernt.",
                    },
                  ] as const
                ).map((opt) => (
                  <label
                    key={opt.value}
                    htmlFor={`admin-mfa-override-${opt.value}`}
                    aria-label={opt.title}
                    className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
                      selectedOverride === opt.value
                        ? "border-[#5080D8] bg-[#5080D8]/5"
                        : "border-gray-200 hover:bg-gray-50"
                    }`}
                  >
                    <input
                      id={`admin-mfa-override-${opt.value}`}
                      type="radio"
                      name="admin-mfa-override"
                      value={opt.value}
                      checked={selectedOverride === opt.value}
                      onChange={() => setSelectedOverride(opt.value)}
                      className="mt-1 h-4 w-4"
                    />
                    <span className="flex-1">
                      <span className="block text-sm font-medium text-gray-900">
                        {opt.title}
                        {override === opt.value && (
                          <span className="ml-2 text-xs font-normal text-gray-500">
                            (aktuell)
                          </span>
                        )}
                      </span>
                      <span className="block text-xs text-gray-600">
                        {opt.desc}
                      </span>
                    </span>
                  </label>
                ))}
              </fieldset>

              {selectedOverride !== override && (
                <div className="mt-3 space-y-2">
                  <label
                    htmlFor="admin-mfa-override-reason"
                    className="block text-sm font-medium text-gray-700"
                  >
                    Begründung (Pflichtfeld)
                  </label>
                  <textarea
                    id="admin-mfa-override-reason"
                    rows={2}
                    value={overrideReason}
                    onChange={(e) => setOverrideReason(e.target.value)}
                    maxLength={500}
                    placeholder="z. B. „Mitarbeiter im Urlaub, Postfach gesperrt"
                    disabled={overrideWorking}
                    className="block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 focus-visible:ring-[#5080D8] disabled:bg-gray-50"
                  />
                </div>
              )}

              {overrideError && (
                <div
                  role="alert"
                  className="mt-3 rounded-lg bg-red-50 p-3 text-sm text-red-700"
                >
                  {overrideError}
                </div>
              )}

              <div className="mt-4 flex justify-end">
                <button
                  type="button"
                  disabled={overrideWorking || selectedOverride === override}
                  onClick={() => void handleOverrideSubmit()}
                  className="rounded-lg bg-gray-900 px-4 py-2 text-sm font-semibold text-white transition-all hover:bg-gray-800 disabled:opacity-50"
                >
                  {overrideWorking ? "Wird gespeichert…" : "Änderung speichern"}
                </button>
              </div>
            </section>
          </>
        )}

        {view.kind === "reset-done" && (
          <div className="space-y-4">
            <p className="text-sm text-gray-700">
              Das 2FA-Setup für{" "}
              <span className="font-semibold">{accountLabel}</span> wurde
              vollständig entfernt. Beim nächsten Login kann sich der Account
              ohne 2FA-Code anmelden — oder wird, wenn 2FA für die Schule
              Pflicht ist, durch die 2FA-Einrichtung geführt.
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
