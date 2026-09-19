import { fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type { PhaseResponseOverview as Overview } from "~/lib/enrollment-phase-api";

const mocks = vi.hoisted(() => ({
  push: vi.fn(),
  hasPermission: vi.fn(),
  parentNewsEnabled: true as boolean | undefined,
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ data: { user: { token: "t" } } }),
}));

vi.mock("~/lib/auth-utils", () => ({
  hasPermission: mocks.hasPermission,
}));

vi.mock("~/lib/hooks/use-settings-schema", () => ({
  useSettingsSchema: () => ({ data: {} }),
}));

vi.mock("~/lib/settings-api", () => ({
  getSettingValue: () => mocks.parentNewsEnabled,
}));

vi.mock("~/lib/tenant-router", () => ({
  useTenantRouter: () => ({ push: mocks.push }),
}));

vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => path,
}));

import { PhaseResponseOverview } from "./phase-response-overview";

const overview: Overview = {
  applicable: true,
  expected: 4,
  responded: 1,
  children: [
    {
      studentId: "1",
      firstName: "Mia",
      lastName: "Arslan",
      schoolClass: "2a",
      hasParentApp: true,
      responded: false,
      requestId: null,
      pendingRequestId: null,
      childStatus: null,
    },
    {
      studentId: "2",
      firstName: "Ole",
      lastName: "Ernst",
      schoolClass: "1b",
      hasParentApp: false,
      responded: false,
      requestId: null,
      pendingRequestId: null,
      childStatus: null,
    },
    {
      studentId: "3",
      firstName: "Ida",
      lastName: "Fuchs",
      schoolClass: "3a",
      hasParentApp: true,
      responded: false,
      requestId: null,
      pendingRequestId: "901",
      childStatus: "pending_renewal",
    },
    {
      studentId: "4",
      firstName: "Ben",
      lastName: "Yilmaz",
      schoolClass: "1a",
      hasParentApp: true,
      responded: true,
      requestId: "900",
      pendingRequestId: null,
      childStatus: "submitted",
    },
  ],
  excluded: [
    { reason: "care_ending", count: 1 },
    { reason: "graduating", count: 25 },
  ],
};

beforeEach(() => {
  mocks.hasPermission.mockReturnValue(true);
  mocks.parentNewsEnabled = true;
});

afterEach(() => {
  vi.clearAllMocks();
  window.sessionStorage.clear();
});

// DataTable renders the desktop table and the stacked phone list side by
// side and lets CSS pick one, so row assertions look at the table only.
const table = () => within(screen.getByRole("table"));

describe("PhaseResponseOverview", () => {
  it("states the figure and explains who is not counted", () => {
    render(<PhaseResponseOverview overview={overview} search="" />);

    expect(
      screen.getByRole("heading", {
        name: "1 von 4 Kindern haben die Anmeldung abgegeben",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Nicht mitgezählt: 1 Kind mit eingetragenem Betreuungsende, 25 Kinder im letzten Jahrgang.",
      ),
    ).toBeInTheDocument();
  });

  it("opens on the children still missing and links each to its record", () => {
    render(<PhaseResponseOverview overview={overview} search="" />);

    expect(
      screen.getByRole("button", { name: "Fehlt noch (3)" }),
    ).toBeInTheDocument();
    expect(table().getByRole("link", { name: "Mia Arslan" })).toHaveAttribute(
      "href",
      "/students/1",
    );
    expect(table().queryByText("Ben Yilmaz")).not.toBeInTheDocument();
    expect(screen.getAllByText("Fehlt noch").length).toBeGreaterThan(0);
  });

  it("shows a carried-over row as waiting and links the open entry", () => {
    render(<PhaseResponseOverview overview={overview} search="" />);

    const row = table().getByRole("link", { name: "Ida Fuchs" }).closest("tr");
    expect(row).not.toBeNull();
    const cells = within(row as HTMLElement);
    expect(cells.getByText("Wartet auf Verlängerung")).toBeInTheDocument();
    expect(cells.getByRole("link", { name: "Ansehen" })).toHaveAttribute(
      "href",
      "/admin/enrollments/901",
    );
  });

  it("switches to the children that answered", () => {
    render(<PhaseResponseOverview overview={overview} search="" />);

    fireEvent.click(screen.getByRole("button", { name: "Abgegeben (1)" }));

    expect(
      table().getByRole("link", { name: "Ben Yilmaz" }),
    ).toBeInTheDocument();
    expect(table().getByText("Eingegangen")).toBeInTheDocument();
    expect(table().queryByText("Mia Arslan")).not.toBeInTheDocument();
    expect(table().getByRole("link", { name: "Ansehen" })).toHaveAttribute(
      "href",
      "/admin/enrollments/900",
    );
  });

  it("keeps the parents app apart from the answer and names who needs a call", () => {
    render(<PhaseResponseOverview overview={overview} search="" />);

    expect(
      screen.getByRole("columnheader", { name: /Eltern-App/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("1 Familie hat keine Eltern-App. Bitte anrufen."),
    ).toBeInTheDocument();
  });

  it("hands the reachable missing children to the announcement composer", () => {
    render(<PhaseResponseOverview overview={overview} search="" />);

    fireEvent.click(
      screen.getByRole("button", { name: "2 Familien erinnern" }),
    );

    expect(mocks.push).toHaveBeenCalledWith("/parent-announcements?neu=kinder");
    expect(
      JSON.parse(
        window.sessionStorage.getItem("moto:announcement-prefill-students") ??
          "[]",
      ),
    ).toEqual([
      { id: "1", name: "Mia Arslan" },
      { id: "3", name: "Ida Fuchs" },
    ]);
  });

  it("offers the reminder only next to the children still missing", () => {
    render(<PhaseResponseOverview overview={overview} search="" />);
    expect(
      screen.getByRole("button", { name: "2 Familien erinnern" }),
    ).toBeInTheDocument();

    // Next to "Abgegeben (1)" a button with a number reads as a message to the
    // families that already answered.
    fireEvent.click(screen.getByRole("button", { name: "Abgegeben (1)" }));
    expect(
      screen.queryByRole("button", { name: /erinnern/ }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Fehlt noch (3)" }));
    expect(
      screen.getByRole("button", { name: "2 Familien erinnern" }),
    ).toBeInTheDocument();
  });

  it("offers no announcement without the permission", () => {
    mocks.hasPermission.mockReturnValue(false);
    render(<PhaseResponseOverview overview={overview} search="" />);
    expect(
      screen.queryByRole("button", { name: /erinnern/ }),
    ).not.toBeInTheDocument();
  });

  it("offers no announcement when the school has parent news switched off", () => {
    mocks.parentNewsEnabled = false;
    render(<PhaseResponseOverview overview={overview} search="" />);
    expect(
      screen.queryByRole("button", { name: /erinnern/ }),
    ).not.toBeInTheDocument();
  });

  it("filters by child and class from the page search", () => {
    const { rerender } = render(
      <PhaseResponseOverview overview={overview} search="ernst" />,
    );
    expect(
      table().getByRole("link", { name: "Ole Ernst" }),
    ).toBeInTheDocument();
    expect(table().queryByText("Mia Arslan")).not.toBeInTheDocument();

    rerender(<PhaseResponseOverview overview={overview} search="2A" />);
    expect(
      table().getByRole("link", { name: "Mia Arslan" }),
    ).toBeInTheDocument();

    rerender(<PhaseResponseOverview overview={overview} search="zzz" />);
    expect(
      table().getByText("Kein Kind für diese Suche gefunden"),
    ).toBeInTheDocument();
  });

  it("says so when every child is enrolled", () => {
    render(
      <PhaseResponseOverview
        overview={{
          ...overview,
          expected: 1,
          responded: 1,
          children: overview.children.filter((child) => child.responded),
          excluded: [],
        }}
        search=""
      />,
    );
    expect(
      table().getByText("Alle Kinder sind angemeldet"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Nicht mitgezählt/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /erinnern/ }),
    ).not.toBeInTheDocument();
  });
});
