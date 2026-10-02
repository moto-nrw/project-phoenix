import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const counts = vi.hoisted(() => ({
  changeRequests: 0,
  enrollment: 0,
  withdrawals: 0,
  staffAbsences: 0,
  unreadEnrollments: 0,
}));

vi.mock("~/lib/hooks/use-change-requests-pending", () => ({
  useChangeRequestsPending: () => ({ unreadCount: counts.changeRequests }),
}));
vi.mock("~/lib/hooks/use-enrollment-requests-pending", () => ({
  useEnrollmentRequestsPending: () => ({ unreadCount: counts.enrollment }),
}));
vi.mock("~/lib/hooks/use-care-withdrawals-pending", () => ({
  useCareWithdrawalsPending: () => ({ unreadCount: counts.withdrawals }),
}));
vi.mock("~/lib/hooks/use-staff-absences-pending", () => ({
  useStaffAbsencesPending: () => ({ unreadCount: counts.staffAbsences }),
}));
vi.mock("~/lib/hooks/use-enrollments-unread", () => ({
  useEnrollmentsUnread: () => ({ unreadCount: counts.unreadEnrollments }),
}));
vi.mock("~/lib/tenant-path", () => ({
  useTenantAwarePath: () => (path: string) => `/test-tenant${path}`,
}));

import { OpenRequestsBlock } from "./open-requests-block";

describe("OpenRequestsBlock (#2180)", () => {
  beforeEach(() => {
    counts.changeRequests = 0;
    counts.enrollment = 0;
    counts.withdrawals = 0;
    counts.staffAbsences = 0;
    counts.unreadEnrollments = 0;
  });

  it("sagt es, wenn nichts auf eine Entscheidung wartet", () => {
    render(<OpenRequestsBlock />);

    expect(
      screen.getByText("Nichts wartet auf eine Entscheidung"),
    ).toBeInTheDocument();
  });

  // Eine Warteschlange ohne offene Anfrage ist keine Zeile: die Startseite
  // zeigt, was zu tun ist, nicht die Liste aller möglichen Arten.
  it("zeigt nur Arten mit offenen Anfragen", () => {
    counts.changeRequests = 3;
    counts.staffAbsences = 1;

    render(<OpenRequestsBlock />);

    expect(screen.getByText("Wünsche von Eltern")).toBeInTheDocument();
    expect(screen.getByText("3 offen")).toBeInTheDocument();
    expect(screen.getByText("Anträge des Teams")).toBeInTheDocument();
    expect(
      screen.queryByText("Änderungen an Anmeldungen"),
    ).not.toBeInTheDocument();
  });

  it("führt jede Zeile in die Anfragen-Liste der Einrichtung", () => {
    counts.withdrawals = 2;

    render(<OpenRequestsBlock />);

    expect(
      screen.getByRole("link", { name: /Abmeldungen aus der Betreuung/ }),
    ).toHaveAttribute("href", "/test-tenant/anfragen");
  });

  // Ungelesene Anmeldungen (#3778): eigene Zeile, führt zu den Anmeldungen.
  it("zeigt ungelesene Anmeldungen als eigene Zeile", () => {
    counts.unreadEnrollments = 2;

    render(<OpenRequestsBlock />);

    expect(screen.getByText("2 ungelesen")).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: /Neue Anmeldungen/ }),
    ).toHaveAttribute("href", "/test-tenant/admin/enrollments");
  });
});
