"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { Mail, Plus, Trash2 } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { Input } from "~/components/ui/input";
import { EmptyState } from "~/components/ui/empty-state";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import {
  FormErrorAlert,
  LoadErrorAlert,
} from "~/components/ui/form-error-alert";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { credentialError } from "~/components/auth/credential-error";
import {
  useApiFormError,
  useApiLoadError,
  useToast,
} from "~/contexts/ToastContext";
import { suggestCurrentDeviceLabel } from "~/lib/device-label";
import { createLogger } from "~/lib/logger";
import {
  isPasskeyCeremonyIncompleteError,
  isPasskeySupported,
  listPasskeys,
  registerPasskey,
  revokePasskey,
  startPasskeyEnrollment,
  type PasskeyCredentialSummary,
  type PasskeyScope,
} from "~/lib/passkey-api";

const logger = createLogger({ component: "PasskeySettingsSection" });

interface PasskeySettingsSectionProps {
  readonly scope?: PasskeyScope;
}

export function PasskeySettingsSection({
  scope = "tenant",
}: PasskeySettingsSectionProps) {
  const [credentials, setCredentials] = useState<PasskeyCredentialSummary[]>(
    [],
  );
  const [supported, setSupported] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [busy, setBusy] = useState(false);
  const [confirmingEnrollment, setConfirmingEnrollment] = useState(false);
  const [enrolling, setEnrolling] = useState(false);
  const [maskedEmail, setMaskedEmail] = useState("");
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  // Ein Passkey ist ein Zugangsmittel: die Zeilenaktion öffnet erst die
  // Rückfrage (BAUARTEN-SPEC Bauart 2 Regel 6, #3109); entfernt wird im Dialog.
  const [revokeTarget, setRevokeTarget] =
    useState<PasskeyCredentialSummary | null>(null);
  const toast = useToast();
  // Fehlerweg (#2517): Ladefehler vor Ort mit Wiederholen, Fehler beim
  // Einrichten im Einrichtungsbereich, Fehler beim Entfernen im Dialog.
  const { error: loadError, show: showLoadError, clear } = useApiLoadError();
  const enrollRef = useRef<HTMLDivElement>(null);
  const enrollErrors = useApiFormError(enrollRef);
  const revokeErrors = useApiFormError();
  const latestLoadRef = useRef<() => void>(() => undefined);
  const latestStartRef = useRef<() => void>(() => undefined);
  const latestFinishRef = useRef<() => void>(() => undefined);
  const latestRevokeRef = useRef<() => void>(() => undefined);

  const loadCredentials = useCallback(async () => {
    setLoading(true);
    setLoadFailed(false);
    clear();
    try {
      setCredentials(await listPasskeys(scope));
    } catch (err) {
      logger.error("passkey_list_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setLoadFailed(true);
      void showLoadError(err, {
        object: "die Liste der Passkeys",
        retry: () => latestLoadRef.current(),
      });
    } finally {
      setLoading(false);
    }
  }, [clear, scope, showLoadError]);

  useEffect(() => {
    setSupported(isPasskeySupported());
    void loadCredentials();
  }, [loadCredentials]);

  const startEnrollment = async () => {
    setBusy(true);
    enrollErrors.clear();
    try {
      const challenge = await startPasskeyEnrollment(scope);
      setMaskedEmail(challenge.masked_email);
      setName(suggestCurrentDeviceLabel());
      setCode("");
      setConfirmingEnrollment(false);
      setEnrolling(true);
    } catch (err) {
      logger.warn("passkey_enrollment_start_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void enrollErrors.show(err, {
        object: "das Senden des Sicherheitscodes",
        retry: () => latestStartRef.current(),
      });
    } finally {
      setBusy(false);
    }
  };

  const finishEnrollment = async () => {
    setBusy(true);
    enrollErrors.clear();
    try {
      await registerPasskey(scope, { code, name });
      setConfirmingEnrollment(false);
      setEnrolling(false);
      setCode("");
      setName("");
      setMaskedEmail("");
      toast.success("Der Passkey ist hinzugefügt.");
      await loadCredentials();
    } catch (err) {
      // Abgebrochen am Gerät: kein Fehler, der Bereich bleibt offen und
      // „Speichern“ startet den Vorgang erneut.
      if (isPasskeyCeremonyIncompleteError(err)) {
        logger.info("passkey_registration_not_completed", {
          error: err instanceof Error ? err.message : String(err),
        });
        return;
      }
      logger.warn("passkey_registration_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      // Ein falscher Code kommt als 401: das ist keine abgelaufene Sitzung,
      // also kein Sprung zur Anmeldung.
      void enrollErrors.show(
        credentialError(err, ["identity.mfa_code_invalid"]),
        {
          object: "das Hinzufügen des Passkeys",
          retry: () => latestFinishRef.current(),
        },
      );
    } finally {
      setBusy(false);
    }
  };

  const beginRevoke = (credential: PasskeyCredentialSummary) => {
    enrollErrors.clear();
    revokeErrors.clear();
    setRevokeTarget(credential);
  };

  const revoke = async () => {
    if (!revokeTarget) return;
    setBusy(true);
    revokeErrors.clear();
    try {
      await revokePasskey(scope, revokeTarget.id);
      setRevokeTarget(null);
      toast.success("Der Passkey ist entfernt.");
      await loadCredentials();
    } catch (err) {
      logger.warn("passkey_revoke_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void revokeErrors.show(err, {
        object: "das Entfernen des Passkeys",
        retry: () => latestRevokeRef.current(),
      });
    } finally {
      setBusy(false);
    }
  };

  useLayoutEffect(() => {
    latestLoadRef.current = () => void loadCredentials();
    latestStartRef.current = () => void startEnrollment();
    latestFinishRef.current = () => void finishEnrollment();
    latestRevokeRef.current = () => void revoke();
  });

  return (
    <div className="moto-content-surface rounded-2xl border p-4 backdrop-blur-sm md:p-6">
      <ConceptSectionHeader
        className="mb-4"
        // Geschwisterkarten auf /profile und /operator/settings sind h3;
        // als h2 wuerde TrustedDevicesSection darunter einsortiert.
        level={3}
        title="Passkeys"
        concept="passkeys"
        actions={
          supported &&
          !confirmingEnrollment &&
          !enrolling && (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-2"
              disabled={busy}
              onClick={() => {
                enrollErrors.clear();
                setConfirmingEnrollment(true);
              }}
            >
              <Plus className="h-4 w-4" aria-hidden="true" />
              Hinzufügen
            </Button>
          )
        }
      />

      {!supported && (
        <div className="mb-3">
          <Alert
            type="info"
            message="Passkeys werden von diesem Browser nicht unterstützt."
          />
        </div>
      )}

      {confirmingEnrollment && (
        <div
          ref={enrollRef}
          className="mb-4 space-y-3 rounded-lg border border-gray-200 bg-white p-3"
        >
          <FormErrorAlert message={enrollErrors.error} />
          <div className="space-y-1">
            <p className="text-sm font-medium text-gray-900">
              Sicherheitscode per E-Mail senden
            </p>
            <p className="text-sm text-gray-600">
              Zum Einrichten senden wir einen sechsstelligen Code an Ihre
              E-Mail-Adresse. Öffnen Sie danach Ihr E-Mail-Postfach und geben
              Sie den Code hier ein.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              variant="primary"
              size="md"
              className="gap-2"
              disabled={busy}
              onClick={() => void startEnrollment()}
            >
              <Mail className="h-4 w-4" aria-hidden="true" />
              E-Mail senden
            </Button>
            <Button
              type="button"
              variant="outline"
              size="md"
              disabled={busy}
              onClick={() => {
                setConfirmingEnrollment(false);
                enrollErrors.clear();
              }}
            >
              Abbrechen
            </Button>
          </div>
        </div>
      )}

      {enrolling && (
        <div
          ref={enrollRef}
          className="mb-4 space-y-3 rounded-lg border border-gray-200 bg-white p-3"
        >
          <FormErrorAlert message={enrollErrors.error} />
          {maskedEmail && (
            <div className="space-y-1">
              <p className="text-sm font-medium text-gray-900">
                Code gesendet an {maskedEmail}
              </p>
              <p className="text-sm text-gray-600">
                Öffnen Sie Ihr E-Mail-Postfach und tragen Sie den sechsstelligen
                Code ein.
              </p>
            </div>
          )}
          <Input
            label="Code"
            name="passkey-code"
            value={code}
            onChange={(event) => setCode(event.target.value)}
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={6}
          />
          <Input
            label="Name"
            name="passkey-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            maxLength={80}
          />
          <div className="flex gap-2">
            <Button
              type="button"
              variant="primary"
              size="sm"
              disabled={busy || code.trim().length === 0}
              onClick={() => void finishEnrollment()}
            >
              Speichern
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => {
                setConfirmingEnrollment(false);
                setEnrolling(false);
                setCode("");
                setName("");
                setMaskedEmail("");
                enrollErrors.clear();
              }}
            >
              Abbrechen
            </Button>
          </div>
        </div>
      )}

      {/* Bis der Text eines Ladefehlers da ist, bleibt der Ladehinweis
          stehen; danach steht der Fehler statt einer leeren Liste. */}
      {loading || (loadFailed && !loadError) ? (
        <p className="text-sm text-gray-500">Laden...</p>
      ) : loadFailed ? (
        <LoadErrorAlert error={loadError} />
      ) : credentials.length === 0 ? (
        <EmptyState
          variant="compact"
          title="Keine Passkeys hinterlegt."
          description={
            supported
              ? "Legen Sie über „Hinzufügen“ einen Passkey an, um sich ohne Passwort anzumelden."
              : undefined
          }
        />
      ) : (
        <div className="space-y-2">
          {credentials.map((credential) => (
            <div
              key={credential.id}
              className="flex items-center justify-between gap-3 rounded-lg border border-gray-200 bg-white px-3 py-2"
            >
              <div className="min-w-0">
                <p className="truncate text-sm font-medium text-gray-900">
                  {credential.name || "Passkey"}
                </p>
                <p className="text-xs text-gray-500">
                  {credential.last_used_at
                    ? `Zuletzt verwendet: ${formatDate(credential.last_used_at)}`
                    : `Erstellt: ${formatDate(credential.created_at)}`}
                </p>
              </div>
              <OverflowMenu
                ariaLabel={`Aktionen für ${credential.name || "Passkey"}`}
                items={[
                  {
                    label: "Entfernen",
                    icon: <Trash2 className="h-4 w-4" aria-hidden="true" />,
                    destructive: true,
                    disabled: busy,
                    onClick: () => beginRevoke(credential),
                  },
                ]}
              />
            </div>
          ))}
        </div>
      )}

      <ConfirmDeleteModal
        isOpen={revokeTarget !== null}
        title="Passkey entfernen?"
        description={
          <>
            Der Passkey{" "}
            <span className="font-medium text-gray-900">
              {revokeTarget?.name || "Passkey"}
            </span>{" "}
            wird entfernt. Die Anmeldung damit ist danach nicht mehr möglich.
            Sie können jederzeit einen neuen Passkey hinzufügen.
          </>
        }
        gate={{ mode: "twoStep", firstStepLabel: "Ja, entfernen" }}
        confirmLabel="Endgültig entfernen"
        loadingLabel="Wird entfernt…"
        loading={busy}
        error={revokeErrors.error}
        onConfirm={() => void revoke()}
        onClose={() => {
          setRevokeTarget(null);
          revokeErrors.clear();
        }}
      />
    </div>
  );
}

function formatDate(value: string): string {
  return new Date(value).toLocaleDateString("de-DE", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
  });
}
