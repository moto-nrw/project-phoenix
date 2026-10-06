import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  deleteAdminRequest: vi.fn(),
  getAdminRequestDeleteImpact: vi.fn(),
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    deleteAdminRequest: mocks.deleteAdminRequest,
    getAdminRequestDeleteImpact: mocks.getAdminRequestDeleteImpact,
  };
});

import { AdminEnrollmentDeletionModal } from "./admin-enrollment-deletion-modal";
import { ApiError } from "~/lib/api-error";
import type { AdminEnrollmentDeletionImpact } from "~/lib/enrollment-admin-api";
import { catalogText } from "~/test/error-catalog-text";

const impact = {
  can_delete: true,
  deletes_request: true,
  blocking_student_ids: [],
  preserved_guardian_profiles: 0,
  preserved_parent_accounts: 0,
  unlinked_guardian_profiles: 0,
  parent_accounts_without_students: 0,
  counts: {
    total: 1,
    requests: 1,
    request_children: 0,
    request_child_offerings: 0,
    request_guardians: 0,
    change_requests: 0,
    change_request_messages: 0,
    late_invites: 0,
    offering_adjustments: 0,
    email_outbox: 0,
    rollover_links_cleared: 0,
    student_source_links_cleared: 0,
  },
} as unknown as AdminEnrollmentDeletionImpact;

function renderModal() {
  return render(
    <AdminEnrollmentDeletionModal
      isOpen
      requestId="7"
      studentHref={(id) => `/students/${id}`}
      onClose={vi.fn()}
      onDeleted={vi.fn()}
    />,
  );
}

describe("AdminEnrollmentDeletionModal (#2515)", () => {
  beforeEach(() => {
    mocks.deleteAdminRequest.mockReset();
    mocks.getAdminRequestDeleteImpact.mockReset();
  });

  it("shows a failed preview in the dialog with a retry", async () => {
    mocks.getAdminRequestDeleteImpact
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce(impact);

    renderModal();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Vorschau der Löschung"),
      ),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Betroffene Datensätze (1)")).toBeVisible();
    expect(mocks.getAdminRequestDeleteImpact).toHaveBeenCalledTimes(2);
  });
});
