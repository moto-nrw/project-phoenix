// #6593: Während der Brief gesendet wird, lässt sich das Panel nicht
// schließen; eine späte Antwort schließt und füllt kein anderes Formular.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const {
  mockUseSWRAuth,
  mockCreate,
  mockPublish,
  mockToastSuccess,
  mockMutate,
} = vi.hoisted(() => ({
  mockUseSWRAuth: vi.fn(),
  mockCreate: vi.fn(),
  mockPublish: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockMutate: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => "/school/elternbriefe",
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { scope: "school" } },
    status: "authenticated",
  }),
}));
vi.mock("~/lib/swr", () => ({ useSWRAuth: mockUseSWRAuth }));
vi.mock("~/lib/school-url", () => ({ schoolPath: (path: string) => path }));
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({
    success: mockToastSuccess,
    error: vi.fn(),
    warning: vi.fn(),
  }),
}));
vi.mock("~/lib/parent-announcements-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-announcements-api")>()),
  fetchAnnouncements: vi.fn(),
  createAnnouncement: mockCreate,
  publishAnnouncement: mockPublish,
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { unavailableApiError } from "~/lib/api-error";
import type { Announcement } from "~/lib/parent-announcements-api";
import SchoolParentLettersPage from "./page";

const TITLE = "Neuer Elternbrief";
const FOOTER = ["Abbrechen", "Wird gesendet…"];

const draftLetter = {
  id: "12",
  title: "Zoobesuch",
  body: "Wir fahren in den Zoo.",
  status: "draft",
  delivery_mode: "letter",
  sender_school_class: "3b",
} as Announcement;

function startSending() {
  mockUseSWRAuth.mockImplementation((key: string | null) => ({
    data: key === "school-parent-letter-classes" ? { classes: ["3b"] } : [],
    error: undefined,
    isLoading: false,
    mutate: mockMutate,
  }));
  mockCreate.mockResolvedValue(draftLetter);
  const answer = deferred<Announcement>();
  mockPublish.mockReturnValue(answer.promise);
  const view = render(<SchoolParentLettersPage />, { wrapper: ToastProvider });

  fireEvent.click(screen.getByRole("button", { name: /Neuer Elternbrief/ }));
  fireEvent.change(screen.getByLabelText(/Betreff/), {
    target: { value: "Zoobesuch" },
  });
  fireEvent.change(screen.getByLabelText(/^Text/), {
    target: { value: "Wir fahren in den Zoo." },
  });
  fireEvent.click(screen.getByRole("button", { name: "Senden" }));
  return { ...view, answer };
}

describe("Elternbriefe: Senden sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mockMutate.mockResolvedValue(undefined);
  });

  it("locks the panel while sending and unlocks it with the error after a failure", async () => {
    const { answer } = startSending();
    await screen.findByRole("button", { name: "Wird gesendet…" });

    expectSlideOverSaveLocked(TITLE, FOOTER);

    await act(async () => {
      answer.reject(unavailableApiError(new TypeError("Failed to fetch")));
    });

    expect(
      await screen.findByRole("button", { name: /Wiederholen/ }),
    ).toBeInTheDocument();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByLabelText(/Betreff/)).toHaveValue("Zoobesuch");
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });

  it("closes the panel after the letter is sent", async () => {
    const { answer } = startSending();
    await screen.findByRole("button", { name: "Wird gesendet…" });

    await act(async () => {
      answer.resolve({ ...draftLetter, status: "published" });
    });

    expect(mockToastSuccess).toHaveBeenCalledWith(
      expect.stringContaining("Klasse 3b"),
    );
    // Anker: die Seite ist wieder bedienbar.
    expect(
      screen.getByRole("button", { name: /Neuer Elternbrief/ }),
    ).toBeEnabled();
    expect(screen.queryByRole("heading", { name: TITLE })).toBeNull();
  });

  it("refreshes the list but shows no error when the answer arrives after unmount", async () => {
    const { answer, unmount } = startSending();
    await screen.findByRole("button", { name: "Wird gesendet…" });
    expect(mockPublish).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.reject(unavailableApiError(new TypeError("Failed to fetch")));
    });

    expect(mockMutate).toHaveBeenCalledOnce();
    expect(mockToastSuccess).not.toHaveBeenCalled();
  });
});
