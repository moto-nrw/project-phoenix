"use client";

import { useCallback, useEffect, useState } from "react";
import { useToast } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  disableMFA,
  germanMFAErrorMessage,
  getMFAStatus,
  listTrustedDevices,
  regenerateRecoveryCodes,
  revokeTrustedDevice,
  type LoginScope,
  type MFAStatus,
  type MFATrustedDevice,
} from "~/lib/mfa-api";
import { MFAEnrollmentScreen } from "./mfa-enrollment-screen";
import { RecoveryCodesDisplay } from "./recovery-codes-display";

const logger = createLogger({ component: "MFASecuritySettings" });

const PRIMARY_GREEN = "#83CD2D";
const PRIMARY_BLUE = "#5080D8";
const DANGER_RED = "#FF3130";

interface Props {
  readonly scope: LoginScope;
  readonly bearerToken: string;
  readonly userEmail: string;
}

type Modal =
  | { kind: "none" }
  | { kind: "enroll" }
  | { kind: "regenerate-confirm" }
  | { kind: "regenerate-display"; codes: string[] }
  | { kind: "disable-confirm" };

function formatDate(value?: string): string {
  if (!value) return "—";
  try {
    return new Date(value).toLocaleString("de-DE", {
      dateStyle: "medium",
      timeStyle: "short",
    });
  } catch {
    return value;
  }
}

export function MFASecuritySettings({ scope, bearerToken, userEmail }: Props) {
  const toast = useToast();
  const [status, setStatus] = useState<MFAStatus | null>(null);
  const [devices, setDevices] = useState<MFATrustedDevice[] | null>(null);
  const [modal, setModal] = useState<Modal>({ kind: "none" });
  const [isWorking, setIsWorking] = useState(false);

  const refresh = useCallback(async () => {
    try {
      const [s, d] = await Promise.all([
        getMFAStatus(scope, bearerToken),
        listTrustedDevices(scope, bearerToken).catch(() => []),
      ]);
      setStatus(s);
      setDevices(d);
    } catch (err) {
      logger.error("mfa_status_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      toast.error("Status konnte nicht geladen werden.");
    }
  }, [scope, bearerToken, toast]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleRegenerate = async () => {
    setIsWorking(true);
    try {
      const result = await regenerateRecoveryCodes(scope, bearerToken);
      setModal({ kind: "regenerate-display", codes: result.recovery_codes });
      await refresh();
    } catch (err) {
      toast.error(germanMFAErrorMessage(err));
    } finally {
      setIsWorking(false);
    }
  };

  const handleDisable = async () => {
    setIsWorking(true);
    try {
      await disableMFA(scope, bearerToken);
      toast.success("Zwei-Faktor-Authentifizierung wurde deaktiviert.");
      setModal({ kind: "none" });
      await refresh();
    } catch (err) {
      toast.error(germanMFAErrorMessage(err));
    } finally {
      setIsWorking(false);
    }
  };

  const handleRevokeDevice = async (id: number) => {
    try {
      await revokeTrustedDevice(scope, bearerToken, id);
      toast.success("Gerät wurde entfernt.");
      await refresh();
    } catch (err) {
      toast.error(germanMFAErrorMessage(err));
    }
  };

  if (!status) {
    return (
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-6 text-sm text-gray-500 backdrop-blur-sm">
        Status wird geladen…
      </div>
    );
  }

  const canDisable = status.enrolled && !status.mode_required;
  const isOperator = scope === "operator";

  return (
    <div className="space-y-6">
      {/* Status card */}
      <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
        <div className="flex items-start justify-between gap-4">
          <div>
            <h3 className="text-base font-semibold text-gray-900">
              Zwei-Faktor-Authentifizierung
            </h3>
            <p className="mt-1 text-sm text-gray-600">
              Zusätzlicher Schutz beim Login per 6-stelligem Code an Ihre
              E-Mail-Adresse.
            </p>
          </div>
          <span
            className="inline-flex items-center rounded-full px-3 py-1 text-xs font-semibold"
            style={{
              backgroundColor: status.enrolled
                ? `${PRIMARY_GREEN}1A`
                : "#F3F4F6",
              color: status.enrolled ? "#4a7a15" : "#6B7280",
            }}
          >
            {status.enrolled ? "Aktiv" : "Inaktiv"}
          </span>
        </div>

        {status.enrolled && (
          <dl className="mt-4 grid grid-cols-1 gap-2 text-sm sm:grid-cols-2">
            <div>
              <dt className="text-gray-500">Zuletzt verwendet</dt>
              <dd className="text-gray-900">
                {formatDate(status.last_used_at)}
              </dd>
            </div>
            <div>
              <dt className="text-gray-500">Wiederherstellungscodes</dt>
              <dd className="text-gray-900">
                {status.unused_recovery_codes} ungenutzt
              </dd>
            </div>
          </dl>
        )}

        {status.mode_required && (
          <p className="mt-3 rounded-lg bg-blue-50 p-3 text-sm text-blue-900">
            {isOperator
              ? "2FA ist für Operator-Konten verpflichtend und kann nicht deaktiviert werden."
              : "2FA ist für Ihr Konto verpflichtend und kann nicht deaktiviert werden."}
          </p>
        )}

        <div className="mt-5 flex flex-wrap gap-3">
          {!status.enrolled && (
            <button
              type="button"
              onClick={() => setModal({ kind: "enroll" })}
              className="rounded-lg px-4 py-2 text-sm font-semibold text-white transition-all hover:opacity-90 focus:outline-none active:scale-95"
              style={{ backgroundColor: PRIMARY_GREEN }}
            >
              2FA aktivieren
            </button>
          )}
          {status.enrolled && (
            <>
              <button
                type="button"
                onClick={() => setModal({ kind: "regenerate-confirm" })}
                className="rounded-lg px-4 py-2 text-sm font-medium text-white transition-all hover:opacity-90 focus:outline-none active:scale-95"
                style={{ backgroundColor: PRIMARY_BLUE }}
              >
                Neue Wiederherstellungscodes
              </button>
              <button
                type="button"
                disabled={!canDisable}
                onClick={() => setModal({ kind: "disable-confirm" })}
                className="rounded-lg border px-4 py-2 text-sm font-medium transition-all focus:outline-none active:scale-95 disabled:cursor-not-allowed disabled:opacity-50"
                style={{
                  borderColor: DANGER_RED,
                  color: DANGER_RED,
                  backgroundColor: "white",
                }}
                title={
                  canDisable ? undefined : "2FA ist Pflicht für dieses Konto"
                }
              >
                2FA deaktivieren
              </button>
            </>
          )}
        </div>
      </div>

      {/* Trusted devices */}
      {status.enrolled && (
        <div className="rounded-2xl border border-gray-100 bg-white/50 p-4 backdrop-blur-sm md:p-6">
          <h3 className="mb-1 text-base font-semibold text-gray-900">
            Vertrauenswürdige Geräte
          </h3>
          <p className="mb-3 text-sm text-gray-600">
            Geräte, auf denen Sie den 2FA-Schritt für eine Weile übersprungen
            haben.
          </p>
          {devices && devices.length === 0 && (
            <p className="text-sm text-gray-500">
              Keine vertrauenswürdigen Geräte hinterlegt.
            </p>
          )}
          {devices && devices.length > 0 && (
            <ul className="divide-y divide-gray-100">
              {devices.map((d) => (
                <li
                  key={d.id}
                  className="flex flex-col gap-2 py-3 sm:flex-row sm:items-center sm:justify-between"
                >
                  <div className="min-w-0 flex-1 text-sm">
                    <p className="truncate font-medium text-gray-900">
                      {d.user_agent ?? "Unbekanntes Gerät"}
                    </p>
                    <p className="text-xs text-gray-500">
                      {d.ip_address ? `IP ${d.ip_address} · ` : ""}gültig bis{" "}
                      {formatDate(d.expires_at)}
                      {d.last_used_at
                        ? ` · zuletzt ${formatDate(d.last_used_at)}`
                        : ""}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => void handleRevokeDevice(d.id)}
                    className="rounded-lg border border-gray-300 px-3 py-1.5 text-xs font-medium text-gray-700 transition-all hover:bg-gray-50 focus:outline-none active:scale-95"
                  >
                    Entfernen
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {/* Modals */}
      {modal.kind === "enroll" && (
        <ConfirmDialog title="Zwei-Faktor-Authentifizierung einrichten">
          <MFAEnrollmentScreen
            scope={scope}
            bearerToken={bearerToken}
            userEmail={userEmail}
            onComplete={async () => {
              setModal({ kind: "none" });
              toast.success("2FA wurde erfolgreich aktiviert.");
              await refresh();
            }}
          />
        </ConfirmDialog>
      )}

      {modal.kind === "regenerate-confirm" && (
        <ConfirmDialog title="Neue Wiederherstellungscodes erzeugen?">
          <p className="text-sm text-gray-600">
            Ihre bestehenden Wiederherstellungscodes werden ungültig. Diese
            Aktion lässt sich nicht rückgängig machen.
          </p>
          <div className="mt-5 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setModal({ kind: "none" })}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              disabled={isWorking}
              onClick={() => {
                void handleRegenerate();
              }}
              className="rounded-lg px-4 py-2 text-sm font-semibold text-white hover:opacity-90 disabled:opacity-50"
              style={{ backgroundColor: PRIMARY_BLUE }}
            >
              {isWorking ? "Wird erzeugt…" : "Neue Codes erzeugen"}
            </button>
          </div>
        </ConfirmDialog>
      )}

      {modal.kind === "regenerate-display" && (
        <ConfirmDialog title="Neue Wiederherstellungscodes">
          <RecoveryCodesDisplay
            codes={modal.codes}
            onConfirm={() => setModal({ kind: "none" })}
            title="Codes notieren"
            intro="Speichern Sie diese Codes an einem sicheren Ort. Alte Codes sind nicht mehr gültig."
          />
        </ConfirmDialog>
      )}

      {modal.kind === "disable-confirm" && (
        <ConfirmDialog title="Zwei-Faktor-Authentifizierung deaktivieren?">
          <p className="text-sm text-gray-600">
            Ihr Konto verliert den zusätzlichen Schutz. Alle
            Wiederherstellungscodes und vertrauenswürdigen Geräte werden
            entfernt. Sie können 2FA jederzeit wieder aktivieren.
          </p>
          <div className="mt-5 flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setModal({ kind: "none" })}
              className="rounded-lg border border-gray-300 px-4 py-2 text-sm font-medium text-gray-700 hover:bg-gray-50"
            >
              Abbrechen
            </button>
            <button
              type="button"
              disabled={isWorking}
              onClick={() => {
                void handleDisable();
              }}
              className="rounded-lg px-4 py-2 text-sm font-semibold text-white hover:opacity-90 disabled:opacity-50"
              style={{ backgroundColor: DANGER_RED }}
            >
              {isWorking ? "Wird deaktiviert…" : "Endgültig deaktivieren"}
            </button>
          </div>
        </ConfirmDialog>
      )}
    </div>
  );
}

function ConfirmDialog({
  title,
  children,
}: {
  readonly title: string;
  readonly children: React.ReactNode;
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4">
      <div className="w-full max-w-lg rounded-2xl bg-white p-6 shadow-xl">
        <h2 className="mb-3 text-lg font-semibold text-gray-900">{title}</h2>
        {children}
      </div>
    </div>
  );
}
