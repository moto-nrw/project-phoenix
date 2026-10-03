"use client";

import { Columns3 } from "lucide-react";

import { SegmentedControl } from "~/components/ui/segmented-control";
import { OverflowMenu } from "~/components/ui/page-header/OverflowMenu";
import type { CollectionView } from "~/lib/hooks/use-collection-view";

const VIEW_ITEMS = [
  { value: "tiles", label: "Kacheln" },
  { value: "table", label: "Tabelle" },
] as const;

/**
 * Switch between the tile grid and the table of a collection page (#3834).
 * A value choice, so it is a SegmentedControl, not a tab bar. Phones always
 * show tiles; the caller hides the switch below `md`.
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
 * The "Spalten" menu of a table: one switch per column the user may hide
 * (#3834). Stays open while switching so several columns change in one go.
 */
export function DataTableColumnMenu({
  columns,
  hiddenColumns,
  onChange,
}: Readonly<{
  columns: readonly ColumnMenuColumn[];
  hiddenColumns: ReadonlySet<string>;
  onChange: (key: string, visible: boolean) => void;
}>) {
  const hideable = columns.filter((column) => column.hideable !== false);
  if (hideable.length === 0) return null;
  const shownCount = hideable.filter(
    (column) => !hiddenColumns.has(column.key),
  ).length;
  return (
    <OverflowMenu
      ariaLabel="Spalten auswählen"
      triggerContent={
        <span className="flex items-center gap-1.5 px-2 text-sm font-medium text-gray-700">
          <Columns3 className="h-4 w-4" aria-hidden />
          Spalten
        </span>
      }
      items={[
        { kind: "header", label: "Spalten zeigen" },
        ...hideable.map((column) => {
          const visible = !hiddenColumns.has(column.key);
          return {
            kind: "checkbox" as const,
            label: column.header,
            checked: visible,
            keepOpen: true,
            // The last shown column stays: an empty table helps nobody.
            disabled: visible && shownCount === 1,
            onClick: () => onChange(column.key, !visible),
          };
        }),
      ]}
    />
  );
}
