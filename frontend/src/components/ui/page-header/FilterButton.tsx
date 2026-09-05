"use client";

interface FilterButtonProps {
  readonly isOpen: boolean;
  readonly onClick: () => void;
  readonly hasActiveFilters: boolean;
  /** When set and >0, renders a numeric badge instead of the ring affordance. */
  readonly activeCount?: number;
  readonly className?: string;
  /**
   * Test hook. The button serves both the mobile and desktop filter triggers,
   * so callers pass a placement-specific id to tell them apart.
   */
  readonly testId?: string;
}

export function FilterButton({
  isOpen,
  onClick,
  hasActiveFilters,
  activeCount,
  className = "",
  testId,
}: FilterButtonProps) {
  const showCountBadge =
    activeCount !== undefined && activeCount > 0 && !isOpen;
  // When the consumer opted into count-display, the ring becomes redundant —
  // the badge already signals "filters active".
  const showRing = hasActiveFilters && !isOpen && activeCount === undefined;

  return (
    <button
      type="button"
      onClick={onClick}
      data-testid={testId}
      aria-label={showCountBadge ? `Filter (${activeCount} aktiv)` : "Filter"}
      // `aspect-square` statt fester Breite: die Kopfkarte erzwingt die
      // Bedienhöhe (40/44 px) per Nachfahren-Selektor — mit fester Breite
      // wurde der runde Knopf dabei zum Oval. So folgt die Breite der Höhe.
      className={`relative flex aspect-square h-10 items-center justify-center rounded-2xl transition-all duration-200 ${
        isOpen
          ? "border-moto-blue bg-moto-blue border text-white"
          : "border border-gray-200 bg-white text-gray-600 hover:bg-gray-50"
      } ${showRing ? "ring-moto-blue ring-2 ring-offset-1" : ""} ${className}`}
    >
      <svg
        className="h-4 w-4"
        fill="none"
        viewBox="0 0 24 24"
        stroke="currentColor"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4"
        />
      </svg>
      {showCountBadge ? (
        <span
          className="bg-moto-blue absolute -top-1 -right-1 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-bold text-white tabular-nums shadow-sm"
          aria-hidden
        >
          {activeCount}
        </span>
      ) : null}
    </button>
  );
}
