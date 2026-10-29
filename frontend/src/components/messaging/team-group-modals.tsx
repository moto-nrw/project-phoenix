"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useSession } from "next-auth/react";
import { Checkbox } from "~/components/ui/checkbox";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EditActions } from "~/components/ui/edit-actions";
import { formErrorMessage } from "~/components/ui/form-error";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import {
  type MessageableStaff,
  type StaffMessagesApi,
  type StaffThreadDetail,
  staffRoleKindLabel,
} from "~/lib/staff-messages-api";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "TeamGroupModals" });

/** Längster Gruppenname; das Backend prüft dieselbe Grenze. */
export const GROUP_NAME_MAX = 80;

/**
 * Personen der Schule zum Ankreuzen (#4776), mit Suche. Lädt die Liste
 * selbst; `exclude` blendet aus, wer schon in der Gruppe ist.
 */
function PeoplePicker({
  api,
  exclude,
  selected,
  onToggle,
}: {
  readonly api: StaffMessagesApi;
  readonly exclude: ReadonlySet<string>;
  readonly selected: ReadonlySet<string>;
  readonly onToggle: (accountId: string) => void;
}) {
  const [query, setQuery] = useState("");
  const [people, setPeople] = useState<MessageableStaff[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [loadFailed, setLoadFailed] = useState(false);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const loadError = useApiLoadError();
  const showLoadError = loadError.show;
  const clearLoadError = loadError.clear;

  useEffect(() => {
    let cancelled = false;
    setIsLoading(true);
    setLoadFailed(false);
    clearLoadError();
    api
      .fetchRecipients()
      .then((rows) => {
        if (!cancelled) setPeople(rows);
      })
      .catch((err: unknown) => {
        logger.error("team_group_people_load_failed", {
          error: err instanceof Error ? err.message : String(err),
        });
        if (!cancelled) {
          setLoadFailed(true);
          void showLoadError(err, {
            object: "die Liste der Personen",
            retry: () => setLoadAttempt((attempt) => attempt + 1),
          });
        }
      })
      .finally(() => {
        if (!cancelled) setIsLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [api, loadAttempt, showLoadError, clearLoadError]);

  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase();
    return people.filter(
      (person) =>
        !exclude.has(person.account_id) &&
        (!term || person.name.toLowerCase().includes(term)),
    );
  }, [people, query, exclude]);

  const loading =
    isLoading || (loadFailed && formErrorMessage(loadError.error) === null);

  return (
    <div className="space-y-2">
      <Input
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        placeholder="Person suchen..."
        aria-label="Person suchen"
      />
      <LoadErrorAlert error={loadError.error} />
      <div className="max-h-64 space-y-1 overflow-y-auto">
        {loading && (
          <p className="py-3 text-center text-sm text-gray-500">
            Wird geladen...
          </p>
        )}
        {!loading && !loadFailed && filtered.length === 0 && (
          <p className="py-3 text-center text-sm text-gray-500">
            {query
              ? "Niemand gefunden."
              : "Es gibt niemanden mehr zum Hinzufügen."}
          </p>
        )}
        {!loading &&
          filtered.map((person) => {
            const roleLabel = staffRoleKindLabel(person.role_kind, "tenant");
            const checked = selected.has(person.account_id);
            return (
              <ChoiceTile
                key={person.account_id}
                selected={checked}
                className="flex items-center gap-3 px-3 py-2"
              >
                <Checkbox
                  checked={checked}
                  onChange={() => onToggle(person.account_id)}
                />
                <span className="min-w-0 truncate text-sm font-medium text-gray-900">
                  {person.name}
                </span>
                {roleLabel && (
                  <span className="flex-shrink-0 text-xs text-gray-500">
                    {roleLabel}
                  </span>
                )}
              </ChoiceTile>
            );
          })}
      </div>
    </div>
  );
}

function useSelection() {
  const [selected, setSelected] = useState<ReadonlySet<string>>(new Set());
  const toggle = (accountId: string) =>
    setSelected((current) => {
      const next = new Set(current);
      if (next.has(accountId)) next.delete(accountId);
      else next.add(accountId);
      return next;
    });
  return { selected, toggle };
}

/**
 * Neue Gruppe im Team-Chat (#4776): Name und Personen der Schule, auch
 * Lehrkräfte. Wer anlegt, ist automatisch dabei.
 */
export function NewTeamGroupModal({
  api,
  onClose,
  onCreated,
}: {
  readonly api: StaffMessagesApi;
  readonly onClose: () => void;
  readonly onCreated: (threadId: string) => void;
}) {
  const { data: session } = useSession();
  const [name, setName] = useState("");
  const { selected, toggle } = useSelection();
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  const creator = useMemo(
    () => new Set(session?.user.id ? [session.user.id] : []),
    [session?.user.id],
  );

  const submit = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      formError.invalid("Bitte geben Sie dem Gruppenchat einen Namen.", {
        "group-name": "Bitte geben Sie einen Namen ein.",
      });
      return;
    }
    if (selected.size === 0) {
      formError.invalid("Bitte wählen Sie mindestens eine Person aus.");
      return;
    }
    setSaving(true);
    formError.clear();
    try {
      const group = await api.createGroup(trimmed, [...selected]);
      onCreated(group.thread_id);
    } catch (err) {
      logger.error("team_group_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formError.show(err, { object: "der Gruppenchat" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Neuer Gruppenchat"
      size="md"
      error={formError.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Gruppenchat anlegen"
          savingLabel="Wird angelegt..."
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <Input
          name="group-name"
          label="Name des Gruppenchats"
          value={name}
          maxLength={GROUP_NAME_MAX}
          onChange={(e) => setName(e.target.value)}
          placeholder="z. B. Frühbetreuung"
          error={formError.fieldError("group-name")}
          autoFocus
        />
        <div className="space-y-1">
          <p className="text-sm font-medium text-gray-700">
            Mitglieder{selected.size > 0 ? ` (${selected.size} gewählt)` : ""}
          </p>
          <p className="text-sm text-gray-500">
            Sie sind automatisch dabei. Lehrkräfte lesen in „moto schule“ mit.
          </p>
        </div>
        <PeoplePicker
          api={api}
          exclude={creator}
          selected={selected}
          onToggle={toggle}
        />
      </form>
    </FormModal>
  );
}

/** Personen zu einer bestehenden Gruppe hinzufügen (#4776). */
export function AddGroupMembersModal({
  api,
  thread,
  onClose,
  onAdded,
}: {
  readonly api: StaffMessagesApi;
  readonly thread: StaffThreadDetail;
  readonly onClose: () => void;
  readonly onAdded: (thread: StaffThreadDetail) => void;
}) {
  const { selected, toggle } = useSelection();
  const [saving, setSaving] = useState(false);
  const formRef = useRef<HTMLFormElement>(null);
  const formError = useApiFormError(formRef);
  const members = useMemo(
    () => new Set((thread.members ?? []).map((member) => member.account_id)),
    [thread.members],
  );

  const submit = async () => {
    if (selected.size === 0) {
      formError.invalid("Bitte wählen Sie mindestens eine Person aus.");
      return;
    }
    setSaving(true);
    formError.clear();
    try {
      onAdded(await api.addGroupMembers(thread.thread_id, [...selected]));
    } catch (err) {
      logger.error("team_group_members_add_failed", {
        error: err instanceof Error ? err.message : String(err),
        thread_id: thread.thread_id,
      });
      void formError.show(err, { object: "die Personen" });
    } finally {
      setSaving(false);
    }
  };

  return (
    <FormModal
      isOpen
      onClose={onClose}
      closeDisabled={saving}
      isBackdropDismissDisabled
      title="Personen hinzufügen"
      size="md"
      error={formError.error}
      footer={
        <EditActions
          onCancel={onClose}
          onSave={() => formRef.current?.requestSubmit()}
          saving={saving}
          saveLabel="Hinzufügen"
          savingLabel="Wird hinzugefügt..."
        />
      }
    >
      <form
        ref={formRef}
        noValidate
        className="space-y-3"
        onSubmit={(event) => {
          event.preventDefault();
          void submit();
        }}
      >
        <p className="text-sm text-gray-600">
          Neue Mitglieder sehen die Nachrichten ab jetzt, nicht die älteren.
        </p>
        <PeoplePicker
          api={api}
          exclude={members}
          selected={selected}
          onToggle={toggle}
        />
      </form>
    </FormModal>
  );
}
