import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PickupExtension } from "~/lib/pickup-extension-api";

const mockToast = vi.hoisted(() => ({
  success: vi.fn(),
  error: vi.fn(),
  warning: vi.fn(),
  info: vi.fn(),
}));
vi.mock("~/contexts/ToastContext", () => ({
  useToast: () => mockToast,
}));
vi.mock("~/lib/care-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/care-request-review-api")>();
  return { ...actual, decideCareScheduleChangeRequest: vi.fn() };
});
vi.mock("~/lib/pickup-extension-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/pickup-extension-api")>();
  return {
    ...actual,
    fetchPickupExtensions: vi.fn(),
    resolvePickupExtension: vi.fn(),
  };
});

import { PickupExtensionAccessProvider } from "~/components/timetable/pickup-extension-access";
import {
  decideCareScheduleChangeRequest,
  type StaffCareRequest,
} from "~/lib/care-request-review-api";
import {
  fetchPickupExtensions,
  PickupExtensionApiError,
  resolvePickupExtension,
} from "~/lib/pickup-extension-api";
import { CareRequestReviewItem } from "./care-request-review-item";

const mockDecide = vi.mocked(decideCareScheduleChangeRequest);
const mockFetch = vi.mocked(fetchPickupExtensions);
const mockResolve = vi.mocked(resolvePickupExtension);

function laterPickupRow(): StaffCareRequest {
  return {
    id: "300",
    student_id: "42",
    first_name: "Mia",
    last_name: "Beispiel",
    status: "pending",
    request_kind: "pickup_change",
    affected_blocks: [],
    impact_available: true,
    impact_token: "impact-v1",
    diff: [
      {
        label: "15.09.2026 · Abholzeit",
        old: "14:45",
        new: "16:00",
        care_kind: "pickup",
      },
    ],
    created_at: "2026-09-08T12:00:00Z",
  };
}

const task: PickupExtension = {
  id: "7",
  studentId: "42",
  studentName: "Mia Beispiel",
  kind: "day",
  date: "2026-09-15",
  previousPickupTime: "14:45",
  pickupTime: "16:00",
  blocks: [
    { id: "31", title: "Freies Spiel", startTime: "14:45", endTime: "16:00" },
  ],
};

function approve() {
  fireEvent.click(screen.getByRole("button", { name: /Mia Beispiel/ }));
  fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));
}

describe("CareRequestReviewItem after a later pickup (#3261)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDecide.mockResolvedValue(laterPickupRow());
  });

  it("fragt nach dem Freigeben nach dem Termin", async () => {
    mockFetch.mockResolvedValue([task]);
    mockResolve.mockResolvedValue(undefined);
    const onDecided = vi.fn();
    render(
      <PickupExtensionAccessProvider value>
        <CareRequestReviewItem row={laterPickupRow()} onDecided={onDecided} />
      </PickupExtensionAccessProvider>,
    );

    approve();

    expect(
      await screen.findByText("Längere Betreuung eintragen"),
    ).toBeInTheDocument();
    expect(mockFetch).toHaveBeenCalledWith("42");
    expect(onDecided).toHaveBeenCalledWith("Abholzeit übernommen");

    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() =>
      expect(screen.queryByText("Längere Betreuung eintragen")).toBeNull(),
    );
    expect(mockResolve).toHaveBeenCalledWith("7", ["31"]);
  });

  // Die freigegebene Anfrage verschwindet sofort aus der Liste (Neuladen per
  // Ereignis). Die Auswahl darf dabei nicht mit der Karte verschwinden.
  it("hält die Auswahl offen, wenn die Zeile aus der Liste fällt", async () => {
    mockFetch.mockResolvedValue([task]);
    function Queue() {
      const [rows, setRows] = useState([laterPickupRow()]);
      return rows.map((row) => (
        <CareRequestReviewItem
          key={row.id}
          row={row}
          onDecided={() => setRows([])}
        />
      ));
    }
    render(
      <PickupExtensionAccessProvider value>
        <Queue />
      </PickupExtensionAccessProvider>,
    );

    approve();

    expect(
      await screen.findByText("Längere Betreuung eintragen"),
    ).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Freigeben" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    await waitFor(() =>
      expect(screen.queryByText("Längere Betreuung eintragen")).toBeNull(),
    );
    expect(mockResolve).not.toHaveBeenCalled();
  });

  it("lädt die Auswahl nach einer gleichzeitig geänderten Terminliste neu", async () => {
    const changedTask: PickupExtension = {
      ...task,
      blocks: [
        {
          id: "31",
          title: "Spätdienst",
          startTime: "15:00",
          endTime: "16:00",
        },
      ],
    };
    mockFetch
      .mockResolvedValueOnce([task])
      .mockResolvedValueOnce([changedTask]);
    mockResolve
      .mockRejectedValueOnce(
        new PickupExtensionApiError("gone", 409, "pickup_extension_block_gone"),
      )
      .mockResolvedValueOnce(undefined);
    render(
      <PickupExtensionAccessProvider value>
        <CareRequestReviewItem row={laterPickupRow()} onDecided={vi.fn()} />
      </PickupExtensionAccessProvider>,
    );

    approve();
    await screen.findByText("Längere Betreuung eintragen");
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    expect(await screen.findByText("Spätdienst")).toBeInTheDocument();
    expect(mockFetch).toHaveBeenLastCalledWith("42");
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() =>
      expect(mockResolve).toHaveBeenLastCalledWith("7", ["31"]),
    );
  });

  it("geht ohne offene Frage weiter wie bisher", async () => {
    mockFetch.mockResolvedValue([]);
    const onDecided = vi.fn();
    render(
      <PickupExtensionAccessProvider value>
        <CareRequestReviewItem row={laterPickupRow()} onDecided={onDecided} />
      </PickupExtensionAccessProvider>,
    );

    approve();

    await waitFor(() =>
      expect(onDecided).toHaveBeenCalledWith("Abholzeit übernommen"),
    );
    expect(screen.queryByText("Längere Betreuung eintragen")).toBeNull();
  });

  it("fragt ohne Recht am Betreuungsplan nicht nach", async () => {
    const onDecided = vi.fn();
    render(
      <CareRequestReviewItem row={laterPickupRow()} onDecided={onDecided} />,
    );

    approve();

    await waitFor(() =>
      expect(onDecided).toHaveBeenCalledWith("Abholzeit übernommen"),
    );
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("fragt nach einer Ablehnung nicht nach", async () => {
    const onDecided = vi.fn();
    render(
      <PickupExtensionAccessProvider value>
        <CareRequestReviewItem row={laterPickupRow()} onDecided={onDecided} />
      </PickupExtensionAccessProvider>,
    );

    fireEvent.click(screen.getByRole("button", { name: /Mia Beispiel/ }));
    fireEvent.change(screen.getByPlaceholderText(/Begründung/), {
      target: { value: "Passt nicht" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    await waitFor(() =>
      expect(onDecided).toHaveBeenCalledWith("Abholzeit-Anfrage abgelehnt"),
    );
    expect(mockFetch).not.toHaveBeenCalled();
  });

  it("geht weiter, wenn die offenen Fragen nicht laden", async () => {
    mockFetch.mockRejectedValue(new Error("403"));
    const onDecided = vi.fn();
    render(
      <PickupExtensionAccessProvider value>
        <CareRequestReviewItem row={laterPickupRow()} onDecided={onDecided} />
      </PickupExtensionAccessProvider>,
    );

    approve();

    await waitFor(() =>
      expect(onDecided).toHaveBeenCalledWith("Abholzeit übernommen"),
    );
  });
});
