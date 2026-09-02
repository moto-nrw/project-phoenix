// Header bell for staff reminders (issue #1457).
// Always visible in the topbar (desktop + mobile) so the count is instantly
// seen without opening the sidebar. Opens a dropdown preview of the most
// urgent reminders with a link to the full /reminders page. Visual-only — the
// count refreshes on the useReminders 60s tick, there is no sound or push.
// Renders nothing until the tenant has enabled at least one reminder type.

"use client";

import { useEffect, useRef, useState } from "react";
import Link from "next/link";
import { Bell } from "lucide-react";
import { useReminders } from "~/lib/hooks/use-reminders";
import type { Reminder } from "~/lib/reminders-api";
import {
  reminderKey,
  reminderRelativeLabel,
  reminderToneClass,
} from "~/lib/reminders-display";
import { useTenantAwarePath } from "~/lib/tenant-path";

// How many reminders to preview inline before linking out to the full page.
const PREVIEW_LIMIT = 5;

function BellRow({ reminder }: { readonly reminder: Reminder }) {
  return (
    <li className="flex items-center justify-between gap-3 px-4 py-2.5">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {reminder.title}
        </p>
        {reminder.subtitle && (
          <p className="truncate text-xs text-gray-500">{reminder.subtitle}</p>
        )}
      </div>
      <div className="flex flex-shrink-0 flex-col items-end">
        <span className="text-xs font-semibold text-gray-900">
          {reminder.due_time}
        </span>
        <span className={`text-[11px] ${reminderToneClass(reminder)}`}>
          {reminderRelativeLabel(reminder)}
        </span>
      </div>
    </li>
  );
}

export function RemindersBell() {
  const { reminders, count, enabled } = useReminders();
  const tenantPath = useTenantAwarePath();
  const [isOpen, setIsOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!isOpen) return;

    const handlePointerDown = (event: MouseEvent) => {
      const target = event.target;
      if (!(target instanceof Node)) return;
      if (containerRef.current?.contains(target)) return;
      setIsOpen(false);
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") setIsOpen(false);
    };

    document.addEventListener("mousedown", handlePointerDown);
    document.addEventListener("keydown", handleKeyDown);
    return () => {
      document.removeEventListener("mousedown", handlePointerDown);
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isOpen]);

  // Hidden entirely until the school switches on a reminder type.
  if (!enabled) return null;

  const preview = reminders.slice(0, PREVIEW_LIMIT);
  const overflow = count - preview.length;
  const badgeLabel = count > 99 ? "99+" : String(count);

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        onClick={() => setIsOpen((open) => !open)}
        aria-label={count > 0 ? `Erinnerungen (${count})` : "Erinnerungen"}
        aria-expanded={isOpen}
        title="Erinnerungen"
        className="relative flex h-10 w-10 items-center justify-center rounded-lg text-gray-500 transition-colors duration-200 hover:bg-gray-100 hover:text-gray-700"
      >
        <Bell className="h-5 w-5" />
        {count > 0 && (
          <span className="bg-moto-red absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full px-1 text-[10px] font-semibold text-white">
            {badgeLabel}
          </span>
        )}
      </button>

      {/*
        Mobile: pinned to the viewport just under the topbar and spanning the
        full width (the bell sits left of the avatar, so a bell-anchored panel
        would overflow the left edge). Desktop: a narrow dropdown anchored to
        the bell.
      */}
      <div
        className={`moto-popover-surface fixed top-14 right-3 left-3 z-50 overflow-hidden rounded-2xl border transition-all duration-150 ease-out sm:absolute sm:top-full sm:right-0 sm:left-auto sm:mt-2 sm:w-80 ${
          isOpen
            ? "visible translate-y-0 opacity-100"
            : "invisible -translate-y-2 opacity-0"
        }`}
      >
        <div className="border-b border-gray-100 px-4 py-3">
          <p className="font-semibold text-gray-900">Erinnerungen</p>
        </div>

        {count === 0 ? (
          <p className="px-4 py-6 text-center text-sm text-gray-500">
            Keine aktiven Erinnerungen.
          </p>
        ) : (
          <>
            <ul className="divide-y divide-gray-100">
              {preview.map((reminder) => (
                <BellRow key={reminderKey(reminder)} reminder={reminder} />
              ))}
            </ul>
            {overflow > 0 && (
              <p className="px-4 pt-2 text-center text-xs text-gray-500">
                +{overflow} weitere
              </p>
            )}
          </>
        )}

        <div className="p-2">
          <Link
            href={tenantPath("/reminders")}
            onClick={() => setIsOpen(false)}
            className="flex items-center justify-center rounded-xl px-3 py-2.5 text-sm font-medium text-gray-700 transition-colors duration-200 hover:bg-gray-100 hover:text-gray-900"
          >
            Alle ansehen
          </Link>
        </div>
      </div>
    </div>
  );
}
