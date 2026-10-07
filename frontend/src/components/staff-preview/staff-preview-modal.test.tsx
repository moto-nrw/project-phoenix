import { fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

const { mockFetchCandidates, mockStart } = vi.hoisted(() => ({
  mockFetchCandidates: vi.fn(),
  mockStart: vi.fn(),
}));

vi.mock("next-auth/react", () => ({
  useSession: () => ({ update: vi.fn() }),
}));

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    children,
    footer,
  }: {
    isOpen: boolean;
    children: ReactNode;
    footer?: ReactNode;
  }) =>
    isOpen ? (
      <div>
        {children}
        {footer}
      </div>
    ) : null,
}));

vi.mock("~/lib/staff-preview-api", () => ({
  fetchStaffPreviewCandidates: mockFetchCandidates,
  performStartStaffPreview: mockStart,
}));

import { StaffPreviewModal } from "./staff-preview-modal";

describe("StaffPreviewModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // #2517: Katalogtext mit Wiederholen, nie „Keine Person gefunden“ für eine
  // Liste, die nicht geladen wurde.
  it("shows a failed candidate load in the dialog and reloads on retry", async () => {
    mockFetchCandidates
      .mockRejectedValueOnce(
        new ApiError("candidates kaputt", 500, { code: "general.server" }),
      )
      .mockResolvedValueOnce([]);
    render(<StaffPreviewModal isOpen onClose={vi.fn()} />);

    expect(
      await screen.findByText(
        catalogText("general.server", "die Liste der Personen"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/kaputt/)).not.toBeInTheDocument();
    expect(
      screen.queryByText("Keine Person gefunden, die Sie ansehen können."),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText("Keine Person gefunden, die Sie ansehen können."),
    ).toBeInTheDocument();
    expect(mockFetchCandidates).toHaveBeenCalledTimes(2);
  });
});
