"use client";

import { useCallback, useEffect, useState } from "react";
import { KeyRound, Plus, Trash2 } from "lucide-react";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
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
      setName("Dieses Gerät");
      setCode("");
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

  const revoke = async (id: number) => {
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
      <div className="mb-4 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <KeyRound className="h-5 w-5 text-gray-800" aria-hidden="true" />
          <h3 className="text-base font-semibold text-gray-900">Passkeys</h3>
        </div>
        {supported && !enrolling && (
          <Button
            type="button"
            variant="outline"
            size="sm"
            disabled={busy}
            onClick={() => void startEnrollment()}
          >
            <Plus className="h-4 w-4" aria-hidden="true" />
            Hinzufügen
          </Button>
        )}
      </div>

      {!supported && (
        <p className="text-sm text-gray-600">
          Passkeys werden von diesem Browser nicht unterstützt.
        </p>
      )}

      {error && (
        <div
          role="alert"
          className="mb-3 rounded-lg bg-red-50 p-3 text-sm text-red-700"
        >
          {error}
        </div>
      )}
      {message && (
        <div className="mb-3 rounded-lg bg-green-50 p-3 text-sm text-green-700">
          {message}
        </div>
      )}

      {enrolling && (
        <div className="mb-4 space-y-3 rounded-lg border border-gray-200 bg-white p-3">
          {maskedEmail && (
            <p className="text-sm text-gray-600">
              Code gesendet an {maskedEmail}
            </p>
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
                className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-lg border border-gray-200 text-gray-600 transition-colors hover:bg-red-50 hover:text-red-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
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
