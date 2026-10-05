import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  updateAnnouncement,
  type Announcement,
} from "~/lib/parent-announcements-api";
import ParentAnnouncementsPage from "./page";
import { stashAnnouncementStudents } from "~/lib/announcement-prefill";

const { searchParams, pushMock, updateUrlParamsMock, listState } = vi.hoisted(
  () => ({
    searchParams: new URLSearchParams(),
    pushMock: vi.fn(),
    updateUrlParamsMock: vi.fn(),
    listState: {
      data: undefined as Announcement[] | undefined,
      isLoading: false,
      error: null as Error | null,
    },
  }),
);

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
  usePathname: () => "/parent-announcements",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));

vi.mock("~/hooks/useUpdateUrlParams", () => ({
  useUpdateUrlParams: () => updateUrlParamsMock,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: pushMock, replace: vi.fn(), back: vi.fn() }),
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => null,
  useTenantSlugSafe: () => "testschule",
  useTenantRoutingModeSafe: () => "subdomain",
}));

vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key === "parent-announcements-list"
      ? { ...listState, mutate: vi.fn() }
      : { data: undefined, isLoading: false, error: null, mutate: vi.fn() },
}));

vi.mock("~/lib/parent-announcements-api", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("~/lib/parent-announcements-api")>();
  return {
    ...actual,
    fetchAnnouncements: vi.fn(() => Promise.resolve([])),
    updateAnnouncement: vi.fn(() => Promise.resolve({ id: "1" })),
    fetchAnnouncementAttachments: vi.fn(() =>
      Promise.resolve({
        attachments: [],
        max_count: 5,
        max_bytes: 1,
        editable: true,
      }),
    ),
  };
});

vi.mock("~/lib/api", () => ({
  groupService: { getGroups: vi.fn(() => Promise.resolve([])) },
  studentService: { getSchoolClasses: vi.fn(() => Promise.resolve([])) },
}));

vi.mock("~/lib/activity-api", () => ({
  fetchActivities: vi.fn(() => Promise.resolve([])),
}));

const base: Announcement = {
  id: "1",
  title: "Sommerfest",
  body: "Am Freitag.",
  priority: "info",
  requires_acknowledgement: false,
  send_email: false,
  status: "draft",
  active: true,
  created_at: "2026-09-01T10:00:00Z",
  updated_at: "2026-09-01T10:00:00Z",
  targets: [{ target_type: "school_all" }],
  response_type: "none",
  options: [],
  delivery_mode: "standard",
  email_audience: "portal_only",
};

const poll: Announcement = {
  ...base,
  id: "2",
  title: "Kommt Ihr Kind?",
  status: "published",
  published_at: "2026-09-02T10:00:00Z",
  response_type: "single_choice",
  options: [
    { id: "a", label: "Ja" },
    { id: "b", label: "Nein" },
  ],
};

const letter: Announcement = {
  ...base,
  id: "3",
  title: "Elternbrief zum Sommerfest",
  status: "published",
  delivery_mode: "letter",
};

const draftPoll: Announcement = {
  ...poll,
  id: "4",
  title: "Kommt Ihr Kind am Montag?",
  status: "draft",
  published_at: undefined,
};

describe("ParentAnnouncementsPage (#3115)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.delete("art");
    searchParams.delete("bearbeiten");
    searchParams.delete("search");
    searchParams.delete("status");
    listState.data = [base, poll];
    listState.isLoading = false;
    listState.error = null;
  });

  it("opens the object page from the row and from „Anzeigen“", async () => {
    render(<ParentAnnouncementsPage />);

    fireEvent.change(screen.getAllByPlaceholderText("Titel suchen…")[0]!, {
      target: { value: "Sommer" },
    });

    // Die Tabelle rendert jede Zeile für Desktop und Telefon; die erste
    // Ausprägung reicht für den Klick.
    fireEvent.click((await screen.findAllByText("Sommerfest"))[0]!);
    expect(pushMock).toHaveBeenCalledWith(
      `/parent-announcements/1?from=${encodeURIComponent("/parent-announcements?art=mitteilungen&search=Sommer")}`,
    );

    fireEvent.click(
      screen.getAllByRole("button", { name: "Aktionen für Sommerfest" })[0]!,
    );
    fireEvent.click(screen.getByRole("menuitem", { name: "Anzeigen" }));
    expect(pushMock).toHaveBeenCalledTimes(2);
  });

  it("reads the tab from the address and writes it back on change", async () => {
    searchParams.set("art", "umfragen");
    render(<ParentAnnouncementsPage />);

    expect(
      (await screen.findAllByText("Kommt Ihr Kind?")).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText("Sommerfest")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("tab", { name: /Mitteilungen/ }));
    expect(updateUrlParamsMock).toHaveBeenCalledWith({ art: null });

    fireEvent.click((await screen.findAllByText("Kommt Ihr Kind?"))[0]!);
    expect(pushMock).toHaveBeenCalledWith(
      `/parent-announcements/2?from=${encodeURIComponent("/parent-announcements?art=umfragen")}`,
    );
  });

  it("labels the summary on the Elternbriefe tab correctly", async () => {
    searchParams.set("art", "elternbriefe");
    listState.data = [letter];
    render(<ParentAnnouncementsPage />);

    expect(
      await screen.findByText("1 Elternbrief · 1 veröffentlicht"),
    ).toBeInTheDocument();
  });

  it("opens the wizard for a draft requested via ?bearbeiten= and clears the parameter", async () => {
    searchParams.set("bearbeiten", "1");
    render(<ParentAnnouncementsPage />);

    expect(
      await screen.findByText("Elternmitteilung bearbeiten"),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(updateUrlParamsMock).toHaveBeenCalledWith({ bearbeiten: null }),
    );
  });

  it("ignores ?bearbeiten= for a published announcement", async () => {
    searchParams.set("art", "umfragen");
    searchParams.set("bearbeiten", "2");
    render(<ParentAnnouncementsPage />);

    await waitFor(() =>
      expect(updateUrlParamsMock).toHaveBeenCalledWith({ bearbeiten: null }),
    );
    expect(screen.queryByText("Umfrage bearbeiten")).not.toBeInTheDocument();
  });
});

describe("ParentAnnouncementsPage: scheduled reminder (#3162)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.delete("art");
    searchParams.delete("bearbeiten");
    listState.isLoading = false;
    listState.error = null;
  });

  it("shows the reminder state in the list", async () => {
    listState.data = [
      {
        ...base,
        status: "published",
        published_at: "2026-09-02T10:00:00Z",
        reminder_at: "2026-09-24T06:00:00Z",
      },
    ];
    render(<ParentAnnouncementsPage />);

    expect(
      (await screen.findAllByText("Erinnerung am 24.09.2026, 08:00 Uhr"))
        .length,
    ).toBeGreaterThan(0);
  });

  it("offers the reminder in the Mitteilung wizard but not in the Umfrage wizard", async () => {
    listState.data = [
      { ...base, reminder_at: "2026-09-24T06:00:00Z" },
      draftPoll,
    ];
    searchParams.set("bearbeiten", "1");
    const { unmount } = render(<ParentAnnouncementsPage />);

    expect(
      await screen.findByText("Elternmitteilung bearbeiten"),
    ).toBeInTheDocument();
    expect(screen.getByText("Erinnern am (optional)")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die E-Mail hat nur Titel und Link. Den Text sehen Eltern im Eltern-Portal.",
      ),
    ).toBeInTheDocument();
    unmount();

    searchParams.set("art", "umfragen");
    searchParams.set("bearbeiten", "4");
    render(<ParentAnnouncementsPage />);

    expect(await screen.findByText("Umfrage bearbeiten")).toBeInTheDocument();
    expect(
      screen.queryByText("Erinnern am (optional)"),
    ).not.toBeInTheDocument();
  });

  // Terminabstimmung (#3861): a list copied from a spreadsheet becomes one
  // answer per line instead of one long label.
  it("splits a pasted list into one answer per line", async () => {
    listState.data = [draftPoll];
    searchParams.set("art", "umfragen");
    searchParams.set("bearbeiten", "4");
    render(<ParentAnnouncementsPage />);

    const second = await screen.findByRole("textbox", { name: "Antwort 2" });
    fireEvent.paste(second, {
      clipboardData: {
        getData: () => "Di 14.10.\t15:00\nDi 14.10.\t15:15\n\n",
      },
    });

    expect(screen.getByRole("textbox", { name: "Antwort 2" })).toHaveValue(
      "Nein",
    );
    expect(screen.getByRole("textbox", { name: "Antwort 3" })).toHaveValue(
      "Di 14.10. 15:00",
    );
    expect(screen.getByRole("textbox", { name: "Antwort 4" })).toHaveValue(
      "Di 14.10. 15:15",
    );
    expect(
      screen.queryByRole("textbox", { name: "Antwort 5" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText(/Zwei bis 60 Antworten\./)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Als Entwurf speichern" }),
    );
    await waitFor(() =>
      expect(updateAnnouncement).toHaveBeenCalledWith(
        "4",
        expect.objectContaining({
          options: ["Ja", "Nein", "Di 14.10. 15:00", "Di 14.10. 15:15"],
        }),
      ),
    );
  });

  it("says how many pasted lines did not fit", async () => {
    listState.data = [draftPoll];
    searchParams.set("art", "umfragen");
    searchParams.set("bearbeiten", "4");
    render(<ParentAnnouncementsPage />);

    const second = await screen.findByRole("textbox", { name: "Antwort 2" });
    const lines = Array.from({ length: 61 }, (_, i) => `Termin ${i + 1}`);
    fireEvent.paste(second, {
      clipboardData: { getData: () => lines.join("\n") },
    });

    expect(
      await screen.findByText(
        "Es passen höchstens 60 Antworten. 3 Zeilen wurden nicht übernommen.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("textbox", { name: "Antwort 60" })).toHaveValue(
      "Termin 58",
    );
    expect(
      screen.getByRole("button", { name: "Antwort hinzufügen" }),
    ).toBeDisabled();
  });

  it("saves a draft with an elapsed reminder", async () => {
    listState.data = [
      {
        ...base,
        reminder_at: "2026-09-08T06:00:00Z",
      },
    ];
    searchParams.set("bearbeiten", "1");
    render(<ParentAnnouncementsPage />);

    fireEvent.click(await screen.findByRole("button", { name: "Weiter" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Als Entwurf speichern" }),
    );

    await waitFor(() => expect(updateAnnouncement).toHaveBeenCalledTimes(1));
  });

  it("shows a validation error for an incomplete reminder time in a draft", async () => {
    listState.data = [
      {
        ...base,
        reminder_at: "2026-09-24T06:00:00Z",
      },
    ];
    searchParams.set("bearbeiten", "1");
    render(<ParentAnnouncementsPage />);

    fireEvent.change(await screen.findByRole("textbox", { name: /Uhrzeit/ }), {
      target: { value: "8" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(await screen.findByRole("alert")).toHaveTextContent("Uhrzeit");
    expect(updateAnnouncement).not.toHaveBeenCalled();
  });
});

describe("ParentAnnouncementsPage: children handed over by another page (#3379)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.delete("art");
    searchParams.delete("bearbeiten");
    searchParams.delete("neu");
    searchParams.delete("vorbelegung");
    window.sessionStorage.clear();
    listState.data = [base];
    listState.isLoading = false;
    listState.error = null;
  });

  it("opens a new announcement addressed to the handed-over children", async () => {
    const token = stashAnnouncementStudents("testschule", [
      { id: "7", name: "Mia Arslan" },
      { id: "8", name: "Ben Yilmaz" },
    ]);
    expect(token).not.toBeNull();
    searchParams.set("neu", "kinder");
    searchParams.set("vorbelegung", token!);
    render(<ParentAnnouncementsPage />);

    expect(
      await screen.findByText("Neue Elternmitteilung"),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(updateUrlParamsMock).toHaveBeenCalledWith({
        neu: null,
        vorbelegung: null,
      }),
    );
    // Read once: a reload must not address the same families again.
    expect(
      window.sessionStorage.getItem(`moto:announcement-prefill:${token}`),
    ).toBeNull();

    fireEvent.change(screen.getByRole("textbox", { name: "Titel" }), {
      target: { value: "Anmeldung fehlt noch" },
    });
    fireEvent.change(screen.getByRole("textbox", { name: "Text" }), {
      target: { value: "Bitte melden Sie Ihr Kind an." },
    });
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(await screen.findByText("Mia Arslan")).toBeInTheDocument();
    expect(screen.getByText("Ben Yilmaz")).toBeInTheDocument();
  });

  it("stays on the list when nothing was handed over", async () => {
    searchParams.set("neu", "kinder");
    searchParams.set("vorbelegung", "missing");
    render(<ParentAnnouncementsPage />);

    await waitFor(() =>
      expect(updateUrlParamsMock).toHaveBeenCalledWith({
        neu: null,
        vorbelegung: null,
      }),
    );
    expect(screen.queryByText("Neue Elternmitteilung")).not.toBeInTheDocument();
  });
});

describe("ParentAnnouncementsPage: Einverständnisse (#3430)", () => {
  const declarationDraft: Announcement = {
    ...base,
    id: "9",
    title: "Ausflug in den Zoo",
    delivery_mode: "declaration",
    declaration_kind: "consent",
    declaration_signers: "any",
    declaration_revocable: true,
    declaration_requires_password: false,
  };

  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.delete("art");
    searchParams.delete("bearbeiten");
    searchParams.delete("search");
    searchParams.delete("status");
    listState.isLoading = false;
    listState.error = null;
  });

  it("lists Einverständnisse on their own tab", async () => {
    searchParams.set("art", "erklaerungen");
    listState.data = [base, declarationDraft];
    render(<ParentAnnouncementsPage />);

    expect(
      (await screen.findAllByText("Ausflug in den Zoo")).length,
    ).toBeGreaterThan(0);
    expect(screen.queryByText("Sommerfest")).not.toBeInTheDocument();
    expect(
      screen.getByText("1 Einverständnis · 0 veröffentlicht"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("tab", { name: /Einverständnisse/ }),
    ).toBeInTheDocument();
    expect(
      screen.getAllByRole("button", { name: "Neues Einverständnis erstellen" })
        .length,
    ).toBeGreaterThan(0);
  });

  it("sends the Einverständnis settings and offers no Kenntnisnahme, poll or read confirmation", async () => {
    searchParams.set("art", "erklaerungen");
    searchParams.set("bearbeiten", "9");
    listState.data = [declarationDraft];
    render(<ParentAnnouncementsPage />);

    expect(
      await screen.findByText("Einverständnis bearbeiten"),
    ).toBeInTheDocument();
    // Neither a poll nor a read confirmation belongs to an Einverständnis.
    expect(
      screen.queryByText("Lesebestätigung erforderlich"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Antwortmöglichkeiten")).not.toBeInTheDocument();
    expect(
      screen.getByText(/Verlangt ein Gesetz eine Erklärung auf Papier/),
    ).toBeInTheDocument();

    expect(
      screen.getByText(/Dann schreiben Sie einen Elternbrief mit/),
    ).toBeInTheDocument();
    // Only one kind is left: no choice, no Kenntnisnahme anywhere.
    expect(screen.queryByText(/Kenntnis/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Was sollen die Eltern tun?"),
    ).not.toBeInTheDocument();
    const revocable = screen.getByRole("checkbox", {
      name: /Widerruf erlauben/,
    });
    fireEvent.click(revocable);
    fireEvent.click(
      screen.getByRole("radio", { name: /Alle sorgeberechtigten Personen/ }),
    );
    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Passwort vor dem Antworten abfragen/,
      }),
    );
    expect(screen.getByText("Frist (optional)")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    // Pending enrollments are no audience for an Einverständnis.
    expect((await screen.findAllByText("Ganze Schule")).length).toBeGreaterThan(
      0,
    );
    expect(screen.getByText("Wer soll gefragt werden?")).toBeInTheDocument();
    expect(screen.queryByText("Offene Anmeldungen")).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Als Entwurf speichern" }),
    );

    await waitFor(() => expect(updateAnnouncement).toHaveBeenCalledTimes(1));
    expect(updateAnnouncement).toHaveBeenCalledWith(
      "9",
      expect.objectContaining({
        delivery_mode: "declaration",
        response_type: "none",
        requires_acknowledgement: false,
        email_audience: "portal_only",
        response_deadline: null,
        declaration_kind: "consent",
        declaration_signers: "all",
        declaration_revocable: false,
        declaration_requires_password: true,
      }),
    );
  });

  it("keeps a consent revocable by default", async () => {
    searchParams.set("art", "erklaerungen");
    searchParams.set("bearbeiten", "9");
    listState.data = [declarationDraft];
    render(<ParentAnnouncementsPage />);

    expect(
      await screen.findByRole("checkbox", { name: /Widerruf erlauben/ }),
    ).toBeChecked();
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
    fireEvent.click(
      await screen.findByRole("button", { name: "Als Entwurf speichern" }),
    );

    await waitFor(() =>
      expect(updateAnnouncement).toHaveBeenCalledWith(
        "9",
        expect.objectContaining({
          declaration_kind: "consent",
          declaration_signers: "any",
          declaration_revocable: true,
          declaration_requires_password: false,
        }),
      ),
    );
  });

  it("shows the files read-only once a version exists", async () => {
    searchParams.set("art", "erklaerungen");
    searchParams.set("bearbeiten", "9");
    listState.data = [
      { ...declarationDraft, declaration_locked_attachments: true },
    ];
    render(<ParentAnnouncementsPage />);

    expect(
      await screen.findByText(
        "Dieses Einverständnis war schon veröffentlicht. Die Dateien bleiben deshalb gleich. Für andere Dateien legen Sie ein neues Einverständnis an.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Datei auswählen" }),
    ).not.toBeInTheDocument();
  });
});
