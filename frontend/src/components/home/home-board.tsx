"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";
import { ArrowLeft, ArrowRight, GripVertical, Plus, Trash2 } from "lucide-react";

import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SectionCard } from "~/components/ui/section-card";
import { SegmentedControl } from "~/components/ui/segmented-control";
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
 * Im Anpassen-Modus zeigt das Brett nicht die Inhalte, sondern die Anordnung:
 * jede Karte wird zu einer Platzhalter-Kachel mit Symbol, Name und Breite, an
 * derselben Stelle und in derselben Größe wie die echte Karte. Man ordnet
 * hier, man liest nicht. Eine Kachel anklicken wählt sie aus; alles Weitere
 * steht in EINER Leiste über der Fläche, die beim Scrollen stehen bleibt —
 * statt in fünf Knöpfen auf jeder einzelnen Karte.
 */

const SPAN_CLASS: Record<HomeBlockSpan, string> = {
  1: "sm:col-span-1 xl:col-span-1",
  2: "sm:col-span-2 xl:col-span-2",
  4: "sm:col-span-2 xl:col-span-4",
};

const SPAN_LABEL: Record<HomeBlockSpan, string> = {
  1: "Schmal",
  2: "Breit",
  4: "Volle Breite",
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
  const [draggedKey, setDraggedKey] = useState<HomeBlockKey | null>(null);
  const [overKey, setOverKey] = useState<HomeBlockKey | null>(null);
  const [selectedKey, setSelectedKey] = useState<HomeBlockKey | null>(null);
  // Ein Zug, der weit genug ging, darf am Ende nicht auch noch als Klick
  // gelten — sonst wählt das Loslassen die Kachel aus, auf der man landet.
  const draggedFar = useRef(false);

  // Beim Verlassen des Anpassen-Modus die Auswahl fallen lassen, sonst steht
  // beim nächsten Öffnen eine Kachel markiert, die niemand angeklickt hat.
  useEffect(() => {
    if (!editing) {
      setSelectedKey(null);
      setDraggedKey(null);
      setOverKey(null);
    }
  }, [editing]);

  const selectedIndex = placements.findIndex(
    (placement) => placement.key === selectedKey,
  );
  const selected = selectedIndex >= 0 ? placements[selectedIndex] : undefined;
  const selectedDefinition = selected
    ? homeBlockDefinition(selected.key)
    : null;

  const move = (from: number, to: number) => {
    if (to < 0 || to >= placements.length) return;
    onReorder(from, to);
  };

  return (
    <div className="space-y-4">
      {/* Die Bedienleiste steht ÜBER der Fläche und bleibt beim Scrollen
          stehen: unter acht Karten hätte man sie nie gesehen. Der Abstand von
          oben ist die Höhe der App-Kopfzeile (65 px, sticky) plus Luft —
          darunter verschwände die Leiste hinter ihr. */}
      {editing && (
        <div className="sticky top-[4.5rem] z-20">
          <SelectionBar
            definition={selectedDefinition}
            placement={selected}
            index={selectedIndex}
            total={placements.length}
            onMove={move}
            onSpanChange={onSpanChange}
            onRemove={(key) => {
              setSelectedKey(null);
              onRemove(key);
            }}
          />
        </div>
      )}

      <ul
        data-testid="home-board"
        className="grid auto-rows-[7rem] grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4"
      >
        {placements.map((placement, index) => {
          const definition = homeBlockDefinition(placement.key);
          return (
            <li
              key={placement.key}
              data-testid={`home-block-${placement.key}`}
              data-block-key={placement.key}
              data-span={placement.span}
              className={`${SPAN_CLASS[placement.span]} ${rowClass(definition)} min-h-0`}
              // Zeigerereignisse statt HTML5-Ziehen: das native Ziehen kennt
              // kein Tablet und lässt sich nicht testen. Der Zug beginnt erst
              // nach ein paar Pixeln, damit ein Klick ein Klick bleibt.
              onPointerDown={(event) => {
                if (!editing || event.pointerType !== "mouse") return;
                if (event.button !== 0) return;
                draggedFar.current = false;
                setDraggedKey(placement.key);
                setOverKey(null);
                // Optional: in Testumgebungen ohne Zeiger-Erfassung fehlt sie.
                event.currentTarget.setPointerCapture?.(event.pointerId);
              }}
              onPointerMove={(event) => {
                if (!editing || draggedKey !== placement.key) return;
                draggedFar.current = true;
                const element = document.elementFromPoint(
                  event.clientX,
                  event.clientY,
                );
                const target = element?.closest<HTMLElement>("[data-block-key]");
                setOverKey(
                  (target?.dataset.blockKey as HomeBlockKey | undefined) ?? null,
                );
              }}
              onPointerUp={(event) => {
                if (!editing || draggedKey !== placement.key) return;
                event.currentTarget.releasePointerCapture?.(event.pointerId);
                const to = placements.findIndex(
                  (entry) => entry.key === overKey,
                );
                if (draggedFar.current && to >= 0 && to !== index) {
                  move(index, to);
                } else if (!draggedFar.current) {
                  setSelectedKey(placement.key);
                }
                setDraggedKey(null);
                setOverKey(null);
              }}
              onPointerCancel={() => {
                setDraggedKey(null);
                setOverKey(null);
              }}
            >
              {editing && definition ? (
                <ArrangeTile
                  definition={definition}
                  placement={placement}
                  position={index + 1}
                  total={placements.length}
                  selected={placement.key === selectedKey}
                  dragging={placement.key === draggedKey}
                  dropTarget={
                    overKey === placement.key && draggedKey !== placement.key
                  }
                  onSelect={() => {
                    // Nach einem Zug ist der Klick nur das Loslassen.
                    if (draggedFar.current) return;
                    setSelectedKey(placement.key);
                  }}
                />
              ) : (
                <div className="h-full min-h-0">{children(placement)}</div>
              )}
            </li>
          );
        })}
      </ul>

      {editing && <AddPanel addable={addable} onAdd={onAdd} />}
    </div>
  );
}

/**
 * Die Kachel im Anpassen-Modus: Symbol, Name, Breite und der Platz in der
 * Reihenfolge. Sie ist ein Knopf, damit sie sich auch mit der Tastatur
 * auswählen lässt — Ziehen allein wäre nicht bedienbar.
 */
function ArrangeTile({
  definition,
  placement,
  position,
  total,
  selected,
  dragging,
  dropTarget,
  onSelect,
}: {
  readonly definition: HomeBlockDefinition;
  readonly placement: HomeBlockPlacement;
  readonly position: number;
  readonly total: number;
  readonly selected: boolean;
  readonly dragging: boolean;
  readonly dropTarget: boolean;
  readonly onSelect: () => void;
}) {
  return (
    <ChoiceTile
      as="button"
      selected={selected}
      tone={selected ? "green" : dropTarget ? "blue" : "gray"}
      onClick={onSelect}
      aria-pressed={selected}
      aria-label={`${definition.label} auswählen, Platz ${position} von ${total}, ${SPAN_LABEL[placement.span]}`}
      className={`flex h-full w-full cursor-grab flex-col items-start gap-2 p-4 text-left ${
        dropTarget ? "ring-moto-blue ring-2" : ""
      } ${dragging ? "opacity-50" : ""}`}
    >
      <span className="flex w-full items-center gap-2">
        <GripVertical
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
        <MotoConceptIcon concept={definition.concept} size={20} />
        <span className="min-w-0 flex-1 truncate text-sm font-semibold text-gray-900">
          {definition.label}
        </span>
        <span className="shrink-0 text-xs text-gray-500">
          {SPAN_LABEL[placement.span]}
        </span>
      </span>
      {definition.kind === "section" && (
        <>
          <span className="line-clamp-2 text-xs text-gray-500">
            {definition.description}
          </span>
          {/* Angedeuteter Inhalt: ohne ihn wirkt die Kachel im Anpassen-Modus
              wie ein Loch statt wie die Karte, die dort später steht. */}
          <span
            className="mt-auto flex w-full flex-col gap-1.5"
            aria-hidden="true"
          >
            <span className="h-2 w-3/4 rounded-full bg-gray-200/80"></span>
            <span className="h-2 w-1/2 rounded-full bg-gray-200/80"></span>
            <span className="h-2 w-2/3 rounded-full bg-gray-200/80"></span>
          </span>
        </>
      )}
    </ChoiceTile>
  );
}

/**
 * Was mit der ausgewählten Kachel passieren kann — an einer Stelle, statt auf
 * jeder Karte. Ohne Auswahl steht hier, wie man eine trifft.
 */
function SelectionBar({
  definition,
  placement,
  index,
  total,
  onMove,
  onSpanChange,
  onRemove,
}: {
  readonly definition: HomeBlockDefinition | null;
  readonly placement: HomeBlockPlacement | undefined;
  readonly index: number;
  readonly total: number;
  readonly onMove: (from: number, to: number) => void;
  readonly onSpanChange: (key: HomeBlockKey, span: HomeBlockSpan) => void;
  readonly onRemove: (key: HomeBlockKey) => void;
}) {
  if (!definition || !placement) {
    return (
      <div className="moto-content-surface rounded-2xl border p-4 text-sm text-gray-600 shadow-lg">
        Eine Karte anklicken, um Breite und Platz zu ändern. Zum Umsortieren
        die Karte an ihren neuen Platz ziehen.
      </div>
    );
  }

  return (
    <div className="moto-content-surface flex flex-wrap items-center gap-3 rounded-2xl border p-4 shadow-lg">
      <span className="flex items-center gap-2">
        <MotoConceptIcon concept={definition.concept} size={20} />
        <span className="text-sm font-semibold text-gray-900">
          {definition.label}
        </span>
      </span>

      {definition.spans.length > 1 && (
        <span className="flex items-center gap-2">
          <span className="text-sm text-gray-600">Breite</span>
          <SegmentedControl
            ariaLabel={`Breite von ${definition.label}`}
            items={definition.spans.map((span) => ({
              value: String(span),
              label: SPAN_LABEL[span],
            }))}
            value={String(placement.span)}
            onChange={(next) =>
              onSpanChange(placement.key, Number(next) as HomeBlockSpan)
            }
          />
        </span>
      )}

      <span className="flex items-center gap-1">
        <Button
          type="button"
          variant="outline"
          size="md"
          className="gap-1"
          disabled={index === 0}
          onClick={() => onMove(index, index - 1)}
        >
          <ArrowLeft className="h-4 w-4" aria-hidden="true" />
          Nach vorne
        </Button>
        <Button
          type="button"
          variant="outline"
          size="md"
          className="gap-1"
          disabled={index === total - 1}
          onClick={() => onMove(index, index + 1)}
        >
          Nach hinten
          <ArrowRight className="h-4 w-4" aria-hidden="true" />
        </Button>
      </span>

      <Button
        type="button"
        variant="outline_danger"
        size="md"
        className="ml-auto gap-1"
        onClick={() => onRemove(placement.key)}
      >
        <Trash2 className="h-4 w-4" aria-hidden="true" />
        Entfernen
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
              onClick={() => onAdd(block.key)}
              className="flex items-start gap-3 p-3 text-left"
            >
              <span className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-gray-50">
                <MotoConceptIcon concept={block.concept} size={18} />
              </span>
              <span className="min-w-0 flex-1">
                <span className="block truncate text-sm font-medium text-gray-900">
                  {block.label}
                </span>
                <span className="block text-xs text-gray-500">
                  {block.description}
                </span>
              </span>
              <Plus
                className="mt-1 h-4 w-4 shrink-0 text-gray-400"
                aria-hidden="true"
              />
            </ChoiceTile>
          ))}
        </div>
      )}
    </SectionCard>
  );
}
