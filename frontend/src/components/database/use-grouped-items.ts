"use client";

import { useMemo } from "react";
import type { GroupDefinition } from "./grouped-list";

export interface GroupingTarget {
  id: string;
  title: string;
  /** Optional sort key for ordering groups; falls back to `title`. */
  sortKey?: string;
}

export type Grouper<T> = (item: T) => GroupingTarget;

export type GroupDecorator<T> = (
  group: GroupDefinition<T>,
) => Partial<Omit<GroupDefinition<T>, "id" | "items">>;

const FLAT_GROUP_ID = "__flat__";

export function useGroupedItems<T, K extends string>(
  items: T[],
  mode: K | "none",
  groupers: Partial<Record<K, Grouper<T>>>,
  pluralLabel: string,
  decorateGroup?: GroupDecorator<T>,
): GroupDefinition<T>[] {
  // When grouping is off, the `groupers` argument is irrelevant. Several call
  // sites pass a fresh `{}` literal in that case, which would otherwise bust
  // the memo on every parent render. Resolving the active grouper outside
  // the memo lets us depend only on the resolved function, so a stable
  // identity for `groupers[mode]` (or `undefined`) keeps the memo warm.
  const grouper = mode === "none" ? undefined : groupers[mode as K];

  return useMemo(() => {
    if (items.length === 0) return [];

    const decorate = (group: GroupDefinition<T>): GroupDefinition<T> =>
      decorateGroup ? { ...group, ...decorateGroup(group) } : group;

    if (!grouper) {
      return [
        decorate({
          id: FLAT_GROUP_ID,
          title: `Alle ${pluralLabel}`,
          items,
        }),
      ];
    }

    const buckets = new Map<
      string,
      { title: string; sortKey: string; items: T[] }
    >();
    for (const item of items) {
      const target = grouper(item);
      const existing = buckets.get(target.id);
      if (existing) {
        existing.items.push(item);
      } else {
        buckets.set(target.id, {
          title: target.title,
          sortKey: target.sortKey ?? target.title,
          items: [item],
        });
      }
    }

    return [...buckets.entries()]
      .sort((a, b) => a[1].sortKey.localeCompare(b[1].sortKey, "de"))
      .map(([id, bucket]) =>
        decorate({
          id,
          title: bucket.title,
          items: bucket.items,
        }),
      );
  }, [items, grouper, pluralLabel, decorateGroup]);
}
