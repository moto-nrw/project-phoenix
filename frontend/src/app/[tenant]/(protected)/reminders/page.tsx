"use client";

import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { useReminders } from "~/lib/hooks/use-reminders";
import type { Reminder } from "~/lib/reminders-api";
import {
  REMINDER_SECTIONS,
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

  return (
    <div className="mx-auto w-full max-w-3xl px-4 py-6">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Erinnerungen</h1>
        <p className="mt-1 text-sm text-gray-500">
          Abholungen und Aktivitäten, die anstehen oder überfällig sind.
        </p>
      </div>

      {error && (
        <Alert
          type="error"
          message="Erinnerungen konnten nicht geladen werden."
        />
      )}

      {isLoading && !reminders.length ? (
        <Loading message="Erinnerungen werden geladen…" fullPage={false} />
      ) : error && !data ? null : count === 0 ? (
        data?.enabled === false ? (
          <div className="rounded-2xl border border-gray-200 bg-white p-10 text-center shadow-sm">
            <p className="font-medium text-gray-900">
              Keine aktiven Erinnerungen
            </p>
            <p className="mt-1 text-sm text-gray-500">
              Sobald eine Abholung näher rückt oder eine Aktivität startet,
              erscheint sie hier. Erinnerungstypen werden in den Einstellungen
              unter „Erinnerungen“ aktiviert.
            </p>
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
                className="overflow-hidden rounded-2xl border border-gray-200 bg-white shadow-sm"
              >
                <div className="flex items-center justify-between border-b border-gray-100 px-4 py-3">
                  <h2 className="font-semibold text-gray-900">
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
  );
}
