import { describe, it, expect } from "vitest";
import type { Reminder, ReminderType } from "~/lib/reminders-api";
import {
  REMINDER_SECTIONS,
  isReminderOverdue,
  reminderKey,
  reminderRelativeLabel,
  reminderSubtitle,
  reminderToneClass,
} from "~/lib/reminders-display";

function makeReminder(
  minutes_away: number,
  type: ReminderType = "pickup_upcoming",
): Reminder {
  return { type, title: "Test", due_time: "10:00", minutes_away };
}

describe("reminders-display", () => {
  describe("activity_running_over (#4765)", () => {
    const running: Reminder = {
      type: "activity_running_over",
      activity_instance_id: "7",
      title: "Fußball-AG",
      subtitle: "Turnhalle",
      due_time: "16:00",
      minutes_away: -30,
      pickup_due_count: 2,
    };

    it("says how much longer the block runs instead of 'überfällig'", () => {
      expect(reminderRelativeLabel(running)).toBe("30 Min länger");
    });

    it("names the room and the children whose pickup time is reached", () => {
      expect(reminderSubtitle(running)).toBe(
        "Turnhalle · 2 Kinder zur Abholung",
      );
      expect(reminderSubtitle({ ...running, pickup_due_count: 1 })).toBe(
        "Turnhalle · 1 Kind zur Abholung",
      );
    });

    it("shows only the room when no pickup time is reached", () => {
      expect(reminderSubtitle({ ...running, pickup_due_count: 0 })).toBe(
        "Turnhalle",
      );
    });

    it("leaves other subtitles untouched", () => {
      expect(reminderSubtitle({ ...makeReminder(5), subtitle: "3b" })).toBe(
        "3b",
      );
    });

    it("has its own section and a stable key", () => {
      expect(REMINDER_SECTIONS.map((s) => s.type)).toContain(
        "activity_running_over",
      );
      expect(reminderKey(running)).toBe("activity_running_over-activity-7");
    });
  });

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
      expect(reminderToneClass(makeReminder(-5))).toBe("text-moto-red");
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
    it("lists the five types most-urgent-first with German titles", () => {
      expect(REMINDER_SECTIONS.map((s) => s.type)).toEqual([
        "pickup_overdue",
        "activity_overdue",
        "activity_running_over",
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
