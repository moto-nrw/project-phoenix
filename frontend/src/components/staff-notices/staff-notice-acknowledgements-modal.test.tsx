import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import type { StaffNotice } from "~/lib/staff-notices-api";
import { catalogText } from "~/test/error-catalog-text";

import { StaffNoticeAcknowledgementsModal } from "./staff-notice-acknowledgements-modal";

const fetchNoticeAcknowledgements = vi.hoisted(() => vi.fn());

vi.mock("~/components/ui/modal", () => ({
  Modal: ({ isOpen, children }: { isOpen: boolean; children: ReactNode }) =>
    isOpen ? <div>{children}</div> : null,
}));

vi.mock("~/lib/staff-notices-api", () => ({
  fetchNoticeAcknowledgements,
}));

const notice: StaffNotice = {
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
};

describe("StaffNoticeAcknowledgementsModal", () => {
  beforeEach(() => {
    fetchNoticeAcknowledgements.mockReset();
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
      <StaffNoticeAcknowledgementsModal notice={notice} onClose={vi.fn()} />,
    );

    expect(await screen.findByText("15.01.2026, 14:30")).toBeInTheDocument();
  });

  // #2517: Katalogtext statt Serversatz, nie „Noch niemand hat bestätigt“
  // für eine Liste, die nicht geladen wurde, und Wiederholen lädt neu.
  it("zeigt einen Ladefehler im Dialog und lädt bei Wiederholen neu", async () => {
    fetchNoticeAcknowledgements.mockRejectedValueOnce(
      new ApiError("acknowledgements kaputt", 500, { code: "general.server" }),
    );
    render(
      <StaffNoticeAcknowledgementsModal notice={notice} onClose={vi.fn()} />,
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Bestätigungen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Noch niemand hat bestätigt."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("15.01.2026, 14:30")).toBeInTheDocument();
    expect(fetchNoticeAcknowledgements).toHaveBeenCalledTimes(2);
  });
});
