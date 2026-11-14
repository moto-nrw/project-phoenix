// Header bell for staff reminders (issue #1457).
// Always visible in the topbar (desktop + mobile) so the count is instantly
// seen without opening the sidebar. Opens a dropdown preview of the most
// urgent reminders with a link to the full /reminders page. Visual-only — the
// count refreshes on the useReminders 60s tick, there is no sound or push.
// Renders nothing until the tenant has enabled at least one reminder type.

"use client";

import { EmptyState } from "~/components/ui/empty-state";
import { useState } from "react";
import { AnchoredPopover } from "~/components/ui/anchored-popover";
import { Button } from "~/components/ui/button";
import NavigationLink from "~/components/ui/navigation-link";
import { NotificationBadge } from "~/components/ui/notification-badge";
import { Bell } from "lucide-react";
import { useReminders } from "~/lib/hooks/use-reminders";
import type { Reminder } from "~/lib/reminders-api";
import {
  reminderKey,
  reminderRelativeLabel,
  reminderSubtitle,
  reminderToneClass,
} from "~/lib/reminders-display";
import { useTenantAwarePath } from "~/lib/tenant-path";

// How many reminders to preview inline before linking out to the full page.
const PREVIEW_LIMIT = 5;

function BellRow({ reminder }: { readonly reminder: Reminder }) {
  const subtitle = reminderSubtitle(reminder);
  return (
    <li className="flex items-center justify-between gap-3 px-4 py-2.5">
      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-gray-900">
          {reminder.title}
        </p>
        {subtitle && (
          <p className="truncate text-xs text-gray-500">{subtitle}</p>
        )}
      </div>
      <div className="flex flex-shrink-0 flex-col items-end">
        <span className="text-xs font-semibold text-gray-900">
          {reminder.due_time}
        </span>
        <span className={`text-xs ${reminderToneClass(reminder)}`}>
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

  // Hidden entirely until the school switches on a reminder type.
  if (!enabled) return null;

  const preview = reminders.slice(0, PREVIEW_LIMIT);
  const overflow = count - preview.length;

  return (
    <AnchoredPopover
      open={isOpen}
      onOpenChange={setIsOpen}
      ariaLabel="Erinnerungen"
      preferredWidth={320}
      align="end"
      focusPanelOnOpen
      header={<p className="font-semibold text-gray-900">Erinnerungen</p>}
      footer={
        <div className="p-2">
          <NavigationLink
            href={tenantPath("/reminders")}
            onClick={() => setIsOpen(false)}
            className="flex items-center justify-center rounded-xl px-3 py-2.5 text-sm font-medium text-gray-700 transition-colors duration-200 hover:bg-gray-100 hover:text-gray-900"
          >
            Alle ansehen
          </NavigationLink>
        </div>
      }
      renderTrigger={({ ref, toggle, panelId }) => (
        <Button
          ref={ref}
          type="button"
          variant="ghost"
          size="icon"
          onClick={toggle}
          aria-label={count > 0 ? `Erinnerungen (${count})` : "Erinnerungen"}
          aria-expanded={isOpen}
          aria-haspopup="dialog"
          aria-controls={isOpen ? panelId : undefined}
          tooltip="Zeigt Ihre Erinnerungen, ohne die Seite zu verlassen."
          className="relative"
        >
          <Bell className="h-5 w-5" />
          <NotificationBadge
            count={count}
            tone="red"
            size="sm"
            ariaLabel={`${count} Erinnerungen`}
            className="absolute -top-0.5 -right-0.5"
          />
        </Button>
      )}
    >
      {() =>
        count === 0 ? (
          <EmptyState
            variant="compact"
            title="Keine aktiven Erinnerungen."
            className="px-4 py-4"
          />
        ) : (
          <>
            <ul className="divide-y divide-gray-100">
              {preview.map((reminder) => (
                <BellRow key={reminderKey(reminder)} reminder={reminder} />
              ))}
            </ul>
            {overflow > 0 && (
              <p className="px-4 py-2 text-center text-xs text-gray-500">
                +{overflow} weitere
              </p>
            )}
          </>
        )
      }
    </AnchoredPopover>
  );
}
