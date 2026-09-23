import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { setTestClock } from "~/test/clock";

import { ModalProvider } from "~/components/dashboard/modal-context";
import { CareResumeModal } from "./care-resume-modal";

const { mockResume } = vi.hoisted(() => ({ mockResume: vi.fn() }));

vi.mock("~/lib/care-exit-api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("~/lib/care-exit-api")>();
  return { ...actual, resumeCare: mockResume };
});

function renderModal() {
  const onClose = vi.fn();
  const onResumed = vi.fn();
  render(
    <ModalProvider>
      <CareResumeModal
        isOpen
        studentId="7"
        displayName="Mia Muster"
        onClose={onClose}
        onResumed={onResumed}
      />
    </ModalProvider>,
  );
  return { onClose, onResumed };
}

describe("CareResumeModal", () => {
  beforeEach(() => {
    setTestClock("2026-09-23T10:00:00+02:00");
    mockResume.mockReset();
  });

  it("explains a full Kinderkontingent and keeps the dialog as entered", async () => {
    mockResume.mockRejectedValue(
      Object.assign(new Error("child quota reached"), {
        status: 409,
        code: "students.child_quota_reached",
        details: {
          booked_places: 50,
          occupied_places: 50,
          requested_places: 1,
        },
      }),
    );
    const { onClose, onResumed } = renderModal();

    fireEvent.click(screen.getByLabelText(/Ich habe Gruppe, Angebote/));
    fireEvent.click(
      screen.getByRole("button", { name: "Betreuung wieder aufnehmen" }),
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "Das Kinderkontingent Ihrer Schule ist voll. Die Kontingentzahl beträgt 50 von 50 Kindern. Für weitere Kinder melden Sie sich bitte beim moto-Team.",
        ),
      ).toBeInTheDocument(),
    );
    expect(mockResume).toHaveBeenCalledWith("7", "2026-09-23", true);
    expect(screen.getByLabelText(/Ich habe Gruppe, Angebote/)).toBeChecked();
    expect(onResumed).not.toHaveBeenCalled();
    expect(onClose).not.toHaveBeenCalled();
  });
});
