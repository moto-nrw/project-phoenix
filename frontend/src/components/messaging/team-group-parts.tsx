"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Button } from "~/components/ui/button";
import { EditActions } from "~/components/ui/edit-actions";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { Input } from "~/components/ui/input";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import { SectionCard } from "~/components/ui/section-card";
import { AddGroupMembersModal, GROUP_NAME_MAX } from "./team-group-modals";
import { useApiFormError } from "~/contexts/ToastContext";
import {
  type MessageableStaff,
  type StaffMessage,
  type StaffMessagesApi,
  type StaffThreadDetail,
  staffRoleKindLabel,
} from "~/lib/staff-messages-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TeamGroupParts" });

/**
 * Lesestatus einer eigenen Gruppennachricht (#4776): die Zahl als Knopf,
 * die Namen auf Klick, wie bei den Tagesinformationen.
 */
export function GroupReadReceipt({
  message,
  members,
}: {
  readonly message: StaffMessage;
  readonly members: readonly MessageableStaff[];
}) {
  const [open, setOpen] = useState(false);
  const total = message.reader_count ?? 0;
  if (total === 0) return null;
  const readIds = new Set(message.read_by_account_ids ?? []);
  const eligibleIds = new Set(message.eligible_reader_account_ids ?? []);
  const nameOf = new Map(members.map((m) => [m.account_id, m.name]));
  const readers = [...readIds].map(
    (id) => nameOf.get(id) ?? "Ehemaliges Mitglied",
  );
  const pending = members
    .filter(
      (m) =>
        eligibleIds.has(m.account_id) &&
        m.account_id !== message.sender_account_id &&
        !readIds.has(m.account_id),
    )
    .map((m) => m.name);
  const label =
    readIds.size === total
      ? "Von allen gelesen"
      : `${readIds.size} von ${total} gelesen`;

  return (
    <AnchoredPopover
      open={open}
      onOpenChange={setOpen}
      ariaLabel="Wer die Nachricht gelesen hat"
      align="end"
      preferredWidth={260}
      renderTrigger={({ ref, open: isOpen, panelId, toggle }) => (
        <button
          ref={ref}
          type="button"
          onClick={toggle}
          aria-expanded={isOpen}
          aria-controls={panelId}
          className="mt-1 px-1 text-xs text-gray-500 underline-offset-2 hover:underline"
        >
          {label}
        </button>
      )}
    >
      {() => (
        <div className="space-y-3 p-3 text-sm">
          <ReaderList title="Gelesen" names={readers} empty="Noch niemand" />
          {pending.length > 0 && (
            <ReaderList title="Noch nicht gelesen" names={pending} empty="" />
          )}
        </div>
      )}
    </AnchoredPopover>
  );
}

function ReaderList({
  title,
  names,
  empty,
}: {
  readonly title: string;
  readonly names: readonly string[];
  readonly empty: string;
}) {
  return (
    <div>
      <p className="text-xs font-medium text-gray-500">{title}</p>
      {names.length === 0 ? (
        <p className="text-gray-500">{empty}</p>
      ) : (
        <ul className="mt-1 space-y-0.5 text-gray-900">
          {names.map((name, index) => (
            <li key={`${name}-${index}`}>{name}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

/**
 * Mitglieder einer Gruppe (#4776): wer dabei ist, und für die, die die Gruppe
 * angelegt haben oder die Schule leiten, Name ändern, hinzufügen und
 * entfernen. Verlassen kann jedes Mitglied.
 */
export function GroupMembersPanel({
  api,
  portal,
  thread,
  myAccountId,
  onChanged,
  onLeft,
}: {
  readonly api: StaffMessagesApi;
  readonly portal: "tenant" | "school";
  readonly thread: StaffThreadDetail;
  readonly myAccountId: string;
  readonly onChanged: (thread: StaffThreadDetail) => void;
  readonly onLeft: () => void;
}) {
  const members = thread.members ?? [];
  const manage = portal === "tenant" && thread.can_manage === true;
  const [addOpen, setAddOpen] = useState(false);
  const [removing, setRemoving] = useState<MessageableStaff | null>(null);
  const [leaving, setLeaving] = useState(false);
  const [busy, setBusy] = useState(false);
  const actionError = useApiFormError();
  const removeError = useApiFormError();

  const remove = async (member: MessageableStaff) => {
    setBusy(true);
    removeError.clear();
    try {
      onChanged(
        await api.removeGroupMember(thread.thread_id, member.account_id),
      );
      setRemoving(null);
    } catch (err) {
      logger.error("team_group_member_remove_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: thread.thread_id,
      });
      void removeError.show(err, { object: "die Person" });
    } finally {
      setBusy(false);
    }
  };

  const leave = async () => {
    setBusy(true);
    actionError.clear();
    try {
      await api.leaveGroup(thread.thread_id);
      onLeft();
    } catch (err) {
      logger.error("team_group_leave_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: thread.thread_id,
      });
      setLeaving(false);
      void actionError.show(err, { object: "der Gruppenchat" });
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="space-y-4">
      <FormErrorAlert message={actionError.error} />

      {manage && (
        <GroupNameForm api={api} thread={thread} onChanged={onChanged} />
      )}

      <SectionCard
        title={`Mitglieder (${members.length})`}
        action={
          manage ? (
            <Button
              type="button"
              variant="outline"
              size="md"
              onClick={() => setAddOpen(true)}
            >
              Personen hinzufügen
            </Button>
          ) : undefined
        }
      >
        <ul className="divide-y divide-gray-100">
          {members.map((member) => {
            const me = member.account_id === myAccountId;
            const roleLabel = staffRoleKindLabel(member.role_kind, portal);
            return (
              <li
                key={member.account_id}
                className="flex items-center justify-between gap-3 py-2"
              >
                <span className="flex min-w-0 items-baseline gap-2">
                  <span className="truncate text-sm font-medium text-gray-900">
                    {member.name}
                    {me && " (Sie)"}
                  </span>
                  {roleLabel && (
                    <span className="flex-shrink-0 text-xs text-gray-500">
                      {roleLabel}
                    </span>
                  )}
                </span>
                {manage && !me && (
                  <OverflowMenu
                    ariaLabel={`Aktionen für ${member.name}`}
                    items={[
                      {
                        label: "Aus dem Gruppenchat entfernen",
                        destructive: true,
                        onClick: () => setRemoving(member),
                      },
                    ]}
                  />
                )}
              </li>
            );
          })}
        </ul>
        {!manage && portal === "tenant" && (
          <p className="mt-3 text-sm text-gray-500">
            Nur Mitglieder dürfen den Namen und die Mitglieder ändern. Das
            dürfen die Person, die den Gruppenchat angelegt hat, und die
            OGS-Leitung.
          </p>
        )}
      </SectionCard>

      <div>
        <Button
          type="button"
          variant="outline_danger"
          size="md"
          onClick={() => setLeaving(true)}
        >
          Gruppenchat verlassen
        </Button>
        <p className="mt-2 text-sm text-gray-500">
          Danach sehen Sie den Gruppenchat und seine Nachrichten nicht mehr.
        </p>
      </div>

      {addOpen && (
        <AddGroupMembersModal
          api={api}
          thread={thread}
          onClose={() => setAddOpen(false)}
          onAdded={(next) => {
            setAddOpen(false);
            onChanged(next);
          }}
        />
      )}

      <ConfirmDeleteModal
        isOpen={removing !== null}
        onClose={() => {
          removeError.clear();
          setRemoving(null);
        }}
        onConfirm={() => (removing ? remove(removing) : undefined)}
        error={removeError.error}
        title="Aus dem Gruppenchat entfernen?"
        description={`${removing?.name ?? ""} sieht den Gruppenchat und seine Nachrichten danach nicht mehr.`}
        gate={{ mode: "twoStep", firstStepLabel: "Entfernen" }}
        confirmLabel="Ja, entfernen"
        loadingLabel="Wird entfernt..."
        loading={busy}
      />

      <ConfirmationModal
        isOpen={leaving}
        onClose={() => setLeaving(false)}
        onConfirm={() => void leave()}
        title="Gruppenchat verlassen?"
        confirmText="Verlassen"
        confirmVariant="danger"
        isConfirmLoading={busy}
      >
        <p className="text-sm text-gray-700">
          Sie sehen „{thread.group_name}“ und seine Nachrichten danach nicht
          mehr. Wer den Gruppenchat verwaltet, kann Sie wieder hinzufügen.
        </p>
      </ConfirmationModal>
    </div>
  );
}

/** Name der Gruppe ändern, mit Speichern und Abbrechen. */
function GroupNameForm({
  api,
  thread,
  onChanged,
}: {
  readonly api: StaffMessagesApi;
  readonly thread: StaffThreadDetail;
  readonly onChanged: (thread: StaffThreadDetail) => void;
}) {
  const current = thread.group_name ?? "";
  const [name, setName] = useState(current);
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  // Ein Name, den jemand anderes inzwischen geändert hat, kommt nach.
  useLayoutEffect(() => setName(current), [current]);
  const dirty = name.trim() !== current;

  const save = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      formError.invalid("Bitte geben Sie dem Gruppenchat einen Namen.", {
        "group-name": "Bitte geben Sie einen Namen ein.",
      });
      return;
    }
    setSaving(true);
    formError.clear();
    try {
      onChanged(await api.renameGroup(thread.thread_id, trimmed));
    } catch (err) {
      logger.error("team_group_rename_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: thread.thread_id,
      });
      void formError.show(err, { object: "der Name" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <SectionCard title="Name des Gruppenchats">
      <form
        ref={formRef}
        noValidate
        className="space-y-3"
        onSubmit={(event) => {
          event.preventDefault();
          void save();
        }}
      >
        <FormErrorAlert message={formError.error} />
        <Input
          name="group-name"
          aria-label="Name des Gruppenchats"
          value={name}
          maxLength={GROUP_NAME_MAX}
          onChange={(e) => setName(e.target.value)}
          error={formError.fieldError("group-name")}
        />
        {dirty && (
          <EditActions
            onCancel={() => setName(current)}
            saving={saving}
            saveType="submit"
            saveLabel="Name speichern"
          />
        )}
      </form>
    </SectionCard>
  );
}
