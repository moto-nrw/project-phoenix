"use client";

import Link from "next/link";
import { ArrowRight } from "lucide-react";
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

export function childInitials(first: string, last = ""): string {
  return `${first.at(0) ?? ""}${last.at(0) ?? ""}`.toUpperCase();
}

/** Initials from a rendered full name ("Felix Schneider" → "FS"). */
export function initialsFromFullName(fullName: string): string {
  const parts = fullName.trim().split(/\s+/);
  return childInitials(
    parts[0] ?? "",
    parts.at(-1) === parts[0] ? "" : (parts.at(-1) ?? ""),
  );
}

/** Neutral initials avatar. Brand green, used sparingly as the one accent. */
export function ChildAvatar({
  initials,
  size = "sm",
}: Readonly<{ initials: string; size?: "sm" | "lg" }>) {
  const box = size === "lg" ? "h-12 w-12 text-base" : "h-10 w-10 text-sm";
  return (
    <span
      className={`flex shrink-0 items-center justify-center rounded-xl bg-[#83CD2D]/15 font-semibold text-[#5A8E1F] ${box}`}
      aria-hidden="true"
    >
      {initials}
    </span>
  );
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
      <ChildAvatar
        initials={initialsFromFullName(item.name)}
        size={variant === "card" ? "lg" : "sm"}
      />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-semibold text-gray-900">
          {item.name}
        </p>
        <p className="truncate text-sm text-gray-600">
          {item.detail
            ? `${item.schoolName} · ${item.detail}`
            : item.schoolName}
        </p>
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
