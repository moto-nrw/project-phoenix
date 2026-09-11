import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StaffNoticeAcknowledgementsModal } from "./staff-notice-acknowledgements-modal";

const fetchNoticeAcknowledgements = vi.hoisted(() => vi.fn());

vi.mock("~/components/ui/modal", () => ({
  Modal: ({ isOpen, children }: { isOpen: boolean; children: ReactNode }) =>
    isOpen ? <div>{children}</div> : null,
}));

vi.mock("~/lib/staff-notices-api", () => ({
  fetchNoticeAcknowledgements,
}));

describe("StaffNoticeAcknowledgementsModal", () => {
  beforeEach(() => {
    fetchNoticeAcknowledgements.mockResolvedValue([
      {
        account_id: "7",
        name: "Anna Beispiel",
        acknowledged_at: "2026-01-15T13:30:00Z",
      },
    ]);
  });

  it("zeigt den Zeitpunkt der Bestätigung mit Datum und Uhrzeit", async () => {
    render(
      <StaffNoticeAcknowledgementsModal
        notice={{
          id: "42",
          title: "Dienstbesprechung",
          body: "",
          priority: "info",
          audience: "all",
          valid_from: "2026-01-15",
          weekdays: [],
          week_pattern: 0,
          requires_acknowledgement: true,
          active: true,
        }}
        onClose={vi.fn()}
      />,
    );

    expect(await screen.findByText("15.01.2026, 14:30")).toBeInTheDocument();
  });
});
