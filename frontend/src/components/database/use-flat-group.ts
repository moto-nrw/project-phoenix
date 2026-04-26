"use client";

import { useMemo } from "react";
import type { GroupDefinition } from "./grouped-list";

const FLAT_GROUP_ID = "__flat__";

export function useFlatGroup<T>(
  items: T[],
  pluralLabel: string,
): GroupDefinition<T>[] {
  return useMemo(() => {
    if (items.length === 0) return [];
    return [
      {
        id: FLAT_GROUP_ID,
        title: `Alle ${pluralLabel} (${items.length})`,
        items,
      },
    ];
  }, [items, pluralLabel]);
}
