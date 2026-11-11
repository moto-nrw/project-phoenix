// #6593: Speichern und das Entfernen eines Anhangs sperren das Panel; eine
// späte Antwort frischt nur die Liste auf.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { searchParams, listState, listMutate, updateUrlParams } = vi.hoisted(
  () => {
    const params = new URLSearchParams();
    return {
      searchParams: params,
      listState: { data: [] as unknown[], isLoading: false, error: null },
      listMutate: vi.fn(),
      // Entfernt `?bearbeiten=` wie die echte Adresse, sonst öffnet die Seite
      // den Entwurf nach dem Schließen gleich wieder.
      updateUrlParams: (patch: Record<string, string | null>) => {
        for (const [key, value] of Object.entries(patch)) {
          if (value === null) params.delete(key);
          else params.set(key, value);
        }
      },
    };
  },
);

vi.mock("next/navigation", () => ({
  useSearchParams: () => searchParams,
  usePathname: () => "/parent-announcements",
  useRouter: () => ({ push: vi.fn(), replace: vi.fn() }),
}));
vi.mock("~/hooks/useUpdateUrlParams", () => ({
  useUpdateUrlParams: () => updateUrlParams,
}));
vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: vi.fn(), replace: vi.fn(), back: vi.fn() }),
}));
vi.mock("~/lib/tenant-context", () => ({
  useTenantSafe: () => null,
  useTenantSlugSafe: () => "testschule",
  useTenantRoutingModeSafe: () => "subdomain",
}));
vi.mock("~/lib/swr", () => ({
  useSWRAuth: (key: string | null) =>
    key === "parent-announcements-list"
      ? { ...listState, mutate: listMutate }
      : { data: undefined, isLoading: false, error: null, mutate: vi.fn() },
}));
vi.mock("~/lib/parent-announcements-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-announcements-api")>()),
  fetchAnnouncements: vi.fn(() => Promise.resolve([])),
  updateAnnouncement: vi.fn(),
  deleteAnnouncementAttachment: vi.fn(),
  fetchAnnouncementAttachments: vi.fn(),
}));
vi.mock("~/lib/api", () => ({
  groupService: { getGroups: vi.fn(() => Promise.resolve([])) },
}));
vi.mock("~/lib/student-api", () => ({
  fetchSchoolClasses: vi.fn(() => Promise.resolve([])),
}));
vi.mock("~/lib/activity-api", () => ({
  fetchActivities: vi.fn(() => Promise.resolve([])),
}));

import { ApiError } from "~/lib/api-error";
import {
  deleteAnnouncementAttachment,
  fetchAnnouncementAttachments,
  updateAnnouncement,
  type Announcement,
} from "~/lib/parent-announcements-api";
import ParentAnnouncementsPage from "./page";

const TITLE = "Elternmitteilung bearbeiten";

const draft: Announcement = {
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

async function startSavingDraft() {
  const answer = deferred<Announcement>();
  vi.mocked(updateAnnouncement).mockReturnValue(answer.promise);
  const view = render(<ParentAnnouncementsPage />);
  fireEvent.click(await screen.findByRole("button", { name: "Weiter" }));
  fireEvent.click(
    await screen.findByRole("button", { name: "Als Entwurf speichern" }),
  );
  await screen.findByRole("button", { name: "Wird gespeichert…" });
  return { ...view, answer };
}

describe("Elternmitteilungen: Speichern sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    searchParams.set("bearbeiten", "1");
    listState.data = [draft];
    listMutate.mockResolvedValue(undefined);
    vi.mocked(fetchAnnouncementAttachments).mockResolvedValue({
      attachments: [],
      max_count: 5,
      max_bytes: 1,
      editable: true,
    });
  });

  it("locks the panel while saving and unlocks it with the error after a failure", async () => {
    const { answer } = await startSavingDraft();

    expectSlideOverSaveLocked(TITLE, [
      "Zurück",
      "Wird gespeichert…",
      "Veröffentlichen",
    ]);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(
      await screen.findByRole("button", { name: "Wiederholen" }),
    ).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(
      screen.getByRole("button", { name: "Als Entwurf speichern" }),
    ).toBeEnabled();
    expect(listMutate).not.toHaveBeenCalled();
  });

  it("closes the panel after the draft is saved", async () => {
    const { answer } = await startSavingDraft();

    await act(async () => {
      answer.resolve(draft);
    });

    expect(listMutate).toHaveBeenCalledOnce();
    // Anker: die Seite ist wieder bedienbar.
    expect(
      screen.getByRole("button", { name: "Neue Elternmitteilung erstellen" }),
    ).toBeEnabled();
    expect(screen.queryByRole("heading", { name: TITLE })).toBeNull();
  });

  it("refreshes the list when the answer arrives after unmount", async () => {
    const { answer, unmount } = await startSavingDraft();

    unmount();
    await act(async () => {
      answer.resolve(draft);
    });

    expect(listMutate).toHaveBeenCalledOnce();
  });

  it("locks the panel while an attachment is removed", async () => {
    vi.mocked(fetchAnnouncementAttachments).mockResolvedValue({
      attachments: [
        {
          id: "a1",
          filename: "Plan.pdf",
          size_bytes: 1000,
          content_type: "application/pdf",
          uploaded_at: "2026-09-01T10:00:00Z",
        },
      ],
      max_count: 5,
      max_bytes: 1,
      editable: true,
    });
    const answer = deferred<void>();
    vi.mocked(deleteAnnouncementAttachment).mockReturnValue(answer.promise);
    render(<ParentAnnouncementsPage />);

    const remove = await screen.findByRole("button", {
      name: "Anhang entfernen: Plan.pdf",
    });
    // Anker: die Anhangsliste zeigt Plan.pdf.
    expect(remove).toBeEnabled();
    fireEvent.click(remove);

    expectSlideOverSaveLocked(TITLE, ["Abbrechen", "Weiter"]);
    expect(remove).toBeDisabled();

    await act(async () => {
      answer.resolve();
    });

    expectSlideOverUnlocked(TITLE);
    expect(screen.queryByText("Plan.pdf")).toBeNull();
    expect(screen.getByRole("button", { name: "Weiter" })).toBeEnabled();
  });
});
