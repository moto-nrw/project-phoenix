import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const {
  mockUseSWRAuth,
  mockCreate,
  mockPublish,
  mockToastSuccess,
  mockPush,
  mockMutate,
} = vi.hoisted(() => ({
  mockUseSWRAuth: vi.fn(),
  mockCreate: vi.fn(),
  mockPublish: vi.fn(),
  mockToastSuccess: vi.fn(),
  mockPush: vi.fn(),
  mockMutate: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: mockPush }),
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
import type { Announcement } from "~/lib/parent-announcements-api";
import SchoolParentLettersPage from "./page";

const sentLetter = {
  id: "11",
  title: "Wandertag",
  body: "Feste Schuhe",
  status: "published",
  delivery_mode: "letter",
  published_at: "2026-09-08T08:00:00Z",
  sender_name: "Lena Lehrerin",
  sender_school_class: "3b",
} as Announcement;

function mockData(classes: string[], letters: Announcement[]) {
  mockUseSWRAuth.mockImplementation((key: string | null) => {
    if (key === "school-parent-letter-classes") {
      return { data: { classes }, error: undefined, isLoading: false };
    }
    return {
      data: letters,
      error: undefined,
      isLoading: false,
      mutate: mockMutate,
    };
  });
}

function renderPage() {
  return render(<SchoolParentLettersPage />, { wrapper: ToastProvider });
}

describe("Elternbriefe in moto schule (#4800)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockMutate.mockResolvedValue(undefined);
  });

  it("lists the own letters with their class and opens one", () => {
    mockData(["3b"], [sentLetter]);
    renderPage();

    expect(screen.getAllByText("Wandertag").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Klasse 3b").length).toBeGreaterThan(0);
    fireEvent.click(screen.getAllByText("Wandertag")[0]!);
    expect(mockPush).toHaveBeenCalledWith("/school/elternbriefe/11");
  });

  it("writes and sends a signed consent to the one class of the Lehrkraft", async () => {
    mockData(["3b"], []);
    mockCreate.mockResolvedValue({ ...sentLetter, id: "12", status: "draft" });
    mockPublish.mockResolvedValue({ ...sentLetter, id: "12" });
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: /Neuer Elternbrief/ }));
    fireEvent.change(screen.getByLabelText(/Betreff/), {
      target: { value: "Zoobesuch" },
    });
    fireEvent.change(screen.getByLabelText(/^Text/), {
      target: { value: "Wir fahren am Freitag in den Zoo." },
    });
    fireEvent.click(screen.getByLabelText(/Einverständnis mit Unterschrift/));
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    await waitFor(() => expect(mockPublish).toHaveBeenCalled());
    expect(mockCreate).toHaveBeenCalledWith(
      expect.objectContaining({
        title: "Zoobesuch",
        delivery_mode: "declaration",
        declaration_requires_signature: true,
        targets: [{ target_type: "class", ref_text: "3b" }],
      }),
      "/api/school/parent-letters",
    );
    expect(mockPublish).toHaveBeenCalledWith(
      "12",
      "/api/school/parent-letters",
    );
    expect(mockToastSuccess).toHaveBeenCalledWith(
      expect.stringContaining("Klasse 3b"),
    );
  });

  it("names the missing class instead of sending", async () => {
    mockData(["3b", "4a"], []);
    renderPage();

    fireEvent.click(screen.getByRole("button", { name: /Neuer Elternbrief/ }));
    fireEvent.click(screen.getByRole("button", { name: "Senden" }));

    expect(
      await screen.findByText("Bitte wählen Sie eine Klasse."),
    ).toBeInTheDocument();
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it("says why nothing can be written without an own class", () => {
    mockData([], []);
    renderPage();

    expect(
      screen.getByText(/Ihnen ist keine Klasse zugeordnet/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Neuer Elternbrief/ }),
    ).toBeDisabled();
  });
});
