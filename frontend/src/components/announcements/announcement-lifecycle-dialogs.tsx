"use client";

import { useState } from "react";
import { Pencil, Send, Trash2, Undo2 } from "lucide-react";
import { Alert } from "~/components/ui/alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { ConfirmationModal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { OverflowMenuItem } from "~/components/ui/page-header/OverflowMenu";
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

function errorMessage(err: unknown, fallback: string): string {
  return err instanceof Error ? err.message : fallback;
}

export function PublishAnnouncementDialog({
  announcement,
  onClose,
  onDone,
}: LifecycleDialogProps) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  const confirm = async () => {
    setPending(true);
    setError("");
    try {
      await publishAnnouncement(announcement.id);
      await onDone();
      onClose();
    } catch (err) {
      const message = errorMessage(
        err,
        "Elternmitteilung konnte nicht veröffentlicht werden",
      );
      setError(message);
      logger.error("announcement_publish_failed", { error: message });
    } finally {
      setPending(false);
    }
  };

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
        {error && <Alert type="error" message={error} />}
      </div>
    </ConfirmationModal>
  );
}

export function UnpublishAnnouncementDialog({
  announcement,
  onClose,
  onDone,
}: LifecycleDialogProps) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  const confirm = async () => {
    setPending(true);
    setError("");
    try {
      await unpublishAnnouncement(announcement.id);
      await onDone();
      onClose();
    } catch (err) {
      const message = errorMessage(
        err,
        "Elternmitteilung konnte nicht zurückgezogen werden",
      );
      setError(message);
      logger.error("announcement_unpublish_failed", { error: message });
    } finally {
      setPending(false);
    }
  };

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
        <p>
          „{announcement.title}“ wird für die Eltern nicht mehr sichtbar und
          kehrt in den Entwurfsstatus zurück.
        </p>
        <p className="text-xs text-gray-500">
          Noch nicht versendete E-Mail-Benachrichtigungen werden abgebrochen.
        </p>
        {error && <Alert type="error" message={error} />}
      </div>
    </ConfirmationModal>
  );
}

export function DeleteAnnouncementDialog({
  announcement,
  onClose,
  onDone,
}: LifecycleDialogProps) {
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");

  const confirm = async () => {
    setPending(true);
    setError("");
    try {
      await deleteAnnouncement(announcement.id);
      await onDone();
      onClose();
    } catch (err) {
      const message = errorMessage(
        err,
        "Elternmitteilung konnte nicht gelöscht werden",
      );
      setError(message);
      logger.error("announcement_delete_failed", { error: message });
    } finally {
      setPending(false);
    }
  };

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
