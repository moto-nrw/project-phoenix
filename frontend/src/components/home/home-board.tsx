"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent,
  type PointerEvent as ReactPointerEvent,
  type ReactNode,
} from "react";
import { GripVertical, Plus, Trash2 } from "lucide-react";
import {
  animate,
  motion,
  useMotionValue,
  useReducedMotion,
  type Transition,
} from "framer-motion";

import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import { EmptyState } from "~/components/ui/empty-state";
import { Modal } from "~/components/ui/modal";
import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import { SegmentedControl } from "~/components/ui/segmented-control";
import {
  HOME_BOARD_COLUMNS,
  computeBoardCells,
  homeBlockDefinition,
  rowsOf,
  type HomeBlockDefinition,
  type HomeBlockKey,
  type HomeBlockPlacement,
  type HomeBlockSpan,
  type HomeBoardCell,
  type HomeMoveTarget,
} from "~/lib/home-blocks";
import { BELOW_SM, useMediaQuery } from "~/lib/hooks/use-media-query";

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
 * ANORDNEN HEISST ZIEHEN, wie auf einem Startbildschirm. Eine Kachel folgt
 * dem Zeiger, sobald man sie ein paar Pixel bewegt (am Handy: kurz halten,
 * dann ziehen), und die anderen gleiten sofort beiseite — nicht erst beim
 * Loslassen. Beim Loslassen federt die Kachel an ihren Platz.
 *
 * Drei Entscheidungen machen das ruhig statt zappelig:
 *
 * 1. Wo eine Kachel HINGEHÖRT, entscheidet die Geometrie des Rasters
 *    (offsetLeft/offsetTop der Zellen), nicht das Element unter dem Zeiger.
 *    Die Zellen bewegen sich während der Animation per Transform, ihre
 *    Rasterposition steht sofort fest — sonst tauscht die Kachel unter dem
 *    Zeiger im Sekundentakt hin und her, während sie noch gleitet.
 * 2. Die gezogene Kachel bewegt sich über Motion-Werte, ohne Rendern je
 *    Zeigerereignis. Rückt sie beim Umsortieren im Fluss an eine andere
 *    Zelle, gleicht der Versatz das aus, damit sie unter dem Zeiger bleibt.
 * 3. Das Raster füllt im Anpassen-Modus keine Lücken (kein dense): die
 *    Reihenfolge, die man sieht, ist die, die man baut.
 *
 * Knöpfe „nach vorne / nach hinten" gibt es nicht mehr; die Pfeiltasten
 * verschieben eine Kachel für alle, die keine Maus benutzen. Ein Klick wählt
 * die Kachel aus; alles Weitere (Breite, Entfernen) steht in EINER Leiste über
 * der Fläche, die beim Scrollen stehen bleibt.
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

/** Die Feder, mit der Kacheln an ihren Platz gleiten. */
const SETTLE: Transition = {
  type: "spring",
  stiffness: 380,
  damping: 36,
  mass: 0.9,
};
const INSTANT: Transition = { duration: 0 };

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
  /** Versetzt einen Baustein in eine Reihe oder in eine neue Reihe. */
  readonly onMove: (key: HomeBlockKey, target: HomeMoveTarget) => void;
  readonly onSpanChange: (key: HomeBlockKey, span: HomeBlockSpan) => void;
  readonly onRemove: (key: HomeBlockKey) => void;
  readonly onAdd: (key: HomeBlockKey) => void;
  /** Zurück zur Rollenansicht. Steht in der Leiste, weil sie immer sichtbar ist. */
  readonly onRestoreDefault: () => void;
  readonly restoring: boolean;
  readonly children: (placement: HomeBlockPlacement) => ReactNode;
}

/** Was das Brett an einer Kachel bewegen kann, ohne sie neu zu rendern. */
interface ItemHandle {
  readonly element: HTMLLIElement | null;
  readonly set: (x: number, y: number) => void;
  readonly settle: () => void;
}

type MoveDirection = "left" | "right" | "up" | "down";

const ARROW_DIRECTION: Readonly<Record<string, MoveDirection>> = {
  ArrowLeft: "left",
  ArrowRight: "right",
  ArrowUp: "up",
  ArrowDown: "down",
};

/** Alles, was ein Zug zwischen zwei Ereignissen wissen muss. */
interface DragSession {
  key: HomeBlockKey;
  pointerId: number;
  element: HTMLLIElement;
  /** Zeiger beim Anfassen. */
  startX: number;
  startY: number;
  /** Zelle der Kachel im Raster beim Anfassen — unabhängig von Transformationen. */
  originLeft: number;
  originTop: number;
  /** Ab hier folgt die Kachel dem Zeiger. */
  active: boolean;
  /** Letzte Zeigerposition — für den Versatz nach einem Umsortieren. */
  lastX: number;
  lastY: number;
  holdTimer: ReturnType<typeof setTimeout> | null;
  preventTouchScroll: ((event: TouchEvent) => void) | null;
  /** Löst die Fenster-Listener des laufenden Zugs. */
  detach: (() => void) | null;
  /**
   * Das Ereignis, das den Zug ausgelöst hat. Es erreicht danach auch das
   * Fenster, dessen Listener gerade erst dazukam — ein zweites Mal gezählt,
   * würde es die Kachel sofort wieder zurücksortieren.
   */
  activatedBy: Event | null;
}

export function HomeBoard({
  placements,
  addable,
  editing,
  onMove,
  onSpanChange,
  onRemove,
  onAdd,
  onRestoreDefault,
  restoring,
  children,
}: HomeBoardProps) {
  const reduceMotion = useReducedMotion() === true;
  const [selectedKey, setSelectedKey] = useState<HomeBlockKey | null>(null);
  const [draggedKey, setDraggedKey] = useState<HomeBlockKey | null>(null);
  const grid = useRef<HTMLUListElement>(null);
  const handles = useRef(new Map<HomeBlockKey, ItemHandle>());
  const session = useRef<DragSession | null>(null);
  // Ein Zug, der lief, darf am Ende nicht auch noch als Klick gelten — sonst
  // wählt das Loslassen die Kachel aus, auf der man landet.
  const dragHappened = useRef(false);
  // Die aktuelle Reihenfolge für Ereignisse zwischen zwei Renderings: der
  // Index der gezogenen Kachel wandert mit jedem Umsortieren.
  const order = useRef(placements);
  order.current = placements;

  const register = useCallback(
    (key: HomeBlockKey, handle: ItemHandle | null) => {
      if (handle) handles.current.set(key, handle);
      else handles.current.delete(key);
    },
    [],
  );

  const endDrag = useCallback(() => {
    const current = session.current;
    if (!current) return;
    if (current.holdTimer) clearTimeout(current.holdTimer);
    if (current.preventTouchScroll) {
      document.removeEventListener("touchmove", current.preventTouchScroll);
    }
    current.detach?.();
    if (current.element.hasPointerCapture?.(current.pointerId)) {
      current.element.releasePointerCapture(current.pointerId);
    }
    if (current.active) handles.current.get(current.key)?.settle();
    session.current = null;
    setDraggedKey(null);
    // Der Klick, der zum Loslassen gehört, kommt nach diesem Ereignis; erst
    // danach darf ein Klick wieder auswählen (auch einer per Tastatur).
    setTimeout(() => {
      dragHappened.current = false;
    }, 0);
  }, []);

  // Beim Verlassen des Anpassen-Modus die Auswahl fallen lassen, sonst steht
  // beim nächsten Öffnen eine Kachel markiert, die niemand angeklickt hat.
  useEffect(() => {
    if (!editing) {
      setSelectedKey(null);
      endDrag();
    }
  }, [editing, endDrag]);

  // Verliert das Fenster den Fokus oder kommt das Loslassen nie an, bleibt
  // keine Kachel am Zeiger kleben.
  useEffect(() => {
    window.addEventListener("blur", endDrag);
    return () => {
      window.removeEventListener("blur", endDrag);
      endDrag();
    };
  }, [endDrag]);

  /**
   * Wohin die gezogene Kachel gehört — nach der Geometrie des Rasters, nicht
   * nach dem Element, das gerade darunter durchgleitet.
   *
   * Drei Fälle, in dieser Reihenfolge: der Zeiger steht auf einer anderen
   * Kachel (deren Platz in deren Reihe), im freien Rest einer Reihe (ans Ende
   * dieser Reihe), oder in der Lücke zwischen zwei Reihen beziehungsweise
   * über der ersten und unter der letzten (eine neue Reihe dort). So bekommt
   * auch eine schmale Kennzahl eine Reihe für sich, wenn man sie unter eine
   * Karte legt, statt dass das Raster sie daneben spült.
   */
  const targetUnderPointer = (
    current: DragSession,
    clientX: number,
    clientY: number,
  ): HomeMoveTarget | null => {
    const container = grid.current;
    if (!container) return null;
    const bounds = container.getBoundingClientRect();
    const px = clientX - bounds.left;
    const py = clientY - bounds.top;
    const cellOf = (key: HomeBlockKey) =>
      container.querySelector<HTMLElement>(`[data-block-key="${key}"]`);

    const bands = rowsOf(order.current).map((entries, row) => {
      let top = Number.POSITIVE_INFINITY;
      let bottom = Number.NEGATIVE_INFINITY;
      for (const entry of entries) {
        const cell = cellOf(entry.key);
        if (!cell) continue;
        top = Math.min(top, cell.offsetTop);
        bottom = Math.max(bottom, cell.offsetTop + cell.offsetHeight);
      }
      return { row, entries, top, bottom };
    });

    const draggedSpan =
      order.current.find((entry) => entry.key === current.key)?.span ?? 1;
    // Hat eine Reihe keinen Platz mehr für die gezogene Kachel, meint der
    // Zeiger über ihr die Reihe DAVOR oder DANACH — je nachdem, ob er in der
    // oberen oder unteren Hälfte steht. So legt man eine Kennzahl unter eine
    // volle Reihe, ohne die schmale Lücke treffen zu müssen.
    const roomIn = (band: (typeof bands)[number]) =>
      HOME_BOARD_COLUMNS -
      band.entries
        .filter((entry) => entry.key !== current.key)
        .reduce((sum, entry) => sum + entry.span, 0);

    for (const band of bands) {
      for (const [index, entry] of band.entries.entries()) {
        const cell = cellOf(entry.key);
        if (!cell) continue;
        if (
          px >= cell.offsetLeft &&
          px <= cell.offsetLeft + cell.offsetWidth &&
          py >= cell.offsetTop &&
          py <= cell.offsetTop + cell.offsetHeight
        ) {
          // Die eigene Zelle: da ist sie schon.
          if (entry.key === current.key) return null;
          if (roomIn(band) < draggedSpan) {
            const below = py > (band.top + band.bottom) / 2;
            return { kind: "newRow", before: below ? band.row + 1 : band.row };
          }
          return { kind: "into", row: band.row, index };
        }
      }
    }

    // Die Lücke zwischen zwei Reihen ist schmal; ein Rand innerhalb der
    // Reihe zählt mit dazu, damit man sie mit dem Zeiger trifft.
    const edge = 10;
    for (const band of bands) {
      if (py < band.top + edge || py > band.bottom - edge) continue;
      const first = cellOf(band.entries[0]!.key);
      const last = cellOf(band.entries[band.entries.length - 1]!.key);
      if (last && px > last.offsetLeft + last.offsetWidth) {
        return { kind: "into", row: band.row, index: band.entries.length };
      }
      if (first && px < first.offsetLeft) {
        return { kind: "into", row: band.row, index: 0 };
      }
      return null;
    }

    for (const band of bands) {
      if (py < band.top + edge) return { kind: "newRow", before: band.row };
    }
    return { kind: "newRow", before: bands.length };
  };

  /**
   * Die Kachel folgt dem Zeiger. Rückt sie beim Umsortieren im Raster in eine
   * andere Zelle, gleicht der Versatz das aus, damit sie unter dem Zeiger
   * bleibt statt zu springen.
   */
  const place = (current: DragSession) => {
    const element = current.element;
    handles.current
      .get(current.key)
      ?.set(
        current.lastX -
          current.startX +
          (current.originLeft - element.offsetLeft),
        current.lastY -
          current.startY +
          (current.originTop - element.offsetTop),
      );
  };

  const follow = (current: DragSession, clientX: number, clientY: number) => {
    current.lastX = clientX;
    current.lastY = clientY;
    place(current);

    const target = targetUnderPointer(current, clientX, clientY);
    if (target) onMove(current.key, target);
  };

  /**
   * Ab hier folgt die Kachel dem Zeiger. Bewegen und Loslassen hört das
   * FENSTER, nicht die Kachel: beim Umsortieren wandert ihr Knoten im DOM,
   * und ein Knoten, der den Platz wechselt, verliert die Zeigererfassung —
   * das Loslassen käme dann nie mehr bei ihr an, und die Kachel bliebe am
   * Zeiger kleben.
   */
  const activate = (current: DragSession) => {
    current.active = true;
    dragHappened.current = true;
    current.element.setPointerCapture?.(current.pointerId);
    const onMove = (event: PointerEvent) => {
      if (event.pointerId !== current.pointerId) return;
      if (event === current.activatedBy) return;
      follow(current, event.clientX, event.clientY);
    };
    const onUp = (event: PointerEvent) => {
      if (event.pointerId !== current.pointerId) return;
      endDrag();
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
    window.addEventListener("pointercancel", onUp);
    current.detach = () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
      window.removeEventListener("pointercancel", onUp);
    };
    setDraggedKey(current.key);
  };

  // Nach einem Umsortieren steht die gezogene Kachel im Raster in einer
  // anderen Zelle. Der Versatz wird noch im selben Bild nachgezogen, nicht
  // erst beim nächsten Zeigerereignis — sonst springt sie für ein Bild.
  useLayoutEffect(() => {
    const current = session.current;
    if (current?.active) place(current);
  });

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
      lastX: event.clientX,
      lastY: event.clientY,
      holdTimer: null,
      preventTouchScroll: null,
      detach: null,
      activatedBy: null,
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

  /**
   * Vor dem Zug hört die Kachel selbst: erst ab der Bewegungsschwelle wird
   * aus dem Klick ein Zug, und ab dann übernimmt das Fenster.
   */
  const moveDrag = (event: ReactPointerEvent<HTMLLIElement>) => {
    const current = session.current;
    if (!current || current.active || current.pointerId !== event.pointerId) {
      return;
    }
    const dx = event.clientX - current.startX;
    const dy = event.clientY - current.startY;
    if (Math.hypot(dx, dy) <= DRAG_THRESHOLD_PX) return;
    if (current.holdTimer) {
      // Der Finger wischt, bevor die Wartezeit um ist: das ist ein Scrollen.
      clearTimeout(current.holdTimer);
      session.current = null;
      return;
    }
    current.activatedBy = event.nativeEvent;
    activate(current);
    follow(current, event.clientX, event.clientY);
  };

  const finishDrag = (event: ReactPointerEvent<HTMLLIElement>) => {
    const current = session.current;
    if (!current || current.pointerId !== event.pointerId) return;
    endDrag();
  };

  /**
   * Ohne Maus: links und rechts wandern durch die Reihe und an deren Rand in
   * die Nachbarreihe; oben und unten geben der Kachel eine eigene Reihe
   * darüber oder darunter (oder rücken ihre eigene Reihe).
   */
  const moveByKeyboard = (key: HomeBlockKey, direction: MoveDirection) => {
    const rows = rowsOf(placements);
    const row = rows.findIndex((entries) =>
      entries.some((entry) => entry.key === key),
    );
    if (row < 0) return;
    const entries = rows[row]!;
    const index = entries.findIndex((entry) => entry.key === key);
    const alone = entries.length === 1;
    switch (direction) {
      case "left":
        if (index > 0) onMove(key, { kind: "into", row, index: index - 1 });
        else if (row > 0) {
          onMove(key, {
            kind: "into",
            row: row - 1,
            index: rows[row - 1]!.length,
          });
        }
        return;
      case "right":
        if (index < entries.length - 1) {
          onMove(key, { kind: "into", row, index: index + 1 });
        } else if (row < rows.length - 1) {
          onMove(key, { kind: "into", row: row + 1, index: 0 });
        }
        return;
      case "up":
        if (alone && row === 0) return;
        onMove(key, { kind: "newRow", before: alone ? row - 1 : row });
        return;
      case "down":
        if (alone && row === rows.length - 1) return;
        onMove(key, { kind: "newRow", before: alone ? row + 2 : row + 1 });
        return;
    }
  };

  // Ab der zweispaltigen Ansicht zeichnet das Brett die Reihen ausdrücklich;
  // vier Spalten gibt es erst auf einem breiten Bildschirm.
  const flow = useMediaQuery(BELOW_SM);
  const wide = useMediaQuery("(min-width: 1280px)");
  const cells = computeBoardCells(placements, wide ? 4 : 2);

  const selectedIndex = placements.findIndex(
    (placement) => placement.key === selectedKey,
  );
  const selected = selectedIndex >= 0 ? placements[selectedIndex] : undefined;
  const selectedDefinition = selected
    ? homeBlockDefinition(selected.key)
    : null;

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
        ref={grid}
        data-testid="home-board"
        // Jede Zelle bekommt ihren Platz ausdrücklich zugewiesen (Spalte und
        // Rasterzeile aus den Reihen der Person); das Raster verteilt nichts
        // selbst und füllt keine Lücken. Nur auf dem Handy fließen die
        // Kacheln in Reihenfolge, dort gibt es keine Reihe, neben der etwas
        // stehen müsste. `relative`, damit die Zellen ihre Lage relativ zum
        // Raster kennen.
        className="relative grid grid-cols-2 gap-4 sm:auto-rows-[7rem] xl:grid-cols-4"
      >
        {placements.map((placement, index) => {
          const definition = homeBlockDefinition(placement.key);
          const dragging = draggedKey === placement.key;
          return (
            <BoardItem
              key={placement.key}
              placement={placement}
              definition={definition}
              cell={flow ? null : (cells.get(placement.key) ?? null)}
              editing={editing}
              dragging={dragging}
              reduceMotion={reduceMotion}
              register={register}
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
                  onMove={(direction) =>
                    moveByKeyboard(placement.key, direction)
                  }
                />
              ) : (
                <div className="h-full min-h-0">{children(placement)}</div>
              )}
            </BoardItem>
          );
        })}
      </ul>
    </div>
  );
}

/**
 * Eine Zelle des Bretts. Sie besitzt die Motion-Werte, mit denen das Brett
 * sie beim Ziehen bewegt, ohne sie je Zeigerereignis neu zu rendern; beim
 * Loslassen federn die Werte auf null zurück, und `layout` lässt die übrigen
 * Zellen an ihren neuen Platz gleiten.
 */
function BoardItem({
  placement,
  definition,
  cell,
  editing,
  dragging,
  reduceMotion,
  register,
  onPointerDown,
  onPointerMove,
  onPointerUp,
  onPointerCancel,
  children,
}: {
  readonly placement: HomeBlockPlacement;
  readonly definition: HomeBlockDefinition | null;
  /** Die ausdrückliche Rasterzelle; `null` lässt die Kachel fließen (Handy). */
  readonly cell: HomeBoardCell | null;
  readonly editing: boolean;
  readonly dragging: boolean;
  readonly reduceMotion: boolean;
  readonly register: (key: HomeBlockKey, handle: ItemHandle | null) => void;
  readonly onPointerDown: (event: ReactPointerEvent<HTMLLIElement>) => void;
  readonly onPointerMove: (event: ReactPointerEvent<HTMLLIElement>) => void;
  readonly onPointerUp: (event: ReactPointerEvent<HTMLLIElement>) => void;
  readonly onPointerCancel: (event: ReactPointerEvent<HTMLLIElement>) => void;
  readonly children: ReactNode;
}) {
  const ref = useRef<HTMLLIElement>(null);
  const x = useMotionValue(0);
  const y = useMotionValue(0);

  useEffect(() => {
    register(placement.key, {
      get element() {
        return ref.current;
      },
      set: (nextX, nextY) => {
        x.set(nextX);
        y.set(nextY);
      },
      settle: () => {
        if (reduceMotion) {
          x.set(0);
          y.set(0);
          return;
        }
        animate(x, 0, SETTLE);
        animate(y, 0, SETTLE);
      },
    });
    return () => register(placement.key, null);
  }, [placement.key, register, reduceMotion, x, y]);

  return (
    <motion.li
      ref={ref}
      // `layout` lässt jede Zelle an ihren neuen Platz gleiten, wenn sich
      // Reihenfolge oder Breite ändern. Die gezogene Zelle selbst springt
      // ohne Übergang in ihre Rasterzelle — den sichtbaren Weg legt sie
      // unter dem Zeiger zurück, und beim Loslassen federt sie zurück.
      layout={editing}
      transition={dragging || reduceMotion ? INSTANT : SETTLE}
      style={{
        x,
        y,
        position: "relative",
        zIndex: dragging ? 10 : undefined,
        ...(cell
          ? {
              gridColumn: `${cell.columnStart} / span ${cell.columnSpan}`,
              gridRow: `${cell.rowStart} / span ${cell.rowSpan}`,
            }
          : {}),
      }}
      data-testid={`home-block-${placement.key}`}
      data-block-key={placement.key}
      data-span={placement.span}
      data-row={placement.row}
      className={`min-h-0 ${cell ? "" : `${SPAN_CLASS[placement.span]} ${rowClass(definition)}`} ${
        editing ? "touch-pan-y select-none" : ""
      }`}
      // Zeigerereignisse statt HTML5-Ziehen: das native Ziehen kennt kein
      // Tablet und lässt sich nicht testen.
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerCancel}
    >
      {children}
    </motion.li>
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
  readonly onMove: (direction: MoveDirection) => void;
}) {
  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    const direction = ARROW_DIRECTION[event.key];
    if (!direction) return;
    event.preventDefault();
    onMove(direction);
  };

  return (
    <ChoiceTile
      as="button"
      selected={selected}
      tone={selected ? "green" : "gray"}
      onClick={onSelect}
      onKeyDown={onKeyDown}
      aria-pressed={selected}
      aria-label={`${definition.label} auswählen, Platz ${position} von ${total}, ${SPAN_LABEL[placement.span]}. Pfeiltasten verschieben: links und rechts in der Reihe, oben und unten in eine eigene Reihe.`}
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
