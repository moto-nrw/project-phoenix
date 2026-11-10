// #6533: Schließen während des Speicherns ist gesperrt, eine späte Antwort
// schließt keinen anderen Dialog. Echter FormModal.
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { updateActivity, type Activity } from "~/lib/activity-api";
import { ApiError } from "~/lib/api-error";
import {
  deferred,
  expectDialogUnlocked,
  expectSaveLocksDialog,
} from "~/test/helpers/form-modal-pending";

vi.mock("./activity-waitlist-section", () => ({
  ActivityWaitlistSection: () => null,
}));
vi.mock("~/lib/activity-api", () => ({
  updateActivity: vi.fn(),
  deleteActivity: vi.fn(),
}));
const { setForm } = vi.hoisted(() => ({ setForm: vi.fn() }));
vi.mock("~/hooks/useActivityForm", () => ({
  parseParticipantLimit: (value: string) =>
    value ? Number.parseInt(value, 10) : null,
  useActivityForm: vi.fn(() => ({
    form: { name: "Test Activity", category_id: "1", max_participants: "15" },
    setForm,
    categories: [{ id: "1", name: "Category 1" }],
    loading: false,
    loadError: null,
    handleInputChange: vi.fn(),
    validateForm: vi.fn(() => null),
  })),
}));

import { ActivityManagementModal } from "./activity-management-modal";

const TITLE = "Aktivität: Test Activity";
const activity: Activity = {
  id: "1",
  name: "Test Activity",
  ag_category_id: "1",
  max_participant: 15,
  is_open_ags: true,
  supervisor_id: "1",
  created_at: new Date(),
  updated_at: new Date(),
  supervisors: [
    { id: "1", staff_id: "1", is_primary: true, full_name: "John Doe" },
  ],
};

function renderAndSave() {
  const answer = deferred<Activity>();
  vi.mocked(updateActivity).mockReturnValue(answer.promise);
  const onClose = vi.fn();
  const view = render(
    <ActivityManagementModal isOpen onClose={onClose} activity={activity} />,
  );
  fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
  return { ...view, answer, onClose };
}

describe("ActivityManagementModal pending lock (#6533)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("keeps the dialog shut while saving and unlocks it after a failure", async () => {
    const { answer, onClose } = renderAndSave();

    expectSaveLocksDialog(TITLE);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });
    expect(await screen.findByRole("alert")).toBeInTheDocument();
    expectDialogUnlocked(TITLE);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("closes nothing when the answer arrives after unmount", async () => {
    const { answer, onClose, unmount } = renderAndSave();
    expect(updateActivity).toHaveBeenCalledOnce();

    unmount();
    await act(async () => {
      answer.resolve(activity);
    });

    expect(onClose).not.toHaveBeenCalled();
  });
});
