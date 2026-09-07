"use client";

import { useState, type ReactNode } from "react";
import {
  ArrowDown,
  ArrowUp,
  ChevronLeft,
  ChevronRight,
  GripVertical,
  Plus,
  X,
} from "lucide-react";

import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { SectionCard } from "~/components/ui/section-card";
import {
  homeBlockDefinition,
  type HomeBlockDefinition,
  type HomeBlockKey,
  type HomeBlockPlacement,
  type HomeBlockSpan,
} from "~/lib/home-blocks";

/**
 * Das Brett der Startseite (#2180): die Bausteine an ihren Plätzen.
 *
 * Vier Spalten, feste Reihenhöhe. Eine Kennzahl belegt eine Zelle, eine Liste
 * zwei Reihen — dadurch stehen die Karten immer auf einer Linie, und die
 * Fläche wächst nicht mit dem Inhalt: was nicht hineinpasst, scrollt IN seiner
 * Karte. Die Startseite bleibt so ein Einstieg und wird keine lange Liste.
 *
 * Im Anpassen-Modus liegt über jeder Karte eine Leiste: ziehen zum Umsortieren,
 * schmaler und breiter, vor und zurück (für Tastatur und Vorleseprogramm) und
 * entfernen. Der Inhalt darunter nimmt in diesem Modus keine Klicks an, damit
 * ein Zug an der Karte nicht versehentlich einem Link folgt.
 */

const SPAN_CLASS: Record<HomeBlockSpan, string> = {
  1: "sm:col-span-1 xl:col-span-1",
  2: "sm:col-span-2 xl:col-span-2",
  4: "sm:col-span-2 xl:col-span-4",
};

/** Eine Kennzahl ist eine Reihe hoch, eine Liste zwei. */
function rowClass(definition: HomeBlockDefinition | null): string {
  return definition?.kind === "tile" ? "row-span-1" : "row-span-2";
}

export interface HomeBoardProps {
  readonly placements: readonly HomeBlockPlacement[];
  readonly addable: readonly HomeBlockDefinition[];
  readonly editing: boolean;
  readonly onReorder: (from: number, to: number) => void;
  readonly onSpanChange: (key: HomeBlockKey, span: HomeBlockSpan) => void;
  readonly onRemove: (key: HomeBlockKey) => void;
  readonly onAdd: (key: HomeBlockKey) => void;
  readonly children: (placement: HomeBlockPlacement) => ReactNode;
}

export function HomeBoard({
  placements,
  addable,
  editing,
  onReorder,
  onSpanChange,
  onRemove,
  onAdd,
  children,
}: HomeBoardProps) {
  const [draggedIndex, setDraggedIndex] = useState<number | null>(null);

  return (
    <div className="space-y-4">
      <ul
        data-testid="home-board"
        // 7rem je Reihe: eine Kennzahl ist eine Reihe hoch, eine Liste zwei
        // (15rem inklusive Abstand). Mit dem Kopf der Seite passen so vier
        // Kennzahlen und vier Listen auf einen 900px-Bildschirm — mehr Höhe
        // hiesse scrollen, und die Startseite soll ein Einstieg bleiben.
        className="grid auto-rows-[7rem] grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
      >
        {placements.map((placement, index) => {
          const definition = homeBlockDefinition(placement.key);
          return (
            <li
              key={placement.key}
              data-testid={`home-block-${placement.key}`}
              data-span={placement.span}
              className={`${SPAN_CLASS[placement.span]} ${rowClass(definition)} min-h-0`}
              draggable={editing}
              onDragStart={() => setDraggedIndex(index)}
              onDragEnd={() => setDraggedIndex(null)}
              onDragOver={(event) => {
                if (!editing || draggedIndex === null) return;
                event.preventDefault();
              }}
              onDrop={(event) => {
                if (!editing || draggedIndex === null) return;
                event.preventDefault();
                if (draggedIndex !== index) onReorder(draggedIndex, index);
                setDraggedIndex(null);
              }}
            >
              <div className="relative h-full min-h-0">
                {editing && definition && (
                  <BlockToolbar
                    definition={definition}
                    placement={placement}
                    index={index}
                    count={placements.length}
                    onReorder={onReorder}
                    onSpanChange={onSpanChange}
                    onRemove={onRemove}
                  />
                )}
                <div
                  className={
                    editing
                      ? // Die Leiste liegt über der Karte; der Inhalt rückt um
                        // ihre Höhe nach unten, damit nicht zwei Überschriften
                        // übereinander stehen.
                        "pointer-events-none h-full min-h-0 pt-9 select-none"
                      : "h-full min-h-0"
                  }
                >
                  {children(placement)}
                </div>
              </div>
            </li>
          );
        })}
      </ul>

      {editing && <AddPanel addable={addable} onAdd={onAdd} />}
    </div>
  );
}

function BlockToolbar({
  definition,
  placement,
  index,
  count,
  onReorder,
  onSpanChange,
  onRemove,
}: {
  readonly definition: HomeBlockDefinition;
  readonly placement: HomeBlockPlacement;
  readonly index: number;
  readonly count: number;
  readonly onReorder: (from: number, to: number) => void;
  readonly onSpanChange: (key: HomeBlockKey, span: HomeBlockSpan) => void;
  readonly onRemove: (key: HomeBlockKey) => void;
}) {
  const spans = definition.spans;
  const spanIndex = spans.indexOf(placement.span);
  const narrower = spanIndex > 0 ? spans[spanIndex - 1] : undefined;
  const wider =
    spanIndex >= 0 && spanIndex < spans.length - 1
      ? spans[spanIndex + 1]
      : undefined;

  return (
    <div className="absolute inset-x-0 top-0 z-10 flex items-center gap-1 rounded-t-2xl border-b border-gray-200 bg-white/95 px-2 py-1">
      <GripVertical
        className="h-4 w-4 shrink-0 cursor-grab text-gray-400"
        aria-hidden="true"
      />
      <span className="min-w-0 flex-1 truncate text-xs font-medium text-gray-600">
        {definition.label}
      </span>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={`${definition.label} nach vorne`}
        disabled={index === 0}
        onClick={() => onReorder(index, index - 1)}
      >
        <ArrowUp className="h-4 w-4" aria-hidden="true" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={`${definition.label} nach hinten`}
        disabled={index === count - 1}
        onClick={() => onReorder(index, index + 1)}
      >
        <ArrowDown className="h-4 w-4" aria-hidden="true" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={`${definition.label} schmaler`}
        disabled={narrower === undefined}
        onClick={() =>
          narrower !== undefined && onSpanChange(placement.key, narrower)
        }
      >
        <ChevronLeft className="h-4 w-4" aria-hidden="true" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={`${definition.label} breiter`}
        disabled={wider === undefined}
        onClick={() => wider !== undefined && onSpanChange(placement.key, wider)}
      >
        <ChevronRight className="h-4 w-4" aria-hidden="true" />
      </Button>
      <Button
        type="button"
        variant="ghost"
        size="icon"
        aria-label={`${definition.label} entfernen`}
        onClick={() => onRemove(placement.key)}
      >
        <X className="text-moto-red-strong h-4 w-4" aria-hidden="true" />
      </Button>
    </div>
  );
}

function AddPanel({
  addable,
  onAdd,
}: {
  readonly addable: readonly HomeBlockDefinition[];
  readonly onAdd: (key: HomeBlockKey) => void;
}) {
  return (
    <SectionCard
      title="Bausteine hinzufügen"
      description={
        addable.length === 0
          ? "Sie haben alles auf Ihrer Startseite, was für Sie verfügbar ist."
          : "Was Sie hinzufügen, erscheint am Ende Ihrer Startseite."
      }
    >
      {addable.length > 0 && (
        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
          {addable.map((block) => (
            <ChoiceTile
              key={block.key}
              as="button"
              className="p-3 text-left"
              onClick={() => onAdd(block.key)}
            >
              <span className="flex items-start gap-2">
                <Plus
                  className="mt-0.5 h-4 w-4 shrink-0 text-gray-400"
                  aria-hidden="true"
                />
                <span className="min-w-0">
                  <span className="block text-sm font-medium text-gray-900">
                    {block.label}
                  </span>
                  <span className="block text-xs text-gray-500">
                    {block.description}
                  </span>
                </span>
              </span>
            </ChoiceTile>
          ))}
        </div>
      )}
    </SectionCard>
  );
}
