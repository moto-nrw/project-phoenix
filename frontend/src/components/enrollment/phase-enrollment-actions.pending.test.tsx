// #6593: Solange eine manuelle Anmeldung gesendet wird (auch während der
// Rückfrage „Ist das Kind schon angemeldet?“), lässt sich das Panel nicht
// schließen; eine späte Antwort meldet keinen Erfolg in einem neu geöffneten
// Panel.
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { useState } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  deferred,
  expectSlideOverSaveLocked,
  expectSlideOverUnlocked,
} from "~/test/helpers/form-modal-pending";

vi.mock("vaul", () => import("~/test/mocks/vaul"));

const mocks = vi.hoisted(() => ({
  createManualApprovedEnrollment: vi.fn(),
  fetchManualEnrollmentBootstrap: vi.fn(),
  fetchCareOfferingBookingStats: vi.fn(),
  // Called once the stub form has handled the submitter's outcome.
  submitSettled: vi.fn(),
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  createManualApprovedEnrollment: mocks.createManualApprovedEnrollment,
  fetchManualEnrollmentBootstrap: mocks.fetchManualEnrollmentBootstrap,
}));
vi.mock("~/lib/care-offering-booking-stats", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  fetchCareOfferingBookingStats: mocks.fetchCareOfferingBookingStats,
}));

// The enrollment form is not under test: its send button hands one child to
// the panel's submitter and reports like the real form (a null result stays
// quiet, a thrown error shows in the form).
vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: function EnrollmentFormStub({
    submitter,
    onSubmitted,
    submitLabel,
  }: {
    submitter: (payload: unknown) => Promise<{ status_url: string } | null>;
    onSubmitted: (url: string) => void;
    submitLabel: string;
  }) {
    const [failed, setFailed] = useState(false);
    return (
      <div>
        {failed ? <p>Senden fehlgeschlagen</p> : null}
        <button
          type="button"
          onClick={() => {
            setFailed(false);
            submitter({
              children: [{ first_name: "Mia", last_name: "Muster" }],
            })
              .then((result) => {
                if (result) onSubmitted(result.status_url);
              })
              .catch(() => setFailed(true))
              .finally(() => mocks.submitSettled());
          }}
        >
          {submitLabel}
        </button>
      </div>
    );
  },
}));

import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { Phase } from "~/lib/enrollment-phase-api";
import { schema } from "~/test/enrollment-editors";
import { ManualApprovedEnrollmentModal } from "./phase-enrollment-actions";

const TITLE = "Kind manuell über Anmeldung freigeben";
const SUBMIT = "Kind anlegen und freigeben";
const SUCCESS = "Die manuelle Anmeldung wurde angelegt und freigegeben.";
const phase = { id: "5", name: "Schuljahr 2026/27" } as Phase;

function Modal({ isOpen = true }: { isOpen?: boolean }) {
  return (
    <ManualApprovedEnrollmentModal
      isOpen={isOpen}
      onClose={vi.fn()}
      phase={phase}
      gradeLevelMax={13}
    />
  );
}

async function fillAndSend() {
  fireEvent.change(await screen.findByLabelText("Interner Grund"), {
    target: { value: "telefonisch bestätigt" },
  });
  fireEvent.click(
    screen.getByLabelText("Einwilligung der Eltern liegt extern vor."),
  );
  const submit = await screen.findByRole("button", { name: SUBMIT });
  await waitFor(() => expect(submit).toBeEnabled());
  fireEvent.click(submit);
  await waitFor(() =>
    expect(mocks.createManualApprovedEnrollment).toHaveBeenCalled(),
  );
}

describe("ManualApprovedEnrollmentModal: Senden sperrt das Panel (#6593)", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mocks.fetchManualEnrollmentBootstrap.mockResolvedValue({
      schema: schema(),
      offerings: [],
      care_offering_selection_mode: "optional",
      captcha_config: null,
      legal_texts: { blocks: [] },
    });
    mocks.fetchCareOfferingBookingStats.mockResolvedValue({});
  });

  it("locks the panel while sending and unlocks it after a failure", async () => {
    const answer = deferred<{ status_url: string }>();
    mocks.createManualApprovedEnrollment.mockReturnValue(answer.promise);
    render(<Modal />, { wrapper: ToastProvider });
    await fillAndSend();

    expectSlideOverSaveLocked(TITLE, []);

    await act(async () => {
      answer.reject(new ApiError("kaputt", 500, { code: "general.server" }));
    });

    expect(await screen.findByText("Senden fehlgeschlagen")).toBeVisible();
    expectSlideOverUnlocked(TITLE);
    expect(screen.getByLabelText("Interner Grund")).toHaveValue(
      "telefonisch bestätigt",
    );
  });

  it("stays locked while the office answers the question and shows the result", async () => {
    mocks.createManualApprovedEnrollment
      .mockRejectedValueOnce(
        new ApiError("possibly enrolled", 409, {
          code: "enrollment.child_possibly_enrolled",
          details: {
            candidates: [
              { student_id: "77", first_name: "Mia", last_name: "Muster" },
            ],
          },
        }),
      )
      .mockResolvedValueOnce({ status_url: "/anmeldung/status/abc" });
    render(<Modal />, { wrapper: ToastProvider });
    await fillAndSend();

    expect(
      await screen.findByRole("dialog", {
        name: "Ist das Kind schon angemeldet?",
      }),
    ).toBeInTheDocument();
    expectSlideOverSaveLocked(TITLE, []);

    fireEvent.click(
      screen.getByRole("button", { name: /Als neues Kind anlegen/ }),
    );

    expect(await screen.findByText(SUCCESS)).toBeVisible();
    expectSlideOverUnlocked(TITLE);
    expect(mocks.createManualApprovedEnrollment).toHaveBeenLastCalledWith(
      "5",
      expect.objectContaining({ create_new_student: true }),
    );
  });

  it("shows no success of a late answer in a panel opened again", async () => {
    const answer = deferred<{ status_url: string }>();
    mocks.createManualApprovedEnrollment.mockReturnValue(answer.promise);
    const { rerender } = render(<Modal />, { wrapper: ToastProvider });
    await fillAndSend();

    rerender(<Modal isOpen={false} />);
    rerender(<Modal />);
    // Anker: das neu geöffnete Panel ist geladen und leer.
    expect(await screen.findByRole("button", { name: SUBMIT })).toBeEnabled();
    expect(screen.getByLabelText("Interner Grund")).toHaveValue("");

    await act(async () => {
      answer.resolve({ status_url: "/anmeldung/status/alt" });
    });
    await waitFor(() => expect(mocks.submitSettled).toHaveBeenCalledOnce());

    expectSlideOverUnlocked(TITLE);
    expect(screen.queryByText(SUCCESS)).not.toBeInTheDocument();
  });
});
