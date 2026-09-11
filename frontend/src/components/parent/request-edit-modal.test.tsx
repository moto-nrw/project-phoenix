import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  listParentRequestEvents,
  updateExcusedRequest,
  updateMasterDataRequest,
} from "~/lib/parent-api";
import { RequestEditModal } from "./request-edit-modal";

vi.mock("~/lib/parent-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return {
    ...actual,
    listParentRequestEvents: vi.fn(),
    updateExcusedRequest: vi.fn(),
    updatePickupChangeRequest: vi.fn(),
    updateMasterDataRequest: vi.fn(),
  };
});

vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

const events = vi.mocked(listParentRequestEvents);
const updateExcused = vi.mocked(updateExcusedRequest);
const updateMasterData = vi.mocked(updateMasterDataRequest);

describe("RequestEditModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    events.mockResolvedValue([
      {
        event_type: "submitted",
        version: "v1",
        created_at: "2026-08-20T08:00:00Z",
      },
      {
        event_type: "guardian_edited",
        version: "v2",
        created_at: "2026-08-24T09:30:00Z",
      },
    ]);
    updateExcused.mockResolvedValue({
      id: "req-1",
      student_id: "1",
      absence_status: "excused",
      status: "pending",
      dates: ["2026-09-01"],
      note: "Zahnarzt",
      created_at: "2026-08-20T08:00:00Z",
      is_self: true,
    });
  });

  // Eine offene Anfrage wird geändert, nicht zurückgezogen (#2267).
  it("offers editing and never a withdrawal", async () => {
    render(
      <RequestEditModal
        studentId="1"
        request={{
          type: "excused",
          id: "req-1",
          dates: ["2026-09-01"],
          note: "Zahnarzt",
        }}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Änderung speichern" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/zurückziehen/i)).not.toBeInTheDocument();
    expect(
      await screen.findByText("Geändert am 24.08.2026"),
    ).toBeInTheDocument();
  });

  it("sends the changed request with the version it was loaded at", async () => {
    const onSaved = vi.fn();
    const onClose = vi.fn();
    render(
      <RequestEditModal
        studentId="1"
        request={{
          type: "excused",
          id: "req-1",
          dates: ["2026-09-01"],
          note: "Zahnarzt",
        }}
        onClose={onClose}
        onSaved={onSaved}
      />,
    );

    await screen.findByText("Geändert am 24.08.2026");
    fireEvent.change(document.querySelector("textarea")!, {
      target: { value: "Doch Kinderarzt" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Änderung speichern" }));

    await waitFor(() =>
      expect(updateExcused).toHaveBeenCalledWith("1", "req-1", {
        dates: ["2026-09-01"],
        note: "Doch Kinderarzt",
        expectedVersion: "v2",
      }),
    );
    expect(onSaved).toHaveBeenCalledTimes(1);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("changes a Stammdaten request through the same dialog", async () => {
    updateMasterData.mockResolvedValue({
      id: "md-1",
      target: "person",
      field_key: "first_name",
      new_value: "Lea",
      status: "pending",
      created_at: "2026-08-20T08:00:00Z",
    });
    render(
      <RequestEditModal
        studentId="1"
        request={{
          type: "master_data",
          id: "md-1",
          label: "Vorname",
          value: "Lena",
        }}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    await screen.findByText("Geändert am 24.08.2026");
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Lea" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Änderung speichern" }));

    await waitFor(() =>
      expect(updateMasterData).toHaveBeenCalledWith("1", "md-1", {
        newValue: "Lea",
        expectedVersion: "v2",
      }),
    );
  });

  // Ohne Änderungsgeschichte bleibt das Bearbeiten möglich; die Fassung geht
  // dann leer raus, das Backend prüft sie nicht.
  it("still saves when the history cannot be loaded", async () => {
    events.mockRejectedValue(new Error("offline"));
    render(
      <RequestEditModal
        studentId="1"
        request={{
          type: "excused",
          id: "req-1",
          dates: ["2026-09-01"],
          note: "Zahnarzt",
        }}
        onClose={vi.fn()}
        onSaved={vi.fn()}
      />,
    );

    expect(screen.queryByText(/Geändert am/)).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Änderung speichern" }));

    await waitFor(() =>
      expect(updateExcused).toHaveBeenCalledWith(
        "1",
        "req-1",
        expect.objectContaining({ expectedVersion: "" }),
      ),
    );
  });
});
