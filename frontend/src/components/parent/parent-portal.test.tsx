import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type React from "react";

const mocks = vi.hoisted(() => ({
  listMyChildren: vi.fn(),
  // child-detail now pulls in the child-care hook + the per-child message
  // thread list, which call these on mount; stub them so the rendered detail
  // view loads cleanly.
  listSickDays: vi.fn().mockResolvedValue([]),
  listExcusedRequests: vi.fn().mockResolvedValue([]),
  withdrawExcusedRequest: vi.fn(),
  listCareExceptions: vi.fn().mockResolvedValue([]),
  // The child-care hook now also loads the weekly care schedule (for the
  // "Heute → Abholung" tile's base time + today_absent signal); stub it so the
  // detail view settles cleanly instead of throwing on an undefined export.
  getChildCareSchedule: vi.fn().mockResolvedValue({
    weekdays: [],
    can_request: false,
    request_capabilities: {
      arrival: false,
      pickup: false,
      departure_mode: false,
    },
    today_absent: false,
  }),
  listChildThreads: vi.fn().mockResolvedValue([]),
  submitSickNote: vi.fn().mockResolvedValue({ status_days: [] }),
  submitCareException: vi.fn(),
  deleteCareException: vi.fn(),
  getChildFeatures: vi.fn().mockResolvedValue({
    sick_note_enabled: true,
    notes_enabled: true,
    excused_requires_approval: false,
    pickup_change_enabled: false,
    related_accounts_invite_enabled: false,
    related_accounts_remove_enabled: false,
    master_data_edit_enabled: false,
    master_data_contact_edit_enabled: false,
    master_data_request_enabled: false,
    meal_plan_enabled: false,
  }),
  // The dashboard news panel loads the announcement feed on mount; stub it so
  // the rendered dashboard settles cleanly (empty feed → placeholder).
  listAnnouncements: vi.fn().mockResolvedValue([]),
  listMessageThreads: vi.fn().mockResolvedValue([]),
  getParentCalendar: vi.fn().mockResolvedValue({
    from: "2026-08-15",
    to: "2026-11-13",
    events: [],
  }),
  markAnnouncementRead: vi.fn().mockResolvedValue(undefined),
  acknowledgeAnnouncement: vi.fn().mockResolvedValue(undefined),
  setBreadcrumb: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn(), replace: vi.fn(), prefetch: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("next/link", () => ({
  default: ({
    href,
    children,
    ...props
  }: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a href={href?.toString()} {...props}>
      {children}
    </a>
  ),
}));

vi.mock("~/lib/parent-api", () => ({
  listMyChildren: mocks.listMyChildren,
  listSickDays: mocks.listSickDays,
  listExcusedRequests: mocks.listExcusedRequests,
  withdrawExcusedRequest: mocks.withdrawExcusedRequest,
  listCareExceptions: mocks.listCareExceptions,
  getChildCareSchedule: mocks.getChildCareSchedule,
  listChildThreads: mocks.listChildThreads,
  submitSickNote: mocks.submitSickNote,
  submitCareException: mocks.submitCareException,
  deleteCareException: mocks.deleteCareException,
  getChildFeatures: mocks.getChildFeatures,
  listAnnouncements: mocks.listAnnouncements,
  listMessageThreads: mocks.listMessageThreads,
  markAnnouncementRead: mocks.markAnnouncementRead,
  acknowledgeAnnouncement: mocks.acknowledgeAnnouncement,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: mocks.setBreadcrumb,
}));

vi.mock("~/lib/personal-calendar-api", () => ({
  getParentCalendar: mocks.getParentCalendar,
}));

import { ChildDetail } from "./child-detail";
import { ParentChildrenPage } from "./parent-children-page";
import type { Child } from "~/lib/parent-api";
import { todayISO } from "~/lib/date-helpers";

function child(overrides: Partial<Child> = {}): Child {
  return {
    tenant_id: "7",
    school_name: "OGS Demo",
    school_slug: "demo",
    student_id: "42",
    first_name: "Lina",
    last_name: "Muster",
    school_class: "2a",
    status: "active",
    enrolled_from: "2026-08-01",
    enrolled_until: "2027-07-31",
    ...overrides,
  };
}

describe("Parent portal components", () => {
  beforeEach(() => {
    mocks.listMyChildren.mockReset();
    mocks.listSickDays.mockResolvedValue([]);
    mocks.listCareExceptions.mockResolvedValue([]);
    mocks.listChildThreads.mockResolvedValue([]);
    mocks.listAnnouncements.mockResolvedValue([]);
    mocks.listMessageThreads.mockResolvedValue([]);
    mocks.getParentCalendar.mockResolvedValue({
      from: "2026-08-15",
      to: "2026-11-13",
      events: [],
    });
    mocks.getChildFeatures.mockResolvedValue({
      sick_note_enabled: true,
      notes_enabled: true,
      pickup_change_enabled: false,
      related_accounts_invite_enabled: false,
      related_accounts_remove_enabled: false,
      master_data_edit_enabled: false,
      master_data_contact_edit_enabled: false,
      master_data_request_enabled: false,
      meal_plan_enabled: false,
    });
    mocks.setBreadcrumb.mockReset();
  });

  it("renders children overview, empty state, and error state", async () => {
    mocks.listMyChildren.mockResolvedValueOnce([child()]);
    render(<ParentChildrenPage />);

    expect(
      await screen.findByRole("heading", { name: "Kinderübersicht" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Lina Muster/ })).toHaveAttribute(
      "href",
      "/parents/children/42",
    );
    // School, class and care period share one detail line on the row.
    expect(
      screen.getByText(/Betreuung 01\.08\.2026 bis 31\.07\.2027/),
    ).toBeInTheDocument();

    cleanup();
    mocks.listMyChildren.mockResolvedValueOnce([]);
    render(<ParentChildrenPage />);
    expect(await screen.findByText("Noch keine Kinder")).toBeInTheDocument();

    cleanup();
    mocks.listMyChildren.mockRejectedValueOnce(new Error("network"));
    render(<ParentChildrenPage />);
    expect(
      await screen.findByText(
        /Die Kinderübersicht konnte nicht geladen werden/,
      ),
    ).toBeInTheDocument();
  });

  it("renders child detail content and missing-child state", async () => {
    mocks.listMyChildren.mockResolvedValueOnce([child()]);
    render(<ChildDetail studentId="42" />);

    expect(
      (await screen.findAllByRole("heading", { name: "Lina Muster" })).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText("Krank oder entschuldigt melden").length,
    ).toBeGreaterThan(0);
    expect(screen.getAllByText("Abholung für heute").length).toBeGreaterThan(0);
    await waitFor(() => {
      expect(mocks.setBreadcrumb).toHaveBeenCalledWith({
        pageTitle: "Lina Muster",
      });
    });

    cleanup();
    mocks.listMyChildren.mockResolvedValueOnce([]);
    render(<ChildDetail studentId="missing" />);
    expect(
      await screen.findByText(/Kein Kind mit dieser Kennung/),
    ).toBeInTheDocument();
  });

  it("files an excused absence end-to-end through the Abmelden flow", async () => {
    mocks.listMyChildren.mockResolvedValueOnce([child()]);
    render(<ChildDetail studentId="42" />);

    await screen.findAllByRole("heading", { name: "Lina Muster" });

    // Open the "Abmelden" modal from its action button.
    const abmeldenButtons = screen.getAllByRole("button", {
      name: "Krank oder entschuldigt melden",
    });
    const enabled = abmeldenButtons.find(
      (button) => !button.hasAttribute("disabled"),
    );
    expect(enabled).toBeTruthy();
    fireEvent.click(enabled!);

    await screen.findByRole("dialog", { name: "Abmelden" });

    // Switch to "Entschuldigt". A note is now mandatory for an excused absence
    // (issue #1845), so fill it in before submitting.
    fireEvent.click(
      screen.getByRole("combobox", { name: "Art der Abmeldung" }),
    );
    fireEvent.click(screen.getByRole("option", { name: "Entschuldigt" }));
    fireEvent.change(document.querySelector("textarea")!, {
      target: { value: "Zahnarzttermin" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Abmeldung senden" }));

    // The hook forwards the chosen status + note to the API for the right child.
    await waitFor(() =>
      expect(mocks.submitSickNote).toHaveBeenCalledWith(
        "42",
        expect.any(Array),
        "Zahnarzttermin",
        "excused",
      ),
    );
  });

  it("shows child detail error state", async () => {
    mocks.listMyChildren.mockRejectedValueOnce(new Error("boom"));

    render(<ChildDetail studentId="42" />);

    expect(
      await screen.findByText(/Die Kinderdaten konnten nicht geladen werden/),
    ).toBeInTheDocument();
  });

  it("keeps existing parent pickup overrides clearable when new changes are disabled", async () => {
    mocks.listMyChildren.mockResolvedValueOnce([child()]);
    mocks.getChildFeatures.mockResolvedValueOnce({
      sick_note_enabled: true,
      notes_enabled: true,
      pickup_change_enabled: false,
      related_accounts_invite_enabled: false,
      related_accounts_remove_enabled: false,
      master_data_edit_enabled: false,
      master_data_contact_edit_enabled: false,
      master_data_request_enabled: false,
      meal_plan_enabled: false,
    });
    mocks.listCareExceptions.mockResolvedValueOnce([
      {
        date: todayISO(),
        pickup_time: "14:45",
        source: "guardian",
        updated_at: "2026-06-16T10:00:00Z",
      },
    ]);

    render(<ChildDetail studentId="42" />);

    await screen.findAllByRole("heading", { name: "Lina Muster" });
    const enabledPickupButton = await waitFor(() => {
      const button = screen
        .getAllByRole("button", {
          name: /Abholung für heute/,
        })
        .find((candidate) => !candidate.hasAttribute("disabled"));
      if (!button) throw new Error("pickup action is still disabled");
      return button;
    });

    fireEvent.click(enabledPickupButton);

    expect(
      await screen.findByRole("dialog", {
        name: "Abholzeit ändern",
      }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Zurücksetzen" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Speichern" })).toBeDisabled();
  });
});
