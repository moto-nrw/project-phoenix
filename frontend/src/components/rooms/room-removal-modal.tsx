"use client";

import {
  Archive,
  CalendarClock,
  History,
  PlayCircle,
  Shapes,
  Tablet,
  Users,
  HeartHandshake,
} from "lucide-react";
import { type ReactNode, useEffect, useState } from "react";

import { Alert } from "~/components/ui/alert";
import { ConfirmDeleteModal } from "~/components/ui/confirm-delete-modal";
import { LoadErrorAlert } from "~/components/ui/form-error-alert";
import { FramedList } from "~/components/ui/framed-list";
import { IconTile } from "~/components/ui/icon-tile";
import { SectionLabel } from "~/components/ui/section-label";
import { SkeletonRows } from "~/components/ui/skeleton";
import { useApiFormError, useApiLoadError } from "~/contexts/ToastContext";
import { createLogger } from "~/lib/logger";
import {
  fetchRoomRemovalImpact,
  removeRoom,
  type RoomRemovalImpact,
  type RoomRemovalOutcome,
} from "~/lib/room-removal-api";

const logger = createLogger({ component: "RoomRemovalModal" });

interface ImpactRow {
  readonly key: string;
  readonly icon: ReactNode;
  readonly label: string;
  /** Was die Schule tun kann, damit der Raum frei wird. */
  readonly hint?: string;
  readonly value: string;
}

/** Was den Raum noch hält, mit dem nächsten Schritt je Zeile. */
function blockingRows(impact: RoomRemovalImpact): ImpactRow[] {
  const refs = impact.references;
  const rows: Array<ImpactRow | false> = [
    impact.active_session && {
      key: "active_session",
      icon: <PlayCircle aria-hidden />,
      label: "Aktivität läuft gerade",
      hint: "Beenden Sie sie zuerst.",
      value: "jetzt",
    },
    impact.care_offering && {
      key: "care_offering",
      icon: <HeartHandshake aria-hidden />,
      label: "Betreuungsangebot",
      hint: "Wählen Sie im Angebot einen anderen Raum.",
      value: "1",
    },
    refs.activity_groups > 0 && {
      key: "activity_groups",
      icon: <Shapes aria-hidden />,
      label:
        refs.activity_groups === 1
          ? "Aktivität plant diesen Raum"
          : "Aktivitäten planen diesen Raum",
      hint: "Wählen Sie bei der Aktivität einen anderen Raum.",
      value: String(refs.activity_groups),
    },
    refs.upcoming_blocks > 0 && {
      key: "upcoming_blocks",
      icon: <CalendarClock aria-hidden />,
      label:
        refs.upcoming_blocks === 1 ? "Kommender Termin" : "Kommende Termine",
      hint: "Verlegen oder sagen Sie sie im Betreuungsplan ab.",
      value: String(refs.upcoming_blocks),
    },
    refs.education_groups > 0 && {
      key: "education_groups",
      icon: <Users aria-hidden />,
      label:
        refs.education_groups === 1
          ? "Gruppe hat hier ihren Raum"
          : "Gruppen haben hier ihren Raum",
      hint: "Wählen Sie bei der Gruppe einen anderen Raum.",
      value: String(refs.education_groups),
    },
    refs.devices > 0 && {
      key: "devices",
      icon: <Tablet aria-hidden />,
      label: refs.devices === 1 ? "Gerät steht hier" : "Geräte stehen hier",
      hint: "Ordnen Sie das Gerät einem anderen Raum zu.",
      value: String(refs.devices),
    },
  ];
  return rows.filter((row): row is ImpactRow => row !== false);
}

/** Was erhalten bleibt, wenn der Raum archiviert wird. */
function historyRows(impact: RoomRemovalImpact): ImpactRow[] {
  const refs = impact.references;
  const rows: Array<ImpactRow | false> = [
    refs.past_blocks > 0 && {
      key: "past_blocks",
      icon: <History aria-hidden />,
      label: refs.past_blocks === 1 ? "Vergangener Termin" : "Vergangene Termine",
      value: String(refs.past_blocks),
    },
    refs.has_session_history && {
      key: "sessions",
      icon: <Users aria-hidden />,
      label: "Anwesenheiten im Verlauf",
      value: "vorhanden",
    },
    refs.archived_activity_groups > 0 && {
      key: "archived_activity_groups",
      icon: <Archive aria-hidden />,
      label: "Archivierte Aktivitäten",
      value: String(refs.archived_activity_groups),
    },
  ];
  return rows.filter((row): row is ImpactRow => row !== false);
}

function ImpactList({
  rows,
  tone,
}: {
  readonly rows: ReadonlyArray<ImpactRow>;
  readonly tone: "orange" | "neutral";
}) {
  return (
    <FramedList>
      {rows.map((row) => (
        <li key={row.key} className="flex items-center gap-3 px-4 py-3">
          <IconTile icon={row.icon} size="xs" tone={tone} />
          <div className="min-w-0 flex-1">
            <p className="text-sm font-medium text-gray-900">{row.label}</p>
            {row.hint ? (
              <p className="text-xs text-gray-600">{row.hint}</p>
            ) : null}
          </div>
          <span className="text-sm font-semibold text-gray-900 tabular-nums">
            {row.value}
          </span>
        </li>
      ))}
    </FramedList>
  );
}

const TITLES: Record<RoomRemovalOutcome, string> = {
  delete: "Raum löschen?",
  archive: "Raum archivieren?",
  blocked: "Raum wird noch genutzt",
};

interface RoomRemovalModalProps {
  readonly isOpen: boolean;
  readonly roomId: string;
  readonly roomName: string;
  readonly onClose: () => void;
  /** Nach dem Löschen oder Archivieren; die Seite meldet das Ergebnis. */
  readonly onRemoved: (
    outcome: Exclude<RoomRemovalOutcome, "blocked">,
  ) => Promise<void> | void;
}

// Entfernen eines Raums (#6845): Die Vorschau sagt vor der Rückfrage, ob der
// Raum gelöscht oder archiviert wird oder was ihn noch hält. Ein Raum mit
// Verlauf wird archiviert, damit vergangene Termine und Anwesenheiten ihren
// Raum behalten; ein noch genutzter Raum bleibt, bis die Schule umplant.
export function RoomRemovalModal({
  isOpen,
  roomId,
  roomName,
  onClose,
  onRemoved,
}: RoomRemovalModalProps) {
  const [impact, setImpact] = useState<RoomRemovalImpact | null>(null);
  const [loadingImpact, setLoadingImpact] = useState(false);
  const [removing, setRemoving] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const errors = useApiFormError();
  const clearErrors = errors.clear;
  const impactLoad = useApiLoadError();
  const showImpactError = impactLoad.show;
  const clearImpactError = impactLoad.clear;

  useEffect(() => {
    if (!isOpen) {
      setImpact(null);
      clearErrors();
      clearImpactError();
      return;
    }
    let active = true;
    setLoadingImpact(true);
    clearImpactError();
    void fetchRoomRemovalImpact(roomId)
      .then((result) => {
        if (active) setImpact(result);
      })
      .catch((loadError: unknown) => {
        logger.warn("room_removal_impact_failed", {
          room_id: roomId,
          error:
            loadError instanceof Error ? loadError.message : String(loadError),
        });
        if (!active) return;
        void showImpactError(loadError, {
          object: "die Vorschau",
          retry: () => setAttempt((count) => count + 1),
        });
      })
      .finally(() => {
        if (active) setLoadingImpact(false);
      });
    return () => {
      active = false;
    };
  }, [attempt, clearErrors, clearImpactError, isOpen, roomId, showImpactError]);

  const outcome = impact?.outcome ?? "delete";
  const blockers = impact ? blockingRows(impact) : [];
  const history = impact ? historyRows(impact) : [];

  const handleConfirm = async () => {
    if (!impact || impact.outcome === "blocked") return;
    setRemoving(true);
    clearErrors();
    let result: Exclude<RoomRemovalOutcome, "blocked">;
    try {
      result = await removeRoom(roomId);
    } catch (removeError) {
      logger.warn("room_removal_failed", {
        room_id: roomId,
        error:
          removeError instanceof Error
            ? removeError.message
            : String(removeError),
      });
      // Hält jemand den Raum inzwischen, zeigt die neue Vorschau, was.
      setAttempt((count) => count + 1);
      await errors.show(removeError, { object: "das Entfernen des Raums" });
      setRemoving(false);
      return;
    }
    try {
      await onRemoved(result);
    } finally {
      setRemoving(false);
    }
  };

  const description = (() => {
    if (!impact) {
      return (
        <p>
          moto prüft, wo <span className="font-medium">{roomName}</span> noch
          genutzt wird.
        </p>
      );
    }
    if (impact.outcome === "blocked") {
      return (
        <Alert
          type="warning"
          message={
            <>
              <span className="font-medium">{roomName}</span> wird noch
              gebraucht. Planen Sie die Punkte unten um. Danach können Sie den
              Raum entfernen.
            </>
          }
        />
      );
    }
    if (impact.outcome === "archive") {
      return (
        <p>
          <span className="font-medium">{roomName}</span> verschwindet aus
          allen Listen und Auswahlfeldern. Vergangene Termine und Anwesenheiten
          behalten den Raum. Den Namen können Sie für einen neuen Raum nutzen.
        </p>
      );
    }
    return (
      <p>
        <span className="font-medium">{roomName}</span> wird nirgends genutzt
        und wird endgültig gelöscht.
      </p>
    );
  })();

  return (
    <ConfirmDeleteModal
      isOpen={isOpen}
      title={impact ? TITLES[outcome] : "Raum entfernen"}
      description={description}
      warningSlot={
        <div className="space-y-3">
          <LoadErrorAlert error={impactLoad.error} />
          {loadingImpact && !impact ? (
            <SkeletonRows rows={3} />
          ) : null}
          {impact && blockers.length > 0 ? (
            <ImpactList rows={blockers} tone="orange" />
          ) : null}
          {impact && impact.outcome === "archive" && history.length > 0 ? (
            <div className="space-y-2">
              <SectionLabel as="h3">Bleibt im Verlauf erhalten</SectionLabel>
              <ImpactList rows={history} tone="neutral" />
            </div>
          ) : null}
        </div>
      }
      gate={{
        mode: "twoStep",
        firstStepLabel: outcome === "archive" ? "Ja, archivieren" : undefined,
      }}
      confirmDisabled={!impact || impact.outcome === "blocked"}
      confirmLabel={outcome === "archive" ? "Archivieren" : "Endgültig löschen"}
      loadingLabel={
        outcome === "archive" ? "Wird archiviert…" : "Wird gelöscht…"
      }
      onConfirm={handleConfirm}
      onClose={onClose}
      loading={removing}
      error={errors.error}
    />
  );
}
