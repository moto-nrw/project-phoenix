// #6593: Während der Elternsprechtag gespeichert wird, lässt sich das Panel
// nicht schließen; eine späte Antwort schließt nichts und wählt keinen Tag.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { mockUseSWRAuth, mockCreate, mockReplace, mockMutate } = vi.hoisted(
  () => ({
    mockUseSWRAuth: vi.fn(),
    mockCreate: vi.fn(),
    mockReplace: vi.fn(),
    mockMutate: vi.fn(),
  }),
);

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace, push: vi.fn() }),
  usePathname: () => "/school/elternsprechtag",
  useSearchParams: () => new URLSearchParams(""),
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { scope: "school" } },
    status: "authenticated",
  }),
}));
vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);
vi.mock("~/lib/swr", () => ({ useSWRAuth: mockUseSWRAuth }));
vi.mock("~/lib/school-url", () => ({ schoolPath: (path: string) => path }));
vi.mock("~/lib/parent-conferences", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-conferences")>()),
  fetchSchoolParentConferences: vi.fn(),
  createSchoolParentConference: mockCreate,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { unavailableApiError } from "~/lib/api-error";
import type { ParentConference } from "~/lib/parent-conferences";
import SchoolParentConferencePage from "./page";

const TITLE = "Neuer Elternsprechtag";
const FOOTER = ["Abbrechen", "Wird gespeichert…"];

const created: ParentConference = {
  id: "6",
  school_class: "2a",
  date: "2026-11-12",
  start_time: "14:00",
  end_time: "17:00",
  slot_minutes: 15,
  breaks: [],
  location: "",
  booked_count: 0,
  slots: [],
};

async function startCreating() {
  mockUseSWRAuth.mockReturnValue({
    data: { classes: ["2a"], conferences: [] },
    error: undefined,
    isLoading: false,
    mutate: mockMutate,
  });
  const answer = deferred<ParentConference>();
  mockCreate.mockReturnValue(answer.promise);
  const view = render(<SchoolParentConferencePage />, {
    wrapper: ToastProvider,
  });
  fireEvent.click(
    screen.getByRole("button", { name: /Neuer Elternsprechtag/ }),
  );
  fireEvent.change(screen.getByLabelText("Tag"), {
    target: { value: "2026-11-12" },
  });
  fireEvent.change(screen.getByLabelText("Raum (freiwillig)"), {
    target: { value: "Raum 12" },
  });
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  await screen.findByRole("button", { name: "Wird gespeichert…" });
  return { ...view, answer };
}

describe("Elternsprechtag: Speichern sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mockMutate.mockResolvedValue(undefined);
  });

  it("locks the panel while saving and unlocks it with the error after a failure", async () => {
    const { answer } = await startCreating();

    expectSlideOverSaveLocked(TITLE, FOOTER);

    await act(async () => {
      answer.reject(unavailableApiError(new TypeError("Failed to fetch")));
    });

    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByLabelText("Raum (freiwillig)")).toHaveValue("Raum 12");
    expect(mockReplace).not.toHaveBeenCalled();
  });

  it("closes the panel and selects the new day after saving", async () => {
    const { answer } = await startCreating();

    await act(async () => {
      answer.resolve(created);
    });

    expect(mockReplace).toHaveBeenCalledWith(
      "/school/elternsprechtag?sprechtag=6",
      { scroll: false },
    );
    expect(
      screen.getByRole("button", { name: /Neuer Elternsprechtag/ }),
    ).toBeEnabled();
    expect(screen.queryByRole("heading", { name: TITLE })).toBeNull();
  });

  it("refreshes the list but selects nothing when the answer arrives after unmount", async () => {
    const { answer, unmount } = await startCreating();

    unmount();
    await act(async () => {
      answer.resolve(created);
    });

    expect(mockMutate).toHaveBeenCalledOnce();
    expect(mockReplace).not.toHaveBeenCalled();
  });
});
