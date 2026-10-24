import { describe, it, expect } from "vitest";
import { mapVisitsToSupervisionStudents } from "~/components/active-supervisions/view-model";
import { mapSlimStudentResponse, teacherHintText } from "./student-helpers";
import { mapRoster } from "./timetable-operations-types";

// Der Hinweis der Lehrkraft (#4795) steht in Aufsicht, Gruppe und Kindersuche
// am Kind, gekennzeichnet als „Hinweis der Lehrkraft“.

describe("teacherHintText", () => {
  it("labels the hint and names its author and the class it is for", () => {
    expect(teacherHintText({ body: "Hat Bauchweh" })).toBe(
      "Hinweis der Lehrkraft: Hat Bauchweh",
    );
    expect(
      teacherHintText({
        body: "Heute Sportfest",
        author_name: "Lena Lehrerin",
        school_class: "4a",
      }),
    ).toBe(
      "Hinweis der Lehrkraft Lena Lehrerin an die Klasse 4a: Heute Sportfest",
    );
  });
});

describe("teacher hints on the child", () => {
  it("reaches the Kindersuche row from the slim list", () => {
    const student = mapSlimStudentResponse({
      id: 7,
      first_name: "Ida",
      last_name: "Hinweis",
      school_class: "4a",
      current_location: "Anwesend",
      sick: false,
      excused: false,
      class_trip: false,
      has_full_access: true,
      teacher_hints: [{ body: "Hat Bauchweh", author_name: "Lena Lehrerin" }],
    });
    expect(student.teacher_hints).toEqual([
      { body: "Hat Bauchweh", author_name: "Lena Lehrerin" },
    ]);
  });

  it("reaches the supervision card of the child it is for", () => {
    const [ida, bruno] = mapVisitsToSupervisionStudents(
      [
        {
          studentId: "7",
          studentName: "Ida Hinweis",
          activeGroupId: "a1",
          checkInTime: "2026-09-07T08:00:00.000Z",
          isActive: true,
        },
        {
          studentId: "8",
          studentName: "Bruno Banknachbar",
          activeGroupId: "a1",
          checkInTime: "2026-09-07T08:00:00.000Z",
          isActive: true,
        },
      ],
      { teacherHints: new Map([["7", [{ body: "Hat Bauchweh" }]]]) },
    );
    expect(ida?.teacher_hints).toEqual([{ body: "Hat Bauchweh" }]);
    expect(bruno?.teacher_hints).toBeUndefined();
  });

  it("reaches the block roster row and stays absent without hints", () => {
    const roster = mapRoster({
      instance: {
        id: 1,
        title: "Lesen",
        status: "active",
        room_id: 2,
        date: "2099-03-02",
      },
      rows: [
        {
          student_id: 7,
          student_name: "Ida Hinweis",
          planned: true,
          status: "present",
          teacher_hints: [{ body: "Hat Bauchweh" }],
        },
        {
          student_id: 8,
          student_name: "Bruno Banknachbar",
          planned: true,
          status: "expected",
        },
      ],
    } as Parameters<typeof mapRoster>[0]);
    expect(roster.rows[0]?.teacherHints).toEqual([{ body: "Hat Bauchweh" }]);
    expect(roster.rows[1]).not.toHaveProperty("teacherHints");
  });
});
