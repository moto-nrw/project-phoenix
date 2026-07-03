import { describe, it, expect } from "vitest";
import type { Reminder, ReminderType } from "~/lib/reminders-api";
import {
  REMINDER_SECTIONS,
  isReminderOverdue,
  reminderRelativeLabel,
  reminderToneClass,
} from "~/lib/reminders-display";

function makeReminder(
  minutes_away: number,
  type: ReminderType = "pickup_upcoming",
): Reminder {
  return { type, title: "Test", due_time: "10:00", minutes_away };
}

describe("reminders-display", () => {
  describe("reminderRelativeLabel", () => {
    it("renders overdue minutes as '… Min überfällig'", () => {
      expect(reminderRelativeLabel(makeReminder(-26))).toBe(
        "26 Min überfällig",
      );
    });

    it("renders zero as 'jetzt'", () => {
      expect(reminderRelativeLabel(makeReminder(0))).toBe("jetzt");
    });

    it("renders upcoming minutes as 'in … Min'", () => {
      expect(reminderRelativeLabel(makeReminder(10))).toBe("in 10 Min");
    });
  });

  describe("isReminderOverdue", () => {
    it("is true only for negative minutes_away", () => {
      expect(isReminderOverdue(makeReminder(-1))).toBe(true);
      expect(isReminderOverdue(makeReminder(0))).toBe(false);
      expect(isReminderOverdue(makeReminder(5))).toBe(false);
    });
  });

  describe("reminderToneClass", () => {
    it("uses brand red only for overdue, gray otherwise", () => {
      expect(reminderToneClass(makeReminder(-5))).toBe("text-[#FF3130]");
      expect(reminderToneClass(makeReminder(5))).toBe("text-gray-500");
      expect(reminderToneClass(makeReminder(0))).toBe("text-gray-500");
    });
  });

  describe("REMINDER_SECTIONS", () => {
    it("lists the four types most-urgent-first with German titles", () => {
      expect(REMINDER_SECTIONS.map((s) => s.type)).toEqual([
        "pickup_overdue",
        "activity_overdue",
        "pickup_upcoming",
        "activity_start",
      ]);
      const overdueActivity = REMINDER_SECTIONS.find(
        (s) => s.type === "activity_overdue",
      );
      expect(overdueActivity?.title).toBe("Überfällige Aktivität");
    });
  });
});
