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

import { unavailableApiError } from "~/lib/api-error";
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
      return {
        data: { classes },
        error: undefined,
        isLoading: false,
        mutate: mockMutate,
      };
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
    vi.resetAllMocks();
    mockMutate.mockResolvedValue(undefined);
  });

  it("lists the own letters with their class and opens one", () => {
    mockData(["3b"], [sentLetter]);
    renderPage();

    expect(screen.getAllByText("Wandertag").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Klasse 3b").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Gesendet").length).toBeGreaterThan(0);
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

  it.each(["Senden", "Wiederholen"])(
    "retries the created letter via %s after a lost response",
    async (action) => {
      mockData(["3b"], []);
      mockCreate.mockResolvedValue({
        ...sentLetter,
        id: "12",
        status: "draft",
      });
      mockPublish
        .mockRejectedValueOnce(
          unavailableApiError(new TypeError("Failed to fetch")),
        )
        .mockResolvedValueOnce({ ...sentLetter, id: "12" });
      renderPage();
      fireEvent.click(
        screen.getByRole("button", { name: /Neuer Elternbrief/ }),
      );
      fireEvent.change(screen.getByLabelText(/Betreff/), {
        target: { value: "Zoobesuch" },
      });
      fireEvent.change(screen.getByLabelText(/^Text/), {
        target: { value: "Wir fahren in den Zoo." },
      });
      fireEvent.click(screen.getByRole("button", { name: "Senden" }));
      const retry = await screen.findByRole("button", { name: /Wiederholen/ });
      expect(screen.getByLabelText(/Betreff/)).toBeDisabled();
      fireEvent.click(
        action === "Wiederholen"
          ? retry
          : screen.getByRole("button", { name: "Senden" }),
      );
      await waitFor(() => expect(mockToastSuccess).toHaveBeenCalled());
      expect(mockCreate).toHaveBeenCalledTimes(1);
      expect(mockPublish).toHaveBeenCalledTimes(2);
      expect(mockPublish.mock.calls.map((call) => call[0])).toEqual([
        "12",
        "12",
      ]);
    },
  );

  it("disables creation while classes are loading", () => {
    mockUseSWRAuth.mockReturnValue({
      data: undefined,
      error: undefined,
      isLoading: true,
      mutate: mockMutate,
    });
    renderPage();
    expect(
      screen.getByRole("button", { name: /Neuer Elternbrief/ }),
    ).toBeDisabled();
  });

  it("shows class loading errors and retries before enabling creation", async () => {
    const failure = unavailableApiError(new TypeError("Failed to fetch"));
    let loaded = false;
    mockMutate.mockImplementation(() => {
      loaded = true;
    });
    mockUseSWRAuth.mockImplementation((key: string | null) =>
      key === "school-parent-letter-classes"
        ? {
            data: loaded ? { classes: ["3b"] } : undefined,
            error: loaded ? undefined : failure,
            isLoading: false,
            mutate: mockMutate,
          }
        : {
            data: [sentLetter],
            error: undefined,
            isLoading: false,
            mutate: mockMutate,
          },
    );
    const view = renderPage();
    const retry = await screen.findByRole("button", { name: /Wiederholen/ });
    expect(screen.getByRole("alert")).toHaveTextContent(/Klassen/);
    expect(
      screen.getByRole("button", { name: /Neuer Elternbrief/ }),
    ).toBeDisabled();
    fireEvent.click(retry);
    expect(mockMutate).toHaveBeenCalledTimes(1);
    view.rerender(<SchoolParentLettersPage />);
    expect(
      screen.getByRole("button", { name: /Neuer Elternbrief/ }),
    ).toBeEnabled();
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
