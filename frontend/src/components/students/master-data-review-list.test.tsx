import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MasterDataReviewList } from "./master-data-review-list";
import {
  decideMasterDataChangeRequest,
  listMasterDataChangeRequests,
  type StaffMasterDataChange,
} from "~/lib/master-data-review-api";

vi.mock("~/lib/master-data-review-api", () => ({
  listMasterDataChangeRequests: vi.fn(),
  decideMasterDataChangeRequest: vi.fn(),
}));

const mockList = vi.mocked(listMasterDataChangeRequests);
const mockDecide = vi.mocked(decideMasterDataChangeRequest);

// Cards are collapsed by default (compact queue); expand every card so the diff
// and the Freigeben/Ablehnen actions render. The header button's accessible
// name contains the child name.
function expandAll() {
  for (const btn of screen.getAllByRole("button", { name: /Lara Beispiel/ })) {
    fireEvent.click(btn);
  }
}

function row(
  overrides: Partial<StaffMasterDataChange> = {},
): StaffMasterDataChange {
  return {
    id: "100",
    student_id: "42",
    first_name: "Lara",
    last_name: "Beispiel",
    target: "person",
    field_key: "first_name",
    old_value: "Lara",
    new_value: "Lea",
    status: "pending",
    created_at: "2026-06-24T12:00:00Z",
    ...overrides,
  };
}

describe("MasterDataReviewList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockReset();
    mockDecide.mockReset();
  });

  it("renders pending requests and approves with a trimmed reason", async () => {
    mockList.mockResolvedValue([
      row(),
      row({
        id: "101",
        field_key: "allowed_departure_modes",
        old_value: { mon: ["pickup"] },
        new_value: { mon: ["bus", "alone"] },
      }),
    ]);
    mockDecide.mockResolvedValue(row({ status: "approved" }));

    render(<MasterDataReviewList />);

    expect(await screen.findAllByText("Lara Beispiel")).toHaveLength(2);
    expandAll();
    expect(screen.getByText("Vorname")).toBeInTheDocument();
    expect(screen.getByText("Montag: Wird abgeholt")).toBeInTheDocument();
    expect(
      screen.getByText("Montag: Fährt Bus / Geht zu Fuß"),
    ).toBeInTheDocument();

    const reasonInputs = screen.getAllByPlaceholderText(
      "Begründung (optional)",
    );
    fireEvent.change(reasonInputs[0]!, { target: { value: " passt " } });
    fireEvent.click(screen.getAllByRole("button", { name: "Freigeben" })[0]!);

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("100", true, "passt"),
    );
    await waitFor(() =>
      expect(screen.queryByText("Vorname")).not.toBeInTheDocument(),
    );
    expect(screen.getByText("Änderung übernommen")).toBeInTheDocument();
  });

  it("shows German labels for contact method and language values", async () => {
    mockList.mockResolvedValue([
      row({
        field_key: "preferred_contact_method",
        old_value: "email",
        new_value: "mobile",
      }),
      row({
        id: "101",
        field_key: "language_preference",
        old_value: "de",
        new_value: "tr",
      }),
    ]);

    render(<MasterDataReviewList />);

    expect(await screen.findAllByText("Lara Beispiel")).toHaveLength(2);
    expandAll();
    expect(screen.getByText("E-Mail")).toBeInTheDocument();
    expect(screen.getByText("Mobiltelefon")).toBeInTheDocument();
    expect(screen.getByText("Deutsch")).toBeInTheDocument();
    expect(screen.getByText("Türkisch")).toBeInTheDocument();
  });

  it("renders birthdays in German date format, invalid dates raw", async () => {
    mockList.mockResolvedValue([
      row({
        field_key: "birthday",
        old_value: "2018-05-03",
        new_value: "2018-06-14",
      }),
      row({
        id: "101",
        field_key: "birthday",
        old_value: null,
        new_value: "kein-datum",
      }),
    ]);

    render(<MasterDataReviewList />);

    expect(await screen.findAllByText("Lara Beispiel")).toHaveLength(2);
    expandAll();
    expect(screen.getByText("03.05.2018")).toBeInTheDocument();
    expect(screen.getByText("14.06.2018")).toBeInTheDocument();
    expect(screen.getByText("kein-datum")).toBeInTheDocument();
  });

  it("falls back to raw values for unknown keys without crashing", async () => {
    mockList.mockResolvedValue([
      row({
        field_key: "allowed_departure_modes",
        old_value: { mon: ["teleport"], someday: ["pickup"] },
        new_value: { tue: "not-an-array" },
      }),
      row({
        id: "101",
        field_key: "preferred_contact_method",
        old_value: "carrier_pigeon",
        new_value: "email",
      }),
      row({
        id: "102",
        field_key: "language_preference",
        old_value: "xx",
        new_value: "de",
      }),
    ]);

    render(<MasterDataReviewList />);

    expect(await screen.findAllByText("Lara Beispiel")).toHaveLength(3);
    expandAll();
    // Unknown mode, weekday, contact, and language keys stay visible as raw values.
    expect(
      screen.getByText("Montag: teleport, someday: Wird abgeholt"),
    ).toBeInTheDocument();
    expect(screen.getByText("Dienstag")).toBeInTheDocument();
    expect(screen.getByText("carrier_pigeon")).toBeInTheDocument();
    expect(screen.getByText("E-Mail")).toBeInTheDocument();
    expect(screen.getByText("xx")).toBeInTheDocument();
    expect(screen.getByText("Deutsch")).toBeInTheDocument();
  });

  it("rejects requests and shows the rejection notice", async () => {
    mockList.mockResolvedValue([row({ field_key: "unknown_field" })]);
    mockDecide.mockResolvedValue(row({ status: "rejected" }));

    render(<MasterDataReviewList />);

    expect(await screen.findByText("unknown_field")).toBeInTheDocument();
    expandAll();
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("100", false, undefined),
    );
    expect(screen.getByText("Änderung abgelehnt")).toBeInTheDocument();
  });

  it("shows load and decision errors without removing the row", async () => {
    mockList.mockRejectedValueOnce(new Error("queue down"));

    const { unmount } = render(<MasterDataReviewList />);

    expect(await screen.findByText("queue down")).toBeInTheDocument();
    unmount();

    mockList.mockResolvedValueOnce([row()]);
    mockDecide.mockRejectedValueOnce(new Error("boom"));
    render(<MasterDataReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expandAll();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    expect(
      await screen.findByText(
        "Die Entscheidung konnte nicht gespeichert werden.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Lara Beispiel")).toBeInTheDocument();
  });

  it("renders the empty state", async () => {
    mockList.mockResolvedValue([]);

    render(<MasterDataReviewList />);

    expect(
      await screen.findByText("Keine offenen Änderungsanfragen."),
    ).toBeInTheDocument();
  });

  it("refetches in place when the sibling queue dispatches change-requests-refresh", async () => {
    // A care-schedule decision changes the departure modes this queue's
    // allowed_departure_modes diffs are computed against; the sibling list emits
    // change-requests-refresh, and this list must refetch so it doesn't show
    // stale "current → requested" values.
    mockList
      .mockResolvedValueOnce([row()])
      .mockResolvedValueOnce([
        row({ id: "102", first_name: "Max", last_name: "Muster" }),
      ]);

    render(<MasterDataReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expect(mockList).toHaveBeenCalledTimes(1);

    await act(async () => {
      window.dispatchEvent(new Event("change-requests-refresh"));
    });

    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("Max Muster")).toBeInTheDocument();
    expect(screen.queryByText("Lara Beispiel")).not.toBeInTheDocument();
  });

  it("refetches in place on the SSE-derived messages-unread-refresh event", async () => {
    // A decision made in another tab or by another staffer never fires
    // change-requests-refresh here; it only arrives as the parent-message pill
    // use-global-sse fans out as messages-unread-refresh. The open queue must
    // refetch so it drops a row the backend has already decided.
    mockList
      .mockResolvedValueOnce([row()])
      .mockResolvedValueOnce([
        row({ id: "102", first_name: "Max", last_name: "Muster" }),
      ]);

    render(<MasterDataReviewList />);

    expect(await screen.findByText("Lara Beispiel")).toBeInTheDocument();
    expect(mockList).toHaveBeenCalledTimes(1);

    await act(async () => {
      window.dispatchEvent(new Event("messages-unread-refresh"));
    });

    await waitFor(() => expect(mockList).toHaveBeenCalledTimes(2));
    expect(await screen.findByText("Max Muster")).toBeInTheDocument();
    expect(screen.queryByText("Lara Beispiel")).not.toBeInTheDocument();
  });
});
