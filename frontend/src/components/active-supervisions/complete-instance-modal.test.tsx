import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { TimetableRoster } from "~/lib/timetable-operations-types";
import { CompleteInstanceModal } from "./complete-instance-modal";

const roster = {
  instance: { id: "99", title: "Kreativ AG", endTime: "15:00" },
  rows: [],
} as unknown as TimetableRoster;

describe("CompleteInstanceModal (#2517)", () => {
  it("shows a failed completion inside the open dialog", () => {
    render(
      <CompleteInstanceModal
        isOpen
        roster={roster}
        isCompleting={false}
        error="Der Termin hat sich inzwischen geändert. Bitte laden Sie die Seite neu."
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    const dialog = screen.getByRole("dialog");
    expect(
      within(dialog).getByText(
        "Der Termin hat sich inzwischen geändert. Bitte laden Sie die Seite neu.",
      ),
    ).toBeInTheDocument();
  });

  it("shows no error area without an error", () => {
    render(
      <CompleteInstanceModal
        isOpen
        roster={roster}
        isCompleting={false}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );

    expect(
      within(screen.getByRole("dialog")).queryByRole("alert"),
    ).not.toBeInTheDocument();
  });
});
