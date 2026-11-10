"use client";

import {
  lazy,
  Suspense,
  type ComponentType,
  type CSSProperties,
  type ReactNode,
} from "react";

import {
  HOME_BOARD_COLUMNS,
  computeBoardCells,
  type HomeBlockDefinition,
  type HomeBlockKey,
  type HomeBlockPlacement,
  type HomeBlockSpan,
  type HomeMoveDirection,
  type HomeMoveTarget,
} from "~/lib/home-blocks";

import {
  BOARD_GRID_CLASSES,
  CELL_CLASSES,
  NARROW_BOARD_COLUMNS,
  SPAN_CLASS,
  cellVariables,
} from "./home-board-cells";
import type { HomeBoardEditorProps } from "./home-board-editor";

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
 * Gelesen wird die Startseite bei jedem Aufruf, angepasst selten. Die
 * Leseansicht ist deshalb ein schlichtes Raster ohne Animationsbibliothek;
 * das Anpassen mit Ziehen, Federn und Leiste steckt in `home-board-editor`
 * und wird erst geladen, wenn jemand auf „Anpassen“ zeigt oder es öffnet
 * (#6469).
 */

export interface HomeBoardProps {
  readonly placements: readonly HomeBlockPlacement[];
  readonly addable: readonly HomeBlockDefinition[];
  /** Bausteine, die die Schule für alle Personen verlangt. */
  readonly requiredKeys: ReadonlySet<HomeBlockKey>;
  readonly editing: boolean;
  /**
   * Legt einen Baustein in eine Zelle. `base` ist die Anordnung, von der aus
   * gerechnet wird: beim Ziehen die vom Anfassen, sonst die aktuelle.
   */
  readonly onMove: (
    key: HomeBlockKey,
    target: HomeMoveTarget,
    base: readonly HomeBlockPlacement[],
  ) => void;
  /** Rückt einen Baustein ohne Maus in der Reihenfolge (Pfeiltasten). */
  readonly onMoveBy: (key: HomeBlockKey, direction: HomeMoveDirection) => void;
  readonly onSpanChange: (key: HomeBlockKey, span: HomeBlockSpan) => void;
  readonly onRemove: (key: HomeBlockKey) => void;
  readonly onAdd: (key: HomeBlockKey) => void;
  /** Zurück zur Rollenansicht. Steht in der Leiste, weil sie immer sichtbar ist. */
  readonly onRestoreDefault: () => void;
  readonly restoring: boolean;
  readonly children: (placement: HomeBlockPlacement) => ReactNode;
}

type EditorModule = { default: ComponentType<HomeBoardEditorProps> };

let loadedEditor: EditorModule | null = null;
let editorRequest: Promise<EditorModule> | null = null;

function loadEditor(): Promise<EditorModule> {
  editorRequest ??= import("./home-board-editor").then(
    (module) => {
      loadedEditor = { default: module.HomeBoardEditor };
      return loadedEditor;
    },
    (error: unknown) => {
      // Ein gescheiterter Abruf (Netz weg, neue Version ausgerollt) darf den
      // nächsten Versuch nicht blockieren.
      editorRequest = null;
      throw error;
    },
  );
  return editorRequest;
}

/**
 * Ist das Modul schon geladen, bekommt `lazy` ein Thenable, das sofort
 * erfüllt: React zeichnet den Editor dann im selben Durchgang, statt einen
 * Moment lang den Platzhalter zu zeigen. Sonst wartet `lazy` auf den Abruf.
 */
function editorThenable(): PromiseLike<EditorModule> {
  const loaded = loadedEditor;
  if (!loaded) return loadEditor();
  return {
    then: (onFulfilled) => {
      onFulfilled?.(loaded);
      return Promise.resolve() as never;
    },
  };
}

const HomeBoardEditor = lazy(editorThenable as () => Promise<EditorModule>);

/**
 * Lädt das Anpassen vorab, etwa wenn der Zeiger auf „Anpassen“ liegt oder
 * der Knopf den Fokus bekommt. Ein gescheitertes Vorladen ist harmlos: das
 * Öffnen lädt erneut und meldet seinen eigenen Fehler.
 */
export function preloadHomeBoardEditor(): Promise<void> {
  return loadEditor().then(
    () => undefined,
    () => undefined,
  );
}

export function HomeBoard({ editing, ...props }: HomeBoardProps) {
  const grid = (
    <HomeBoardGrid placements={props.placements}>
      {props.children}
    </HomeBoardGrid>
  );
  if (!editing) return grid;
  // Bis das Anpassen geladen ist, bleibt die Leseansicht stehen: kein
  // Ladekreis und kein Sprung, die Kacheln wechseln an derselben Stelle.
  return (
    <Suspense fallback={grid}>
      <HomeBoardEditor {...props} />
    </Suspense>
  );
}

/** Die Leseansicht: jede Kachel in ihrer Zelle, sonst nichts. */
function HomeBoardGrid({
  placements,
  children,
}: {
  readonly placements: readonly HomeBlockPlacement[];
  readonly children: (placement: HomeBlockPlacement) => ReactNode;
}) {
  // Beide Raster werden immer gerechnet; welches gilt, entscheidet CSS am
  // Breakpoint (siehe CELL_CLASSES).
  const wideCells = computeBoardCells(placements, HOME_BOARD_COLUMNS);
  const narrowCells = computeBoardCells(placements, NARROW_BOARD_COLUMNS);

  return (
    <ul data-testid="home-board" className={BOARD_GRID_CLASSES}>
      {placements.map((placement) => {
        const style: CSSProperties = {
          position: "relative",
          // Beide Zellen als Variablen; CELL_CLASSES wählt am Breakpoint aus.
          ...cellVariables(
            narrowCells.get(placement.key) ?? null,
            NARROW_BOARD_COLUMNS,
          ),
          ...cellVariables(
            wideCells.get(placement.key) ?? null,
            HOME_BOARD_COLUMNS,
          ),
        };
        return (
          <li
            key={placement.key}
            style={style}
            data-testid={`home-block-${placement.key}`}
            data-block-key={placement.key}
            data-span={placement.span}
            data-col={placement.col}
            data-row={placement.row}
            className={`min-h-0 ${SPAN_CLASS[placement.span]} ${CELL_CLASSES}`}
          >
            <div className="h-full min-h-0">{children(placement)}</div>
          </li>
        );
      })}
    </ul>
  );
}
