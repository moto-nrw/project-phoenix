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

const FLAT_GROUP_ID = "__flat__";

export function useGroupedItems<T, K extends string>(
  items: T[],
  mode: K | "none",
  groupers: Partial<Record<K, Grouper<T>>>,
  pluralLabel: string,
): GroupDefinition<T>[] {
  return useMemo(() => {
    if (items.length === 0) return [];

    const grouper = mode === "none" ? undefined : groupers[mode as K];
    if (!grouper) {
      return [
        {
          id: FLAT_GROUP_ID,
          title: `Alle ${pluralLabel} (${items.length})`,
          items,
        },
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
      .map(([id, bucket]) => ({
        id,
        title: `${bucket.title} (${bucket.items.length})`,
        items: bucket.items,
      }));
  }, [items, mode, groupers, pluralLabel]);
}
