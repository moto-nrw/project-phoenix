"use client";

import {
  ArchiveRestore,
  Check,
  ChevronDown,
  ChevronUp,
  Pencil,
  Plus,
} from "lucide-react";
import {
  type KeyboardEvent,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import { Alert } from "~/components/ui/alert";
import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Button } from "~/components/ui/button";
import { ColorPickerField } from "~/components/ui/color-picker-field";
import { Input } from "~/components/ui/input";
import { useToast } from "~/contexts/ToastContext";
import { LOCATION_COLORS } from "~/lib/location-helper";
import {
  planningTrackService,
  type PlanningTrack,
} from "~/lib/planning-track-api";

type View = "select" | "manage" | "form";

interface PlanningTrackSelectProps {
  readonly value: string;
  readonly tracks: readonly PlanningTrack[];
  readonly onChange: (value: string) => void;
  readonly onTracksChanged: (created?: PlanningTrack) => void | Promise<void>;
  readonly canManage?: boolean;
  readonly disabled?: boolean;
}

const DEFAULT_COLOR: string = LOCATION_COLORS.OTHER_ROOM;

function normalize(value: string): string {
  return value.trim().toLocaleLowerCase("de");
}

export function PlanningTrackSelect({
  value,
  tracks,
  onChange,
  onTracksChanged,
  canManage = false,
  disabled = false,
}: PlanningTrackSelectProps) {
  const toast = useToast();
  const searchRef = useRef<HTMLInputElement>(null);
  const nameRef = useRef<HTMLInputElement>(null);
  const optionsRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [view, setView] = useState<View>("select");
  const [items, setItems] = useState<PlanningTrack[]>([...tracks]);
  const [query, setQuery] = useState("");
  const [editing, setEditing] = useState<PlanningTrack | null>(null);
  const [name, setName] = useState("");
  const [color, setColor] = useState(DEFAULT_COLOR);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => setItems([...tracks]), [tracks]);

  useEffect(() => {
    if (!open) return;
    if (view === "select") searchRef.current?.focus();
    if (view === "form") nameRef.current?.focus();
  }, [open, view]);

  const active = useMemo(
    () => items.filter((track) => !track.archivedAt),
    [items],
  );
  const archived = useMemo(
    () => items.filter((track) => track.archivedAt),
    [items],
  );
  const selected = items.find((track) => track.id === value);
  const normalizedQuery = normalize(query);
  const visibleTracks = items.filter(
    (track) =>
      (!track.archivedAt || track.id === value) &&
      normalize(track.name).includes(normalizedQuery),
  );
  const canCreate =
    canManage &&
    query.trim().length > 0 &&
    !items.some((track) => normalize(track.name) === normalizedQuery);

  const handleOpenChange = (next: boolean) => {
    setOpen(next);
    if (next) return;
    setView("select");
    setQuery("");
    setEditing(null);
    setError(null);
  };

  const runAction = async (action: () => Promise<void>) => {
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

  const beginCreate = () => {
    setEditing(null);
    setName(query.trim());
    setColor(DEFAULT_COLOR);
    setError(null);
    setView("form");
  };

  const beginEdit = (track: PlanningTrack) => {
    setEditing(track);
    setName(track.name);
    setColor(track.color);
    setError(null);
    setView("form");
  };

  const save = async () => {
    const trimmedName = name.trim();
    if (!trimmedName) {
      setError("Name ist erforderlich");
      nameRef.current?.focus();
      return;
    }

    await runAction(async () => {
      if (editing) {
        const updated = await planningTrackService.update(editing.id, {
          name: trimmedName,
          color,
          sort_order: editing.sortOrder,
        });
        setItems((current) =>
          current.map((track) => (track.id === updated.id ? updated : track)),
        );
        await onTracksChanged();
        setEditing(null);
        setView("manage");
        return;
      }

      const created = await planningTrackService.create({
        name: trimmedName,
        color,
        sort_order:
          active.reduce(
            (highest, track) => Math.max(highest, track.sortOrder),
            -1,
          ) + 1,
      });
      setItems((current) => [...current, created]);
      onChange(created.id);
      await onTracksChanged(created);
      handleOpenChange(false);
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

    await runAction(async () => {
      const saved = await planningTrackService.reorder(
        reordered.map((track) => track.id),
      );
      setItems(saved);
      await onTracksChanged();
    });
  };

  const restoreTrack = async (track: PlanningTrack) => {
    await runAction(async () => {
      const restored = await planningTrackService.restore(track.id);
      setItems((current) =>
        current.map((item) => (item.id === track.id ? restored : item)),
      );
      await onTracksChanged();
      toast.success(`Planungsspur „${track.name}“ wiederhergestellt`);
    });
  };

  const archiveTrack = async (track: PlanningTrack) => {
    await runAction(async () => {
      await planningTrackService.archive(track.id);
      setItems((current) =>
        current.map((item) =>
          item.id === track.id
            ? { ...item, archivedAt: new Date().toISOString() }
            : item,
        ),
      );
      await onTracksChanged();
      const toastId = `planning-track-archived-${track.id}`;
      toast.success(`Planungsspur „${track.name}“ archiviert`, {
        id: toastId,
        duration: 0,
        action: {
          label: "Rückgängig",
          onClick: () => {
            toast.remove(toastId);
            void restoreTrack(track);
          },
        },
      });
    });
  };

  const focusOption = (
    event: KeyboardEvent<HTMLElement>,
    direction: 1 | -1,
  ) => {
    const options = Array.from(
      optionsRef.current?.querySelectorAll<HTMLElement>('[role="option"]') ??
        [],
    );
    if (options.length === 0) return;
    event.preventDefault();
    const currentIndex = options.indexOf(event.currentTarget);
    const nextIndex =
      currentIndex < 0
        ? direction === 1
          ? 0
          : options.length - 1
        : (currentIndex + direction + options.length) % options.length;
    options[nextIndex]?.focus();
  };

  const renderSelectView = (close: () => void) => (
    <div>
      <div className="border-b border-gray-100 p-2">
        <label htmlFor="planning_track_search" className="sr-only">
          Planungsspur suchen oder anlegen
        </label>
        <Input
          ref={searchRef}
          id="planning_track_search"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "ArrowDown") focusOption(event, 1);
            if (event.key === "ArrowUp") focusOption(event, -1);
          }}
          placeholder="Planungsspur suchen oder anlegen …"
          autoComplete="off"
        />
      </div>
      <div
        ref={optionsRef}
        role="listbox"
        aria-label="Planungsspuren"
        className="py-1"
      >
        {normalizedQuery.length === 0 && (
          <button
            type="button"
            role="option"
            aria-selected={value === ""}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") focusOption(event, 1);
              if (event.key === "ArrowUp") focusOption(event, -1);
            }}
            onClick={() => {
              onChange("");
              close();
            }}
            className="flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50 focus-visible:bg-gray-50 focus-visible:outline-none"
          >
            <span className="size-4" aria-hidden="true" />
            <span className="min-w-0 flex-1 truncate">Keine Planungsspur</span>
            {value === "" && <Check className="size-4" aria-hidden="true" />}
          </button>
        )}
        {visibleTracks.map((track) => (
          <button
            key={track.id}
            type="button"
            role="option"
            aria-selected={track.id === value}
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") focusOption(event, 1);
              if (event.key === "ArrowUp") focusOption(event, -1);
            }}
            onClick={() => {
              onChange(track.id);
              close();
            }}
            className="flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50 focus-visible:bg-gray-50 focus-visible:outline-none"
          >
            <span
              className="size-4 shrink-0 rounded-full border border-black/10"
              style={{ backgroundColor: track.color }}
              aria-hidden="true"
            />
            <span className="min-w-0 flex-1 truncate">
              {track.name}
              {track.archivedAt ? " (archiviert)" : ""}
            </span>
            {track.id === value && (
              <Check className="size-4 shrink-0" aria-hidden="true" />
            )}
          </button>
        ))}
        {visibleTracks.length === 0 && !canCreate && (
          <p className="px-3 py-4 text-center text-sm text-gray-500">
            Keine Planungsspur gefunden.
          </p>
        )}
        {canCreate && (
          <button
            type="button"
            role="option"
            aria-selected="false"
            onKeyDown={(event) => {
              if (event.key === "ArrowDown") focusOption(event, 1);
              if (event.key === "ArrowUp") focusOption(event, -1);
            }}
            onClick={beginCreate}
            className="flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-sm font-medium text-gray-900 hover:bg-gray-50 focus-visible:bg-gray-50 focus-visible:outline-none"
          >
            <Plus className="size-4 shrink-0" aria-hidden="true" />
            <span className="min-w-0 truncate">
              „{query.trim()}“ als Planungsspur anlegen
            </span>
          </button>
        )}
      </div>
      {canManage && (
        <div className="border-t border-gray-100 p-2">
          <Button
            type="button"
            variant="ghost"
            size="compact"
            className="w-full justify-start"
            onClick={() => {
              setError(null);
              setView("manage");
            }}
          >
            Planungsspuren verwalten
          </Button>
        </div>
      )}
    </div>
  );

  const renderManageView = () => (
    <div className="p-3">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h3 className="text-sm font-semibold text-gray-900">
          Planungsspuren verwalten
        </h3>
        <Button
          type="button"
          variant="ghost"
          size="compact"
          onClick={() => setView("select")}
        >
          Fertig
        </Button>
      </div>
      {error && <Alert type="error" message={error} />}
      <Button
        type="button"
        variant="outline"
        size="compact"
        className="mb-3"
        onClick={beginCreate}
      >
        <Plus className="size-4" aria-hidden="true" /> Neue Planungsspur
      </Button>
      {active.length === 0 ? (
        <p className="rounded-lg border border-dashed border-gray-200 p-4 text-center text-sm text-gray-500">
          Noch keine Planungsspuren angelegt.
        </p>
      ) : (
        <ul className="divide-y divide-gray-100 rounded-lg border border-gray-200">
          {active.map((track, index) => (
            <li key={track.id} className="flex items-center gap-1 px-2 py-1.5">
              <span
                className="size-4 shrink-0 rounded-full border border-black/10"
                style={{ backgroundColor: track.color }}
                aria-hidden="true"
              />
              <span className="min-w-0 flex-1 truncate text-sm font-medium text-gray-900">
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
                <ChevronUp className="size-4" aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`${track.name} nach unten`}
                disabled={busy || index === active.length - 1}
                onClick={() => void move(index, 1)}
              >
                <ChevronDown className="size-4" aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="icon"
                aria-label={`${track.name} bearbeiten`}
                disabled={busy}
                onClick={() => beginEdit(track)}
              >
                <Pencil className="size-4" aria-hidden="true" />
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="compact"
                aria-label={`${track.name} archivieren`}
                disabled={busy}
                onClick={() => void archiveTrack(track)}
              >
                Archivieren
              </Button>
            </li>
          ))}
        </ul>
      )}
      {archived.length > 0 && (
        <details className="mt-3">
          <summary className="cursor-pointer text-xs font-semibold text-gray-600">
            Archivierte Planungsspuren ({archived.length})
          </summary>
          <ul className="mt-2 divide-y divide-gray-100 rounded-lg border border-gray-200">
            {archived.map((track) => (
              <li
                key={track.id}
                className="flex items-center gap-2 px-2 py-1.5 text-gray-500"
              >
                <span className="min-w-0 flex-1 truncate text-sm">
                  {track.name}
                </span>
                <Button
                  type="button"
                  variant="ghost"
                  size="compact"
                  disabled={busy}
                  onClick={() => void restoreTrack(track)}
                >
                  <ArchiveRestore className="size-4" aria-hidden="true" />
                  Wiederherstellen
                </Button>
              </li>
            ))}
          </ul>
        </details>
      )}
    </div>
  );

  const renderFormView = () => (
    <div className="space-y-4 p-3">
      <h3 className="text-sm font-semibold text-gray-900">
        {editing ? "Planungsspur bearbeiten" : "Neue Planungsspur"}
      </h3>
      {error && <Alert type="error" message={error} />}
      <div>
        <label
          htmlFor="planning_track_name"
          className="mb-1 block text-sm font-medium text-gray-700"
        >
          Name
        </label>
        <Input
          ref={nameRef}
          id="planning_track_name"
          value={name}
          maxLength={100}
          onChange={(event) => setName(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              void save();
            }
          }}
          required
        />
      </div>
      <ColorPickerField
        label="Farbe"
        value={color}
        fallbackColor={DEFAULT_COLOR}
        onChange={(next) => setColor(next ?? DEFAULT_COLOR)}
      />
      <div className="flex justify-end gap-2 border-t border-gray-100 pt-3">
        <Button
          type="button"
          variant="outline"
          size="md"
          disabled={busy}
          onClick={() => setView(editing ? "manage" : "select")}
        >
          Abbrechen
        </Button>
        <Button
          type="button"
          variant="primary"
          size="md"
          isLoading={busy}
          onClick={() => void save()}
        >
          {editing ? "Speichern" : "Anlegen"}
        </Button>
      </div>
    </div>
  );

  return (
    <AnchoredPopover
      open={open}
      onOpenChange={handleOpenChange}
      ariaLabel="Planungsspur auswählen und verwalten"
      initialFocusRef={view === "form" ? nameRef : searchRef}
      renderTrigger={({ ref, open: isOpen, panelId, toggle }) => (
        <button
          ref={ref}
          id="event_planning_track"
          type="button"
          role="combobox"
          aria-label="Planungsspur"
          aria-haspopup="dialog"
          aria-expanded={isOpen}
          aria-controls={isOpen ? panelId : undefined}
          disabled={disabled}
          onClick={toggle}
          onKeyDown={(event) => {
            if (event.key !== "Escape" || !isOpen) return;
            event.preventDefault();
            event.stopPropagation();
            event.nativeEvent.stopImmediatePropagation();
            handleOpenChange(false);
          }}
          className="moto-content-surface flex h-10 w-full items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80"
        >
          <span className="flex min-w-0 flex-1 items-center gap-2">
            {selected && (
              <span
                className="size-4 shrink-0 rounded-full border border-black/10"
                style={{ backgroundColor: selected.color }}
                aria-hidden="true"
              />
            )}
            <span className="truncate text-gray-900">
              {selected
                ? `${selected.name}${selected.archivedAt ? " (archiviert)" : ""}`
                : "Keine Planungsspur"}
            </span>
          </span>
          <ChevronDown
            className={`size-4 shrink-0 text-gray-400 ${isOpen ? "rotate-180" : ""}`}
            aria-hidden="true"
          />
        </button>
      )}
    >
      {({ close }) => {
        if (view === "manage") return renderManageView();
        if (view === "form") return renderFormView();
        return renderSelectView(close);
      }}
    </AnchoredPopover>
  );
}
