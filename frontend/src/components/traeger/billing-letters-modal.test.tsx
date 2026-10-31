import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import { fireEvent, render, screen } from "@testing-library/react";
import { expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { BillingLettersModal } from "./billing-letters-modal";

const { createBillingLetters } = vi.hoisted(() => ({
  createBillingLetters: vi.fn(),
}));
vi.mock("~/lib/traeger-billing-api", () => ({
  createBillingLetters,
  previewBillingLetter: vi.fn(),
}));

it.each(schoolDateCases("2026-05-27"))(
  "names the failed creation with a singular subject in %s",
  async (zone, instant) => {
    setSchoolDateClock(zone, instant, "2026-05-27");

    createBillingLetters.mockRejectedValue(
      new ApiError("Internal error", 500, { code: "general.server" }),
    );
    render(
      <ToastProvider>
        <BillingLettersModal
          runId="9"
          isOpen
          onClose={vi.fn()}
          onCreated={vi.fn()}
        />
      </ToastProvider>,
    );
    expect(screen.getByLabelText("Zahlen bis")).toHaveValue("10.06.2026");
    fireEvent.click(
      screen.getByRole("button", { name: "Schreiben erstellen" }),
    );
    expect(
      await screen.findByText(/Das Erstellen der Schreiben konnte nicht/),
    ).toBeInTheDocument();
  },
);
