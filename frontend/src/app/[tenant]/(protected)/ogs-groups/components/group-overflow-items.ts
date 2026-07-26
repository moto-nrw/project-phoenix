import type { OverflowMenuItem } from "~/components/ui/page-header/types";

interface BuildArgs {
  /** Whether the current group is being supervised via substitution. */
  readonly viaSubstitution: boolean;
  /** Number of currently outgoing transfers — surfaced as a badge. */
  readonly activeTransfersCount: number;
  /** Opens the group transfer modal. */
  readonly onOpenTransfer: () => void;
}

/**
 * Builds the kebab-menu items for the OGS-Groups page header. Returns an
 * empty array when nothing is appropriate (e.g. while supervising via
 * substitution — you can't transfer a group you don't own), which causes
 * `<OverflowMenu>` to render nothing.
 */
export function buildGroupOverflowItems({
  viaSubstitution,
  activeTransfersCount,
  onOpenTransfer,
}: BuildArgs): OverflowMenuItem[] {
  if (viaSubstitution) {
    return [];
  }

  return [
    {
      label: "Gruppe übergeben",
      badge: activeTransfersCount > 0 ? activeTransfersCount : undefined,
      onClick: onOpenTransfer,
    },
  ];
}
