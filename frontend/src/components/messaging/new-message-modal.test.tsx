import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const {
  mockFetchGuardians,
  mockOpenThread,
  mockFetchStudents,
  mockFetchReachableStudentIds,
} = vi.hoisted(() => ({
  mockFetchGuardians: vi.fn(),
  mockOpenThread: vi.fn(),
  mockFetchStudents: vi.fn(),
  mockFetchReachableStudentIds: vi.fn(),
}));

vi.mock("~/lib/parent-messages-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-messages-api")>()),
  fetchGuardians: mockFetchGuardians,
  fetchReachableStudentIds: mockFetchReachableStudentIds,
  openThread: mockOpenThread,
}));

vi.mock("~/lib/student-api", () => ({
  fetchStudents: mockFetchStudents,
}));

import type { Student } from "~/lib/student-helpers";
import { NewMessageModal } from "./new-message-modal";

function student(id: string, name: string, schoolClass = "2a"): Student {
  return {
    id,
    name,
    school_class: schoolClass,
    current_location: "home",
  } as Student;
}

const names = () =>
  screen
    .getAllByRole("radio")
    .map((radio) => radio.closest("label")?.textContent);

const anna = {
  account_id: "7",
  name: "Anna Muster",
  relationship_type: "parent",
  is_primary: true,
};

function renderPreset(onOpened = vi.fn()) {
  render(
    <NewMessageModal
      onClose={vi.fn()}
      onOpened={onOpened}
      presetStudentId="42"
      presetStudentName="Max Muster"
    />,
  );
  return { onOpened };
}

describe("NewMessageModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchReachableStudentIds.mockResolvedValue(
      new Set(["1", "2", "3", "4"]),
    );
  });

  // #2517: kein „kein Eltern-Zugang hinterlegt“ für eine Liste, die nicht
  // geladen wurde; Katalogtext und Wiederholen stattdessen.
  it("shows a failed guardian load with a retry", async () => {
    mockFetchGuardians
      .mockRejectedValueOnce(
        new ApiError("guardians kaputt", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce([anna]);
    renderPreset();

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Eltern"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Für dieses Kind ist kein Eltern-Zugang hinterlegt."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Anna Muster")).toBeInTheDocument();
  });

  it("keeps a failed open in the panel and retries it", async () => {
    mockFetchGuardians.mockResolvedValue([anna]);
    const thread = {
      thread_id: "t1",
      student_id: "42",
      student_name: "Max Muster",
      guardian_name: "Anna Muster",
      messages: [],
    };
    mockOpenThread
      .mockRejectedValueOnce(
        new ApiError("forbidden", 403, { code: "general.permission" }),
      )
      .mockResolvedValueOnce(thread);
    const { onOpened } = renderPreset();

    fireEvent.click(await screen.findByText("Anna Muster"));

    expect(
      await screen.findByText(
        catalogText("general.permission", "die Unterhaltung"),
      ),
    ).toBeInTheDocument();
    expect(onOpened).not.toHaveBeenCalled();

    fireEvent.click(screen.getByText("Anna Muster"));
    await waitFor(() => expect(onOpened).toHaveBeenCalledWith(thread));
  });

  // #4362: die Liste steht beim Öffnen, die Suche filtert sie nur.
  it("lists every child alphabetically on open and filters from the first character", async () => {
    mockFetchStudents
      .mockResolvedValueOnce({
        students: [
          student("3", "Zoe Weber"),
          student("1", "Ömer Kaya"),
          student("4", "Marie Beier"),
        ],
        pagination: {
          current_page: 1,
          page_size: 100,
          total_pages: 2,
          total_records: 4,
        },
      })
      .mockResolvedValueOnce({
        students: [student("2", "Anna Beier", "3b")],
        pagination: {
          current_page: 2,
          page_size: 100,
          total_pages: 2,
          total_records: 4,
        },
      });
    render(<NewMessageModal onClose={vi.fn()} onOpened={vi.fn()} />);

    expect(await screen.findByText("4 Kinder")).toBeInTheDocument();
    expect(names()).toEqual([
      expect.stringContaining("Anna Beier"),
      expect.stringContaining("Marie Beier"),
      expect.stringContaining("Ömer Kaya"),
      expect.stringContaining("Zoe Weber"),
    ]);
    expect(screen.getByRole("button", { name: "Weiter" })).toBeDisabled();

    fireEvent.change(screen.getByRole("searchbox", { name: "Kind suchen" }), {
      target: { value: "Bei" },
    });

    expect(screen.getByText("2 Kinder")).toBeInTheDocument();
    expect(screen.queryByRole("radio", { name: /Zoe Weber/ })).toBeNull();
    expect(mockFetchStudents).toHaveBeenCalledTimes(2);
  });

  it("opens the parents step for the chosen child", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [student("1", "Marie Beier"), student("2", "Ben Neu")],
    });
    mockFetchGuardians.mockResolvedValue([anna]);
    render(<NewMessageModal onClose={vi.fn()} onOpened={vi.fn()} />);

    fireEvent.click(await screen.findByRole("radio", { name: /Ben Neu/ }));
    fireEvent.click(screen.getByRole("button", { name: "Weiter" }));

    expect(await screen.findByText("Anna Muster")).toBeInTheDocument();
    expect(mockFetchGuardians).toHaveBeenCalledWith("2");
    expect(screen.getByText("Ben Neu")).toBeInTheDocument();
  });

  it("takes the only match straight on with Enter", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [student("1", "Marie Beier"), student("2", "Ben Neu")],
    });
    mockFetchGuardians.mockResolvedValue([anna]);
    render(<NewMessageModal onClose={vi.fn()} onOpened={vi.fn()} />);
    await screen.findByText("2 Kinder");

    const search = screen.getByRole("searchbox", { name: "Kind suchen" });
    fireEvent.change(search, { target: { value: "ben" } });
    expect(screen.getByRole("radio", { name: /Ben Neu/ })).toBeChecked();
    fireEvent.submit(search);

    expect(await screen.findByText("Anna Muster")).toBeInTheDocument();
    expect(mockFetchGuardians).toHaveBeenCalledWith("2");
  });

  it("does not report a failed child load as no match", async () => {
    mockFetchStudents
      .mockRejectedValueOnce(
        new ApiError("load kaputt", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({ students: [student("1", "Marie Beier")] });
    render(<NewMessageModal onClose={vi.fn()} onOpened={vi.fn()} />);

    expect(
      await screen.findByText(catalogText("general.unavailable", "die Kinder")),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Kein Kind/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByRole("radio", { name: /Marie Beier/ }),
    ).toBeInTheDocument();
  });

  // #4362: Kinder ohne erreichbare Eltern stehen in der Liste, sind aber
  // markiert und nicht wählbar; die Elternwahl wäre für sie leer.
  it("marks children without parent access and keeps them out of reach", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [student("1", "Marie Beier"), student("2", "Ben Neu")],
    });
    mockFetchReachableStudentIds.mockResolvedValue(new Set(["1"]));
    render(<NewMessageModal onClose={vi.fn()} onOpened={vi.fn()} />);

    const ben = await screen.findByRole("radio", { name: /Ben Neu/ });
    expect(ben).toBeDisabled();
    expect(ben.closest("label")).toHaveTextContent("Kein Eltern-Zugang");
    expect(screen.getByRole("radio", { name: /Marie Beier/ })).toBeEnabled();
    expect(
      screen.getByText(
        "Bei „Kein Eltern-Zugang“ kann niemand die Nachricht in der Eltern-App lesen. Den Zugang der Eltern legen Sie beim Kind fest.",
      ),
    ).toBeInTheDocument();

    // The only reachable match is taken straight on, never the marked child.
    const search = screen.getByRole("searchbox", { name: "Kind suchen" });
    fireEvent.change(search, { target: { value: "e" } });
    expect(screen.getByRole("radio", { name: /Marie Beier/ })).toBeChecked();
  });

  it("still lists every child when parent access cannot be loaded", async () => {
    mockFetchStudents.mockResolvedValue({
      students: [student("1", "Marie Beier"), student("2", "Ben Neu")],
    });
    mockFetchReachableStudentIds.mockRejectedValue(
      new ApiError("reach kaputt", 500, { code: "general.server" }),
    );
    render(<NewMessageModal onClose={vi.fn()} onOpened={vi.fn()} />);

    expect(await screen.findByRole("radio", { name: /Ben Neu/ })).toBeEnabled();
    expect(screen.getByText("2 Kinder")).toBeInTheDocument();
    expect(screen.queryByText("Kein Eltern-Zugang")).not.toBeInTheDocument();
  });
});
