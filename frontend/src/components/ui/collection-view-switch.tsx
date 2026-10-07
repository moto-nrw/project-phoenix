"use client";

import { LayoutGrid, List } from "lucide-react";

import { Button } from "~/components/ui/button";
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
 * choice, so it is a SegmentedControl, not a tab bar.
 *
 * Below `sm` the head row has no room for the pair next to the title and the
 * ⋮ menu, so a phone gets ONE symbol button that shows the other view (list
 * symbol while tiles show, and back). The caller marks its wrapper
 * `data-icon-only`, so either form stays beside the title.
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
  const next = value === "tiles" ? "table" : "tiles";
  const nextLabel =
    value === "tiles" ? "Als Liste zeigen" : "Als Kacheln zeigen";
  return (
    <>
      <SegmentedControl
        ariaLabel="Ansicht"
        iconOnly
        items={VIEW_ITEMS}
        value={value}
        onChange={onChange}
        className={`max-sm:hidden ${className ?? ""}`}
      />
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="sm:hidden"
        aria-label={nextLabel}
        title={nextLabel}
        onClick={() => onChange(next)}
      >
        {next === "table" ? (
          <List className="h-5 w-5" aria-hidden />
        ) : (
          <LayoutGrid className="h-5 w-5" aria-hidden />
        )}
      </Button>
    </>
  );
}

export interface ColumnMenuColumn {
  readonly key: string;
  readonly header: string;
  /** Columns that carry the row (the name) stay out of the menu. */
  readonly hideable?: boolean;
  /** The phone row's own parts (title, meta) are not offered as its detail. */
  readonly stacked?: string;
}

/**
 * The phone list's "In der Zeile zeigen" choice (#3834), as entries for the
 * page's ⋮ menu: which one column shows under each name, or none. A phone
 * has room for name, status and one more fact per line; this lets each
 * person pick the fact their round needs (Gehzeit, Heimweg, Mensa …).
 */
export function phoneDetailMenuEntries(
  columns: readonly ColumnMenuColumn[],
  current: string | null,
  onChange: (key: string | null) => void,
): OverflowMenuEntry[] {
  const choices = columns.filter(
    (column) =>
      column.hideable !== false &&
      column.stacked !== "title" &&
      column.stacked !== "meta",
  );
  if (choices.length === 0) return [];
  return [
    { kind: "header", label: "In der Zeile zeigen" },
    {
      kind: "radio",
      label: "Nur Name und Status",
      checked: current === null,
      onClick: () => onChange(null),
    },
    ...choices.map((column) => ({
      kind: "radio" as const,
      label: column.header,
      checked: current === column.key,
      onClick: () => onChange(column.key),
    })),
  ];
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
