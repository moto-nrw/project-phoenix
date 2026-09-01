"use client";

import { useCallback, useEffect, useState } from "react";
import { Mail, Plus, Trash2 } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";
import { suggestCurrentDeviceLabel } from "~/lib/device-label";
import {
  isPasskeySupported,
  listPasskeys,
  registerPasskey,
  revokePasskey,
  startPasskeyEnrollment,
  type PasskeyCredentialSummary,
  type PasskeyScope,
} from "~/lib/passkey-api";

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
  const [busy, setBusy] = useState(false);
  const [confirmingEnrollment, setConfirmingEnrollment] = useState(false);
  const [enrolling, setEnrolling] = useState(false);
  const [maskedEmail, setMaskedEmail] = useState("");
  const [code, setCode] = useState("");
  const [name, setName] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const loadCredentials = useCallback(async () => {
    setLoading(true);
    try {
      setCredentials(await listPasskeys(scope));
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Passkeys konnten nicht geladen werden.",
      );
    } finally {
      setLoading(false);
    }
  }, [scope]);

  useEffect(() => {
    setSupported(isPasskeySupported());
    void loadCredentials();
  }, [loadCredentials]);

  const startEnrollment = async () => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      const challenge = await startPasskeyEnrollment(scope);
      setMaskedEmail(challenge.masked_email);
      setName(suggestCurrentDeviceLabel());
      setCode("");
      setConfirmingEnrollment(false);
      setEnrolling(true);
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Passkey-Einrichtung konnte nicht gestartet werden.",
      );
    } finally {
      setBusy(false);
    }
  };

  const finishEnrollment = async () => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await registerPasskey(scope, { code, name });
      setConfirmingEnrollment(false);
      setEnrolling(false);
      setCode("");
      setName("");
      setMaskedEmail("");
      setMessage("Passkey wurde hinzugefügt.");
      await loadCredentials();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Passkey konnte nicht hinzugefügt werden.",
      );
    } finally {
      setBusy(false);
    }
  };

  const revoke = async (id: string) => {
    setBusy(true);
    setError(null);
    setMessage(null);
    try {
      await revokePasskey(scope, id);
      setMessage("Passkey wurde entfernt.");
      await loadCredentials();
    } catch (err) {
      setError(
        err instanceof Error
          ? err.message
          : "Passkey konnte nicht entfernt werden.",
      );
    } finally {
      setBusy(false);
    }
  };

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
                setError(null);
                setMessage(null);
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
        <p className="text-sm text-gray-600">
          Passkeys werden von diesem Browser nicht unterstützt.
        </p>
      )}

      {error && (
        <div className="mb-3">
          <Alert type="error" message={error} />
        </div>
      )}
      {message && (
        <div className="mb-3">
          <Alert type="success" message={message} />
        </div>
      )}

      {confirmingEnrollment && (
        <div className="mb-4 space-y-3 rounded-lg border border-gray-200 bg-white p-3">
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
                setError(null);
              }}
            >
              Abbrechen
            </Button>
          </div>
        </div>
      )}

      {enrolling && (
        <div className="mb-4 space-y-3 rounded-lg border border-gray-200 bg-white p-3">
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
              }}
            >
              Abbrechen
            </Button>
          </div>
        </div>
      )}

      {loading ? (
        <p className="text-sm text-gray-500">Laden...</p>
      ) : credentials.length === 0 ? (
        <p className="text-sm text-gray-600">Keine Passkeys hinterlegt.</p>
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
              <button
                type="button"
                aria-label="Passkey entfernen"
                disabled={busy}
                onClick={() => void revoke(credential.id)}
                className="hover:bg-moto-red-soft hover:text-moto-red-strong inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-gray-200 text-gray-600 transition-colors focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Trash2 className="h-4 w-4" aria-hidden="true" />
              </button>
            </div>
          ))}
        </div>
      )}
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
