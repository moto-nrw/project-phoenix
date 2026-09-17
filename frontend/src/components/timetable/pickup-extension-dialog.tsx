"use client";

import { useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { CheckboxCard } from "~/components/ui/checkbox-card";
import { Modal } from "~/components/ui/modal";
import { useToast } from "~/contexts/ToastContext";
import { formatStatusDate } from "~/lib/date-helpers";
import { createLogger } from "~/lib/logger";
import {
  PickupExtensionApiError,
  resolvePickupExtension,
  type PickupExtension,
} from "~/lib/pickup-extension-api";
import { getWeekdayLabel } from "~/lib/pickup-schedule-helpers";

const logger = createLogger({ component: "PickupExtensionDialog" });

/** "am Freitag, 18.09.2026" oder "ab Donnerstag, 18.09.2026". */
export function pickupExtensionWhen(task: PickupExtension): string {
  if (task.kind === "day" && task.date) {
    return `am ${formatStatusDate(task.date)}`;
  }
  if (task.effectiveFrom) return `ab ${formatStatusDate(task.effectiveFrom)}`;
  return `ab jetzt jeden ${getWeekdayLabel(task.weekday ?? 0)}`;
}

function resolveErrorMessage(err: unknown): string {
  if (err instanceof PickupExtensionApiError) {
    if (err.code === "pickup_extension_block_gone") {
      return "Der Termin hat sich inzwischen geändert. Bitte wählen Sie noch einmal.";
    }
    if (err.code === "pickup_extension_not_found") {
      return "Das wurde schon erledigt.";
    }
  }
  return "Das hat leider nicht geklappt. Bitte versuchen Sie es noch einmal.";
}

function selectedPickupExtensionBlocks(
  task: PickupExtension,
): ReadonlySet<string> {
  return task.blocks.length === 1 && task.blocks[0]
    ? new Set([task.blocks[0].id])
    : new Set<string>();
}

/**
 * Fragt nach, in welchem Termin ein Kind die zusätzliche Zeit verbringt, wenn
 * es später abgeholt wird als bisher (#3261). Mehrere offene Fragen (etwa
 * zwei Wochentage aus einer Anfrage) kommen nacheinander. Wer schließt, lässt
 * die Frage offen; sie steht dann als Aufgabe auf der Startseite.
 */
export function PickupExtensionDialog({
  tasks,
  isOpen,
  onClose,
  onChanged,
  onStale,
}: Readonly<{
  tasks: readonly PickupExtension[];
  isOpen: boolean;
  onClose: () => void;
  /** Nach jeder gespeicherten Entscheidung, zum Neuladen der Listen. */
  onChanged?: () => void;
  /** Lädt eine Aufgabe nach einer gleichzeitig geänderten Terminliste neu. */
  onStale?: (task: PickupExtension) => void;
}>) {
  const [index, setIndex] = useState(0);
  const task = tasks[index];
  if (!isOpen || !task) return null;

  const advance = () => {
    onChanged?.();
    if (index + 1 < tasks.length) {
      setIndex(index + 1);
      return;
    }
    setIndex(0);
    onClose();
  };
  const close = () => {
    setIndex(0);
    onClose();
  };

  return (
    <PickupExtensionStep
      key={`${task.id}:${task.blocks.map((block) => block.id).join(",")}`}
      task={task}
      position={tasks.length > 1 ? `${index + 1} von ${tasks.length}` : null}
      onDone={advance}
      onClose={close}
      onStale={() => onStale?.(task)}
    />
  );
}

function PickupExtensionStep({
  task,
  position,
  onDone,
  onClose,
  onStale,
}: Readonly<{
  task: PickupExtension;
  position: string | null;
  onDone: () => void;
  onClose: () => void;
  onStale: () => void;
}>) {
  const toast = useToast();
  // Gibt es nur einen passenden Termin, ist er schon gewählt: ein Tipp genügt.
  const [selected, setSelected] = useState<ReadonlySet<string>>(() =>
    selectedPickupExtensionBlocks(task),
  );
  const [busy, setBusy] = useState<"assign" | "none" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [stale, setStale] = useState(false);

  // A stale response can retain the task and template IDs but change its
  // selectable details. Start over whenever the caller replaces the task.
  useEffect(() => {
    setSelected(selectedPickupExtensionBlocks(task));
    setBusy(null);
    setError(null);
    setStale(false);
  }, [task]);

  const toggle = (blockId: string, checked: boolean) => {
    setSelected((current) => {
      const next = new Set(current);
      if (checked) next.add(blockId);
      else next.delete(blockId);
      return next;
    });
  };

  const decide = async (blockIds: readonly string[]) => {
    setBusy(blockIds.length > 0 ? "assign" : "none");
    setError(null);
    try {
      await resolvePickupExtension(task.id, blockIds);
      const titles = task.blocks
        .filter((block) => blockIds.includes(block.id))
        .map((block) => block.title);
      toast.success(
        titles.length > 0
          ? `${task.studentName} ist jetzt bei ${titles.join(", ")} eingetragen.`
          : "Erledigt. Das Kind wurde keinem Termin zugeordnet.",
      );
      onDone();
    } catch (err) {
      logger.warn("pickup_extension_resolve_failed", {
        error: err instanceof Error ? err.message : String(err),
        task_id: task.id,
      });
      if (
        err instanceof PickupExtensionApiError &&
        err.code === "pickup_extension_block_gone"
      ) {
        setError(resolveErrorMessage(err));
        setStale(true);
        setBusy(null);
        onStale();
        return;
      }
      setError(resolveErrorMessage(err));
      setBusy(null);
    }
  };

  const firstName = task.studentName.split(" ")[0] ?? task.studentName;
  const title = position
    ? `Längere Betreuung eintragen (${position})`
    : "Längere Betreuung eintragen";

  return (
    <Modal
      isOpen
      onClose={onClose}
      title={title}
      isDismissDisabled={busy !== null}
      footer={
        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <Button
            type="button"
            variant="secondary"
            size="md"
            disabled={busy !== null || stale}
            isLoading={busy === "none"}
            loadingText="Wird gespeichert …"
            onClick={() => void decide([])}
          >
            Keinem Termin zuordnen
          </Button>
          <Button
            type="button"
            variant="primary"
            size="md"
            disabled={busy !== null || stale || selected.size === 0}
            isLoading={busy === "assign"}
            loadingText="Wird eingetragen …"
            onClick={() => void decide([...selected])}
          >
            Eintragen
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        {error && <Alert type="error" message={error} />}
        <p className="text-sm text-gray-700">
          <span className="font-semibold text-gray-900">
            {task.studentName}
          </span>{" "}
          wird {pickupExtensionWhen(task)} erst um {task.pickupTime} Uhr
          abgeholt. Bisher: {task.previousPickupTime} Uhr.
        </p>
        <fieldset className="space-y-2">
          <legend className="mb-2 text-sm font-medium text-gray-900">
            In welchem Termin ist {firstName} in dieser Zeit?
          </legend>
          {task.blocks.map((block) => (
            <CheckboxCard
              key={block.id}
              checked={selected.has(block.id)}
              disabled={busy !== null}
              onChange={(checked) => toggle(block.id, checked)}
              label={block.title}
              hint={`${block.startTime}–${block.endTime} Uhr`}
            />
          ))}
        </fieldset>
        <p className="text-xs text-gray-500">
          {task.kind === "weekday"
            ? `Gilt für alle kommenden ${getWeekdayLabel(task.weekday ?? 0)}e. `
            : ""}
          Wenn Sie jetzt schließen, bleibt die Frage als Aufgabe auf der
          Startseite.
        </p>
      </div>
    </Modal>
  );
}
