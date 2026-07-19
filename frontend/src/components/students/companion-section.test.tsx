import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import "@testing-library/jest-dom/vitest";
import type { StudentCompanion } from "~/lib/student-companion-api";

const { mockFetchCompanions, mockUpdateCompanions, mockFetchStudents } =
  vi.hoisted(() => ({
    mockFetchCompanions: vi.fn(),
    mockUpdateCompanions: vi.fn(),
    mockFetchStudents: vi.fn(),
  }));

vi.mock("~/lib/student-companion-api", async () => {
  const actual = await vi.importActual<
    typeof import("~/lib/student-companion-api")
  >("~/lib/student-companion-api");
  return {
    ...actual,
    fetchStudentCompanions: mockFetchCompanions,
    updateStudentCompanions: mockUpdateCompanions,
  };
});

vi.mock("~/lib/student-api", () => ({
  fetchStudents: (...args: unknown[]) => mockFetchStudents(...args),
}));

import CompanionSection from "./companion-section";

const lina: StudentCompanion = {
  companion_student_id: 7,
  first_name: "Lina",
  last_name: "Klein",
  weekdays: ["mon", "tue"],
};

describe("CompanionSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchCompanions.mockResolvedValue([]);
    mockFetchStudents.mockResolvedValue({ students: [] });
  });

  afterEach(() => {
    cleanup();
  });

  it("shows a loader while the companions are loading", () => {
    mockFetchCompanions.mockReturnValueOnce(new Promise(() => undefined));

    const { container } = render(<CompanionSection studentId="1" />);

    expect(container.querySelector(".animate-spin")).toBeInTheDocument();
  });

  it("shows the empty state when no link exists", async () => {
    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(
        screen.getByText("Noch keine Verknüpfung hinterlegt."),
      ).toBeInTheDocument(),
    );
    expect(mockFetchCompanions).toHaveBeenCalledWith("1");
  });

  it("shows a German error when loading fails", async () => {
    mockFetchCompanions.mockRejectedValueOnce(new Error("API down"));

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(
        screen.getByText("Laufgemeinschaft konnte nicht geladen werden."),
      ).toBeInTheDocument(),
    );
  });

  it("renders the loaded companions with a count badge", async () => {
    mockFetchCompanions.mockResolvedValueOnce([lina]);

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );
    expect(screen.getByText("1 Kind")).toBeInTheDocument();
  });

  it("ticks only the weekdays the link applies to", async () => {
    mockFetchCompanions.mockResolvedValueOnce([lina]);

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );

    expect(screen.getByLabelText("Lina Klein: Montag")).toBeChecked();
    expect(screen.getByLabelText("Lina Klein: Dienstag")).toBeChecked();
    expect(screen.getByLabelText("Lina Klein: Mittwoch")).not.toBeChecked();
  });

  it("hides the editing affordances in read-only mode", async () => {
    mockFetchCompanions.mockResolvedValueOnce([lina]);

    render(<CompanionSection studentId="1" canEdit={false} />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );

    expect(
      screen.queryByRole("button", { name: "Kind hinzufügen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Lina Klein entfernen" }),
    ).not.toBeInTheDocument();
    expect(screen.getByLabelText("Lina Klein: Montag")).toBeDisabled();
  });

  it("does not offer saving while nothing changed", async () => {
    mockFetchCompanions.mockResolvedValueOnce([lina]);

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );

    expect(
      screen.queryByRole("button", { name: /Laufgemeinschaft speichern/ }),
    ).not.toBeInTheDocument();
  });

  it("saves the toggled weekdays through the API", async () => {
    mockFetchCompanions.mockResolvedValueOnce([lina]);
    mockUpdateCompanions.mockResolvedValueOnce([
      { ...lina, weekdays: ["mon", "tue", "wed"] },
    ]);

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByLabelText("Lina Klein: Mittwoch"));

    const saveButton = await screen.findByRole("button", {
      name: /Laufgemeinschaft speichern/,
    });
    fireEvent.click(saveButton);

    await waitFor(() => expect(mockUpdateCompanions).toHaveBeenCalled());
    expect(mockUpdateCompanions).toHaveBeenCalledWith("1", [
      { companion_student_id: 7, weekdays: ["mon", "tue", "wed"] },
    ]);

    // Saving re-baselines the list, so the button disappears again.
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: /Laufgemeinschaft speichern/ }),
      ).not.toBeInTheDocument(),
    );
  });

  it("refuses to save a link without a weekday", async () => {
    mockFetchCompanions.mockResolvedValueOnce([
      { ...lina, weekdays: ["mon"] as StudentCompanion["weekdays"] },
    ]);

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByLabelText("Lina Klein: Montag"));
    fireEvent.click(
      await screen.findByRole("button", {
        name: /Laufgemeinschaft speichern/,
      }),
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "Bitte mindestens einen Wochentag für Lina Klein auswählen.",
        ),
      ).toBeInTheDocument(),
    );
    expect(mockUpdateCompanions).not.toHaveBeenCalled();
  });

  it("surfaces the backend message when saving fails", async () => {
    mockFetchCompanions.mockResolvedValueOnce([lina]);
    mockUpdateCompanions.mockRejectedValueOnce(
      new Error("Ein Kind kann nicht mit sich selbst laufen."),
    );

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );

    fireEvent.click(screen.getByLabelText("Lina Klein: Mittwoch"));
    fireEvent.click(
      await screen.findByRole("button", {
        name: /Laufgemeinschaft speichern/,
      }),
    );

    await waitFor(() =>
      expect(
        screen.getByText("Ein Kind kann nicht mit sich selbst laufen."),
      ).toBeInTheDocument(),
    );
  });

  it("removes a companion from the list", async () => {
    mockFetchCompanions.mockResolvedValueOnce([lina]);

    render(<CompanionSection studentId="1" />);

    await waitFor(() =>
      expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Lina Klein entfernen" }),
    );

    expect(screen.queryByText("Lina Klein")).not.toBeInTheDocument();
    expect(
      await screen.findByRole("button", {
        name: /Laufgemeinschaft speichern/,
      }),
    ).toBeInTheDocument();
  });

  describe("child picker", () => {
    async function openPicker() {
      render(<CompanionSection studentId="1" />);
      await waitFor(() =>
        expect(
          screen.getByText("Noch keine Verknüpfung hinterlegt."),
        ).toBeInTheDocument(),
      );
      fireEvent.click(screen.getByRole("button", { name: /Kind hinzufügen/ }));
      return screen.getByLabelText("Kind für die Laufgemeinschaft suchen");
    }

    it("asks for at least two characters before searching", async () => {
      const input = await openPicker();

      fireEvent.change(input, { target: { value: "L" } });

      expect(
        screen.getByText("Mindestens 2 Zeichen eingeben."),
      ).toBeInTheDocument();
      // Give the debounce a chance to fire — it must not.
      await new Promise((resolve) => setTimeout(resolve, 400));
      expect(mockFetchStudents).not.toHaveBeenCalled();
    });

    it("searches with a debounce and hides the child itself", async () => {
      mockFetchStudents.mockResolvedValue({
        students: [
          { id: "1", name: "Eigenes Kind", school_class: "1a" },
          { id: "7", name: "Lina Klein", school_class: "2b" },
        ],
      });

      const input = await openPicker();
      fireEvent.change(input, { target: { value: "in" } });

      await waitFor(() => expect(mockFetchStudents).toHaveBeenCalled());
      expect(mockFetchStudents).toHaveBeenCalledWith({
        search: "in",
        page_size: 25,
      });

      expect(await screen.findByText("Lina Klein")).toBeInTheDocument();
      expect(screen.queryByText("Eigenes Kind")).not.toBeInTheDocument();
    });

    it("shows an empty result message when nothing matches", async () => {
      mockFetchStudents.mockResolvedValue({ students: [] });

      const input = await openPicker();
      fireEvent.change(input, { target: { value: "zz" } });

      expect(
        await screen.findByText("Kein Kind gefunden."),
      ).toBeInTheDocument();
    });

    it("adds the picked child with the whole week ticked", async () => {
      mockFetchStudents.mockResolvedValue({
        students: [{ id: "7", name: "Lina Klein", school_class: "2b" }],
      });

      const input = await openPicker();
      fireEvent.change(input, { target: { value: "Li" } });

      fireEvent.click(await screen.findByText("Lina Klein"));

      await waitFor(() =>
        expect(screen.getByLabelText("Lina Klein: Freitag")).toBeChecked(),
      );
      expect(screen.getByLabelText("Lina Klein: Montag")).toBeChecked();
      expect(
        screen.queryByLabelText("Kind für die Laufgemeinschaft suchen"),
      ).not.toBeInTheDocument();

      fireEvent.click(
        await screen.findByRole("button", {
          name: /Laufgemeinschaft speichern/,
        }),
      );
      await waitFor(() =>
        expect(mockUpdateCompanions).toHaveBeenCalledWith("1", [
          {
            companion_student_id: 7,
            weekdays: ["mon", "tue", "wed", "thu", "fri"],
          },
        ]),
      );
    });

    it("leaves already linked children out of the results", async () => {
      mockFetchCompanions.mockResolvedValueOnce([lina]);
      mockFetchStudents.mockResolvedValue({
        students: [
          { id: "7", name: "Lina Klein", school_class: "2b" },
          { id: "9", name: "Tom Berg", school_class: "2b" },
        ],
      });

      render(<CompanionSection studentId="1" />);
      await waitFor(() =>
        expect(screen.getByText("Lina Klein")).toBeInTheDocument(),
      );

      fireEvent.click(screen.getByRole("button", { name: /Kind hinzufügen/ }));
      fireEvent.change(
        screen.getByLabelText("Kind für die Laufgemeinschaft suchen"),
        { target: { value: "ei" } },
      );

      const result = await screen.findByText("Tom Berg");
      // "Lina Klein" is still on screen as the existing row, but must not be
      // offered as a pickable result — she is already linked.
      const resultList = result.closest("ul");
      expect(resultList).not.toBeNull();
      expect(resultList!.querySelectorAll("li")).toHaveLength(1);
    });

    it("closes the picker without adding anything", async () => {
      const input = await openPicker();
      expect(input).toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));

      expect(
        screen.queryByLabelText("Kind für die Laufgemeinschaft suchen"),
      ).not.toBeInTheDocument();
      expect(
        screen.getByText("Noch keine Verknüpfung hinterlegt."),
      ).toBeInTheDocument();
    });
  });
});
