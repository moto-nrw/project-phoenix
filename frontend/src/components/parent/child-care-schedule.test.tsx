import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ChildCareScheduleSection } from "./child-care-schedule";
import {
  getChildCareSchedule,
  withdrawCareScheduleRequest,
  type ChildCareSchedule,
} from "~/lib/parent-api";

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: vi.fn(),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: {
    href: string;
    children: React.ReactNode;
  }) => (
    <a href={href} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("~/lib/parent-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/parent-api")>();
  return {
    ...actual,
    getChildCareSchedule: vi.fn(),
    submitCareScheduleRequest: vi.fn(),
    withdrawCareScheduleRequest: vi.fn(),
  };
});

const mockGet = vi.mocked(getChildCareSchedule);
const mockWithdraw = vi.mocked(withdrawCareScheduleRequest);

function schedule(
  overrides: Partial<ChildCareSchedule> = {},
): ChildCareSchedule {
  return {
    weekdays: [
      {
        weekday: 1,
        arrival: "08:00",
        pickup: "15:00",
        modes: ["bus", "pickup"],
      },
      { weekday: 2, arrival: "08:00", pickup: "16:00", modes: ["pickup"] },
      { weekday: 3, arrival: "", pickup: "", modes: [] },
      { weekday: 4, arrival: "08:00", pickup: "15:00", modes: ["alone"] },
      { weekday: 5, arrival: "08:00", pickup: "15:00", modes: ["bus"] },
    ],
    can_request: true,
    request_capabilities: {
      arrival: true,
      pickup: true,
      departure_mode: true,
    },
    today_absent: false,
    ...overrides,
  };
}

const pending: ChildCareSchedule["pending_request"] = {
  id: "12",
  created_at: "2026-07-02T10:00:00Z",
  diff: [
    {
      label: "Montag · Abholzeit",
      old: "—",
      new: "16:00",
      weekday: 1,
      care_kind: "pickup",
    },
  ],
  submitted_by_self: true,
};

describe("ChildCareScheduleSection", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockGet.mockResolvedValue(schedule());
  });

  it("renders the weekly grid from the API data", async () => {
    render(<ChildCareScheduleSection studentId="42" />);

    expect(await screen.findByText("Betreuungszeiten")).toBeInTheDocument();
    // Modes for Monday: Bus + Wird abgeholt joined.
    expect(screen.getByText("Bus, Wird abgeholt")).toBeInTheDocument();
    // Tuesday pickup differs (16:00) from the shared 15:00.
    expect(screen.getByText("16:00")).toBeInTheDocument();
    // With can_request and no pending request, the request button shows.
    expect(
      screen.getByRole("button", { name: "Änderung anfragen" }),
    ).toBeInTheDocument();
  });

  it("shows the pending request with diff and a withdraw button when submitted by self", async () => {
    mockGet.mockResolvedValue(schedule({ pending_request: pending }));
    render(<ChildCareScheduleSection studentId="42" />);

    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(screen.getByText("Ihre Änderungswünsche")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Zurückziehen" }),
    ).toBeInTheDocument();
    // The request button is hidden while a request is pending.
    expect(
      screen.queryByRole("button", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("hides the withdraw button for a request submitted by someone else", async () => {
    mockGet.mockResolvedValue(
      schedule({ pending_request: { ...pending, submitted_by_self: false } }),
    );
    render(<ChildCareScheduleSection studentId="42" />);

    expect(await screen.findByText("In Prüfung")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Zurückziehen" }),
    ).not.toBeInTheDocument();
  });

  it("hides the request button when requesting is not allowed", async () => {
    mockGet.mockResolvedValue(schedule({ can_request: false }));
    render(<ChildCareScheduleSection studentId="42" />);

    expect(await screen.findByText("Betreuungszeiten")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("hides the request action when every permanent-care field is disabled", async () => {
    mockGet.mockResolvedValue(
      schedule({
        can_request: false,
        request_capabilities: {
          arrival: false,
          pickup: false,
          departure_mode: false,
        },
      }),
    );
    render(<ChildCareScheduleSection studentId="42" />);

    expect(await screen.findByText("Betreuungszeiten")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Änderung anfragen" }),
    ).not.toBeInTheDocument();
  });

  it("withdraws the pending request through the confirmation modal", async () => {
    mockGet.mockResolvedValue(schedule({ pending_request: pending }));
    mockWithdraw.mockResolvedValue(schedule({ can_request: true }));
    render(<ChildCareScheduleSection studentId="42" />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Zurückziehen" }),
    );
    // Confirm modal opens with its own "Zurückziehen" confirm button; the last
    // matching button is the confirm action inside the dialog.
    const withdrawButtons = screen.getAllByRole("button", {
      name: "Zurückziehen",
    });
    fireEvent.click(withdrawButtons[withdrawButtons.length - 1]!);

    await waitFor(() => expect(mockWithdraw).toHaveBeenCalledWith("42", "12"));
    // After withdrawal the request button returns (pending cleared).
    expect(
      await screen.findByRole("button", { name: "Änderung anfragen" }),
    ).toBeInTheDocument();
  });

  it("shows an error card when the schedule fails to load", async () => {
    mockGet.mockRejectedValue(new Error("boom"));
    render(<ChildCareScheduleSection studentId="42" />);

    expect(
      await screen.findByText(
        "Die Betreuungszeiten konnten nicht geladen werden. Bitte aktualisieren Sie die Seite.",
      ),
    ).toBeInTheDocument();
  });
});
