"use client";

// The recipient of the finance exports of a Träger (#6824, ADR 0217): the
// form of the SFTP access, saved with a password that never comes back,
// and the newest handovers with their outcome.

import { KeyRound, Send } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";
import {
  destinationText,
  HANDOVER_FORMAT_LABELS,
  HANDOVER_STATE_LABELS,
  HANDOVER_STATE_TONES,
  HANDOVER_UNKNOWN_TEXT,
  handoverReasonText,
} from "~/components/traeger/export-handover-format";
import { Button } from "~/components/ui/button";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { DataTable, type DataTableColumn } from "~/components/ui/data-table";
import { EmptyState } from "~/components/ui/empty-state";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { SectionCard } from "~/components/ui/section-card";
import { StatusBadge } from "~/components/ui/status-badge";
import { SwitchRow } from "~/components/ui/switch";
import { useApiFormError, useToast } from "~/contexts/ToastContext";
import { formatBerlinDate, formatBerlinTime } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import type {
  ExportHandover,
  ExportRecipient,
  ExportRecipientDraft,
} from "~/lib/traeger-export-handovers-api";

const logger = createLogger({ component: "ExportRecipientCards" });

interface Draft {
  readonly enabled: boolean;
  readonly host: string;
  readonly port: string;
  readonly username: string;
  readonly password: string;
  readonly remoteDirectory: string;
  readonly hostKeyFingerprint: string;
}

function draftOf(recipient: ExportRecipient): Draft {
  return {
    enabled: recipient.configured ? recipient.enabled : true,
    host: recipient.host,
    port: String(recipient.port || 22),
    username: recipient.username,
    password: "",
    remoteDirectory: recipient.remoteDirectory,
    hostKeyFingerprint: recipient.hostKeyFingerprint,
  };
}

/**
 * A stored password goes only to the destination it was entered for: a
 * change of address, port, user or fingerprint needs it again.
 */
function destinationChanged(recipient: ExportRecipient, draft: Draft) {
  return (
    draft.host.trim() !== recipient.host ||
    Number(draft.port) !== recipient.port ||
    draft.username.trim() !== recipient.username ||
    draft.hostKeyFingerprint.trim() !== recipient.hostKeyFingerprint
  );
}

const PASSWORD_AGAIN =
  "Bitte geben Sie das Passwort noch einmal ein. moto schickt es nur an die Adresse, für die Sie es eingegeben haben.";

/** The form of the SFTP access of the recipient. */
export function ExportRecipientFormCard({
  recipient,
  onSave,
  onDelete,
}: Readonly<{
  recipient: ExportRecipient;
  onSave: (draft: ExportRecipientDraft) => Promise<void>;
  onDelete: () => Promise<void>;
}>) {
  const [draft, setDraft] = useState<Draft>(() => draftOf(recipient));
  const [saving, setSaving] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formErrors = useApiFormError(formRef);
  const deleteErrors = useApiFormError();
  const { success } = useToast();

  useEffect(() => setDraft(draftOf(recipient)), [recipient]);

  const set = (patch: Partial<Draft>) =>
    setDraft((current) => ({ ...current, ...patch }));
  const changed = useMemo(
    () => JSON.stringify(draft) !== JSON.stringify(draftOf(recipient)),
    [draft, recipient],
  );

  const submit = async (event: { preventDefault: () => void }) => {
    event.preventDefault();
    formErrors.clear();
    const port = Number(draft.port);
    const fields: Record<string, string> = {};
    if (!Number.isInteger(port) || port < 1 || port > 65535)
      fields.port = "Bitte geben Sie eine Zahl zwischen 1 und 65535 ein.";
    if (
      recipient.passwordSet &&
      !draft.password &&
      destinationChanged(recipient, draft)
    )
      fields.password = PASSWORD_AGAIN;
    if (Object.keys(fields).length > 0) {
      formErrors.invalid("Bitte prüfen Sie die markierten Angaben.", fields);
      return;
    }
    setSaving(true);
    try {
      await onSave({
        enabled: draft.enabled,
        host: draft.host.trim(),
        port,
        username: draft.username.trim(),
        password: draft.password || undefined,
        remoteDirectory: draft.remoteDirectory.trim(),
        hostKeyFingerprint: draft.hostKeyFingerprint.trim(),
      });
      success("Der Empfänger ist gespeichert.");
    } catch (err) {
      logger.error("traeger_export_recipient_save_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formErrors.show(err, { object: "der Empfänger" });
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    setDeleting(true);
    deleteErrors.clear();
    try {
      await onDelete();
      setConfirmDelete(false);
      success("Der Empfänger ist entfernt. Das Passwort ist gelöscht.");
    } catch (err) {
      logger.error("traeger_export_recipient_delete_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void deleteErrors.show(err, { object: "der Empfänger" });
    } finally {
      setDeleting(false);
    }
  };

  const passwordHint = recipient.passwordSet
    ? "Ist gespeichert und wird nicht angezeigt. Nur ausfüllen, um es zu ändern."
    : "moto speichert es verschlüsselt und zeigt es nie wieder an.";

  return (
    <SectionCard
      title="Zugang zum Empfänger"
      description="Diese Angaben bekommen Sie von der Stelle, die die Dateien annimmt, zum Beispiel von Ihrer Kämmerei."
      icon={KeyRound}
      footer={
        <div className="flex flex-wrap items-center justify-between gap-3">
          {recipient.configured ? (
            <Button
              type="button"
              variant="outline_danger"
              size="md"
              tooltip="Löscht Adresse und Passwort. moto fragt vorher nach."
              onClick={() => setConfirmDelete(true)}
            >
              Empfänger entfernen
            </Button>
          ) : (
            <span />
          )}
          <Button
            type="submit"
            form="export-recipient-form"
            variant="primary"
            size="md"
            isLoading={saving}
            loadingText="Speichert…"
            disabled={!changed && recipient.configured}
            disabledReason="Sie haben noch nichts geändert."
            noTooltip="Ein reines Speichern ohne Folgen."
          >
            Speichern
          </Button>
        </div>
      }
    >
      <form
        ref={formRef}
        id="export-recipient-form"
        noValidate
        className="space-y-5"
        onSubmit={(event) => void submit(event)}
      >
        <FormErrorAlert message={formErrors.error} />
        <SwitchRow
          label="Übergabe einschalten"
          description="Bei den Exporten erscheint dann neben dem Herunterladen auch „Übergeben“."
          checked={draft.enabled}
          onCheckedChange={(enabled) => set({ enabled })}
        />
        <div className="grid gap-4 sm:grid-cols-[minmax(0,1fr)_8rem]">
          <Input
            name="host"
            label="Adresse"
            placeholder="dateien.kaemmerei-beispiel.de"
            autoComplete="off"
            spellCheck={false}
            value={draft.host}
            error={formErrors.fieldError("host") ?? undefined}
            onChange={(event) => set({ host: event.target.value })}
          />
          <Input
            name="port"
            label="Port"
            inputMode="numeric"
            hint="Meist 22"
            value={draft.port}
            error={formErrors.fieldError("port") ?? undefined}
            onChange={(event) => set({ port: event.target.value })}
          />
        </div>
        <div className="grid gap-4 sm:grid-cols-2">
          <Input
            name="username"
            label="Benutzername"
            autoComplete="off"
            spellCheck={false}
            value={draft.username}
            error={formErrors.fieldError("username") ?? undefined}
            onChange={(event) => set({ username: event.target.value })}
          />
          <Input
            name="password"
            label="Passwort"
            type="password"
            revealable
            autoComplete="new-password"
            placeholder={recipient.passwordSet ? "••••••••" : undefined}
            hint={passwordHint}
            value={draft.password}
            error={formErrors.fieldError("password") ?? undefined}
            onChange={(event) => set({ password: event.target.value })}
          />
        </div>
        <Input
          name="remote_directory"
          label="Zielordner"
          placeholder="/eingang/moto"
          hint="Der Ordner muss beim Empfänger schon vorhanden sein."
          spellCheck={false}
          value={draft.remoteDirectory}
          error={formErrors.fieldError("remote_directory") ?? undefined}
          onChange={(event) => set({ remoteDirectory: event.target.value })}
        />
        <Input
          name="host_key_fingerprint"
          label="Fingerabdruck"
          placeholder="SHA256:…"
          hint="Beginnt mit SHA256:. Fragen Sie beim Empfänger nach dem Fingerabdruck für den RSA-Schlüssel. Passt er nicht, übergibt moto nichts."
          spellCheck={false}
          className="font-mono"
          value={draft.hostKeyFingerprint}
          error={formErrors.fieldError("host_key_fingerprint") ?? undefined}
          onChange={(event) => set({ hostKeyFingerprint: event.target.value })}
        />
      </form>
      <ConfirmDeleteModal
        isOpen={confirmDelete}
        title="Empfänger entfernen?"
        description="moto löscht Adresse, Benutzername und Passwort. Bis Sie einen neuen Empfänger eintragen, können Sie die Dateien nur herunterladen. Der Verlauf der Übergaben bleibt."
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Ja, entfernen"
        loading={deleting}
        error={deleteErrors.error}
        onConfirm={remove}
        onClose={() => setConfirmDelete(false)}
      />
    </SectionCard>
  );
}

function handoverColumns(): DataTableColumn<ExportHandover>[] {
  return [
    {
      key: "when",
      header: "Zeitpunkt",
      sortValue: (handover) => handover.createdAt,
      render: (handover) => (
        <span className="whitespace-nowrap tabular-nums">
          {formatBerlinDate(handover.createdAt)},{" "}
          {formatBerlinTime(handover.createdAt)} Uhr
        </span>
      ),
      stacked: "meta",
    },
    {
      key: "file",
      header: "Datei",
      render: (handover) => (
        <span className="min-w-0">
          <span className="block font-medium break-all text-gray-900">
            {handover.fileName}
          </span>
          <span className="block text-sm text-gray-500">
            {HANDOVER_FORMAT_LABELS[handover.format] ?? handover.format}
          </span>
        </span>
      ),
      stacked: "title",
    },
    {
      key: "target",
      header: "Ziel",
      render: (handover) => (
        <span className="text-sm break-all text-gray-600">
          {destinationText(handover.targetHost, handover.targetDirectory)}
        </span>
      ),
    },
    {
      key: "state",
      header: "Ergebnis",
      render: (handover) => (
        <span className="flex min-w-0 flex-col items-start gap-1">
          <StatusBadge
            label={HANDOVER_STATE_LABELS[handover.state]}
            tone={HANDOVER_STATE_TONES[handover.state]}
          />
          {handover.state === "failed" ? (
            <span className="text-sm text-gray-600">
              {handoverReasonText(handover.reason)}
            </span>
          ) : null}
          {handover.state === "unknown" ? (
            <span className="text-sm text-gray-600">
              {HANDOVER_UNKNOWN_TEXT}
            </span>
          ) : null}
        </span>
      ),
    },
  ];
}

/** The newest handovers of the Träger, every attempt on its own row. */
export function ExportHandoverHistoryCard({
  handovers,
}: Readonly<{ handovers: readonly ExportHandover[] }>) {
  return (
    <SectionCard
      title="Letzte Übergaben"
      description="Jeder Versuch steht hier, auch wenn die Datei nicht angekommen ist. moto versucht nichts von selbst noch einmal."
      icon={Send}
      flush
    >
      <DataTable
        columns={handoverColumns()}
        rows={[...handovers]}
        getRowKey={(handover) => handover.id}
        stackedOnMobile
        emptyState={
          <EmptyState
            title="Noch keine Übergabe."
            description="Übergeben Sie eine Datei bei den Exporten. Sie steht dann hier."
          />
        }
      />
    </SectionCard>
  );
}
