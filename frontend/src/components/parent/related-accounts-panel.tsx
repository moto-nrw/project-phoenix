"use client";

import { useCallback, useEffect, useState } from "react";
import { UserPlus, Users, X, Check } from "lucide-react";
import {
  listRelatedAccounts,
  inviteRelatedAccount,
  removeRelatedAccount,
  type RelatedAccount,
} from "~/lib/parent-api";
import { createLogger } from "~/lib/logger";
import { Button } from "~/components/ui/button";
import { Input } from "~/components/ui/input";
import { Alert } from "~/components/ui/alert";
import { SectionCard } from "~/components/ui/section-card";

const logger = createLogger({ component: "RelatedAccountsPanel" });

const STATUS_META: Record<RelatedAccount["status"], { label: string }> = {
  active: { label: "Konto aktiv" },
  pending: { label: "Einladung offen" },
  no_account: { label: "Kontakt ohne Konto" },
};

function initials(first: string, last: string): string {
  return `${first.charAt(0)}${last.charAt(0)}`.toUpperCase() || "?";
}

interface RelatedAccountsPanelProps {
  readonly studentId: string;
  readonly canInvite: boolean;
  readonly canRemove: boolean;
}

export default function RelatedAccountsPanel({
  studentId,
  canInvite,
  canRemove,
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
    <SectionCard
      icon={Users}
      title="Verbundene Konten"
      description="Wer Zugriff auf dieses Kind in der Eltern-App hat."
      bodyClassName="mt-4 space-y-3"
      actions={
        canInvite ? (
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={() => {
              setInviteOpen((v) => !v);
              setMessage(null);
            }}
          >
            <UserPlus className="mr-2 h-4 w-4" aria-hidden="true" />
            Einladen
          </Button>
        ) : undefined
      }
    >
      {message && (
        <Alert
          type={message.kind === "success" ? "success" : "error"}
          message={message.text}
        />
      )}

      {inviteOpen && canInvite && (
        <div className="flex flex-col gap-2 rounded-xl bg-gray-50 p-3 sm:flex-row">
          <Input
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="E-Mail-Adresse"
            aria-label="E-Mail-Adresse der einzuladenden Person"
            className="flex-1"
          />
          <Button
            type="button"
            size="md"
            className="shrink-0"
            onClick={() => void handleInvite()}
            disabled={busy || !email.trim()}
          >
            <Check className="mr-1.5 h-4 w-4" aria-hidden="true" />
            Senden
          </Button>
        </div>
      )}

      <div className="space-y-2">
        {isLoading ? (
          <p className="text-sm text-gray-500">Wird geladen…</p>
        ) : accounts.length === 0 ? (
          <p className="text-sm text-gray-500">
            Noch keine verbundenen Konten.
          </p>
        ) : (
          accounts.map((acc) => {
            const meta = STATUS_META[acc.status];
            const name =
              `${acc.first_name} ${acc.last_name}`.trim() || acc.email;
            return (
              <div
                key={acc.guardian_profile_id}
                className="flex items-center gap-3 rounded-xl bg-gray-50 p-3"
              >
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-white text-sm font-semibold text-gray-700 ring-1 ring-gray-200">
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
                  {/* Status as plain text, not a colored dot: three accounts
                      each carrying a green/amber/gray dot read as an alert
                      panel rather than a list of people. */}
                  <p className="truncate text-xs text-gray-600">
                    {meta.label}
                    {acc.email && (
                      <span className="text-gray-400"> · {acc.email}</span>
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
    </SectionCard>
  );
}
