import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";

import { AdminSessionEditModal } from "./admin-session-edit-modal";

vi.mock("~/components/ui/modal", () => ({
  Modal: ({
    isOpen,
    title,
    children,
    footer,
  }: {
    isOpen: boolean;
    title: string;
    children: React.ReactNode;
    footer?: React.ReactNode;
  }) =>
    isOpen ? (
      <div role="dialog" aria-label={title}>
        {children}
        {footer}
      </div>
    ) : null,
}));

const mocks = vi.hoisted(() => ({
  createSession: vi.fn(),
  updateSession: vi.fn(),
}));

vi.mock("~/lib/staff-api", () => ({
  staffSessionService: {
    createSession: mocks.createSession,
    updateSession: mocks.updateSession,
  },
}));

function renderBackfill() {
  const onClose = vi.fn();
  const onSaved = vi.fn();
  render(
    <AdminSessionEditModal
      isOpen
      mode="nachtragen"
      staffId="7"
      date={new Date(2026, 8, 7)}
      session={null}
      onClose={onClose}
      onSaved={onSaved}
    />,
  );
  fireEvent.change(
    screen.getByPlaceholderText(
      "z. B. Mitarbeiter hatte vergessen einzustempeln",
    ),
    { target: { value: "Vergessen" } },
  );
  fireEvent.click(screen.getByRole("button", { name: "Eintrag anlegen" }));
  return { onClose, onSaved };
}

describe("AdminSessionEditModal", () => {
  beforeEach(() => {
    mocks.createSession.mockReset();
    mocks.updateSession.mockReset();
  });

  it("explains an overlapping block in the dialog and keeps it open", async () => {
    mocks.createSession.mockRejectedValueOnce(
      new ApiError("work session overlaps an existing block", 409, {
        code: "workforce.work_session_overlap",
        errors: [{ field: "check_in_time", reason: "overlap" }],
      }),
    );
    const { onClose, onSaved } = renderBackfill();

    expect(
      await screen.findByText(
        catalogText("workforce.work_session_overlap", "die Arbeitszeit"),
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(
        document.querySelector('input[name="check_in_time"]'),
      ).toHaveAttribute("aria-invalid", "true"),
    );
    expect(onClose).not.toHaveBeenCalled();
    expect(onSaved).not.toHaveBeenCalled();
  });

  it("offers a retry for a server error", async () => {
    mocks.createSession
      .mockRejectedValueOnce(
        new ApiError("boom", 500, { code: "general.server", instance: "r1" }),
      )
      .mockResolvedValueOnce(undefined);
    const { onClose } = renderBackfill();

    await screen.findByText(catalogText("general.server", "die Arbeitszeit"));
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(onClose).toHaveBeenCalled());
    expect(mocks.createSession).toHaveBeenCalledTimes(2);
  });
});
