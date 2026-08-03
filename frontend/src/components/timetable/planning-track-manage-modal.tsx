"use client";

import { useEffect, useState } from "react";
import {
  ArchiveRestore,
  ChevronDown,
  ChevronUp,
  Pencil,
  Plus,
} from "lucide-react";

import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ColorPickerField } from "~/components/ui/color-picker-field";
import { Input } from "~/components/ui/input";
import { ConfirmationModal, Modal } from "~/components/ui/modal";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  planningTrackService,
  type PlanningTrack,
} from "~/lib/planning-track-api";

interface PlanningTrackManageModalProps {
  readonly isOpen: boolean;
  readonly initialView?: "list" | "create";
  readonly onClose: () => void;
  readonly onChanged: (created?: PlanningTrack) => void | Promise<void>;
}

export function PlanningTrackManageModal({
  isOpen,
  initialView = "list",
  onClose,
  onChanged,
}: PlanningTrackManageModalProps) {
  const [tracks, setTracks] = useState<PlanningTrack[]>([]);
  const [view, setView] = useState<"list" | "form">("list");
  const [editing, setEditing] = useState<PlanningTrack | null>(null);
  const [archiveTarget, setArchiveTarget] = useState<PlanningTrack | null>(
    null,
  );
  const [name, setName] = useState("");
  const [color, setColor] = useState<string>(LOCATION_COLORS.OTHER_ROOM);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reload = async () => {
    setLoading(true);
    try {
      setTracks(await planningTrackService.list());
      setError(null);
    } catch (err: unknown) {
      setError(
        err instanceof Error
          ? err.message
          : "Planungsspuren konnten nicht geladen werden",
      );
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isOpen) return;
    setView(initialView === "create" ? "form" : "list");
    setEditing(null);
    setName("");
    setColor(LOCATION_COLORS.OTHER_ROOM);
    setError(null);
    void reload();
  }, [initialView, isOpen]);

  const active = tracks.filter((track) => !track.archivedAt);
  const archived = tracks.filter((track) => track.archivedAt);

  const run = async (action: () => Promise<void>) => {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : "Aktion fehlgeschlagen");
    } finally {
      setBusy(false);
    }
  };

  const save = async () => {
    const trimmed = name.trim();
    if (!trimmed) {
      setError("Name ist erforderlich");
      return;
    }
    await run(async () => {
      if (editing) {
        await planningTrackService.update(editing.id, {
          name: trimmed,
          color,
          sort_order: editing.sortOrder,
        });
        await reload();
        await onChanged();
        setView("list");
        return;
      }
      const created = await planningTrackService.create({
        name: trimmed,
        color,
        sort_order:
          active.reduce(
            (highest, track) => Math.max(highest, track.sortOrder),
            -1,
          ) + 1,
      });
      await onChanged(created);
    });
  };

  const move = async (index: number, offset: -1 | 1) => {
    const target = index + offset;
    if (target < 0 || target >= active.length) return;
    const reordered = [...active];
    [reordered[index], reordered[target]] = [
      reordered[target]!,
      reordered[index]!,
    ];
    await run(async () => {
      setTracks(
        await planningTrackService.reorder(reordered.map((track) => track.id)),
      );
      await onChanged();
    });
  };

  const footer =
    view === "form" ? (
      <div className="flex w-full justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          onClick={() => setView("list")}
          disabled={busy}
        >
          Abbrechen
        </Button>
        <Button
          type="button"
          variant="primary"
          onClick={() => void save()}
          isLoading={busy}
        >
          Speichern
        </Button>
      </div>
    ) : (
      <div className="flex w-full justify-end">
        <Button type="button" variant="outline" onClick={onClose}>
          Schließen
        </Button>
      </div>
    );

  return (
    <>
      <Modal
        isOpen={isOpen && !archiveTarget}
        onClose={onClose}
        title={
          view === "form"
            ? editing
              ? "Planungsspur bearbeiten"
              : "Neue Planungsspur"
            : "Planungsspuren verwalten"
        }
        footer={footer}
      >
        {error && <Alert type="error" message={error} />}
        {view === "form" ? (
          <div className="space-y-4">
            <div>
              <label
                htmlFor="planning_track_name"
                className="mb-1 block text-sm font-medium text-gray-700"
              >
                Name
              </label>
              <Input
                id="planning_track_name"
                value={name}
                maxLength={100}
                onChange={(event) => setName(event.target.value)}
                required
              />
            </div>
            <ColorPickerField
              label="Farbe"
              value={color}
              fallbackColor={LOCATION_COLORS.OTHER_ROOM}
              onChange={(next) => setColor(next ?? LOCATION_COLORS.OTHER_ROOM)}
            />
          </div>
        ) : loading ? (
          <p className="py-8 text-center text-sm text-gray-500">
            Planungsspuren werden geladen…
          </p>
        ) : (
          <div className="space-y-5">
            <p className="text-sm text-gray-600">
              Planungsspuren geben wiederkehrenden Terminen eine eigene Farbe
              und eine stabile Reihenfolge im Betreuungsplan.
            </p>
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setEditing(null);
                setName("");
                setColor(LOCATION_COLORS.OTHER_ROOM);
                setView("form");
              }}
            >
              <Plus className="h-4 w-4" /> Neue Planungsspur
            </Button>
            {active.length === 0 ? (
              <p className="rounded-xl border border-dashed border-gray-200 p-5 text-center text-sm text-gray-500">
                Noch keine Planungsspuren angelegt.
              </p>
            ) : (
              <ul className="divide-y divide-gray-100 rounded-xl border border-gray-200">
                {active.map((track, index) => (
                  <li
                    key={track.id}
                    className="flex items-center gap-2 px-3 py-2"
                  >
                    <span
                      className="h-4 w-4 shrink-0 rounded-full border border-black/10"
                      style={{ backgroundColor: track.color }}
                      aria-hidden
                    />
                    <span className="min-w-0 flex-1 truncate font-medium text-gray-900">
                      {track.name}
                    </span>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={`${track.name} nach oben`}
                      disabled={busy || index === 0}
                      onClick={() => void move(index, -1)}
                    >
                      <ChevronUp className="h-4 w-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={`${track.name} nach unten`}
                      disabled={busy || index === active.length - 1}
                      onClick={() => void move(index, 1)}
                    >
                      <ChevronDown className="h-4 w-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={`${track.name} bearbeiten`}
                      onClick={() => {
                        setEditing(track);
                        setName(track.name);
                        setColor(track.color);
                        setView("form");
                      }}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="compact"
                      onClick={() => setArchiveTarget(track)}
                    >
                      Archivieren
                    </Button>
                  </li>
                ))}
              </ul>
            )}
            {archived.length > 0 && (
              <div>
                <h3 className="mb-2 text-xs font-semibold tracking-wide text-gray-500 uppercase">
                  Archiviert
                </h3>
                <ul className="divide-y divide-gray-100 rounded-xl border border-gray-200">
                  {archived.map((track) => (
                    <li
                      key={track.id}
                      className="flex items-center gap-2 px-3 py-2 text-gray-500"
                    >
                      <span className="min-w-0 flex-1 truncate">
                        {track.name}
                      </span>
                      <Button
                        type="button"
                        variant="ghost"
                        size="compact"
                        disabled={busy}
                        onClick={() =>
                          void run(async () => {
                            await planningTrackService.restore(track.id);
                            await reload();
                            await onChanged();
                          })
                        }
                      >
                        <ArchiveRestore className="h-4 w-4" /> Wiederherstellen
                      </Button>
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </Modal>
      <ConfirmationModal
        isOpen={archiveTarget !== null}
        onClose={() => setArchiveTarget(null)}
        onConfirm={() =>
          void run(async () => {
            if (!archiveTarget) return;
            await planningTrackService.archive(archiveTarget.id);
            setArchiveTarget(null);
            await reload();
            await onChanged();
          })
        }
        title="Planungsspur archivieren?"
        confirmText="Archivieren"
        cancelText="Abbrechen"
        isConfirmLoading={busy}
      >
        <p className="text-sm text-gray-600">
          Bereits zugeordnete Regeltermine behalten diese Planungsspur. Für neue
          Zuordnungen steht sie nicht mehr zur Auswahl.
        </p>
      </ConfirmationModal>
    </>
  );
}
