import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type {
  StudentNote,
  StudentNoteTeamNotice,
} from "~/lib/student-notes-api";
import { SWRConfig } from "~/lib/swr/testing";
import { StudentNotizenTab } from "./student-notizen-tab";

vi.unmock("swr");
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", tenantId: 1 }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSlugSafe: () => "school-a",
}));
vi.mock("~/lib/session-cache", () => ({ announceSessionChange: vi.fn() }));
const { list, markRead, team } = vi.hoisted(() => ({
  list: vi.fn(),
  markRead: vi.fn(async () => undefined),
  team: { notices: [] as StudentNoteTeamNotice[] },
}));
vi.mock("~/lib/student-notes-api", async (original) => ({
  ...(await original<typeof import("~/lib/student-notes-api")>()),
  studentNotesService: { list, listActivities: async () => [] },
}));
vi.mock("~/lib/hooks/use-student-note-team-notices", () => ({
  useStudentNoteTeamNotices: () => ({ notices: team.notices, markRead }),
}));

const note: StudentNote = {
  id: "1",
  studentId: "7",
  kind: "journal",
  visibility: "all_staff",
  category: "",
  body: "Bisheriger Eintrag",
  origin: "staff",
  authorName: "Sara",
  subjectDate: "2026-09-09",
  activityGroupId: "",
  educationGroupId: "",
  sharedWithSchool: false,
  canEdit: false,
  canDelete: false,
  edited: false,
  createdAt: "2026-09-09T10:00:00Z",
  updatedAt: "2026-09-09T10:00:00Z",
};
const notice: StudentNoteTeamNotice = {
  noteId: "1",
  studentId: "7",
  studentFirstName: "Mia",
  studentLastName: "M",
  authorName: "Sara",
  informedAt: "2026-09-09T10:00:00Z",
};

function pendingNotes() {
  let resolve!: (notes: StudentNote[]) => void;
  const promise = new Promise<StudentNote[]>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}

function renderTab() {
  const cache = new Map();
  function View({ active }: { active: boolean }) {
    return (
      <SWRConfig value={{ provider: () => cache, shouldRetryOnError: false }}>
        <ToastProvider>
          <StudentNotizenTab studentId="7" active={active} />
        </ToastProvider>
      </SWRConfig>
    );
  }
  const view = render(<View active />);
  return {
    ...view,
    show: (active: boolean) => view.rerender(<View active={active} />),
  };
}

beforeEach(() => {
  list.mockReset().mockResolvedValue([note]);
  markRead.mockReset().mockResolvedValue(undefined);
  team.notices = [];
});

describe("reading shared notes", () => {
  it("keeps notices unread while another tab is selected, including an in-flight refresh", async () => {
    const view = renderTab();
    expect(await screen.findByText(note.body)).toBeVisible();
    view.show(false);
    team.notices = [notice];
    await act(async () => {
      view.show(false);
    });
    expect(markRead).not.toHaveBeenCalled();
    expect(list).toHaveBeenCalledTimes(1);

    const refresh = pendingNotes();
    list.mockReturnValue(refresh.promise);
    view.show(true);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    view.show(false);
    await act(async () => {
      refresh.resolve([note]);
    });
    expect(screen.getByText(note.body)).toBeVisible();
    expect(markRead).not.toHaveBeenCalled();

    list.mockResolvedValue([note]);
    view.show(true);
    await waitFor(() => expect(markRead).toHaveBeenCalledWith("7"));
  });

  it("refreshes new and repeatedly shared notes before acknowledging them", async () => {
    const view = renderTab();
    expect(await screen.findByText(note.body)).toBeVisible();
    const refresh = pendingNotes();
    list.mockReturnValue(refresh.promise);
    team.notices = [{ ...notice, noteId: "2" }];
    view.show(true);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    expect(screen.getByText(note.body)).toBeVisible();
    expect(markRead).not.toHaveBeenCalled();

    const added = { ...note, id: "2", body: "Neu geteilter Eintrag" };
    let expectedBody = added.body;
    const visibleAtRead: boolean[] = [];
    markRead.mockImplementation(async () => {
      visibleAtRead.push(screen.queryByText(expectedBody) !== null);
    });
    await act(async () => {
      refresh.resolve([note, added]);
    });
    expect(await screen.findByText(added.body)).toBeVisible();
    await waitFor(() => expect(markRead).toHaveBeenCalledTimes(1));

    const correction = pendingNotes();
    list.mockReturnValue(correction.promise);
    team.notices = [
      { ...notice, noteId: "2", informedAt: "2026-09-09T11:00:00Z" },
    ];
    view.show(true);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(3));
    expect(screen.getByText(added.body)).toBeVisible();
    expect(markRead).toHaveBeenCalledTimes(1);
    expectedBody = "Berichtigter Eintrag";
    await act(async () => {
      correction.resolve([note, { ...added, body: "Berichtigter Eintrag" }]);
    });
    expect(await screen.findByText("Berichtigter Eintrag")).toBeVisible();
    await waitFor(() => expect(markRead).toHaveBeenCalledTimes(2));
    expect(visibleAtRead).toEqual([true, true]);
  });

  it("keeps notices unread when refreshing the cached notes fails", async () => {
    const view = renderTab();
    expect(await screen.findByText(note.body)).toBeVisible();
    list.mockRejectedValue(new Error("unavailable"));
    team.notices = [notice];
    await act(async () => {
      view.show(true);
    });
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    expect(screen.getByText(note.body)).toBeVisible();
    expect(markRead).not.toHaveBeenCalled();
  });
});
