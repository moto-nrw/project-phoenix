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

it("names the failed creation with a singular subject", async () => {
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
  fireEvent.click(screen.getByRole("button", { name: "Schreiben erstellen" }));
  expect(
    await screen.findByText(/Das Erstellen der Schreiben konnte nicht/),
  ).toBeInTheDocument();
});
