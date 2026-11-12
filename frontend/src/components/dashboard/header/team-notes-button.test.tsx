import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import type { StudentNoteTeamNotice } from "~/lib/student-notes-api";

const mockNotices = vi.fn();
vi.mock("~/lib/hooks/use-student-note-team-notices", () => ({
  useStudentNoteTeamNotices: () => mockNotices(),
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/schule${path}`,
}));

import { TeamNotesButton } from "./team-notes-button";

function notice(
  over: Partial<StudentNoteTeamNotice> = {},
): StudentNoteTeamNotice {
  return {
    noteId: "11",
    studentId: "7",
    studentFirstName: "Mia",
    studentLastName: "Muster",
    authorName: "Sara Betreuerin",
    informedAt: "2026-09-09T10:00:00Z",
    ...over,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
});

// „Neue Notizen vom Team“ (#6854): the red count is the unread marker; it is
// there exactly while a shared note waits, and each row opens the child's
// notes, where it clears.
describe("TeamNotesButton", () => {
  it("shows nothing while no shared note waits", () => {
    mockNotices.mockReturnValue({ notices: [] });
    const { container } = render(<TeamNotesButton />);
    expect(container).toBeEmptyDOMElement();
  });

  it("counts the unread notes and links each to the child's notes", () => {
    mockNotices.mockReturnValue({
      notices: [
        notice(),
        notice({
          noteId: "12",
          studentId: "8",
          studentFirstName: "Noa",
          studentLastName: "Beispiel",
        }),
      ],
    });
    render(<TeamNotesButton />);

    const button = screen.getByRole("button", {
      name: "Neue Notizen vom Team (2)",
    });
    expect(screen.getByText("2")).toBeInTheDocument();
    fireEvent.click(button);

    const mia = screen.getByRole("link", { name: /Mia Muster/ });
    expect(mia).toHaveAttribute("href", "/schule/students/7?tab=notizen");
    expect(mia).toHaveTextContent("Von Sara Betreuerin");
    expect(screen.getByRole("link", { name: /Noa Beispiel/ })).toHaveAttribute(
      "href",
      "/schule/students/8?tab=notizen",
    );
  });
});
