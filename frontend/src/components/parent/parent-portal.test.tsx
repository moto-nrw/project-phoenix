import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import type React from "react";

const mocks = vi.hoisted(() => ({
  listMyChildren: vi.fn(),
  listMyEnrollments: vi.fn(),
  // child-detail now pulls in the child-care hook, which calls these on
  // mount; stub them so the rendered detail view loads cleanly.
  listSickDays: vi.fn().mockResolvedValue([]),
  listChildNotes: vi.fn().mockResolvedValue([]),
  submitSickNote: vi.fn().mockResolvedValue([]),
  addChildNote: vi.fn().mockResolvedValue([]),
  getChildFeatures: vi
    .fn()
    .mockResolvedValue({ sick_note_enabled: true, notes_enabled: true }),
  setBreadcrumb: vi.fn(),
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
  listMyEnrollments: mocks.listMyEnrollments,
  listSickDays: mocks.listSickDays,
  listChildNotes: mocks.listChildNotes,
  submitSickNote: mocks.submitSickNote,
  addChildNote: mocks.addChildNote,
  getChildFeatures: mocks.getChildFeatures,
}));

vi.mock("~/lib/breadcrumb-context", () => ({
  useSetBreadcrumb: mocks.setBreadcrumb,
}));

import { ChildDetail } from "./child-detail";
import { ParentChildrenPage } from "./parent-children-page";
import { ParentDashboard } from "./parent-dashboard";
import type { Child, EnrollmentRequest } from "~/lib/parent-api";

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

function enrollmentRequest(
  overrides: Partial<EnrollmentRequest> = {},
): EnrollmentRequest {
  return {
    request_id: "99",
    tenant_id: "7",
    status_token: "status-token",
    school_name: "OGS Demo",
    school_slug: "demo",
    withdrawn_at: undefined,
    phase_id: "22",
    phase_name: "Schuljahr 2026/27",
    service_start_date: "2026-08-01",
    service_end_date: "2027-07-31",
    submitted_at: "2026-01-15T10:00:00Z",
    children: [
      {
        child_id: "12",
        first_name: "Mila",
        last_name: "Neu",
        status: "submitted",
      },
    ],
    ...overrides,
  };
}

describe("Parent portal components", () => {
  beforeEach(() => {
    mocks.listMyChildren.mockReset();
    mocks.listMyEnrollments.mockReset();
    mocks.setBreadcrumb.mockReset();
  });

  it("renders dashboard children and pending enrollments", async () => {
    mocks.listMyChildren.mockResolvedValueOnce([child()]);
    mocks.listMyEnrollments.mockResolvedValueOnce([enrollmentRequest()]);

    render(<ParentDashboard />);

    expect(
      await screen.findByRole("heading", {
        name: "Willkommen im Elternportal",
      }),
    ).toBeInTheDocument();
    expect(screen.getByText("Lina Muster")).toBeInTheDocument();
    expect(screen.getByText("Mila Neu")).toBeInTheDocument();
    expect(screen.getByText("Offen")).toBeInTheDocument();
  });

  it("shows dashboard error state when parent data fails", async () => {
    mocks.listMyChildren.mockRejectedValueOnce(new Error("offline"));
    mocks.listMyEnrollments.mockResolvedValueOnce([]);

    render(<ParentDashboard />);

    expect(
      await screen.findByText(/Die Übersicht konnte nicht geladen werden/),
    ).toBeInTheDocument();
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
    expect(
      screen.getByText("Betreuung 01.08.2026 bis 31.07.2027"),
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
    expect(screen.getAllByText("Krank melden").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Abholzeit ändern").length).toBeGreaterThan(0);
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

  it("shows child detail error state", async () => {
    mocks.listMyChildren.mockRejectedValueOnce(new Error("boom"));

    render(<ChildDetail studentId="42" />);

    expect(
      await screen.findByText(/Die Kinderdaten konnten nicht geladen werden/),
    ).toBeInTheDocument();
  });
});
