"use client";

import { messageKeys } from "~/lib/swr/portal-keys";

import { useMemo, useState } from "react";
import { useSWRAuth } from "~/lib/swr";
import { Button } from "~/components/ui/button";
import { CustomSelect } from "~/components/ui/custom-select";
import { FormModal } from "~/components/ui/form-modal";
import { Input } from "~/components/ui/input";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { StatusBadge } from "~/components/ui/status-badge";
import { Textarea } from "~/components/ui/textarea";
import { useApiFormError } from "~/contexts/ToastContext";
import { groupService, studentService } from "~/lib/api";
import { fetchActivities } from "~/lib/activity-api";
import { useSwrLoadError } from "~/lib/hooks/use-swr-load-error";
import { createLogger } from "~/lib/logger";
import {
  type GroupChild,
  type GroupTargetType,
  type GroupThreadDetail,
  createGroupThread,
  fetchGroupAudience,
} from "~/lib/parent-group-threads-api";

const logger = createLogger({ component: "NewGroupMessageModal" });

const NO_PARENT_ACCESS = "Kein Eltern-Zugang";
const MAX_TITLE = 120;
const MAX_BODY = 2000;

const TARGET_ITEMS = [
  { value: "group", label: "Gruppe" },
  { value: "class", label: "Klasse" },
  { value: "activity_group", label: "AG" },
] as const;

const TARGET_NOUN: Record<GroupTargetType, string> = {
  group: "Gruppe",
  class: "Klasse",
  activity_group: "AG",
};

interface Option {
  readonly value: string;
  readonly label: string;
}

/** The choices for one target kind, loaded only while the kind is picked. */
function useTargetOptions(type: GroupTargetType) {
  const { data, error, mutate } = useSWRAuth<Option[]>(
    messageKeys.groupMessageTargets(type),
    async () => {
      if (type === "group") {
        const groups = await groupService.getGroups();
        return groups.map((g) => ({ value: g.id, label: g.name }));
      }
      if (type === "activity_group") {
        const activities = await fetchActivities();
        return activities.map((a) => ({ value: a.id, label: a.name }));
      }
      const classes = await studentService.getSchoolClasses();
      return classes.map((c) => ({ value: c, label: c }));
    },
    { revalidateOnFocus: false },
  );
  const loadError = useSwrLoadError(
    error,
    type === "group"
      ? "die Liste der Gruppen"
      : type === "class"
        ? "die Liste der Klassen"
        : "die Liste der AGs",
    () => void mutate(),
  );
  const options = useMemo(
    () =>
      [...(data ?? [])].sort((a, b) => a.label.localeCompare(b.label, "de")),
    [data],
  );
  return { options, loading: !data && !error, loadError };
}

/** The children a target reaches today, with who cannot be reached. */
function AudiencePreview({
  type,
  id,
}: Readonly<{ type: GroupTargetType; id: string }>) {
  const { data, error, mutate } = useSWRAuth<GroupChild[]>(
    messageKeys.groupMessageAudience(type, id),
    () => fetchGroupAudience(type, id),
    { revalidateOnFocus: false },
  );
  const loadError = useSwrLoadError(
    error,
    "die Empfänger",
    () => void mutate(),
  );
  if (error) return <LoadErrorAlert error={loadError} />;
  if (!data) {
    return <p className="text-sm text-gray-500">Empfänger werden geladen…</p>;
  }
  const unreachable = data.filter((child) => !child.reachable);
  const reachable = data.length - unreachable.length;
  return (
    <div className="space-y-2" data-testid="group-audience">
      <p className="text-sm text-gray-700">
        {reachable === 1
          ? "1 Kind hat Eltern mit Zugang zu moto."
          : `${reachable} Kinder haben Eltern mit Zugang zu moto.`}
      </p>
      {unreachable.length > 0 && (
        <div className="space-y-1">
          <p className="text-sm text-gray-600">
            Diese Kinder erreicht die Nachricht nicht:
          </p>
          <ul className="space-y-1">
            {unreachable.map((child) => (
              <li
                key={child.student_id}
                className="flex items-center justify-between gap-2 text-sm text-gray-700"
              >
                <span className="truncate">
                  {child.first_name} {child.last_name}
                  {child.school_class ? ` · ${child.school_class}` : ""}
                </span>
                <StatusBadge tone="gray" label={NO_PARENT_ACCESS} compact />
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
}

/**
 * „Neue Gruppennachricht“ (#4784): one message to every family of a group,
 * class or AG. Families answer only the OGS, never each other.
 */
export function NewGroupMessageModal({
  onClose,
  onCreated,
}: Readonly<{
  onClose: () => void;
  onCreated: (detail: GroupThreadDetail) => void;
}>) {
  const [type, setType] = useState<GroupTargetType>("group");
  const [targetId, setTargetId] = useState("");
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [sending, setSending] = useState(false);
  const formError = useApiFormError();
  const { options, loading, loadError } = useTargetOptions(type);
  const selected = options.find((option) => option.value === targetId);

  const changeType = (next: GroupTargetType) => {
    setType(next);
    setTargetId("");
  };

  const submit = async () => {
    if (sending) return;
    const fields: Record<string, string> = {};
    if (!selected)
      fields.target = `Bitte wählen Sie eine ${TARGET_NOUN[type]}.`;
    if (!title.trim()) fields.title = "Bitte geben Sie einen Titel ein.";
    if (!body.trim()) fields.body = "Bitte schreiben Sie eine Nachricht.";
    if (Object.keys(fields).length > 0 || !selected) {
      formError.invalid("Bitte füllen Sie alle Felder aus.", fields);
      return;
    }
    formError.clear();
    setSending(true);
    try {
      const detail = await createGroupThread({
        target: {
          type,
          id: selected.value,
          label: `${TARGET_NOUN[type]} ${selected.label}`,
        },
        title: title.trim(),
        body: body.trim(),
      });
      onCreated(detail);
    } catch (err) {
      logger.error("group_message_create_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
      void formError.show(err, { object: "die Gruppennachricht" });
    } finally {
      setSending(false);
    }
  };

  return (
    <FormModal
      isOpen
      onClose={onClose}
      title="Neue Gruppennachricht"
      closeDisabled={sending}
      isBackdropDismissDisabled
      error={formError.error}
      footer={
        <div className="flex justify-end gap-3">
          <Button
            type="button"
            variant="outline"
            size="md"
            onClick={onClose}
            disabled={sending}
          >
            Abbrechen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            onClick={() => void submit()}
            disabled={sending}
          >
            {sending ? "Wird gesendet…" : "Senden"}
          </Button>
        </div>
      }
    >
      <div className="space-y-5">
        <p className="text-sm text-gray-600">
          Alle Eltern mit Zugang zu moto bekommen die Nachricht. Eltern
          antworten nur Ihnen. Andere Eltern sehen die Antworten nicht.
        </p>
        <div className="space-y-2">
          <p
            id="group-message-target-label"
            className="text-sm font-medium text-gray-700"
          >
            An
          </p>
          <SegmentedControl<GroupTargetType>
            items={TARGET_ITEMS}
            value={type}
            onChange={changeType}
            variant="joined"
            ariaLabel="Empfänger"
          />
          {loadError ? (
            <LoadErrorAlert error={loadError} />
          ) : (
            <CustomSelect
              name="target"
              value={targetId}
              onChange={setTargetId}
              options={options}
              ariaLabelledBy="group-message-target-label"
              placeholder={
                loading ? "Wird geladen…" : `${TARGET_NOUN[type]} wählen`
              }
              disabled={loading}
              invalid={Boolean(formError.fieldError("target"))}
              testId="group-message-target"
            />
          )}
        </div>
        {selected && <AudiencePreview type={type} id={selected.value} />}
        <Input
          name="title"
          label="Titel"
          value={title}
          maxLength={MAX_TITLE}
          onChange={(event) => setTitle(event.target.value)}
          error={formError.fieldError("title")}
        />
        <Textarea
          name="body"
          label="Nachricht"
          rows={5}
          value={body}
          maxLength={MAX_BODY}
          onChange={(event) => setBody(event.target.value)}
          error={formError.fieldError("body")}
        />
      </div>
    </FormModal>
  );
}
