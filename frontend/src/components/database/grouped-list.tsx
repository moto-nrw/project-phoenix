"use client";

import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { cn } from "~/lib/utils";
import { GroupHeader, type GroupHeaderVariant } from "./group-header";

export interface GroupDefinition<T> {
  id: string;
  title: string;
  items: T[];
  variant?: GroupHeaderVariant;
  countSuffix?: string;
  bulkAction?: ReactNode;
}

interface GroupedListProps<T> {
  groups: GroupDefinition<T>[];
  renderItem: (item: T) => ReactNode;
  keyFor: (item: T) => string;
  defaultOpenIds?: string[];
  emptyState?: ReactNode;
  className?: string;
}

export function GroupedList<T>({
  groups,
  renderItem,
  keyFor,
  defaultOpenIds,
  emptyState,
  className,
}: GroupedListProps<T>) {
  const initialOpen = useMemo(() => {
    if (defaultOpenIds) return new Set(defaultOpenIds);
    return new Set(groups.map((g) => g.id));
  }, [defaultOpenIds, groups]);

  const [openIds, setOpenIds] = useState<Set<string>>(initialOpen);

  const toggle = (id: string) => {
    setOpenIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  if (groups.length === 0) {
    return (
      <div className="flex flex-1 items-center justify-center p-8">
        {emptyState ?? <p className="text-sm text-gray-500">Keine Einträge</p>}
      </div>
    );
  }

  return (
    <div className={cn("flex-1 overflow-auto", className)}>
      {groups.map((group) => {
        const isOpen = openIds.has(group.id);
        return (
          <section key={group.id}>
            <GroupHeader
              title={group.title}
              count={group.items.length}
              countSuffix={group.countSuffix}
              variant={group.variant}
              isOpen={isOpen}
              onToggle={() => toggle(group.id)}
              bulkAction={group.bulkAction}
            />
            {isOpen ? (
              <ul role="list">
                {group.items.map((item) => (
                  <li key={keyFor(item)}>{renderItem(item)}</li>
                ))}
              </ul>
            ) : null}
          </section>
        );
      })}
    </div>
  );
}
