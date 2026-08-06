import { MotoConceptIcon } from "~/components/ui/moto-concept-icon";
import type { MotoConceptKey } from "~/lib/moto-concepts";
import { cn } from "~/lib/utils";

type ConceptIconTileVariant = "section" | "page" | "display";

const CONCEPT_ICON_TILE_VARIANTS: Record<
  ConceptIconTileVariant,
  { readonly tile: string; readonly iconSize: number }
> = {
  section: {
    tile: "flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-10 sm:w-10",
    iconSize: 22,
  },
  page: {
    tile: "flex h-11 w-11 shrink-0 items-center justify-center rounded-xl bg-gray-100 sm:h-12 sm:w-12",
    iconSize: 26,
  },
  display: {
    tile: "flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-gray-100",
    iconSize: 30,
  },
};

interface ConceptIconTileProps {
  readonly concept: MotoConceptKey;
  readonly variant: ConceptIconTileVariant;
  readonly className?: string;
}

/**
 * Shared gray icon tile for fachliche Konzepte (see Header-Muster in
 * concept-section-header.tsx). Consolidates the repeated
 * "rounded-xl bg-gray-100 tile + MotoConceptIcon" markup so the tile size,
 * rounding and icon size stay in one place per variant.
 *
 * - "section": 36 to 40 px tile, used in section/card headers.
 * - "page": 44 to 48 px tile, used in page headers.
 * - "display": 56 px tile, used on the public display panels.
 */
export function ConceptIconTile({
  concept,
  variant,
  className,
}: ConceptIconTileProps) {
  const { tile, iconSize } = CONCEPT_ICON_TILE_VARIANTS[variant];

  return (
    <div className={cn(tile, className)}>
      <MotoConceptIcon concept={concept} size={iconSize} />
    </div>
  );
}
