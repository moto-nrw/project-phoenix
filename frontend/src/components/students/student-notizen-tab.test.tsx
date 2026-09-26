import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StudentNotizenTab } from "./student-notizen-tab";
import type { StudentNote } from "~/lib/student-notes-api";

// The tab renders the authority the backend sent and nothing else: an entry
// the reader may not correct carries no Bearbeiten, one they may not remove
// carries no Entfernen, and a carried-over hint carries neither.

const listMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();
const removeMock = vi.fn();

vi.mock("~/lib/student-notes-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/student-notes-api")>();
  return {
    ...actual,
    studentNotesService: {
      list: (...args: unknown[]) => listMock(...args) as unknown,
      create: (...args: unknown[]) => createMock(...args) as unknown,
      update: (...args: unknown[]) => updateMock(...args) as unknown,
      remove: (...args: unknown[]) => removeMock(...args) as unknown,
    },
  };
});

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (_key: string, fetcher: () => Promise<unknown>) => {
    const state = swrState;
    if (!state.started) {
      state.started = true;
      void fetcher().then((data) => {
        state.data = data;
      });
    }
    return {
      data: state.data,
      isLoading: state.isLoading,
      error: state.error,
      mutate: state.mutate,
    };
  },
}));

const swrState: {
  started: boolean;
  data: unknown;
  isLoading: boolean;
  error: unknown;
  mutate: () => Promise<void>;
} = {
  started: false,
  data: undefined,
  isLoading: false,
  error: undefined,
  mutate: vi.fn(async () => undefined),
};

function note(overrides: Partial<StudentNote> = {}): StudentNote {
  return {
    id: "1",
    studentId: "7",
    kind: "journal",
    visibility: "all_staff",
    category: "",
    body: "Hat heute vorgelesen.",
    origin: "staff",
    authorName: "Sara Betreuerin",
    subjectDate: "2026-09-09",
    activityGroupId: "",
    educationGroupId: "",
    canEdit: false,
    canDelete: false,
    edited: false,
    createdAt: "2026-09-09T10:00:00Z",
    updatedAt: "2026-09-09T10:00:00Z",
    ...overrides,
  };
}

function renderTab(notes: StudentNote[]) {
  swrState.started = false;
  swrState.data = notes;
  swrState.isLoading = false;
  swrState.error = undefined;
  listMock.mockResolvedValue(notes);
  return render(<StudentNotizenTab studentId="7" />);
}

describe("StudentNotizenTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    swrState.mutate = vi.fn(async () => undefined);
  });

  it("names its purpose and says parents cannot see the notes", () => {
    renderTab([]);
    expect(screen.getByText("Notizen")).toBeInTheDocument();
    expect(
      screen.getByText("Einträge zu diesem Kind. Eltern sehen sie nicht."),
    ).toBeInTheDocument();
  });

  it("offers a first note on an empty card", () => {
    renderTab([]);
    expect(screen.getByText("Noch keine Notizen")).toBeInTheDocument();
  });

  it("renders the entry with its author and day", () => {
    renderTab([note()]);
    expect(screen.getByText("Hat heute vorgelesen.")).toBeInTheDocument();
    expect(screen.getByText(/Sara Betreuerin/)).toBeInTheDocument();
  });

  it("shows the audience only when it is narrower than the team", () => {
    renderTab([note({ visibility: "care_team" })]);
    expect(screen.getByText("Betreuungsteam")).toBeInTheDocument();
  });

  it("offers no actions on a note the reader may neither correct nor remove", () => {
    renderTab([note()]);
    expect(
      screen.queryByRole("button", { name: "Aktionen zur Notiz" }),
    ).not.toBeInTheDocument();
  });

  it("offers only Bearbeiten to the author", () => {
    renderTab([note({ canEdit: true })]);
    fireEvent.click(screen.getByRole("button", { name: "Aktionen zur Notiz" }));
    expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
    expect(screen.queryByText("Entfernen")).not.toBeInTheDocument();
  });

  it("offers only Entfernen to the group lead", () => {
    renderTab([note({ canDelete: true })]);
    fireEvent.click(screen.getByRole("button", { name: "Aktionen zur Notiz" }));
    expect(screen.getByText("Entfernen")).toBeInTheDocument();
    expect(screen.queryByText("Bearbeiten")).not.toBeInTheDocument();
  });

  it("marks a carried-over hint instead of naming an author", () => {
    renderTab([
      note({
        kind: "permanent",
        origin: "master_data",
        authorName: "",
        subjectDate: "",
      }),
    ]);
    expect(
      screen.getByText(/Übernommen aus den Betreuernotizen/),
    ).toBeInTheDocument();
    expect(screen.getByText("Dauerhafter Hinweis")).toBeInTheDocument();
  });

  it("writes a new entry with the values from the form", async () => {
    createMock.mockResolvedValue(undefined);
    renderTab([]);
    fireEvent.click(screen.getAllByRole("button", { name: "Neue Notiz" })[0]!);

    fireEvent.change(screen.getByLabelText("Notiz"), {
      target: { value: "  Kurzes Gespräch geführt.  " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(createMock).toHaveBeenCalledTimes(1);
    });
    expect(createMock).toHaveBeenCalledWith(
      "7",
      expect.objectContaining({
        body: "Kurzes Gespräch geführt.",
        kind: "journal",
        visibility: "all_staff",
      }),
    );
  });

  it("keeps Speichern out of reach while the note is empty", () => {
    renderTab([]);
    fireEvent.click(screen.getAllByRole("button", { name: "Neue Notiz" })[0]!);
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
  });

  // The leadership audience needs a group reference, and the form does not
  // pick one — offering it would produce a note nobody but its author reads.
  it("does not offer the leadership audience on a note without a group", () => {
    renderTab([]);
    fireEvent.click(screen.getAllByRole("button", { name: "Neue Notiz" })[0]!);
    expect(screen.queryByText("Gruppenleitung")).not.toBeInTheDocument();
  });
});
