"use client";

import { Alert } from "~/components/ui/alert";
import { Loading } from "~/components/ui/loading";
import { LOCATION_COLORS } from "~/lib/location-helper";
import { useReminders } from "~/lib/hooks/use-reminders";
import type { Reminder, ReminderType } from "~/lib/reminders-api";

// Section metadata per reminder type. Colors come from LOCATION_COLORS (brand
// hex), not generic Tailwind hues. Order = display order (most urgent first).
const SECTIONS: {
  type: ReminderType;
  title: string;
  color: string;
}[] = [
  {
    type: "pickup_overdue",
    title: "Überfällige Abholung",
    color: LOCATION_COLORS.HOME,
  },
  {
    type: "pickup_upcoming",
    title: "Anstehende Abholung",
    color: LOCATION_COLORS.SCHOOLYARD,
  },
  {
    type: "activity_start",
    title: "AG-Beginn",
    color: LOCATION_COLORS.OTHER_ROOM,
  },
];

function relativeLabel(reminder: Reminder): string {
  if (reminder.minutes_away < 0) {
    return `${Math.abs(reminder.minutes_away)} Min überfällig`;
  }
  if (reminder.minutes_away === 0) {
    return "jetzt";
  }
  return `in ${reminder.minutes_away} Min`;
}

function ReminderRow({
  reminder,
  color,
}: {
  reminder: Reminder;
  color: string;
}) {
  return (
    <li className="flex items-center justify-between gap-4 px-4 py-3">
      <div className="flex min-w-0 items-center gap-3">
        <span
          className="h-2.5 w-2.5 flex-shrink-0 rounded-full"
          style={{ backgroundColor: color }}
          aria-hidden="true"
        />
        <div className="min-w-0">
          <p className="truncate font-medium text-gray-900">{reminder.title}</p>
          {reminder.subtitle && (
            <p className="truncate text-sm text-gray-500">
              {reminder.subtitle}
            </p>
          )}
        </div>
      </div>
      <div className="flex flex-shrink-0 flex-col items-end">
        <span className="text-sm font-semibold text-gray-900">
          {reminder.due_time}
        </span>
        <span className="text-xs" style={{ color }}>
          {relativeLabel(reminder)}
        </span>
      </div>
    </li>
  );
}

export default function RemindersPage() {
  const { reminders, count, error, isLoading } = useReminders();

  return (
    <div className="mx-auto w-full max-w-3xl px-4 py-6">
      <div className="mb-6">
        <h1 className="text-2xl font-bold text-gray-900">Erinnerungen</h1>
        <p className="mt-1 text-sm text-gray-500">
          Anstehende und überfällige Abholungen sowie startende Aktivitäten.
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
      ) : count === 0 ? (
        <div className="rounded-2xl border border-gray-200 bg-white p-10 text-center shadow-sm">
          <p className="font-medium text-gray-900">
            Keine aktiven Erinnerungen
          </p>
          <p className="mt-1 text-sm text-gray-500">
            Sobald eine Abholung näher rückt oder eine Aktivität startet,
            erscheint sie hier. Erinnerungstypen werden in den Einstellungen
            unter „Erinnerungen" aktiviert.
          </p>
        </div>
      ) : (
        <div className="space-y-6">
          {SECTIONS.map((section) => {
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
                  <span
                    className="flex h-6 min-w-6 items-center justify-center rounded-full px-2 text-xs font-semibold text-white"
                    style={{ backgroundColor: section.color }}
                  >
                    {items.length}
                  </span>
                </div>
                <ul className="divide-y divide-gray-100">
                  {items.map((reminder, index) => (
                    <ReminderRow
                      key={`${reminder.type}-${reminder.student_id ?? reminder.title}-${index}`}
                      reminder={reminder}
                      color={section.color}
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
