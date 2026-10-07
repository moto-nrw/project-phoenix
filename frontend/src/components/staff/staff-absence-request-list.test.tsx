import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffAbsenceRequestRow } from "~/lib/staff-api";
import { catalogText } from "~/test/error-catalog-text";

import { StaffAbsenceRequestList } from "./staff-absence-request-list";

const { listRequests, approve } = vi.hoisted(() => ({
  listRequests: vi.fn(),
  approve: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffAbsenceService: { listRequests, approve },
}));

vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/contexts/ToastContext")>()),
  useToast: () => ({ success: vi.fn(), error: vi.fn() }),
  useApiErrorDisplay: () => ({ show: actionErrors.show }),
}));
const actionErrors = vi.hoisted(() => ({ show: vi.fn() }));

function openRow(): StaffAbsenceRequestRow {
  return {
    id: "17",
    staff_id: "42",
    staff_name: "Mira Muster",
    absence_type: "vacation",
    date_start: "2027-07-10",
    date_end: "2027-07-11",
    half_day: false,
    note: "Vertretung ist geklärt",
    status: "requested",
    working_days: 2,
    requested_at: "2027-06-02T08:00:00Z",
  };
}

function decidedRow(): StaffAbsenceRequestRow {
  return {
    ...openRow(),
    id: "18",
    status: "declined",
    decision_note: "Zu viele Abwesenheiten in der Woche",
    approved_by: "2",
    approved_at: "2027-06-03T09:00:00Z",
    decided_by_name: "Lea Leitung",
  };
}

describe("StaffAbsenceRequestList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("zeigt offene Anträge mit Namen und Entscheidungs-Schaltflächen", async () => {
    listRequests.mockResolvedValue([openRow()]);

    render(
      <StaffAbsenceRequestList
        view="open"
        filters={{ search: "", types: [] }}
      />,
    );

    expect(await screen.findByText("Mira Muster")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Genehmigen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Rückfrage" }),
    ).toBeInTheDocument();
    expect(listRequests).toHaveBeenCalledWith("open", {
      search: "",
      types: [],
    });
  });

  it("reicht Suche und Art-Filter an den Endpunkt durch", async () => {
    listRequests.mockResolvedValue([]);

    render(
      <StaffAbsenceRequestList
        view="open"
        filters={{ search: "Mira", types: ["training"] }}
      />,
    );

    await waitFor(() =>
      expect(listRequests).toHaveBeenCalledWith("open", {
        search: "Mira",
        types: ["training"],
      }),
    );
  });

  it("zeigt in der Historie Status, Entscheiderin und Begründung", async () => {
    listRequests.mockResolvedValue([decidedRow()]);

    render(
      <StaffAbsenceRequestList
        view="history"
        filters={{ search: "", types: [] }}
      />,
    );

    expect(await screen.findByText("Abgelehnt")).toBeInTheDocument();
    expect(screen.getByText(/Lea Leitung/)).toBeInTheDocument();
    // Die Begründung steht im aufgeklappten Teil der Zeile.
    fireEvent.click(screen.getByRole("button"));
    expect(
      screen.getByText(/Zu viele Abwesenheiten in der Woche/),
    ).toBeInTheDocument();
    // Entschiedene Anträge sind read-only.
    expect(screen.queryByRole("button", { name: "Genehmigen" })).toBeNull();
  });

  it("zeigt die eigene Abwesenheitsart in der Historie", async () => {
    listRequests.mockResolvedValue([
      {
        ...decidedRow(),
        absence_type: "other",
        absence_type_label: "Regenerationstag",
      },
    ]);

    render(
      <StaffAbsenceRequestList
        view="history"
        filters={{ search: "", types: [] }}
      />,
    );

    expect(await screen.findByText(/Regenerationstag/)).toBeInTheDocument();
    expect(screen.queryByText(/Sonstige/)).toBeNull();
  });

  it("genehmigt einen Antrag und lädt die Liste neu", async () => {
    listRequests.mockResolvedValueOnce([openRow()]).mockResolvedValueOnce([]);
    approve.mockResolvedValue(undefined);

    render(
      <StaffAbsenceRequestList
        view="open"
        filters={{ search: "", types: [] }}
      />,
    );

    await screen.findByText("Mira Muster");
    await userEvent.click(screen.getByRole("button", { name: "Genehmigen" }));

    await waitFor(() => expect(approve).toHaveBeenCalledWith("17"));
    expect(
      await screen.findByText("Keine offenen Anträge."),
    ).toBeInTheDocument();
  });

  it("nennt eine gelöschte Entscheiderin Unbekannt, eine Rücknahme niemanden", async () => {
    listRequests.mockResolvedValue([
      { ...decidedRow(), approved_by: "9", decided_by_name: "" },
      {
        ...decidedRow(),
        id: "19",
        status: "canceled",
        approved_by: "9",
        decided_by_name: "",
        approved_at: "2027-06-03T09:00:00Z",
        updated_at: "2027-06-04T09:00:00Z",
      },
    ]);

    render(
      <StaffAbsenceRequestList
        view="history"
        filters={{ search: "", types: [] }}
      />,
    );

    // Zeitpunkt und Person stehen im aufgeklappten Teil, also beide Zeilen
    // öffnen. Die Liste ist eine gemeinsame Fläche; eine einzelne Zeile wird
    // über ihren eigenen Anker gegriffen, nicht mehr über die Fläche.
    await screen.findByText("Zurückgezogen");
    for (const row of screen.getAllByRole("button")) fireEvent.click(row);

    expect(screen.getByText(/von Unbekannt/)).toBeInTheDocument();
    const canceledCard = screen
      .getByText("Zurückgezogen")
      .closest("[data-request-row]");
    expect(canceledCard).toBeInTheDocument();
    expect(screen.getAllByText(/von /)).toHaveLength(1);
    expect(canceledCard).toHaveTextContent("Entschieden am 04.06.2027");
    expect(canceledCard).not.toHaveTextContent("Entschieden am 03.06.2027");
  });

  it("nennt einen leeren Namen Unbekannt", async () => {
    listRequests.mockResolvedValue([{ ...openRow(), staff_name: "" }]);

    render(
      <StaffAbsenceRequestList
        view="open"
        filters={{ search: "", types: [] }}
      />,
    );

    expect(await screen.findByText("Unbekannt")).toBeInTheDocument();
  });

  it("zeigt einen Ladefehler an Stelle der Liste, nicht als leere Liste", async () => {
    listRequests.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server", instance: "req-l" }),
    );
    listRequests.mockResolvedValueOnce([openRow()]);

    render(
      <StaffAbsenceRequestList
        view="open"
        filters={{ search: "", types: [] }}
      />,
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Anträge"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Keine offenen Anträge.")).toBeNull();
    expect(
      screen.getByRole("button", { name: "Vorgangskennung kopieren" }),
    ).toHaveTextContent("req-l");

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("Mira Muster")).toBeInTheDocument();
  });

  it("meldet eine abgelehnte Genehmigung über den gemeinsamen Fehlerweg", async () => {
    listRequests.mockResolvedValue([openRow()]);
    const refused = new ApiError("quota", 409, {
      code: "workforce.vacation_quota_exceeded",
    });
    approve.mockRejectedValue(refused);

    render(
      <StaffAbsenceRequestList
        view="open"
        filters={{ search: "", types: [] }}
      />,
    );

    fireEvent.click(await screen.findByRole("button", { name: "Genehmigen" }));
    await waitFor(() =>
      expect(actionErrors.show).toHaveBeenCalledWith(
        refused,
        expect.objectContaining({ object: "die Anfrage" }),
      ),
    );
  });
});
