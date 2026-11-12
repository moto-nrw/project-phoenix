import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { PickupChangeNotice } from "~/lib/pickup-change-notices-api";

import type { PickupException } from "~/lib/pickup-schedule-helpers";

const notices = vi.hoisted(() => ({
  list: [] as PickupChangeNotice[],
  markRead: vi.fn(async () => undefined),
}));
vi.mock("~/lib/hooks/use-pickup-change-notices", () => ({
  usePickupChangeNotices: () => ({
    notices: notices.list,
    markRead: notices.markRead,
  }),
}));
const refreshes = vi.hoisted(() => [] as string[]);
const tenantMutate = vi.hoisted(() => async (key: string) => {
  refreshes.push(key);
});
vi.mock("~/lib/swr", () => ({ useTenantMutate: () => tenantMutate }));

import {
  PickupChangeAlert,
  StudentPickupChanges,
  upcomingPickupChanges,
} from "./pickup-change-alert";

const TODAY = "2026-09-09";

function exception(over: Partial<PickupException>): PickupException {
  return {
    id: "1",
    studentId: "7",
    exceptionDate: TODAY,
    pickupTime: "14:30",
    source: "guardian",
    createdBy: "0",
    createdAt: "2026-09-08T10:00:00Z",
    updatedAt: "2026-09-08T10:00:00Z",
    ...over,
  };
}

describe("upcomingPickupChanges", () => {
  it("keeps changed days from today on, sorted, and drops the rest", () => {
    const changes = upcomingPickupChanges(
      [
        exception({
          id: "2",
          exceptionDate: "2026-09-11",
          pickupTime: "15:00:00",
        }),
        exception({ id: "1" }),
        exception({ id: "3", exceptionDate: "2026-09-08" }),
        exception({ id: "4", exceptionDate: "2026-09-10", source: "staff" }),
        exception({
          id: "6",
          exceptionDate: "2026-09-13",
          pickupTime: undefined,
          source: "staff",
        }),
        exception({
          id: "5",
          exceptionDate: "2026-09-12",
          pickupTime: undefined,
        }),
      ],
      TODAY,
    );

    expect(changes).toEqual([
      { date: TODAY, time: "14:30" },
      { date: "2026-09-10", time: "14:30" },
      { date: "2026-09-11", time: "15:00" },
    ]);
  });
});

describe("PickupChangeAlert", () => {
  it("names the day and the new time", () => {
    render(
      <PickupChangeAlert
        exceptions={[exception({ exceptionDate: "2026-09-10" })]}
        today={TODAY}
      />,
    );

    expect(
      screen.getByText("Abholzeit an einzelnen Tagen geändert"),
    ).toBeInTheDocument();
    expect(screen.getByText("Morgen: 14:30 Uhr")).toBeInTheDocument();
  });

  it("lists several days with their weekday", () => {
    render(
      <PickupChangeAlert
        exceptions={[
          exception({}),
          exception({
            id: "2",
            exceptionDate: "2026-09-14",
            pickupTime: "13:00",
          }),
        ]}
        today={TODAY}
      />,
    );

    expect(screen.getByText("Heute: 14:30 Uhr")).toBeInTheDocument();
    expect(
      screen.getByText("Montag, 14.09.2026: 13:00 Uhr"),
    ).toBeInTheDocument();
  });

  it("shows nothing once the change is withdrawn or past", () => {
    const { container } = render(
      <PickupChangeAlert
        exceptions={[exception({ exceptionDate: "2026-09-08" })]}
        today={TODAY}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });
});

function notice(over: Partial<PickupChangeNotice> = {}): PickupChangeNotice {
  return {
    requestId: "21",
    studentId: "7",
    studentFirstName: "Mia",
    date: "2026-09-10",
    pickupTime: "14:30",
    withdrawn: false,
    informedAt: "2026-09-09T11:00:00Z",
    ...over,
  };
}

// The changes the reader had not seen yet carry „Neu“; a taken-back change
// has no exception any more and says that the regular time applies (#6855).
describe("PickupChangeAlert with unseen changes", () => {
  it("marks unseen days and lists a taken-back day", () => {
    render(
      <PickupChangeAlert
        exceptions={[
          exception({ exceptionDate: "2026-09-10" }),
          exception({ id: "2", exceptionDate: "2026-09-14" }),
        ]}
        today={TODAY}
        fresh={[
          notice(),
          notice({
            requestId: "22",
            date: "2026-09-11",
            pickupTime: "",
            withdrawn: true,
          }),
        ]}
      />,
    );

    expect(screen.getByText("Morgen: 14:30 Uhr")).toBeInTheDocument();
    expect(
      screen.getByText("Freitag, 11.09.2026: wieder normale Abholzeit"),
    ).toBeInTheDocument();
    expect(screen.getAllByText("Neu")).toHaveLength(2);
    expect(
      screen.getByText("Montag, 14.09.2026: 14:30 Uhr"),
    ).toBeInTheDocument();
  });
});

describe("StudentPickupChanges", () => {
  beforeEach(() => {
    notices.list = [];
    notices.markRead.mockClear();
    refreshes.length = 0;
  });

  it("reloads the pickup times, then confirms only this child's loaded changes", async () => {
    const mine = notice();
    notices.list = [mine, notice({ requestId: "30", studentId: "8" })];
    render(
      <StudentPickupChanges
        studentId="7"
        exceptions={[exception({ exceptionDate: "2026-09-10" })]}
        today={TODAY}
      />,
    );

    await waitFor(() =>
      expect(notices.markRead).toHaveBeenCalledWith("7", [mine]),
    );
    expect(refreshes).toEqual([expect.stringContaining("7")]);
    expect(screen.getByText("Neu")).toBeInTheDocument();
  });

  it("confirms nothing before the pickup times are there", () => {
    notices.list = [notice()];
    render(
      <StudentPickupChanges
        studentId="7"
        exceptions={undefined}
        today={TODAY}
      />,
    );
    expect(notices.markRead).not.toHaveBeenCalled();
    expect(refreshes).toEqual([]);
  });
});
