"use client";

import { Alert } from "~/components/ui/alert";
import { EmptyState } from "~/components/ui/empty-state";
import { PageIntro } from "~/components/ui/page-intro";
import { ListSkeleton, SkeletonRegion } from "~/components/ui/page-skeletons";
import { useReminders } from "~/lib/hooks/use-reminders";
import type { Reminder } from "~/lib/reminders-api";
import {
  REMINDER_SECTIONS,
  isReminderOverdue,
  reminderKey,
  reminderRelativeLabel,
  reminderToneClass,
} from "~/lib/reminders-display";
import { Skeleton } from "~/components/ui/skeleton";

function ReminderRow({ reminder }: { reminder: Reminder }) {
  return (
    <li className="flex items-center justify-between gap-4 px-4 py-3">
      <div className="min-w-0">
        <p className="truncate font-medium text-gray-900">{reminder.title}</p>
        {reminder.subtitle && (
          <p className="truncate text-sm text-gray-500">{reminder.subtitle}</p>
        )}
      </div>
      <div className="flex flex-shrink-0 flex-col items-end">
        <span className="text-sm font-semibold text-gray-900">
          {reminder.due_time}
        </span>
        <span className={`text-xs ${reminderToneClass(reminder)}`}>
          {reminderRelativeLabel(reminder)}
        </span>
      </div>
    </li>
  );
}

export default function RemindersPage() {
  const { reminders, count, error, isLoading, data } = useReminders();

  // Statuszeile unter dem Seitentitel, allein aus der geladenen Liste.
  const overdue = reminders.filter(isReminderOverdue).length;
  const upcoming = count - overdue;
  const summary = `${upcoming} anstehend · ${overdue} überfällig`;

  return (
    <div className="w-full">
      {/* Kopfkarte statt Seitenkopf plus frei stehendem Erklärabsatz. */}
      <PageIntro
        title="Erinnerungen"
        description={
          isLoading && !reminders.length ? (
            <Skeleton className="h-4 w-44" />
          ) : (
            summary
          )
        }
        className="mb-6"
      />

      <div className="space-y-6">
        {error && (
          <Alert
            type="error"
            message="Erinnerungen konnten nicht geladen werden."
          />
        )}

        {isLoading && !reminders.length ? (
          <SkeletonRegion label="Erinnerungen werden geladen…">
            <ListSkeleton rows={5} avatar={false} />
          </SkeletonRegion>
        ) : error && !data ? null : count === 0 ? (
          data?.enabled === false ? (
            <div className="moto-content-surface rounded-2xl border p-4 shadow-sm sm:p-6">
              <EmptyState
                title="Keine aktiven Erinnerungen"
                description="Sobald eine Abholung näher rückt oder eine Aktivität startet, erscheint sie hier. Erinnerungstypen werden in den Einstellungen unter „Erinnerungen“ aktiviert."
              />
            </div>
          ) : (
            <Alert
              type="success"
              message="Erinnerungen aktiviert. Aktuell gibt es keine aktiven Erinnerungen."
            />
          )
        ) : (
          <div className="space-y-6">
            {REMINDER_SECTIONS.map((section) => {
              const items = reminders.filter((r) => r.type === section.type);
              if (items.length === 0) return null;
              return (
                <section
                  key={section.type}
                  className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm"
                >
                  <div className="flex items-center justify-between border-b border-gray-100 px-4 py-3">
                    <h2 className="text-base font-semibold text-gray-900">
                      {section.title}
                    </h2>
                    <span className="flex h-6 min-w-6 items-center justify-center rounded-full bg-gray-100 px-2 text-xs font-semibold text-gray-600">
                      {items.length}
                    </span>
                  </div>
                  <ul className="divide-y divide-gray-100">
                    {items.map((reminder) => (
                      <ReminderRow
                        key={reminderKey(reminder)}
                        reminder={reminder}
                      />
                    ))}
                  </ul>
                </section>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
