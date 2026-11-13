import { describe, expect, it } from "vitest";

import type { Student } from "~/lib/student-helpers";
import { roomCheckinCandidates } from "./room-checkin-dialog";

// „Kinder anmelden“ on the room page (#6953): which children the dialog
// offers for this room, and how it says where they are now.

function child(
  id: string,
  first: string,
  location: string,
  extra: Partial<Student> = {},
): Student {
  return {
    id,
    name: `${first} Test`,
    first_name: first,
    second_name: "Test",
    school_class: "2a",
    current_location: location,
    ...extra,
  } as Student;
}

const students = [
  child("1", "Zoe", "Anwesend - Mensa"),
  child("2", "Anna", "Zuhause"),
  child("3", "Ben", "Anwesend - OGS-Raum 1"),
  child("4", "Carl", "Unterwegs"),
  child("5", "Dora", "Zuhause", { sick: true }),
];

describe("roomCheckinCandidates", () => {
  it("offers only absent children without the right to pull", () => {
    const candidates = roomCheckinCandidates(students, "OGS-Raum 1", false);
    expect(candidates.map((candidate) => candidate.name)).toEqual([
      "Anna Test",
      "Dora Test",
    ]);
    expect(candidates.map((candidate) => candidate.where)).toEqual([
      "Zuhause",
      "Krank",
    ]);
  });

  it("adds present children of other rooms after the absent ones", () => {
    const candidates = roomCheckinCandidates(students, "OGS-Raum 1", true);
    expect(
      candidates.map((candidate) => [candidate.name, candidate.where]),
    ).toEqual([
      ["Anna Test", "Zuhause"],
      ["Dora Test", "Krank"],
      ["Carl Test", "Unterwegs"],
      ["Zoe Test", "In Mensa"],
    ]);
  });

  it("leaves out the children already in this room", () => {
    const names = roomCheckinCandidates(students, "OGS-Raum 1", true).map(
      (candidate) => candidate.name,
    );
    expect(names).not.toContain("Ben Test");
    expect(names).toContain("Zoe Test");
  });
});
