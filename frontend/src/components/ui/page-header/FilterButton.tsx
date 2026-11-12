"use client";
import { Button } from "~/components/ui/button";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { cn } from "~/lib/utils";

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
  className,
  testId,
}: FilterButtonProps) {
  const showCountBadge =
    activeCount !== undefined && activeCount > 0 && !isOpen;
  // When the consumer opted into count-display, the ring becomes redundant —
  // the badge already signals "filters active".
  const showRing = hasActiveFilters && !isOpen && activeCount === undefined;

  return (
    <Button
      type="button"
      variant="filter"
      size="icon"
      onClick={onClick}
      data-testid={testId}
      aria-label={showCountBadge ? `Filter (${activeCount} aktiv)` : "Filter"}
      aria-expanded={isOpen}
      tooltip={
        isOpen
          ? "Blendet die Filter wieder aus."
          : "Zeigt die Filter für diese Liste."
      }
      data-filters-active={showRing ? "" : undefined}
      className={cn("relative", className)}
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
        <NotificationBadge
          count={activeCount}
          tone="blue"
          size="sm"
          ariaLabel={`${activeCount} aktive Filter`}
          className="absolute -top-1 -right-1 tabular-nums shadow-sm"
        />
      ) : null}
    </Button>
  );
}
