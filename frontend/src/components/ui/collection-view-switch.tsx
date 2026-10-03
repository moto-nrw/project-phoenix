"use client";

import { LayoutGrid, List } from "lucide-react";

import { SegmentedControl } from "~/components/ui/segmented-control";
import type { OverflowMenuEntry } from "~/components/ui/page-header/OverflowMenu";
import type { CollectionView } from "~/lib/hooks/use-collection-view";

const VIEW_ITEMS = [
  {
    value: "tiles",
    label: "Kacheln",
    icon: <LayoutGrid className="h-4 w-4" aria-hidden />,
  },
  {
    value: "table",
    label: "Liste",
    icon: <List className="h-4 w-4" aria-hidden />,
  },
] as const;

/**
 * Switch between the tile grid and the table of a collection page (#3834):
 * a grid and a list symbol, the familiar pair from file managers. A value
 * choice, so it is a SegmentedControl, not a tab bar. Phones always show
 * tiles; the caller hides the switch below `md`.
 */
export function CollectionViewSwitch({
  value,
  onChange,
  className,
}: Readonly<{
  value: CollectionView;
  onChange: (view: CollectionView) => void;
  className?: string;
}>) {
  return (
    <SegmentedControl
      ariaLabel="Ansicht"
      iconOnly
      items={VIEW_ITEMS}
      value={value}
      onChange={onChange}
      className={className}
    />
  );
}

export interface ColumnMenuColumn {
  readonly key: string;
  readonly header: string;
  /** Columns that carry the row (the name) stay out of the menu. */
  readonly hideable?: boolean;
}

/**
 * The column switches of a list view (#3834), as entries for the page's
 * existing ⋮ menu: a heading, then one checkbox per column the user may hide.
 * They live in that menu rather than in a button of their own, so the head
 * keeps one place for "more" (a separate "Spalten" button read as a filter).
 * The menu stays open while switching, so several columns change in one go.
 */
export function columnMenuEntries(
  columns: readonly ColumnMenuColumn[],
  hiddenColumns: ReadonlySet<string>,
  onChange: (key: string, visible: boolean) => void,
): OverflowMenuEntry[] {
  const hideable = columns.filter((column) => column.hideable !== false);
  if (hideable.length === 0) return [];
  const shownCount = hideable.filter(
    (column) => !hiddenColumns.has(column.key),
  ).length;
  return [
    { kind: "header", label: "Spalten in der Liste" },
    ...hideable.map((column) => {
      const visible = !hiddenColumns.has(column.key);
      return {
        kind: "checkbox" as const,
        label: column.header,
        checked: visible,
        keepOpen: true,
        // The last shown column stays: an empty list helps nobody.
        disabled: visible && shownCount === 1,
        onClick: () => onChange(column.key, !visible),
      };
    }),
  ];
}
