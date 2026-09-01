"use client";

import { useCallback, useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import { MOTO_COLOR_PALETTE } from "~/lib/location-helper";
import {
  adminGetMFAState,
  adminResetMFA,
  adminSetMFAOverride,
  germanMFAErrorMessage,
  operatorAdminGetGlobalMFAOverride,
  operatorAdminResetMFA,
  operatorAdminSetGlobalMFAOverride,
  operatorAdminSetMFAOverride,
  type MFAAdminOverride,
} from "~/lib/mfa-api";

const logger = createLogger({ component: "MFAAdminOverrideModal" });

const DANGER_RED = MOTO_COLOR_PALETTE.red.base;
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

  // Account-wide override section state. Only rendered on the operator
  // surface (`scope === "operator"`) — tenant admins intentionally
  // cannot reach the platform-wide row, since flipping it bypasses MFA
  // across every tenant the account belongs to. The "mailbox lockout
  // emergency switch" is operator-only by design. (#1430 review round 2)
  const [globalOverride, setGlobalOverride] =
    useState<MFAAdminOverride>("none");
  const [selectedGlobalOverride, setSelectedGlobalOverride] =
    useState<MFAAdminOverride>("none");
  const [globalStateLoaded, setGlobalStateLoaded] = useState(false);
  const [globalReason, setGlobalReason] = useState("");
  const [globalError, setGlobalError] = useState("");
  const [globalWorking, setGlobalWorking] = useState(false);

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
      setGlobalOverride("none");
      setSelectedGlobalOverride("none");
      setGlobalStateLoaded(false);
      setGlobalReason("");
      setGlobalError("");
      setGlobalWorking(false);
    }
  }, [isOpen]);

  // Tenant-only: the per-school override state. The operator surface
  // does NOT render the per-school section, so it skips this call
  // entirely — otherwise it would hit the per-school endpoint with a
  // school the operator modal isn't even scoped to and 404.
  const loadState = useCallback(async () => {
    if (scope !== "tenant") return;
    try {
      const state = await adminGetMFAState(bearerToken, accountId);
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
  }, [bearerToken, accountId, scope]);

  // Operator-only: the account-wide override row + enrollment flag.
  // This is the operator modal's single source of state (it renders no
  // per-school section), so it also drives `enrolled` + `stateLoaded`
  // for the header.
  const loadGlobalState = useCallback(async () => {
    if (scope !== "operator") return;
    try {
      const state = await operatorAdminGetGlobalMFAOverride(
        bearerToken,
        accountId,
      );
      setGlobalOverride(state.override);
      setSelectedGlobalOverride(state.override);
      setEnrolled(state.enrolled);
      setStateLoaded(true);
    } catch (err) {
      logger.warn("operator_global_mfa_state_load_failed", {
        account_id: accountId,
        error: err instanceof Error ? err.message : String(err),
      });
      setStateLoaded(true);
    } finally {
      setGlobalStateLoaded(true);
    }
  }, [bearerToken, accountId, scope]);

  useEffect(() => {
    if (isOpen) {
      // Fire-and-forget pattern is the project convention for async work in
      // useEffect; errors are surfaced via the function's own try/catch.
      void loadState(); // NOSONAR typescript:S3735 fire-and-forget pattern matches project convention (10+ existing sites)
      void loadGlobalState(); // NOSONAR typescript:S3735 see above
    }
  }, [isOpen, loadState, loadGlobalState]);

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

  const handleGlobalOverrideSubmit = async () => {
    if (selectedGlobalOverride === globalOverride) return;
    const trimmed = globalReason.trim();
    if (trimmed.length < MIN_REASON_LENGTH) {
      setGlobalError(
        `Bitte geben Sie eine Begründung mit mindestens ${MIN_REASON_LENGTH} Zeichen an.`,
      );
      return;
    }
    setGlobalWorking(true);
    setGlobalError("");
    try {
      await operatorAdminSetGlobalMFAOverride(
        bearerToken,
        accountId,
        selectedGlobalOverride,
        trimmed,
      );
      setGlobalOverride(selectedGlobalOverride);
      let msg: string;
      if (selectedGlobalOverride === "force_off") {
        msg = `2FA für ${accountLabel} wurde Account-weit deaktiviert (alle Schulen).`;
      } else if (selectedGlobalOverride === "force_on") {
        msg = `2FA für ${accountLabel} wurde Account-weit erzwungen (alle Schulen).`;
      } else {
        msg = `Account-weite Notfall-Einstellung für ${accountLabel} wurde aufgehoben.`;
      }
      toast.success(msg);
      setGlobalReason("");
    } catch (err) {
      setGlobalError(germanMFAErrorMessage(err));
      logger.warn("operator_global_mfa_override_failed", {
        account_id: accountId,
        target: selectedGlobalOverride,
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setGlobalWorking(false);
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

  const modalContent = (
    <div className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/40 p-4">
      <div className="max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-2xl bg-white p-6 shadow-xl">
        <header className="mb-4 flex items-start justify-between">
          <div>
            <h2 className="text-lg font-semibold text-gray-900">
              Zwei-Faktor-Authentifizierung verwalten
            </h2>
            <p className="text-sm text-gray-600">{accountLabel}</p>
            {stateLoaded && (
              <p className="mt-1 text-xs text-gray-500">
                Aktueller Status:{" "}
                {overrideLabel(
                  scope === "operator" ? globalOverride : override,
                )}
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
                    className="focus-visible:ring-moto-blue block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 disabled:bg-gray-50"
                  />
                </div>
                {error && (
                  <div
                    role="alert"
                    className="bg-moto-red-soft text-moto-red-strong rounded-lg p-3 text-sm"
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

            {scope === "tenant" && (
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
                <div className="border-moto-amber/20 bg-moto-amber-soft text-moto-amber-strong mb-3 rounded-md border p-3 text-xs">
                  <strong className="font-semibold">Wichtig:</strong> Diese
                  Einstellung gilt nur für diese Schule. Wenn der Mitarbeiter an
                  weiteren Schulen aktiv ist, bleibt 2FA dort unverändert.
                </div>

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
                          ? "border-moto-blue bg-moto-blue/5"
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
                      className="focus-visible:ring-moto-blue block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 disabled:bg-gray-50"
                    />
                  </div>
                )}

                {overrideError && (
                  <div
                    role="alert"
                    className="bg-moto-red-soft text-moto-red-strong mt-3 rounded-lg p-3 text-sm"
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
                    {overrideWorking
                      ? "Wird gespeichert…"
                      : "Änderung speichern"}
                  </button>
                </div>
              </section>
            )}

            {scope === "operator" && (
              <section className="mt-6 rounded-lg border-2 border-[#FFCDB8] bg-[#FFFBEB] p-4">
                <h3 className="mb-1 text-sm font-semibold text-gray-900">
                  Notfall: 2FA Account-weit verwalten
                </h3>
                <p className="mb-3 text-xs text-gray-700">
                  Diese Aktion wirkt für{" "}
                  <strong>alle Schulen, in denen der Account aktiv ist</strong>.
                  Nur einsetzen, wenn der Mitarbeiter komplett keinen Zugriff
                  auf sein E-Mail-Postfach hat — also nicht über die
                  Schul-spezifische Einstellung oben gelöst werden kann.
                </p>

                <fieldset
                  className="space-y-2"
                  disabled={globalWorking || !globalStateLoaded}
                >
                  <legend className="sr-only">
                    Account-weite 2FA-Einstellung
                  </legend>
                  {(
                    [
                      {
                        value: "none",
                        title: "Standard (keine Account-weite Vorgabe)",
                        desc: "Jede Schule entscheidet einzeln über die 2FA-Pflicht für diesen Account.",
                      },
                      {
                        value: "force_off",
                        title: "Account-weit deaktivieren",
                        desc: "2FA ist in jeder Schule abgeschaltet, in der der Account aktiv ist. Alle vertrauenswürdigen Geräte werden entfernt.",
                      },
                      {
                        value: "force_on",
                        title: "Account-weit erzwingen",
                        desc: "2FA ist überall verpflichtend, unabhängig von der jeweiligen Schul-Einstellung.",
                      },
                    ] as const
                  ).map((opt) => (
                    <label
                      key={opt.value}
                      htmlFor={`admin-mfa-global-${opt.value}`}
                      aria-label={opt.title}
                      className={`flex cursor-pointer items-start gap-3 rounded-lg border p-3 transition-colors ${
                        selectedGlobalOverride === opt.value
                          ? "border-[#FFCDB8] bg-white"
                          : "border-gray-200 bg-white hover:bg-gray-50"
                      }`}
                    >
                      <input
                        id={`admin-mfa-global-${opt.value}`}
                        type="radio"
                        name="admin-mfa-global"
                        value={opt.value}
                        checked={selectedGlobalOverride === opt.value}
                        onChange={() => setSelectedGlobalOverride(opt.value)}
                        className="mt-1 h-4 w-4"
                      />
                      <span className="flex-1">
                        <span className="block text-sm font-medium text-gray-900">
                          {opt.title}
                          {globalOverride === opt.value && (
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

                {selectedGlobalOverride !== globalOverride && (
                  <div className="mt-3 space-y-2">
                    <label
                      htmlFor="admin-mfa-global-reason"
                      className="block text-sm font-medium text-gray-700"
                    >
                      Begründung (Pflichtfeld)
                    </label>
                    <textarea
                      id="admin-mfa-global-reason"
                      rows={2}
                      value={globalReason}
                      onChange={(e) => setGlobalReason(e.target.value)}
                      maxLength={500}
                      placeholder="z. B. „Mitarbeiter hat dauerhaft keinen E-Mail-Zugriff mehr — Account-weite Notfall-Freischaltung"
                      disabled={globalWorking}
                      className="focus-visible:ring-moto-red block w-full rounded-lg border-0 bg-white px-3 py-2 text-sm text-gray-900 shadow-sm ring-1 ring-gray-200 transition-all ring-inset focus:outline-none focus-visible:ring-2 disabled:bg-gray-50"
                    />
                  </div>
                )}

                {globalError && (
                  <div
                    role="alert"
                    className="bg-moto-red-soft text-moto-red-strong mt-3 rounded-lg p-3 text-sm"
                  >
                    {globalError}
                  </div>
                )}

                <div className="mt-4 flex justify-end">
                  <button
                    type="button"
                    disabled={
                      globalWorking || selectedGlobalOverride === globalOverride
                    }
                    onClick={() => void handleGlobalOverrideSubmit()}
                    className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition-all hover:opacity-90 disabled:opacity-50"
                    style={{ backgroundColor: DANGER_RED }}
                  >
                    {globalWorking
                      ? "Wird gespeichert…"
                      : "Account-weit speichern"}
                  </button>
                </div>
              </section>
            )}
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

  if (typeof document !== "undefined") {
    return createPortal(modalContent, document.body);
  }

  return modalContent;
}
