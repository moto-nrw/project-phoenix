"use client";

import { useLayoutEffect, useRef, useState } from "react";
import { BellRing, Pencil, Send, Trash2, Undo2 } from "lucide-react";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { FormErrorAlert } from "~/components/ui/form-error-alert";
import { ConfirmationModal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
import { useApiFormError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  deleteAnnouncement,
  publishAnnouncement,
  unpublishAnnouncement,
} from "~/lib/parent-announcements-api";
import type { Announcement } from "~/lib/parent-announcements-api";
import { summarizeTargets } from "./announcement-meta";

const logger = createLogger({ component: "AnnouncementLifecycle" });

/**
 * Die Lebenszyklus-Aktionen einer Mitteilung, geteilt zwischen der Liste
 * und der Objektseite (#3115): dieselbe Rückfrage, derselbe Wortlaut, egal
 * wo die Aktion angestoßen wird.
 */
interface LifecycleDialogProps {
  readonly announcement: Announcement;
  readonly onClose: () => void;
  /** Läuft nach dem erfolgreichen Aufruf, vor dem Schließen (Listen neu laden). */
  readonly onDone: () => Promise<void> | void;
}

/**
 * Ein Lebenszyklus-Schritt mit Rückfrage (#2517): der Fehler bleibt im
 * offenen Dialog (Katalogtext, Wiederholen). Den Erfolg zeigt die Liste
 * selbst, die danach neu lädt.
 */
function useLifecycleAction(
  run: () => Promise<unknown>,
  {
    onDone,
    onClose,
    object,
    logEvent,
  }: {
    onDone: LifecycleDialogProps["onDone"];
    onClose: () => void;
    object: string;
    logEvent: string;
  },
) {
  const [pending, setPending] = useState(false);
  const errors = useApiFormError();
  const latestConfirmRef = useRef<() => void>(() => undefined);

  const confirm = async () => {
    setPending(true);
    errors.clear();
    try {
      await run();
    } catch (err) {
      logger.error(logEvent, {
        error: err instanceof Error ? err.message : String(err),
      });
      await errors.show(err, {
        object,
        retry: () => latestConfirmRef.current(),
      });
      setPending(false);
      return;
    }
    try {
      await onDone();
    } finally {
      setPending(false);
      onClose();
    }
  };
  // „Wiederholen“ ruft die Aktion mit dem dann aktuellen Stand auf.
  useLayoutEffect(() => {
    latestConfirmRef.current = () => void confirm();
  });

  return { pending, error: errors.error, confirm };
}

export function PublishAnnouncementDialog({
  announcement,
  onClose,
  onDone,
}: LifecycleDialogProps) {
  const { pending, error, confirm } = useLifecycleAction(
    () => publishAnnouncement(announcement.id),
    {
      onDone,
      onClose,
      object: "das Veröffentlichen der Mitteilung",
      logEvent: "announcement_publish_failed",
    },
  );

  return (
    <ConfirmationModal
      isOpen
      onClose={onClose}
      onConfirm={() => void confirm()}
      title="Elternmitteilung veröffentlichen"
      confirmText="Jetzt veröffentlichen"
      cancelText="Abbrechen"
      isConfirmLoading={pending}
    >
      <div className="space-y-2 text-sm text-gray-700">
        <FormErrorAlert message={error} />
        <p>
          „{announcement.title}“ wird für{" "}
          <span className="font-medium">
            {summarizeTargets(announcement.targets)}
          </span>{" "}
          sichtbar.
        </p>
        {announcement.send_email && (
          <p>
            Die erreichten Eltern werden zusätzlich per E-Mail benachrichtigt.
          </p>
        )}
        <p className="text-xs text-gray-500">
          Nach dem Veröffentlichen kann die Mitteilung nicht mehr bearbeitet
          werden.
        </p>
      </div>
    </ConfirmationModal>
  );
}

export function UnpublishAnnouncementDialog({
  announcement,
  onClose,
  onDone,
}: LifecycleDialogProps) {
  const { pending, error, confirm } = useLifecycleAction(
    () => unpublishAnnouncement(announcement.id),
    {
      onDone,
      onClose,
      object: "das Zurückziehen der Mitteilung",
      logEvent: "announcement_unpublish_failed",
    },
  );

  return (
    <ConfirmationModal
      isOpen
      onClose={onClose}
      onConfirm={() => void confirm()}
      title="Elternmitteilung zurückziehen"
      confirmText="Zurückziehen"
      cancelText="Abbrechen"
      isConfirmLoading={pending}
    >
      <div className="space-y-2 text-sm text-gray-700">
        <FormErrorAlert message={error} />
        <p>
          „{announcement.title}“ wird für die Eltern nicht mehr sichtbar und
          kehrt in den Entwurfsstatus zurück.
        </p>
        <p className="text-xs text-gray-500">
          Noch nicht versendete E-Mail-Benachrichtigungen werden abgebrochen.
        </p>
        {announcement.delivery_mode === "declaration" && (
          <p className="text-xs text-gray-500">
            Bisherige Antworten bleiben gespeichert. Ändern Sie danach den Text,
            müssen die Eltern noch einmal antworten.
          </p>
        )}
      </div>
    </ConfirmationModal>
  );
}

export function DeleteAnnouncementDialog({
  announcement,
  onClose,
  onDone,
}: LifecycleDialogProps) {
  // Ein Einverständnis mit Antworten (#3430) bleibt als Nachweis stehen:
  // communication.declaration_has_submissions bringt dafür einen eigenen
  // Katalogtext mit.
  const { pending, error, confirm } = useLifecycleAction(
    () => deleteAnnouncement(announcement.id),
    {
      onDone,
      onClose,
      object: "das Löschen der Mitteilung",
      logEvent: "announcement_delete_failed",
    },
  );

  return (
    <ConfirmDeleteModal
      isOpen
      title="Elternmitteilung löschen"
      description={
        <>
          Möchten Sie die Elternmitteilung
          <span className="font-medium text-gray-900">
            {" "}
            „{announcement.title}“{" "}
          </span>
          wirklich löschen? Dies kann nicht rückgängig gemacht werden.
        </>
      }
      gate={{ mode: "twoStep" }}
      onConfirm={confirm}
      onClose={onClose}
      loading={pending}
      error={error}
    />
  );
}

interface MenuHandlers {
  readonly onPublish: () => void;
  /** Nur in der Liste: die Objektseite ist die Ansicht selbst. */
  readonly onView?: () => void;
  readonly onEdit: () => void;
  readonly onUnpublish: () => void;
  readonly onDelete: () => void;
  /**
   * Die geplante Erinnerung (#3162) einer VERÖFFENTLICHTEN Mitteilung planen
   * oder ändern. Entwürfe tragen die Erinnerung im Assistenten; Umfragen
   * haben nur ihre manuelle Erinnerung an offene Antworten.
   */
  readonly onReminder?: () => void;
}

/**
 * Das Aktionsmenü einer Mitteilung (BAUARTEN-SPEC Bauart 1 Regel 4 und
 * Bauart 2 Regel 3): Veröffentlichen eines Entwurfs zuerst. Bearbeiten gibt
 * es nur für Entwürfe, veröffentlichte Mitteilungen sind unveränderlich
 * (erst Zurückziehen, dann den Entwurf bearbeiten). Vom System erzeugte
 * Zeilen (Ausfall-Mitteilung, #2601) lassen sich weder bearbeiten noch
 * löschen.
 */
export function buildAnnouncementMenuItems(
  announcement: Announcement,
  handlers: MenuHandlers,
): OverflowMenuItem[] {
  const items: OverflowMenuItem[] = [];
  if (announcement.status === "draft") {
    items.push({
      label: "Veröffentlichen",
      icon: <Send className="size-4" aria-hidden />,
      onClick: handlers.onPublish,
    });
  }
  if (handlers.onView) {
    items.push({
      label: "Anzeigen",
      icon: <MotoConceptIcon concept="reports" size={16} />,
      onClick: handlers.onView,
    });
  }
  if (!announcement.system_kind && announcement.status === "draft") {
    items.push({
      label: "Bearbeiten",
      icon: <Pencil className="size-4" aria-hidden />,
      onClick: handlers.onEdit,
    });
  }
  if (
    handlers.onReminder &&
    !announcement.system_kind &&
    announcement.status === "published" &&
    announcement.response_type === "none" &&
    !announcement.reminder_sent_at
  ) {
    items.push({
      label: announcement.reminder_at
        ? "Erinnerung ändern"
        : "Erinnerung planen",
      icon: <BellRing className="size-4" aria-hidden />,
      onClick: handlers.onReminder,
    });
  }
  if (!announcement.system_kind && announcement.status === "published") {
    items.push({
      label: "Zurückziehen",
      icon: <Undo2 className="size-4" aria-hidden />,
      onClick: handlers.onUnpublish,
    });
  }
  if (!announcement.system_kind) {
    items.push({
      label: "Löschen",
      icon: <Trash2 className="size-4" aria-hidden />,
      destructive: true,
      onClick: handlers.onDelete,
    });
  }
  return items;
}
