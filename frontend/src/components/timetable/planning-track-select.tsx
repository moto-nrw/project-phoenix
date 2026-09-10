"use client";

import { Check, ChevronDown } from "lucide-react";
import { type KeyboardEvent, useEffect, useRef, useState } from "react";

import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Input } from "~/components/ui/input";
import type { PlanningTrack } from "~/lib/planning-track-api";

interface PlanningTrackSelectProps {
  readonly value: string;
  readonly tracks: readonly PlanningTrack[];
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
}

function normalize(value: string): string {
  return value.trim().toLocaleLowerCase("de");
}

/**
 * Auswahl einer Planungsspur — und nur das (#3114).
 *
 * Bis dahin trug dasselbe Popover drei Ansichten: auswählen, verwalten,
 * Formular. Anlegen, Umbenennen, Umsortieren und Archivieren steckten damit
 * in einem Auswahlfeld, mitten im Termin-Formular. Diese Fläche liegt jetzt
 * unter „Datenverwaltung → Planungsspuren"; das Feld daneben verlinkt sie.
 * Geblieben ist, was eine Auswahl braucht: Suche, Farbpunkt und Tastatur.
 */
export function PlanningTrackSelect({
  value,
  tracks,
  onChange,
  disabled = false,
}: PlanningTrackSelectProps) {
  const searchRef = useRef<HTMLInputElement>(null);
  const optionsRef = useRef<HTMLDivElement>(null);
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");

  useEffect(() => {
    if (open) searchRef.current?.focus();
  }, [open]);

  const selected = tracks.find((track) => track.id === value);
  const normalizedQuery = normalize(query);
  // Eine archivierte Spur steht nur da, solange sie der gewählte Wert ist:
  // sonst würde das Bearbeiten eines alten Termins sie still austauschen.
  const visibleTracks = tracks.filter(
    (track) =>
      (!track.archivedAt || track.id === value) &&
      normalize(track.name).includes(normalizedQuery),
  );

  const handleOpenChange = (next: boolean) => {
    setOpen(next);
    if (!next) setQuery("");
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

  const optionKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "ArrowDown") focusOption(event, 1);
    if (event.key === "ArrowUp") focusOption(event, -1);
  };

  return (
    <AnchoredPopover
      open={open}
      onOpenChange={handleOpenChange}
      ariaLabel="Planungsspur auswählen"
      initialFocusRef={searchRef}
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
      {({ close }) => (
        <div>
          <div className="border-b border-gray-100 p-2">
            <label htmlFor="planning_track_search" className="sr-only">
              Planungsspur suchen
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
              placeholder="Planungsspur suchen …"
              autoComplete="off"
              controlSize="compact"
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
                onKeyDown={optionKeyDown}
                onClick={() => {
                  onChange("");
                  close();
                }}
                className="flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50 focus-visible:bg-gray-50 focus-visible:outline-none"
              >
                <span className="size-4" aria-hidden="true" />
                <span className="min-w-0 flex-1 truncate">
                  Keine Planungsspur
                </span>
                {value === "" && (
                  <Check className="size-4" aria-hidden="true" />
                )}
              </button>
            )}
            {visibleTracks.map((track) => (
              <button
                key={track.id}
                type="button"
                role="option"
                aria-selected={track.id === value}
                onKeyDown={optionKeyDown}
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
            {visibleTracks.length === 0 && (
              <p className="px-3 py-4 text-center text-sm text-gray-500">
                Keine Planungsspur gefunden.
              </p>
            )}
          </div>
        </div>
      )}
    </AnchoredPopover>
  );
}
