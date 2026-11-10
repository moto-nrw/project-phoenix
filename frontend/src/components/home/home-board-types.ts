import type { ReactNode } from "react";

import type {
  HomeBlockDefinition,
  HomeBlockKey,
  HomeBlockPlacement,
  HomeBlockSpan,
  HomeMoveDirection,
  HomeMoveTarget,
} from "~/lib/home-blocks";

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
