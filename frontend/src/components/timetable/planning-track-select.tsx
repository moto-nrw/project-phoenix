"use client";

import { Check, ChevronDown } from "lucide-react";
import { useState } from "react";

import {
  ListboxDropdown,
  type ListboxDropdownOption,
} from "~/components/ui/listbox-dropdown";
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
  const [query, setQuery] = useState("");

  const selected = tracks.find((track) => track.id === value);
  const normalizedQuery = normalize(query);
  // Eine archivierte Spur steht nur da, solange sie der gewählte Wert ist:
  // sonst würde das Bearbeiten eines alten Termins sie still austauschen.
  const visibleTracks = tracks.filter(
    (track) =>
      (!track.archivedAt || track.id === value) &&
      normalize(track.name).includes(normalizedQuery),
  );
  const options: ListboxDropdownOption<string>[] = [
    ...(normalizedQuery.length === 0
      ? [{ value: "", label: "Keine Planungsspur" }]
      : []),
    ...visibleTracks.map((track) => ({
      value: track.id,
      label: `${track.name}${track.archivedAt ? " (archiviert)" : ""}`,
    })),
  ];

  return (
    <ListboxDropdown
      id="event_planning_track"
      value={value}
      options={options}
      onChange={onChange}
      ariaLabel="Planungsspur"
      disabled={disabled}
      placeholder="Keine Planungsspur"
      triggerRole="combobox"
      searchValue={query}
      onSearchChange={setQuery}
      searchPlaceholder="Planungsspur suchen …"
      onOpenChange={(open) => {
        if (!open) setQuery("");
      }}
      className="moto-content-surface flex h-10 w-full items-center justify-between gap-2 rounded-lg border px-3 text-left text-sm shadow-sm transition-colors hover:border-gray-300 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none disabled:cursor-not-allowed disabled:bg-gray-100 disabled:text-gray-500 disabled:opacity-80"
      menuClassName="moto-popover-surface flex max-h-72 flex-col overflow-hidden rounded-xl border"
      listClassName="scrollbar-thin overflow-y-auto py-1"
      emptyState={
        <li role="presentation">
          <p className="px-3 py-4 text-center text-sm text-gray-500">
            Keine Planungsspur gefunden.
          </p>
        </li>
      }
      optionClassName="flex min-h-10 w-full items-center gap-2 px-3 py-2 text-left text-sm text-gray-700 hover:bg-gray-50 focus-visible:bg-gray-50 focus-visible:outline-none"
      activeOptionClassName="flex min-h-10 w-full items-center gap-2 bg-gray-50 px-3 py-2 text-left text-sm text-gray-900 focus-visible:outline-none"
      renderTrigger={({ open }) => (
        <>
          <span className="flex min-w-0 flex-1 items-center gap-2">
            {selected ? (
              <span
                className="size-4 shrink-0 rounded-full border border-black/10"
                style={{ backgroundColor: selected.color }}
                aria-hidden="true"
              />
            ) : null}
            <span className="truncate text-gray-900">
              {selected
                ? `${selected.name}${selected.archivedAt ? " (archiviert)" : ""}`
                : "Keine Planungsspur"}
            </span>
          </span>
          <ChevronDown
            className={`size-4 shrink-0 text-gray-400 transition-transform ${
              open ? "rotate-180" : ""
            }`}
            aria-hidden="true"
          />
        </>
      )}
      renderOption={(option, { selected: isSelected }) => {
        const track = visibleTracks.find((item) => item.id === option.value);
        return (
          <>
            {track ? (
              <span
                className="size-4 shrink-0 rounded-full border border-black/10"
                style={{ backgroundColor: track.color }}
                aria-hidden="true"
              />
            ) : (
              <span className="size-4 shrink-0" aria-hidden="true" />
            )}
            <span className="min-w-0 flex-1 truncate" title={option.label}>
              {option.label}
            </span>
            {isSelected ? (
              <Check className="size-4 shrink-0" aria-hidden="true" />
            ) : null}
          </>
        );
      }}
    />
  );
}
