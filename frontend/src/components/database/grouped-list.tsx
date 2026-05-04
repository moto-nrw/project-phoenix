"use client";

import { useEffect, useMemo, useRef, useState } from "react";
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

  // Track every group id we've ever seen so we can default *newly introduced*
  // groups to "open" without re-opening ones the user has explicitly collapsed.
  // This kicks in when the grouping mode flips (e.g. flat -> by-floor) and a
  // brand new set of group ids appears. Seed with every id present at mount —
  // not just the initially-open ones — so groups that were closed at mount
  // don't get treated as "new" on first effect run.
  const knownIdsRef = useRef<Set<string>>(new Set(groups.map((g) => g.id)));

  useEffect(() => {
    const known = knownIdsRef.current;
    const incoming = groups.map((g) => g.id);
    const newlyIntroduced = incoming.filter((id) => !known.has(id));
    if (newlyIntroduced.length === 0) return;

    setOpenIds((prev) => {
      const next = new Set(prev);
      for (const id of newlyIntroduced) next.add(id);
      return next;
    });
    for (const id of newlyIntroduced) known.add(id);
  }, [groups]);

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
