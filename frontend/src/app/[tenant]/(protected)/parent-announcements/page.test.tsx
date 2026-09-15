import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import {
  updateAnnouncement,
  type Announcement,
} from "~/lib/parent-announcements-api";
import ParentAnnouncementsPage from "./page";

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
  useTenantSlugSafe: () => null,
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
