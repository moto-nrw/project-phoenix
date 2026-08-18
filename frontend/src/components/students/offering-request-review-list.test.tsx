import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OfferingRequestReviewList } from "./offering-request-review-list";
import {
  OfferingRequestApiError,
  decideOfferingChangeRequest,
  listOfferingChangeRequests,
  type StaffOfferingRequest,
} from "~/lib/offering-request-review-api";

// Mock only the two network functions; keep the real error class so the
// component's `err instanceof OfferingRequestApiError` branch resolves.
vi.mock("~/lib/offering-request-review-api", async (importActual) => {
  const actual =
    await importActual<typeof import("~/lib/offering-request-review-api")>();
  return {
    ...actual,
    listOfferingChangeRequests: vi.fn(),
    decideOfferingChangeRequest: vi.fn(),
  };
});

const mockList = vi.mocked(listOfferingChangeRequests);
const mockDecide = vi.mocked(decideOfferingChangeRequest);

function request(
  overrides: Partial<StaffOfferingRequest> = {},
): StaffOfferingRequest {
  return {
    id: "77",
    student_id: "42",
    student_name: "Lara Beispiel",
    status: "pending",
    effective_from: "2027-02-01",
    diff: [
      {
        offering_id: "1",
        label: "Regelbetreuung",
        old: "Mo, Di, Mi",
        new: "Mo, Di",
      },
    ],
    created_at: "2026-07-30T09:00:00Z",
    ...overrides,
  };
}

// Cards are collapsed by default; expand so the diff and the actions render.
function expandAll() {
  for (const btn of screen.getAllByRole("button", { name: /Lara Beispiel/ })) {
    fireEvent.click(btn);
  }
}

describe("OfferingRequestReviewList", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockList.mockResolvedValue([request()]);
  });

  it("lists a pending request with its effective date and diff", async () => {
    render(<OfferingRequestReviewList />);

    expect(await screen.findByText(/Lara Beispiel/)).toBeInTheDocument();
    expect(screen.getByText(/ab 01\.02\.2027/)).toBeInTheDocument();
    expandAll();
    expect(screen.getByText("Mo, Di, Mi")).toBeInTheDocument();
    expect(screen.getByText("Mo, Di")).toBeInTheDocument();
  });

  it("shows the empty state without a queue", async () => {
    mockList.mockResolvedValue([]);
    render(<OfferingRequestReviewList />);

    expect(
      await screen.findByText("Keine offenen Anfragen zu Betreuungsangeboten."),
    ).toBeInTheDocument();
  });

  it("approves a request and removes the row", async () => {
    mockDecide.mockResolvedValue(undefined);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("77", true, undefined),
    );
    expect(
      await screen.findByText(/gültig ab 01\.02\.2027/),
    ).toBeInTheDocument();
  });

  it("requires a reason before rejecting", async () => {
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(screen.getByRole("button", { name: /Ablehnen/ }));

    expect(
      await screen.findByText(
        "Für eine Ablehnung ist eine Begründung erforderlich.",
      ),
    ).toBeInTheDocument();
    expect(mockDecide).not.toHaveBeenCalled();
  });

  it("names the capacity conflict and keeps the row pending", async () => {
    mockDecide.mockRejectedValue(
      new OfferingRequestApiError("full", "offering_change_capacity_full"),
    );
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    expect(await screen.findByText(/kein Platz mehr frei/)).toBeInTheDocument();
    // The row survives a failed approval: the switch was not applied.
    expect(screen.getByText(/Lara Beispiel/)).toBeInTheDocument();
  });

  it("shows the parent note when one was added", async () => {
    mockList.mockResolvedValue([request({ note: "Neuer Arbeitsbeginn" })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    expect(
      screen.getByText(/Nachricht der Eltern: Neuer Arbeitsbeginn/),
    ).toBeInTheDocument();
  });

  // Mitbuchungs-Regeln (#2365/#2370): rule-added lines are marked, name their
  // trigger, and can be unticked for this one approval.
  const ruleDiff = [
    {
      offering_id: "5",
      label: "Randstunde",
      old: "nicht gebucht",
      new: "Mo, Di, Mi, Do, Fr",
    },
    {
      offering_id: "9",
      label: "Ganztagsbetreuung bis 14.30 Uhr",
      old: "nicht gebucht",
      new: "Mo, Di, Mi, Do, Fr",
      automatic: true,
      automatic_days: "Mo, Di, Mi, Do, Fr",
      trigger_ids: ["5"],
      trigger_names: ["Randstunde"],
      optoutable: true,
    },
    {
      offering_id: "11",
      label: "Ganztagsbetreuung bis 16 Uhr",
      old: "nicht gebucht",
      new: "Mo, Di, Mi, Do, Fr",
      automatic: true,
      automatic_days: "Mo, Di, Mi, Do, Fr",
      trigger_ids: ["9"],
      trigger_names: ["Ganztagsbetreuung bis 14.30 Uhr"],
      optoutable: true,
    },
  ];

  const mixedRuleDiff = [
    {
      offering_id: "9",
      label: "Ganztagsbetreuung bis 14.30 Uhr",
      old: "nicht gebucht",
      new: "Mo, Di, Mi",
      automatic: true,
      automatic_days: "Di, Mi",
      rule_days: "Di",
      new_when_excluded: "Mo, Mi",
      trigger_ids: ["5"],
      trigger_names: ["Randstunde"],
      optoutable: true,
    },
  ];

  it("marks a rule-added line and names its trigger", async () => {
    mockList.mockResolvedValue([request({ diff: ruleDiff })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    expect(screen.getAllByText("Automatisch mitgebucht")).toHaveLength(2);
    expect(
      screen.getByText(/weil „Randstunde“ gewählt ist/),
    ).toBeInTheDocument();
  });

  it("attributes only rule-derived days to the selected trigger", async () => {
    mockList.mockResolvedValue([request({ diff: mixedRuleDiff })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    expect(
      screen.getByText(/Die Tage Di kommen automatisch dazu, weil/),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(/Die Tage Di, Mi kommen automatisch dazu, weil/),
    ).not.toBeInTheDocument();
  });

  it("hides the automatic-addition hint after opting out", async () => {
    mockList.mockResolvedValue([request({ diff: mixedRuleDiff })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    expect(
      screen.queryByText(/kommen automatisch dazu/),
    ).not.toBeInTheDocument();
  });

  it("describes the disabled co-booking rule after opting out", async () => {
    mockList.mockResolvedValue([request({ diff: mixedRuleDiff })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    expect(
      screen.getByText("Die Mitbuchungs-Regel gilt für diese Anfrage nicht."),
    ).toBeInTheDocument();
  });

  it("sends unticked rule-added lines as excluded on approve", async () => {
    mockDecide.mockResolvedValue(undefined);
    mockList.mockResolvedValue([request({ diff: ruleDiff })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: /Freigeben/ }));

    await waitFor(() =>
      expect(mockDecide).toHaveBeenCalledWith("77", true, undefined, ["9"]),
    );
  });

  it("keeps manual and required days visible when rule days are unticked", async () => {
    mockList.mockResolvedValue([
      request({
        diff: [
          {
            offering_id: "9",
            label: "Ganztagsbetreuung bis 14.30 Uhr",
            old: "nicht gebucht",
            new: "Mo, Di, Mi",
            automatic: true,
            automatic_days: "Di, Mi",
            new_when_excluded: "Mo, Mi",
            trigger_ids: ["5"],
            trigger_names: ["Randstunde"],
            optoutable: true,
          },
        ],
      }),
    ]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    expect(screen.getByText("Mo, Mi")).toBeInTheDocument();
    expect(screen.queryByText("Mo, Di, Mi")).not.toBeInTheDocument();
  });

  it("greys a line whose trigger was unticked", async () => {
    mockList.mockResolvedValue([request({ diff: ruleDiff })]);
    render(<OfferingRequestReviewList />);
    await screen.findByText(/Lara Beispiel/);
    expandAll();

    fireEvent.click(
      screen.getByRole("checkbox", {
        name: /Ganztagsbetreuung bis 14.30 Uhr automatisch mitbuchen/,
      }),
    );

    expect(
      screen.getByText(
        /Entfällt, weil „Ganztagsbetreuung bis 14.30 Uhr“ nicht mitgebucht wird/,
      ),
    ).toBeInTheDocument();
  });
});
