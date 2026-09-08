"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import { GripVertical, Plus, Trash2 } from "lucide-react";
import { motion, useReducedMotion } from "framer-motion";

import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EmptyState } from "~/components/ui/empty-state";
import { Modal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
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
 * Fläche wächst nicht mit dem Inhalt. Die Startseite bleibt so ein Einstieg
 * und wird keine lange Liste. Was nicht in eine Karte passt, wird dort
 * GEZÄHLT und verlinkt, nicht angeschnitten: eine halb sichtbare Zeile am
 * Kartenrand liest sich, als liefe der Baustein aus seiner Karte heraus.
 *
 * Auf einem Handy gilt das nicht: eine Spalte, keine feste Reihenhöhe, jede
 * Karte so hoch wie ihr Inhalt. Dort steht ohnehin nichts nebeneinander, und
 * gescrollt wird die Seite.
 *
 * Im Anpassen-Modus zeigt das Brett nicht die Inhalte, sondern die Anordnung:
 * jede Karte wird zu einer Platzhalter-Kachel mit Symbol, Name und Breite, an
 * derselben Stelle und in derselben Größe wie die echte Karte. Man ordnet
 * hier, man liest nicht.
 *
 * ANORDNEN HEISST ZIEHEN. Eine Kachel folgt dem Zeiger, sobald man sie ein
 * paar Pixel bewegt (am Handy: kurz halten, dann ziehen), und die anderen
 * rücken sofort beiseite — nicht erst beim Loslassen. Die Karte landet dort,
 * wo man sie sieht. Knöpfe „nach vorne / nach hinten" gibt es nicht mehr;
 * die Pfeiltasten verschieben eine ausgewählte Kachel für alle, die keine
 * Maus benutzen. Die Bewegung der anderen Kacheln animiert framer-motion
 * (`layout`), damit man sieht, was passiert, statt es zu raten.
 *
 * Ein Klick wählt die Kachel aus; alles Weitere (Breite, Entfernen) steht in
 * EINER Leiste über der Fläche, die beim Scrollen stehen bleibt. Rechts trägt
 * sie immer „Bausteine" und „Standardansicht wiederherstellen".
 */

// Auch das Handy hat zwei Spalten: eine Kennzahl ist eine Zahl mit einem Wort
// und braucht keine volle Bildschirmbreite — untereinander gestellt schiebt
// sie alles Wichtige unter den Rand. Listen nehmen dort beide Spalten.
const SPAN_CLASS: Record<HomeBlockSpan, string> = {
  1: "col-span-1",
  2: "col-span-2 xl:col-span-2",
  4: "col-span-2 xl:col-span-4",
};

const SPAN_LABEL: Record<HomeBlockSpan, string> = {
  1: "Schmal",
  2: "Breit",
  4: "Volle Breite",
};

/** So weit muss sich der Zeiger bewegen, bevor ein Klick zum Zug wird. */
const DRAG_THRESHOLD_PX = 6;
/** So lange hält man am Handy, bevor die Kachel dem Finger folgt. */
const TOUCH_HOLD_MS = 220;

/**
 * Eine Kennzahl ist eine Reihe hoch, eine Liste zwei — aber erst ab der
 * zweispaltigen Ansicht. Auf einem Handy gibt es keine Reihe, neben der eine
 * Karte stehen müsste: dort wächst jede Karte mit ihrem Inhalt und die Seite
 * scrollt. Eine feste Höhe würde dort nur Inhalt anschneiden.
 */
function rowClass(definition: HomeBlockDefinition | null): string {
  return definition?.kind === "tile" ? "sm:row-span-1" : "sm:row-span-2";
}

export interface HomeBoardProps {
  readonly placements: readonly HomeBlockPlacement[];
  readonly addable: readonly HomeBlockDefinition[];
  readonly editing: boolean;
  readonly onReorder: (from: number, to: number) => void;
  readonly onSpanChange: (key: HomeBlockKey, span: HomeBlockSpan) => void;
  readonly onRemove: (key: HomeBlockKey) => void;
  readonly onAdd: (key: HomeBlockKey) => void;
  /** Zurück zur Rollenansicht. Steht in der Leiste, weil sie immer sichtbar ist. */
  readonly onRestoreDefault: () => void;
  readonly restoring: boolean;
  readonly children: (placement: HomeBlockPlacement) => ReactNode;
}

/** Der laufende Zug, so wie das Brett ihn zeichnet. */
interface DragVisual {
  readonly key: HomeBlockKey;
  readonly x: number;
  readonly y: number;
}

/** Alles, was ein Zug zwischen zwei Ereignissen wissen muss. */
interface DragSession {
  key: HomeBlockKey;
  pointerId: number;
  element: HTMLElement;
  /** Zeiger beim Anfassen. */
  startX: number;
  startY: number;
  /** Lage der Kachel im Fluss beim Anfassen — unabhängig von Transformationen. */
  originLeft: number;
  originTop: number;
  /** Ab hier folgt die Kachel dem Zeiger. */
  active: boolean;
  holdTimer: ReturnType<typeof setTimeout> | null;
  preventTouchScroll: ((event: TouchEvent) => void) | null;
}

export function HomeBoard({
  placements,
  addable,
  editing,
  onReorder,
  onSpanChange,
  onRemove,
  onAdd,
  onRestoreDefault,
  restoring,
  children,
}: HomeBoardProps) {
  const reduceMotion = useReducedMotion();
  const [selectedKey, setSelectedKey] = useState<HomeBlockKey | null>(null);
  const [drag, setDrag] = useState<DragVisual | null>(null);
  const session = useRef<DragSession | null>(null);
  // Ein Zug, der lief, darf am Ende nicht auch noch als Klick gelten — sonst
  // wählt das Loslassen die Kachel aus, auf der man landet.
  const dragHappened = useRef(false);
  // Die aktuelle Reihenfolge für Ereignisse, die zwischen zwei Renderings
  // kommen: der Index der gezogenen Kachel wandert mit jedem Umsortieren.
  const order = useRef(placements);
  order.current = placements;

  const endDrag = useCallback(() => {
    const current = session.current;
    if (!current) return;
    if (current.holdTimer) clearTimeout(current.holdTimer);
    if (current.preventTouchScroll) {
      document.removeEventListener("touchmove", current.preventTouchScroll);
    }
    current.element.releasePointerCapture?.(current.pointerId);
    session.current = null;
    setDrag(null);
  }, []);

  // Beim Verlassen des Anpassen-Modus die Auswahl fallen lassen, sonst steht
  // beim nächsten Öffnen eine Kachel markiert, die niemand angeklickt hat.
  useEffect(() => {
    if (!editing) {
      setSelectedKey(null);
      endDrag();
    }
  }, [editing, endDrag]);

  useEffect(() => endDrag, [endDrag]);

  const activate = (current: DragSession) => {
    current.active = true;
    dragHappened.current = true;
    current.element.setPointerCapture?.(current.pointerId);
    setDrag({ key: current.key, x: 0, y: 0 });
  };

  const beginDrag = (
    event: ReactPointerEvent<HTMLLIElement>,
    key: HomeBlockKey,
  ) => {
    if (!editing || session.current) return;
    if (event.pointerType === "mouse" && event.button !== 0) return;
    const element = event.currentTarget;
    const current: DragSession = {
      key,
      pointerId: event.pointerId,
      element,
      startX: event.clientX,
      startY: event.clientY,
      originLeft: element.offsetLeft,
      originTop: element.offsetTop,
      active: false,
      holdTimer: null,
      preventTouchScroll: null,
    };
    session.current = current;
    dragHappened.current = false;

    if (event.pointerType === "touch") {
      // Am Handy entscheidet die Wartezeit, ob der Finger scrollt oder zieht:
      // wer still hält, zieht; wer gleich wischt, scrollt. Erst ab dann darf
      // die Seite nicht mehr mitscrollen — das Ereignis muss nicht-passiv
      // abgefangen werden, sonst zieht der Browser die Seite unter der
      // Kachel weg.
      current.holdTimer = setTimeout(() => {
        current.holdTimer = null;
        current.preventTouchScroll = (touch: TouchEvent) =>
          touch.preventDefault();
        document.addEventListener("touchmove", current.preventTouchScroll, {
          passive: false,
        });
        activate(current);
      }, TOUCH_HOLD_MS);
    }
  };

  const moveDrag = (event: ReactPointerEvent<HTMLLIElement>) => {
    const current = session.current;
    if (!current || current.pointerId !== event.pointerId) return;
    const dx = event.clientX - current.startX;
    const dy = event.clientY - current.startY;

    if (!current.active) {
      const far = Math.hypot(dx, dy) > DRAG_THRESHOLD_PX;
      if (!far) return;
      if (current.holdTimer) {
        // Der Finger wischt, bevor die Wartezeit um ist: das ist ein Scrollen.
        clearTimeout(current.holdTimer);
        session.current = null;
        return;
      }
      activate(current);
    }

    // Die Kachel folgt dem Zeiger. Rückt sie beim Umsortieren im Fluss an
    // eine andere Stelle, gleicht der Versatz das aus, damit sie unter dem
    // Zeiger bleibt statt zu springen.
    const element = current.element;
    setDrag({
      key: current.key,
      x: dx + (current.originLeft - element.offsetLeft),
      y: dy + (current.originTop - element.offsetTop),
    });

    // Die gezogene Kachel lässt Zeigerereignisse durch; darunter liegt die
    // Kachel, deren Platz sie einnehmen soll. Sobald der Zeiger über einer
    // anderen steht, rückt diese beiseite — nicht erst beim Loslassen.
    const under = document.elementFromPoint(event.clientX, event.clientY);
    const target = under?.closest<HTMLElement>("[data-block-key]");
    const overKey = target?.dataset.blockKey as HomeBlockKey | undefined;
    if (!overKey || overKey === current.key) return;
    const from = order.current.findIndex((entry) => entry.key === current.key);
    const to = order.current.findIndex((entry) => entry.key === overKey);
    if (from >= 0 && to >= 0 && from !== to) onReorder(from, to);
  };

  const finishDrag = (event: ReactPointerEvent<HTMLLIElement>) => {
    const current = session.current;
    if (!current || current.pointerId !== event.pointerId) return;
    endDrag();
  };

  const moveByKeyboard = (key: HomeBlockKey, delta: -1 | 1) => {
    const from = placements.findIndex((entry) => entry.key === key);
    const to = from + delta;
    if (from < 0 || to < 0 || to >= placements.length) return;
    onReorder(from, to);
  };

  const selectedIndex = placements.findIndex(
    (placement) => placement.key === selectedKey,
  );
  const selected = selectedIndex >= 0 ? placements[selectedIndex] : undefined;
  const selectedDefinition = selected
    ? homeBlockDefinition(selected.key)
    : null;

  const settle = reduceMotion
    ? { duration: 0 }
    : { type: "spring" as const, stiffness: 520, damping: 42, mass: 0.8 };

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
            onSpanChange={onSpanChange}
            onRemove={(key) => {
              setSelectedKey(null);
              onRemove(key);
            }}
            onRestoreDefault={onRestoreDefault}
            restoring={restoring}
            addable={addable}
            onAdd={onAdd}
          />
        </div>
      )}

      <ul
        data-testid="home-board"
        // `grid-flow-row-dense` füllt Lücken: steht eine schmale Kennzahl
        // hinter einer breiten Karte, rutscht sie in das freie Feld davor,
        // statt eine halbe Reihe leer zu lassen — dasselbe Verhalten wie auf
        // einem Startbildschirm mit gemischten Kachelgrößen.
        className="grid grid-cols-2 gap-4 sm:grid-flow-row-dense sm:auto-rows-[7rem] xl:grid-cols-4"
      >
        {placements.map((placement, index) => {
          const definition = homeBlockDefinition(placement.key);
          const dragging = drag?.key === placement.key;
          return (
            <motion.li
              key={placement.key}
              // `layout` lässt jede Kachel an ihren neuen Platz gleiten, wenn
              // sich Reihenfolge oder Breite ändern. Die gezogene Kachel
              // selbst springt ohne Übergang an ihre Flussposition — den
              // sichtbaren Weg legt sie unter dem Zeiger zurück.
              layout={editing}
              transition={dragging ? { duration: 0 } : settle}
              style={
                dragging
                  ? {
                      x: drag.x,
                      y: drag.y,
                      zIndex: 10,
                      pointerEvents: "none",
                      position: "relative",
                    }
                  : undefined
              }
              data-testid={`home-block-${placement.key}`}
              data-block-key={placement.key}
              data-span={placement.span}
              className={`${SPAN_CLASS[placement.span]} ${rowClass(definition)} min-h-0 ${
                editing ? "touch-pan-y select-none" : ""
              }`}
              // Zeigerereignisse statt HTML5-Ziehen: das native Ziehen kennt
              // kein Tablet und lässt sich nicht testen.
              onPointerDown={(event) => beginDrag(event, placement.key)}
              onPointerMove={moveDrag}
              onPointerUp={finishDrag}
              onPointerCancel={finishDrag}
            >
              {editing && definition ? (
                <ArrangeTile
                  definition={definition}
                  placement={placement}
                  position={index + 1}
                  total={placements.length}
                  selected={placement.key === selectedKey}
                  dragging={dragging}
                  onSelect={() => {
                    // Nach einem Zug ist der Klick nur das Loslassen.
                    if (dragHappened.current) return;
                    setSelectedKey(placement.key);
                  }}
                  onMove={(delta) => moveByKeyboard(placement.key, delta)}
                />
              ) : (
                <div className="h-full min-h-0">{children(placement)}</div>
              )}
            </motion.li>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * Die Kachel im Anpassen-Modus: Symbol, Name, Breite und der Platz in der
 * Reihenfolge. Sie ist ein Knopf, damit sie sich auch mit der Tastatur
 * auswählen und mit den Pfeiltasten verschieben lässt — Ziehen allein wäre
 * nicht bedienbar.
 */
function ArrangeTile({
  definition,
  placement,
  position,
  total,
  selected,
  dragging,
  onSelect,
  onMove,
}: {
  readonly definition: HomeBlockDefinition;
  readonly placement: HomeBlockPlacement;
  readonly position: number;
  readonly total: number;
  readonly selected: boolean;
  readonly dragging: boolean;
  readonly onSelect: () => void;
  readonly onMove: (delta: -1 | 1) => void;
}) {
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === "ArrowLeft" || event.key === "ArrowUp") {
      event.preventDefault();
      onMove(-1);
    } else if (event.key === "ArrowRight" || event.key === "ArrowDown") {
      event.preventDefault();
      onMove(1);
    }
  };

  return (
    <ChoiceTile
      as="button"
      selected={selected}
      tone={selected ? "green" : "gray"}
      onClick={onSelect}
      onKeyDown={onKeyDown}
      aria-pressed={selected}
      aria-label={`${definition.label} auswählen, Platz ${position} von ${total}, ${SPAN_LABEL[placement.span]}. Pfeiltasten verschieben.`}
      className={`flex h-full w-full flex-col items-start gap-2 p-4 text-left ${
        dragging
          ? "scale-[1.02] cursor-grabbing shadow-xl"
          : "cursor-grab transition-shadow"
      }`}
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
        {/* Auf dem Handy teilen sich zwei Kacheln eine Zeile: dort trägt der
            Name die Kachel, die Breite steht in der Leiste. */}
        <span className="hidden shrink-0 text-xs text-gray-500 sm:inline">
          {SPAN_LABEL[placement.span]}
        </span>
      </span>
      {definition.kind === "section" && (
        <>
          {/* Auf dem Handy ist die Kachel schmal: zwei Zeilen schneiden den
              Satz mitten durch, drei tragen ihn. */}
          <span className="line-clamp-3 text-xs text-gray-500 sm:line-clamp-2">
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
 * jeder Karte: die Breite und das Entfernen. Verschoben wird durch Ziehen,
 * nicht hier. Ohne Auswahl steht hier, wie man eine trifft.
 */
function SelectionBar({
  definition,
  placement,
  onSpanChange,
  onRemove,
  onRestoreDefault,
  restoring,
  addable,
  onAdd,
}: {
  readonly definition: HomeBlockDefinition | null;
  readonly placement: HomeBlockPlacement | undefined;
  readonly onSpanChange: (key: HomeBlockKey, span: HomeBlockSpan) => void;
  readonly onRemove: (key: HomeBlockKey) => void;
  readonly onRestoreDefault: () => void;
  readonly restoring: boolean;
  readonly addable: readonly HomeBlockDefinition[];
  readonly onAdd: (key: HomeBlockKey) => void;
}) {
  const [adding, setAdding] = useState(false);

  return (
    <div className="moto-content-surface flex flex-wrap items-center gap-3 rounded-2xl border p-4 shadow-lg">
      {definition && placement ? (
        <>
          <span className="flex items-center gap-2">
            <MotoConceptIcon concept={definition.concept} size={20} />
            <span className="text-sm font-semibold text-gray-900">
              {definition.label}
            </span>
          </span>

          {definition.spans.length > 1 ? (
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
          ) : (
            // Eine Kennzahl hat genau eine Breite. Das steht da, damit
            // niemand den fehlenden Regler für einen Fehler hält.
            <span className="text-sm text-gray-500">
              Kennzahlen sind immer schmal.
            </span>
          )}

          <Button
            type="button"
            variant="outline_danger"
            size="md"
            className="gap-1"
            onClick={() => onRemove(placement.key)}
          >
            <Trash2 className="h-4 w-4" aria-hidden="true" />
            Entfernen
          </Button>
        </>
      ) : (
        <span className="min-w-0 flex-1 text-sm text-gray-600">
          <span className="sm:hidden">
            Karte halten und ziehen, um sie zu verschieben.
          </span>
          <span className="hidden sm:inline">
            Karte ziehen, um sie zu verschieben.
          </span>{" "}
          Anklicken, um die Breite zu ändern.
        </span>
      )}

      {/* Rechts stehen die beiden Aktionen, die immer gehen — auch ohne
          ausgewählte Karte. „Bausteine" ist der Weg zu allem, was man noch
          hinzufügen kann; vorher lag diese Liste unter der Fläche und damit
          außerhalb des Bildes. */}
      <span className="flex w-full flex-wrap items-center gap-1 sm:ml-auto sm:w-auto sm:flex-nowrap">
        <Button
          type="button"
          variant="outline"
          size="md"
          className="gap-1"
          onClick={() => setAdding(true)}
        >
          <Plus className="h-4 w-4" aria-hidden="true" />
          Bausteine
        </Button>
        <Button
          type="button"
          variant="ghost"
          size="md"
          className="whitespace-nowrap"
          disabled={restoring}
          onClick={onRestoreDefault}
        >
          Standardansicht wiederherstellen
        </Button>
      </span>

      <AddBlockModal
        open={adding}
        onClose={() => setAdding(false)}
        addable={addable}
        onAdd={(key) => {
          onAdd(key);
          setAdding(false);
        }}
      />
    </div>
  );
}

/**
 * Die Auswahl der Bausteine: alles, was diese Person sehen darf und gerade
 * nicht auf ihrer Startseite hat. Als Dialog, damit sie von jeder Stelle der
 * Fläche aus erreichbar ist.
 */
function AddBlockModal({
  open,
  onClose,
  addable,
  onAdd,
}: {
  readonly open: boolean;
  readonly onClose: () => void;
  readonly addable: readonly HomeBlockDefinition[];
  readonly onAdd: (key: HomeBlockKey) => void;
}) {
  return (
    <Modal
      isOpen={open}
      onClose={onClose}
      title="Baustein hinzufügen"
      widthClass="mx-4 w-[calc(100%-2rem)] max-w-3xl"
    >
      {addable.length === 0 ? (
        <EmptyState
          title="Sie haben schon alles"
          description="Auf Ihrer Startseite steht jeder Baustein, der für Sie verfügbar ist."
        />
      ) : (
        <div className="space-y-3">
          <p className="text-sm text-gray-600">
            Der Baustein erscheint am Ende Ihrer Startseite. Von dort ziehen Sie
            ihn an seinen Platz.
          </p>
          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
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
        </div>
      )}
    </Modal>
  );
}
