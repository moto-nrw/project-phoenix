"use client";

import { TenantPage } from "~/components/ui/tenant-page";
import { useReminders } from "~/lib/hooks/use-reminders";
import type { Reminder } from "~/lib/reminders-api";
import {
  REMINDER_SECTIONS,
  isReminderOverdue,
  reminderKey,
  reminderRelativeLabel,
  reminderToneClass,
} from "~/lib/reminders-display";

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
  const loading = isLoading && reminders.length === 0;

  return (
    <TenantPage
      title="Erinnerungen"
      stats={`${upcoming} anstehend · ${overdue} überfällig`}
      statsLoading={loading}
      loading={loading}
      error={error ? "Erinnerungen konnten nicht geladen werden." : null}
      empty={
        !loading && !error && count === 0
          ? {
              title: "Keine aktiven Erinnerungen",
              description:
                data?.enabled === false
                  ? "Sobald eine Abholung näher rückt oder eine Aktivität startet, erscheint sie hier. Erinnerungstypen werden in den Einstellungen unter „Erinnerungen“ aktiviert."
                  : "Erinnerungen aktiviert. Aktuell gibt es keine aktiven Erinnerungen.",
            }
          : null
      }
    >
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
                <ReminderRow key={reminderKey(reminder)} reminder={reminder} />
              ))}
            </ul>
          </section>
        );
      })}
    </TenantPage>
  );
}
