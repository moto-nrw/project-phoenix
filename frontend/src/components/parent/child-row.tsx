"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
import { Avatar } from "~/components/ui/avatar";
import {
  StatusBadge,
  type StatusBadgeTone,
} from "~/components/ui/status-badge";

/**
 * One child as a list row. Shared by the dashboard overview and the
 * "Meine Kinder" page, which used to render two different cards for the same
 * entity — different avatar sizes, different radii, different metadata.
 */
export interface ChildRowItem {
  readonly key: string;
  readonly name: string;
  readonly schoolName: string;
  /** Secondary line, e.g. class and care period. */
  readonly detail?: string;
  readonly badgeLabel?: string;
  readonly badgeTone?: StatusBadgeTone;
  readonly href?: string;
}

/**
 * `row` sits inside a SectionCard (tighter, subordinate). `card` is the
 * standalone entity card used on the "Meine Kinder" page, where the rows carry
 * the page themselves and no wrapper section is needed.
 */
export function ChildRow({
  item,
  variant = "row",
}: Readonly<{ item: ChildRowItem; variant?: "row" | "card" }>) {
  const surface =
    variant === "card"
      ? "rounded-2xl border border-gray-200 bg-white p-4 shadow-sm"
      : "rounded-xl border border-gray-200 bg-white p-3";
  const content = (
    <div className="flex min-w-0 items-center gap-3">
      <Avatar
        name={item.name}
        size="md"
        shape="rounded"
        decorative
        className={
          variant === "card" ? "h-12 w-12 text-base" : "h-10 w-10 text-sm"
        }
      />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-gray-900">
          {item.name}
        </p>
        {item.detail && (
          <p className="truncate text-sm text-gray-600">{item.detail}</p>
        )}
        <p className="truncate text-xs text-gray-500">{item.schoolName}</p>
      </div>
      {item.badgeLabel ? (
        <span className="shrink-0">
          <StatusBadge
            label={item.badgeLabel}
            tone={item.badgeTone ?? "gray"}
          />
        </span>
      ) : (
        <ArrowRight
          className="h-4 w-4 shrink-0 text-gray-400"
          aria-hidden="true"
        />
      )}
    </div>
  );

  if (!item.href) {
    return <div className={surface}>{content}</div>;
  }

  return (
    <Link
      href={item.href}
      className={`block transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none ${surface}`}
    >
      {content}
    </Link>
  );
}
