import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockFetchGuardians, mockOpenThread, mockFetchStudents } = vi.hoisted(
  () => ({
    mockFetchGuardians: vi.fn(),
    mockOpenThread: vi.fn(),
    mockFetchStudents: vi.fn(),
  }),
);

vi.mock("~/lib/parent-messages-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/parent-messages-api")>()),
  fetchGuardians: mockFetchGuardians,
  openThread: mockOpenThread,
}));

vi.mock("~/lib/student-api", () => ({
  fetchStudents: mockFetchStudents,
}));

import { NewMessageModal } from "./new-message-modal";

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

  it("does not report a failed child search as no match", async () => {
    mockFetchStudents.mockRejectedValueOnce(
      new ApiError("search kaputt", 503, { code: "general.unavailable" }),
    );
    render(<NewMessageModal onClose={vi.fn()} onOpened={vi.fn()} />);

    fireEvent.change(screen.getByPlaceholderText("Kind suchen..."), {
      target: { value: "Max" },
    });

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Suche nach Kindern"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kein Kind gefunden.")).not.toBeInTheDocument();
  });
});
