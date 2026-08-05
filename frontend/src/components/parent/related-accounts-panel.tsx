"use client";

import { useCallback, useEffect, useState } from "react";
import { UserPlus, X, Check } from "lucide-react";
import {
  listRelatedAccounts,
  inviteRelatedAccount,
  removeRelatedAccount,
  type RelatedAccount,
} from "~/lib/parent-api";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { createLogger } from "~/lib/logger";
import { Button } from "~/components/ui/button";
import { ConceptSectionHeader } from "~/components/ui/concept-section-header";

const logger = createLogger({ component: "RelatedAccountsPanel" });

const STATUS_META: Record<
  RelatedAccount["status"],
  { label: string; dot: string }
> = {
  active: { label: "Konto aktiv", dot: LOCATION_COLORS.GROUP_ROOM },
  pending: { label: "Einladung offen", dot: LOCATION_COLORS.SICK },
  no_account: { label: "Kontakt ohne Konto", dot: LOCATION_COLORS.UNKNOWN },
};

function initials(first: string, last: string): string {
  return `${first.charAt(0)}${last.charAt(0)}`.toUpperCase() || "?";
}

interface RelatedAccountsPanelProps {
  readonly studentId: string;
  readonly canInvite: boolean;
  readonly canRemove: boolean;
  readonly mobile?: boolean;
}

export default function RelatedAccountsPanel({
  studentId,
  canInvite,
  canRemove,
  mobile = false,
}: RelatedAccountsPanelProps) {
  const [accounts, setAccounts] = useState<RelatedAccount[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [inviteOpen, setInviteOpen] = useState(false);
  const [email, setEmail] = useState("");
  const [busy, setBusy] = useState(false);
  const [confirmingId, setConfirmingId] = useState<string | null>(null);
  const [message, setMessage] = useState<{
    kind: "success" | "error";
    text: string;
  } | null>(null);

  const load = useCallback(async () => {
    try {
      setIsLoading(true);
      setAccounts(await listRelatedAccounts(studentId));
    } catch (err) {
      logger.error("related_accounts_load_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMessage({
        kind: "error",
        text: "Konten konnten nicht geladen werden",
      });
    } finally {
      setIsLoading(false);
    }
  }, [studentId]);

  useEffect(() => {
    void load();
  }, [load]);

  const handleInvite = async () => {
    const trimmed = email.trim();
    if (!trimmed) return;
    setBusy(true);
    setMessage(null);
    try {
      const result = await inviteRelatedAccount(studentId, trimmed);
      setEmail("");
      setInviteOpen(false);
      await load();
      setMessage({
        kind: "success",
        text:
          result.outcome === "pending_approval"
            ? "Anfrage gesendet – wird von der Einrichtung geprüft."
            : result.outcome === "invited"
              ? `Einladung an ${trimmed} gesendet.`
              : "Person wurde verbunden.",
      });
    } catch (err) {
      logger.error("related_account_invite_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMessage({
        kind: "error",
        text: err instanceof Error ? err.message : "Einladung fehlgeschlagen",
      });
    } finally {
      setBusy(false);
    }
  };

  const handleRemove = async (acc: RelatedAccount) => {
    setBusy(true);
    setMessage(null);
    try {
      await removeRelatedAccount(studentId, acc.guardian_profile_id);
      setConfirmingId(null);
      await load();
      setMessage({ kind: "success", text: "Zugang entfernt." });
    } catch (err) {
      logger.error("related_account_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      setMessage({
        kind: "error",
        text: err instanceof Error ? err.message : "Entfernen fehlgeschlagen",
      });
    } finally {
      setBusy(false);
    }
  };

  return (
    <section
      className={
        mobile
          ? "rounded-[1.75rem] border border-gray-200 bg-white p-5 shadow-sm"
          : "rounded-2xl border border-gray-200 bg-white p-5 shadow-sm sm:p-6"
      }
    >
      <ConceptSectionHeader
        title="Verbundene Konten"
        concept="parents"
        subtitle="Wer Zugriff auf dieses Kind in der Eltern-App hat."
        actions={
          canInvite ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              className="shrink-0 gap-2"
              onClick={() => {
                setInviteOpen((v) => !v);
                setMessage(null);
              }}
            >
              <UserPlus className="h-4 w-4" aria-hidden="true" />
              Einladen
            </Button>
          ) : undefined
        }
      />

      {message && (
        <div
          className={`mt-4 rounded-xl border p-3 text-sm ${
            message.kind === "success"
              ? "border-moto-green/30 bg-moto-green/10 text-moto-green-strong"
              : "border-moto-red/20 bg-moto-red/10 text-moto-red-strong"
          }`}
        >
          {message.text}
        </div>
      )}

      {inviteOpen && canInvite && (
        <div className="mt-4 flex flex-col gap-2 rounded-xl border border-gray-200 bg-gray-50/70 p-3 sm:flex-row">
          <input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="E-Mail-Adresse"
            className="flex-1 rounded-lg border border-gray-300 px-3 py-2 text-sm focus:border-gray-400 focus:outline-none"
          />
          <Button
            type="button"
            size="md"
            className="gap-1"
            onClick={() => void handleInvite()}
            disabled={busy || !email.trim()}
          >
            <Check className="h-4 w-4" />
            Senden
          </Button>
        </div>
      )}

      <div className="mt-5 divide-y divide-gray-100 rounded-xl border border-gray-200 bg-gray-50/70">
        {isLoading ? (
          <div className="p-4 text-sm text-gray-500">Wird geladen…</div>
        ) : accounts.length === 0 ? (
          <div className="p-4 text-sm text-gray-500">
            Noch keine verbundenen Konten.
          </div>
        ) : (
          accounts.map((acc) => {
            const meta = STATUS_META[acc.status];
            const name =
              `${acc.first_name} ${acc.last_name}`.trim() || acc.email;
            return (
              <div
                key={acc.guardian_profile_id}
                className="flex items-center gap-3 p-3"
              >
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-white text-sm font-semibold text-gray-700 ring-1 ring-gray-200">
                  {initials(acc.first_name, acc.last_name)}
                </span>
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-semibold text-gray-900">
                    {name}
                    {acc.is_primary && (
                      <span className="ml-2 text-xs font-medium text-gray-500">
                        (Primär)
                      </span>
                    )}
                  </p>
                  <p className="flex items-center gap-1.5 text-xs text-gray-600">
                    <span
                      className="h-2 w-2 rounded-full"
                      style={{ backgroundColor: meta.dot }}
                    />
                    {meta.label}
                    {acc.email && (
                      <span className="text-gray-400">· {acc.email}</span>
                    )}
                  </p>
                </div>
                {canRemove &&
                  !acc.is_primary &&
                  !acc.is_self &&
                  acc.status !== "no_account" &&
                  (confirmingId === acc.guardian_profile_id ? (
                    <div className="flex shrink-0 items-center gap-1">
                      <Button
                        type="button"
                        variant="danger"
                        size="compact"
                        onClick={() => void handleRemove(acc)}
                        disabled={busy}
                      >
                        Entfernen
                      </Button>
                      <Button
                        type="button"
                        variant="ghost"
                        size="compact"
                        onClick={() => setConfirmingId(null)}
                      >
                        Abbrechen
                      </Button>
                    </div>
                  ) : (
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      onClick={() => setConfirmingId(acc.guardian_profile_id)}
                      title="Zugang entfernen"
                      aria-label="Zugang entfernen"
                      className="shrink-0 text-gray-400"
                    >
                      <X className="h-4 w-4" />
                    </Button>
                  ))}
              </div>
            );
          })
        )}
      </div>
    </section>
  );
}
