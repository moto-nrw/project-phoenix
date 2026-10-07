import {
  fireEvent,
  render as renderComponent,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { ReactElement } from "react";

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";

// Fehler laufen als Toast über den gemeinsamen Fehlerweg; die Karte selbst
// zeigt keinen Fehlerkasten.
function render(ui: ReactElement) {
  return renderComponent(ui, { wrapper: ToastProvider });
}

async function expectErrorToast(pattern: RegExp) {
  expect(
    await screen.findByRole("alert", { name: /^Fehler:/ }),
  ).toHaveTextContent(pattern);
}

import { MasterDataReviewItem } from "./master-data-review-item";
import {
  decideMasterDataChangeRequest,
  type StaffMasterDataChange,
} from "~/lib/master-data-review-api";

vi.mock("~/lib/master-data-review-api", () => ({
  decideMasterDataChangeRequest: vi.fn(),
}));

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

describe("MasterDataReviewItem", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockDecide.mockReset();
  });

  it("renders German field labels and approves with a trimmed reason", async () => {
    mockDecide.mockResolvedValue(row({ status: "approved" }));
    const onDecided = vi.fn();

    render(
      <>
        <MasterDataReviewItem row={row()} onDecided={onDecided} />
        <MasterDataReviewItem
          row={row({
            id: "101",
            field_key: "allowed_departure_modes",
            old_value: { mon: ["pickup"] },
            new_value: { mon: ["bus", "alone"] },
          })}
          onDecided={onDecided}
        />
      </>,
    );

    expect(screen.getAllByText("Lara Beispiel")).toHaveLength(2);
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
      expect(onDecided).toHaveBeenCalledWith("Die Änderung ist übernommen."),
    );
    expect(onDecided).toHaveBeenCalledTimes(1);
  });

  it("shows German labels for contact method and language values", () => {
    render(
      <>
        <MasterDataReviewItem
          row={row({
            field_key: "preferred_contact_method",
            old_value: "email",
            new_value: "mobile",
          })}
          onDecided={vi.fn()}
        />
        <MasterDataReviewItem
          row={row({
            id: "101",
            field_key: "language_preference",
            old_value: "de",
            new_value: "tr",
          })}
          onDecided={vi.fn()}
        />
      </>,
    );

    expect(screen.getAllByText("Lara Beispiel")).toHaveLength(2);
    expandAll();
    expect(screen.getByText("E-Mail")).toBeInTheDocument();
    expect(screen.getByText("Mobiltelefon")).toBeInTheDocument();
    expect(screen.getByText("Deutsch")).toBeInTheDocument();
    expect(screen.getByText("Türkisch")).toBeInTheDocument();
  });

  it("renders birthdays in German date format, invalid dates raw", () => {
    render(
      <>
        <MasterDataReviewItem
          row={row({
            field_key: "birthday",
            old_value: "2018-05-03",
            new_value: "2018-06-14",
          })}
          onDecided={vi.fn()}
        />
        <MasterDataReviewItem
          row={row({
            id: "101",
            field_key: "birthday",
            old_value: null,
            new_value: "kein-datum",
          })}
          onDecided={vi.fn()}
        />
      </>,
    );

    expect(screen.getAllByText("Lara Beispiel")).toHaveLength(2);
    expandAll();
    expect(screen.getByText("03.05.2018")).toBeInTheDocument();
    expect(screen.getByText("14.06.2018")).toBeInTheDocument();
    expect(screen.getByText("kein-datum")).toBeInTheDocument();
  });

  it("falls back to raw values for unknown keys without crashing", () => {
    render(
      <>
        <MasterDataReviewItem
          row={row({
            field_key: "allowed_departure_modes",
            old_value: { mon: ["teleport"], someday: ["pickup"] },
            new_value: { tue: "not-an-array" },
          })}
          onDecided={vi.fn()}
        />
        <MasterDataReviewItem
          row={row({
            id: "101",
            field_key: "preferred_contact_method",
            old_value: "carrier_pigeon",
            new_value: "email",
          })}
          onDecided={vi.fn()}
        />
        <MasterDataReviewItem
          row={row({
            id: "102",
            field_key: "language_preference",
            old_value: "xx",
            new_value: "de",
          })}
          onDecided={vi.fn()}
        />
      </>,
    );

    expect(screen.getAllByText("Lara Beispiel")).toHaveLength(3);
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

  it("rejects requests and reports the rejection notice via onDecided", async () => {
    mockDecide.mockResolvedValue(row({ status: "rejected" }));
    const onDecided = vi.fn();

    render(
      <MasterDataReviewItem
        row={row({ field_key: "unknown_field" })}
        onDecided={onDecided}
      />,
    );

    expandAll();
    // Unknown field keys fall back to the raw key in the summary.
    expect(screen.getAllByText(/unknown_field/).length).toBeGreaterThan(0);
    fireEvent.click(screen.getByRole("button", { name: "Ablehnen" }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("100", false, undefined),
    );
    expect(onDecided).toHaveBeenCalledWith("Die Änderung ist abgelehnt.");
  });

  it("shows a decision error without calling onDecided", async () => {
    mockDecide.mockRejectedValueOnce(
      new ApiError("boom", 500, { code: "general.server" }),
    );
    const onDecided = vi.fn();

    render(<MasterDataReviewItem row={row()} onDecided={onDecided} />);

    expandAll();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await expectErrorToast(
      /Die Anfrage konnte nicht bearbeitet werden\. Bitte versuchen Sie es später erneut\./,
    );
    expect(onDecided).not.toHaveBeenCalled();
    expect(screen.getByText("Lara Beispiel")).toBeInTheDocument();
  });

  it("names the next step from the catalog for a request already decided", async () => {
    mockDecide.mockRejectedValueOnce(
      new ApiError("not pending", 409, {
        code: "students.change_request_not_pending",
      }),
    );

    render(<MasterDataReviewItem row={row()} onDecided={vi.fn()} />);

    expandAll();
    fireEvent.click(screen.getByRole("button", { name: "Freigeben" }));

    await expectErrorToast(/Die Anfrage ist nicht mehr offen/);
    expect(screen.queryByText("not pending")).toBeNull();
  });
});
