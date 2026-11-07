"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { Check } from "lucide-react";
import { useState } from "react";

import {
  ListboxChevron,
  ListboxDropdown,
  type ListboxDropdownOption,
} from "~/components/ui/listbox-dropdown";
import type { PlanningTrack } from "~/lib/planning-track-api";

interface PlanningTrackSelectProps {
  readonly value: string;
  readonly tracks: readonly PlanningTrack[];
  readonly onChange: (value: string) => void;
  readonly disabled?: boolean;
  readonly ariaDescribedBy?: string;
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
  ariaDescribedBy,
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
      ariaDescribedBy={ariaDescribedBy}
      disabled={disabled}
      placeholder="Keine Planungsspur"
      triggerRole="combobox"
      searchValue={query}
      onSearchChange={setQuery}
      searchPlaceholder="Planungsspur suchen …"
      onOpenChange={(open) => {
        if (!open) setQuery("");
      }}
      className="w-full"
      emptyState={
        <li role="presentation">
          <EmptyState
            variant="compact"
            title="Keine Planungsspur gefunden."
            className="px-3"
          />
        </li>
      }
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
          <ListboxChevron open={open} />
        </>
      )}
      renderOption={(option, isSelected) => {
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
