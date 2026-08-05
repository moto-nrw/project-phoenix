// Shared display metadata for staff reminders (issue #1457).
// Single source of truth for the per-type German section title and the
// relative time label, consumed by both the /reminders page and the header
// bell so the two never drift.
//
// Color discipline: the list stays calm and neutral like the rest of the app.
// The ONLY color is red, and only for "überfällig" — the genuine alert (a
// child is still present past its pickup time). Everything else is gray.

import type { Reminder, ReminderType } from "~/lib/reminders-api";

// Color discipline: the only accent is brand red (LOCATION_COLORS.DANGER,
// #DC2626) and only for "überfällig" — applied via arbitrary-value Tailwind
// classes (text-moto-red below, bg-moto-red on the bell badge), never a
// generic Tailwind red.

// Order = display order, most urgent first (overdue → upcoming → activities).
export const REMINDER_SECTIONS: {
  type: ReminderType;
  title: string;
}[] = [
  { type: "pickup_overdue", title: "Überfällige Abholung" },
  { type: "activity_overdue", title: "Überfällige Aktivität" },
  { type: "pickup_upcoming", title: "Anstehende Abholung" },
  { type: "activity_start", title: "Aktivitätsbeginn" },
];

export function isReminderOverdue(reminder: Reminder): boolean {
  return reminder.minutes_away < 0;
}

export function reminderRelativeLabel(reminder: Reminder): string {
  if (reminder.minutes_away < 0) {
    return `${Math.abs(reminder.minutes_away)} Min überfällig`;
  }
  if (reminder.minutes_away === 0) {
    return "jetzt";
  }
  return `in ${reminder.minutes_away} Min`;
}

// Tailwind text-color class for the relative-time label: red only when
// overdue, neutral gray otherwise.
export function reminderToneClass(reminder: Reminder): string {
  return isReminderOverdue(reminder) ? "text-moto-red" : "text-gray-500";
}

/** Stable React key backed by the domain entity that produced the reminder. */
export function reminderKey(reminder: Reminder): string {
  switch (reminder.type) {
    case "pickup_upcoming":
    case "pickup_overdue":
      if (!reminder.student_id) {
        throw new Error(`${reminder.type} reminder is missing student_id`);
      }
      return `${reminder.type}-student-${reminder.student_id}`;
    case "activity_start":
    case "activity_overdue":
      if (!reminder.activity_instance_id) {
        throw new Error(
          `${reminder.type} reminder is missing activity_instance_id`,
        );
      }
      return `${reminder.type}-activity-${reminder.activity_instance_id}`;
  }
}
