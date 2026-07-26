import { describe, it, expect } from "vitest";
import type { Reminder, ReminderType } from "~/lib/reminders-api";
import {
  REMINDER_SECTIONS,
  isReminderOverdue,
  reminderKey,
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

  describe("reminderKey", () => {
    it("uses the student ID for pickup reminders", () => {
      expect(
        reminderKey({
          ...makeReminder(5),
          student_id: "42",
        }),
      ).toBe("pickup_upcoming-student-42");
    });

    it("disambiguates concurrent activities by instance ID", () => {
      const shared = {
        type: "activity_start" as const,
        title: "Schach",
        due_time: "10:00",
        minutes_away: 5,
      };

      expect(
        [
          { ...shared, activity_instance_id: "101" },
          { ...shared, activity_instance_id: "102" },
        ].map(reminderKey),
      ).toEqual(["activity_start-activity-101", "activity_start-activity-102"]);
    });

    it("rejects reminders without their required entity ID", () => {
      expect(() => reminderKey(makeReminder(5))).toThrow(
        "pickup_upcoming reminder is missing student_id",
      );
      expect(() => reminderKey(makeReminder(5, "activity_start"))).toThrow(
        "activity_start reminder is missing activity_instance_id",
      );
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
