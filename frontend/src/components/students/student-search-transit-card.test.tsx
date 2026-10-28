import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Student } from "~/lib/api";
import { StudentSearchTransitCard } from "./student-search-transit-card";

const { state } = vi.hoisted(() => ({
  state: {
    permissions: ["visits:update"] as string[],
    presenceMode: "detailed" as string,
    attendanceWebEnabled: true,
  },
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { permissions: state.permissions, roles: ["user"] } },
    status: "authenticated",
  }),
}));

vi.mock("~/lib/tenant-context", () => ({
  usePresenceMode: () => state.presenceMode,
  useAttendanceWebEnabled: () => state.attendanceWebEnabled,
  useTenantSlugSafe: () => null,
  useTenantRoutingModeSafe: () => "subdomain",
}));

function student(id: string, location: string): Student {
  return { id, name: `Kind ${id}`, current_location: location } as Student;
}

const STUDENTS = [
  student("1", "Unterwegs"),
  student("2", "Unterwegs"),
  student("3", "Anwesend - Mensa"),
];
const FROM = encodeURIComponent("/students/search?status=unterwegs");

describe("StudentSearchTransitCard", () => {
  beforeEach(() => {
    state.permissions = ["visits:update"];
    state.presenceMode = "detailed";
    state.attendanceWebEnabled = true;
  });

  it("leads to the transit page with the way back to Alle Kinder", () => {
    render(
      <StudentSearchTransitCard students={STUDENTS} show fromParam={FROM} />,
    );

    const link = screen.getByRole("link", { name: /Unterwegs/ });
    expect(link).toHaveAttribute("href", `/rooms/unterwegs?from=${FROM}`);
    expect(link).toHaveTextContent("2 Kinder ohne Raumzuweisung");
    expect(link).toHaveTextContent("Zuweisen");
  });

  it("is hidden without the right to assign rooms", () => {
    state.permissions = ["students:read"];
    const { container } = render(
      <StudentSearchTransitCard students={STUDENTS} show fromParam={FROM} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("is hidden in binary presence mode", () => {
    state.presenceMode = "binary";
    const { container } = render(
      <StudentSearchTransitCard students={STUDENTS} show fromParam={FROM} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("is hidden when the school does not record attendance in the app", () => {
    state.attendanceWebEnabled = false;
    const { container } = render(
      <StudentSearchTransitCard students={STUDENTS} show fromParam={FROM} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("is hidden while the list neither filters nor groups by Unterwegs", () => {
    const { container } = render(
      <StudentSearchTransitCard
        students={STUDENTS}
        show={false}
        fromParam={FROM}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("is hidden when no visible child is on the way", () => {
    const { container } = render(
      <StudentSearchTransitCard
        students={[student("3", "Anwesend - Mensa")]}
        show
        fromParam={FROM}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("counts a single child in the singular", () => {
    state.permissions = ["admin:*"];
    render(
      <StudentSearchTransitCard
        students={[student("1", "Unterwegs")]}
        show
        fromParam={FROM}
      />,
    );

    expect(screen.getByRole("link", { name: /Unterwegs/ })).toHaveTextContent(
      "1 Kind ohne Raumzuweisung",
    );
  });
});
