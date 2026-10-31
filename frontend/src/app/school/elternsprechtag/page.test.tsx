import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const { mockUseSWRAuth, mockCreate, mockReplace, mockMutate, mockSearch } =
  vi.hoisted(() => ({
    mockUseSWRAuth: vi.fn(),
    mockCreate: vi.fn(),
    mockReplace: vi.fn(),
    mockMutate: vi.fn(),
    mockSearch: { value: "" },
  }));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: mockReplace, push: vi.fn() }),
  usePathname: () => "/school/elternsprechtag",
  useSearchParams: () => new URLSearchParams(mockSearch.value),
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
import type { ParentConference } from "~/lib/parent-conferences";
import SchoolParentConferencePage from "./page";

const day: ParentConference = {
  id: "5",
  school_class: "2a",
  date: "2026-11-12",
  start_time: "14:00",
  end_time: "14:30",
  slot_minutes: 15,
  breaks: [],
  location: "Raum 12",
  booked_count: 1,
  slots: [
    {
      start_time: "14:00",
      end_time: "14:15",
      booked: true,
      student_id: "9",
      student_name: "Mia Muster",
    },
    { start_time: "14:15", end_time: "14:30", booked: false },
  ],
};

function mockList(classes: string[], conferences: ParentConference[]) {
  mockUseSWRAuth.mockReturnValue({
    data: { classes, conferences },
    error: undefined,
    isLoading: false,
    mutate: mockMutate,
  });
}

function renderPage() {
  return render(<SchoolParentConferencePage />, { wrapper: ToastProvider });
}

describe("Elternsprechtag in moto schule (#4801)", () => {
  beforeEach(() => {
    vi.resetAllMocks();
    mockSearch.value = "";
  });

  it("lists the own days with their occupancy", () => {
    mockList(["2a"], [day]);
    renderPage();
    expect(screen.getByText("Meine Elternsprechtage")).toBeInTheDocument();
    expect(screen.getAllByText("1 von 2").length).toBeGreaterThan(0);
  });

  it("shows who comes when on the chosen day, ready to print", () => {
    mockSearch.value = "sprechtag=5";
    mockList(["2a"], [day]);
    renderPage();
    expect(screen.getAllByText("Mia Muster").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Frei").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: /Drucken/ })).toBeInTheDocument();
  });

  it("offers no new day without an assigned class", () => {
    mockList([], []);
    renderPage();
    expect(
      screen.getByRole("button", { name: /Neuer Elternsprechtag/ }),
    ).toBeDisabled();
    expect(
      screen.getByText(/Ihnen ist keine Klasse zugeordnet/),
    ).toBeInTheDocument();
  });

  it("creates a day for the own class", async () => {
    mockList(["2a"], []);
    mockCreate.mockResolvedValue({ ...day, id: "6", booked_count: 0 });
    renderPage();
    fireEvent.click(
      screen.getByRole("button", { name: /Neuer Elternsprechtag/ }),
    );
    fireEvent.change(screen.getByLabelText("Tag"), {
      target: { value: "2026-11-12" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mockCreate).toHaveBeenCalled());
    expect(mockCreate.mock.calls[0]?.[0]).toMatchObject({
      school_class: "2a",
      date: "2026-11-12",
      start_time: "14:00",
      end_time: "17:00",
      slot_minutes: 15,
      breaks: [],
    });
  });
});
