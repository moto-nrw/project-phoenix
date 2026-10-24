import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StudentNotizenTab } from "./student-notizen-tab";
import { ApiError } from "~/lib/api-error";
import type { StudentNote, StudentNoteDraft } from "~/lib/student-notes-api";

// The tab renders the authority the backend sent and nothing else: an entry
// the reader may not correct carries no Bearbeiten, one they may not remove
// carries no Löschen, and a carried-over hint carries neither.

const listMock = vi.fn();
const createMock = vi.fn();
const updateMock = vi.fn();
const removeMock = vi.fn();
const listActivitiesMock = vi.fn();

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
      listActivities: (...args: unknown[]) =>
        listActivitiesMock(...args) as unknown,
    },
  };
});

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string, fetcher: () => Promise<unknown>) => {
    // The activity read has its own state, so the note card's state stays
    // exactly what each test sets up.
    if (key.startsWith("student-note-activities-")) {
      return {
        data: activityState.data,
        isLoading: false,
        error: activityState.error,
        mutate: vi.fn(),
      };
    }
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
  useTenantMutate: () => swrState.tenantMutate,
}));

const activityState: { data: unknown; error: unknown } = {
  data: [],
  error: undefined,
};

const swrState: {
  started: boolean;
  data: unknown;
  isLoading: boolean;
  error: unknown;
  mutate: () => Promise<void>;
  tenantMutate: (key: string) => Promise<unknown>;
} = {
  started: false,
  data: undefined,
  isLoading: false,
  error: undefined,
  mutate: vi.fn(async () => undefined),
  tenantMutate: vi.fn(async () => undefined),
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

function renderTab(
  notes: StudentNote[],
  props: { educationGroupId?: string; educationGroupName?: string } = {},
) {
  swrState.started = false;
  swrState.data = notes;
  swrState.isLoading = false;
  swrState.error = undefined;
  listMock.mockResolvedValue(notes);
  return render(<StudentNotizenTab studentId="7" {...props} />);
}

function chooseOption(label: string, option: string) {
  fireEvent.click(screen.getByLabelText(label));
  fireEvent.click(screen.getByRole("option", { name: option }));
}

describe("StudentNotizenTab", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    activityState.data = [];
    activityState.error = undefined;
    swrState.mutate = vi.fn(async () => undefined);
    swrState.tenantMutate = vi.fn(async () => undefined);
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
    // Anker: die Notiz selbst ist gerendert.
    expect(screen.getByText("Hat heute vorgelesen.")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Aktionen zur Notiz" }),
    ).not.toBeInTheDocument();
  });

  it("offers only Bearbeiten to the author", () => {
    renderTab([note({ canEdit: true })]);
    fireEvent.click(screen.getByRole("button", { name: "Aktionen zur Notiz" }));
    expect(screen.getByText("Bearbeiten")).toBeInTheDocument();
    expect(screen.queryByText("Löschen")).not.toBeInTheDocument();
  });

  it("clears the day when an entry becomes a permanent hint", async () => {
    updateMock.mockResolvedValue(undefined);
    renderTab([note({ canEdit: true })]);
    fireEvent.click(screen.getByRole("button", { name: "Aktionen zur Notiz" }));
    fireEvent.click(screen.getByText("Bearbeiten"));
    fireEvent.click(
      screen.getByRole("button", { name: "Dauerhafter Hinweis" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(updateMock).toHaveBeenCalledWith(
        "7",
        "1",
        expect.objectContaining({ kind: "permanent", subjectDate: undefined }),
      );
    });
  });

  it("sets a day when a permanent hint becomes an entry", async () => {
    updateMock.mockResolvedValue(undefined);
    renderTab([
      note({
        kind: "permanent",
        subjectDate: "",
        canEdit: true,
      }),
    ]);
    fireEvent.click(screen.getByRole("button", { name: "Aktionen zur Notiz" }));
    fireEvent.click(screen.getByText("Bearbeiten"));
    fireEvent.click(screen.getByRole("button", { name: "Eintrag" }));
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(updateMock).toHaveBeenCalledWith(
        "7",
        "1",
        expect.objectContaining({ kind: "journal", subjectDate: "2026-09-09" }),
      );
    });
  });

  it("offers only Löschen to the group lead", () => {
    renderTab([note({ canDelete: true })]);
    fireEvent.click(screen.getByRole("button", { name: "Aktionen zur Notiz" }));
    expect(screen.getByText("Löschen")).toBeInTheDocument();
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
    expect(
      screen.getByRole("heading", { name: "Dauerhafte Hinweise" }),
    ).toBeInTheDocument();
  });

  it("orders a month by the day an entry describes, not by the write order", () => {
    renderTab([
      note({ id: "1", body: "Am Siebten", subjectDate: "2026-09-07" }),
      note({ id: "2", body: "Am Neunten", subjectDate: "2026-09-09" }),
      note({ id: "3", body: "Am Achten", subjectDate: "2026-09-08" }),
    ]);
    const bodies = screen
      .getAllByText(/^Am /)
      .map((element) => element.textContent);
    expect(bodies).toEqual(["Am Neunten", "Am Achten", "Am Siebten"]);
  });

  it("keeps a durable hint out of the month it happens to be written in", () => {
    renderTab([
      note({ id: "1", body: "Chronik-Eintrag", subjectDate: "2026-09-09" }),
      note({
        id: "2",
        kind: "permanent",
        body: "Gilt immer",
        subjectDate: "",
      }),
    ]);
    const headings = screen
      .getAllByRole("heading", { level: 3 })
      .map((element) => element.textContent);
    expect(headings).toEqual(["Dauerhafte Hinweise", "September 2026"]);
  });

  it("groups an undated legacy entry by its Berlin day", () => {
    renderTab([
      note({
        subjectDate: "",
        createdAt: "2026-08-31T22:30:00Z",
      }),
    ]);
    expect(
      screen.getByRole("heading", { name: "September 2026" }),
    ).toBeInTheDocument();
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

  it("revalidates the Stammdaten hints after creating a permanent note", async () => {
    createMock.mockResolvedValue(undefined);
    renderTab([]);
    fireEvent.click(screen.getAllByRole("button", { name: "Neue Notiz" })[0]!);
    fireEvent.click(
      screen.getByRole("button", { name: "Dauerhafter Hinweis" }),
    );
    fireEvent.change(screen.getByLabelText("Notiz"), {
      target: { value: "Bleibt wichtig." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(swrState.tenantMutate).toHaveBeenCalledWith(
        "student-permanent-notes-7",
      );
    });
  });

  // The leadership audience needs a group reference, and the form does not
  // pick one — offering it would produce a note nobody but its author reads.
  it("does not offer the leadership audience on a note without a group", () => {
    renderTab([]);
    fireEvent.click(screen.getAllByRole("button", { name: "Neue Notiz" })[0]!);
    expect(screen.queryByText("Gruppenleitung")).not.toBeInTheDocument();
  });

  it("shows a failed load in place with its request ID and a retry", async () => {
    swrState.started = true;
    swrState.data = undefined;
    swrState.isLoading = false;
    swrState.error = new ApiError("boom", 500, {
      code: "general.server",
      instance: "req-notes",
    });
    render(<StudentNotizenTab studentId="7" />);

    expect(
      await screen.findByText(
        "Die Liste der Notizen konnte nicht bearbeitet werden. Bitte versuchen Sie es später erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-notes");
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(swrState.mutate).toHaveBeenCalled();
  });

  it("reports a refused save in the form and marks the note field", async () => {
    createMock.mockRejectedValue(
      new ApiError("invalid", 400, {
        code: "general.input",
        errors: [{ field: "body", reason: "too long" }],
      }),
    );
    renderTab([]);
    fireEvent.click(screen.getAllByRole("button", { name: "Neue Notiz" })[0]!);
    fireEvent.change(screen.getByLabelText("Notiz"), {
      target: { value: "Kurz." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        "Die Notiz konnte nicht übernommen werden. Bitte prüfen Sie Ihre Angaben.",
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText("Notiz")).toHaveAttribute(
        "aria-invalid",
        "true",
      ),
    );
  });

  it("names the missing permission when a delete is refused", async () => {
    removeMock.mockRejectedValue(
      new ApiError("forbidden", 403, { code: "general.permission" }),
    );
    renderTab([note({ canDelete: true })]);
    fireEvent.click(screen.getByRole("button", { name: "Aktionen zur Notiz" }));
    fireEvent.click(screen.getByRole("menuitem", { name: "Löschen" }));

    const dialog = await screen.findByRole("dialog");
    const advance = dialog.querySelectorAll("button");
    // Two-step gate: advance first, then confirm.
    fireEvent.click(
      [...advance].find((button) => /löschen/i.test(button.textContent ?? ""))!,
    );
    const confirm = await waitFor(() => {
      const found = [...dialog.querySelectorAll("button")].find((button) =>
        /Endgültig löschen/.test(button.textContent ?? ""),
      );
      expect(found).toBeDefined();
      return found!;
    });
    fireEvent.click(confirm);

    expect(
      await screen.findByText(
        "Für die Notiz fehlt Ihnen die Berechtigung. Bitte fragen Sie die Schule.",
      ),
    ).toBeInTheDocument();
  });

  describe("reference to a group or an activity", () => {
    const activities = [
      { id: "12", name: "Fußball-AG" },
      { id: "15", name: "Schach" },
    ];

    it("offers the child alone, its OGS group and its activities", () => {
      activityState.data = activities;
      renderTab([], { educationGroupId: "4", educationGroupName: "Bären" });
      fireEvent.click(
        screen.getAllByRole("button", { name: "Neue Notiz" })[0]!,
      );
      fireEvent.click(screen.getByLabelText("Bezug"));

      expect(
        screen.getAllByRole("option").map((option) => option.textContent),
      ).toEqual([
        "Nur das Kind",
        "OGS-Gruppe Bären",
        "Angebot Fußball-AG",
        "Angebot Schach",
      ]);
    });

    it("writes an entry about an activity on its day for the activity's leads", async () => {
      activityState.data = activities;
      createMock.mockResolvedValue(undefined);
      renderTab([], { educationGroupId: "4", educationGroupName: "Bären" });
      fireEvent.click(
        screen.getAllByRole("button", { name: "Neue Notiz" })[0]!,
      );

      // Without a reference there is no leadership to choose.
      fireEvent.click(screen.getByLabelText("Wer sieht die Notiz?"));
      expect(
        screen.queryByRole("option", { name: "Angebotsleitung" }),
      ).not.toBeInTheDocument();
      fireEvent.click(screen.getByRole("option", { name: "Ganzes Team" }));

      chooseOption("Bezug", "Angebot Fußball-AG");
      chooseOption("Wer sieht die Notiz?", "Angebotsleitung");
      fireEvent.change(screen.getByLabelText("Notiz"), {
        target: { value: "Hat beim Training gefehlt." },
      });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(createMock).toHaveBeenCalledTimes(1);
      });
      const [, draft] = createMock.mock.calls[0] as [string, StudentNoteDraft];
      expect(draft).toMatchObject({
        kind: "journal",
        visibility: "group_leads",
        activityGroupId: "12",
        subjectDate: "2026-09-09",
      });
      expect(draft.educationGroupId).toBeUndefined();
    });

    it("names the OGS group's leadership for a note about the group", async () => {
      createMock.mockResolvedValue(undefined);
      renderTab([], { educationGroupId: "4", educationGroupName: "Bären" });
      fireEvent.click(
        screen.getAllByRole("button", { name: "Neue Notiz" })[0]!,
      );
      chooseOption("Bezug", "OGS-Gruppe Bären");
      chooseOption("Wer sieht die Notiz?", "Gruppenleitung");
      fireEvent.change(screen.getByLabelText("Notiz"), {
        target: { value: "Streit in der Gruppe geschlichtet." },
      });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(createMock).toHaveBeenCalledTimes(1);
      });
      const [, draft] = createMock.mock.calls[0] as [string, StudentNoteDraft];
      expect(draft).toMatchObject({
        visibility: "group_leads",
        educationGroupId: "4",
      });
      expect(draft.activityGroupId).toBeUndefined();
    });

    // Dropping the reference must not quietly widen a leadership note to the
    // whole team; the care team is the next narrower audience.
    it("keeps the audience narrow when the reference is removed", async () => {
      activityState.data = activities;
      createMock.mockResolvedValue(undefined);
      renderTab([]);
      fireEvent.click(
        screen.getAllByRole("button", { name: "Neue Notiz" })[0]!,
      );
      chooseOption("Bezug", "Angebot Schach");
      chooseOption("Wer sieht die Notiz?", "Angebotsleitung");
      chooseOption("Bezug", "Nur das Kind");
      fireEvent.change(screen.getByLabelText("Notiz"), {
        target: { value: "Wirkt heute müde." },
      });
      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      await waitFor(() => {
        expect(createMock).toHaveBeenCalledTimes(1);
      });
      const [, draft] = createMock.mock.calls[0] as [string, StudentNoteDraft];
      expect(draft.visibility).toBe("care_team");
      expect(draft.activityGroupId).toBeUndefined();
      expect(draft.educationGroupId).toBeUndefined();
    });

    it("names the activity of an entry in the chronicle", () => {
      activityState.data = activities;
      renderTab([
        note({
          activityGroupId: "12",
          visibility: "group_leads",
          body: "Tor geschossen.",
        }),
      ]);
      expect(screen.getByText(/Angebot Fußball-AG/)).toBeInTheDocument();
      expect(screen.getByText("Angebotsleitung")).toBeInTheDocument();
    });

    it("still marks the activity when the child no longer attends it", () => {
      renderTab([note({ activityGroupId: "99", body: "Alter Eintrag." })]);
      expect(screen.getByText(/· Angebot ·/)).toBeInTheDocument();
    });

    it("names the OGS group of an entry about the group", () => {
      renderTab([note({ educationGroupId: "4", visibility: "group_leads" })], {
        educationGroupId: "4",
        educationGroupName: "Bären",
      });
      expect(screen.getByText(/OGS-Gruppe Bären/)).toBeInTheDocument();
      expect(screen.getByText("Gruppenleitung")).toBeInTheDocument();
    });

    it("lets the note be written without activities when they fail to load", () => {
      activityState.error = new Error("boom");
      renderTab([]);
      fireEvent.click(
        screen.getAllByRole("button", { name: "Neue Notiz" })[0]!,
      );
      expect(
        screen.getByText(
          "Die Angebote konnten nicht geladen werden. Sie können die Notiz trotzdem ohne Angebot speichern.",
        ),
      ).toBeInTheDocument();
    });

    it("keeps the reference fixed while correcting an entry", () => {
      activityState.data = activities;
      renderTab([note({ canEdit: true, activityGroupId: "12" })]);
      fireEvent.click(
        screen.getByRole("button", { name: "Aktionen zur Notiz" }),
      );
      fireEvent.click(screen.getByRole("menuitem", { name: "Bearbeiten" }));
      expect(screen.queryByLabelText("Bezug")).not.toBeInTheDocument();
      fireEvent.click(screen.getByLabelText("Wer sieht die Notiz?"));
      expect(
        screen.getByRole("option", { name: "Angebotsleitung" }),
      ).toBeInTheDocument();
    });
  });
});
