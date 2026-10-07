import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FamilyProtectionControl } from "./family-protection-control";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import {
  getFamilyProtection,
  setFamilyProtection,
} from "~/lib/change-request-list-api";
import { catalogText } from "~/test/error-catalog-text";

vi.mock("~/lib/change-request-list-api", () => ({
  getFamilyProtection: vi.fn(),
  setFamilyProtection: vi.fn(),
}));

const mockGet = vi.mocked(getFamilyProtection);
const mockSet = vi.mocked(setFamilyProtection);

function renderControl(initialEnabled?: boolean) {
  return render(
    <ToastProvider>
      <FamilyProtectionControl
        studentId="10"
        canManage
        initialEnabled={initialEnabled}
      />
    </ToastProvider>,
  );
}

describe("FamilyProtectionControl", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("zeigt einen Ladefehler vor Ort und lädt per Wiederholen neu", async () => {
    mockGet
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValueOnce({ student_id: "10", enabled: true });

    renderControl();

    expect(
      await screen.findByText(
        "Die Einstellung zum Familienschutz ist gerade nicht erreichbar. Bitte versuchen Sie es erneut.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("alert", { name: /^Fehler:/ }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(await screen.findByText("Eingeschaltet")).toBeVisible();
    expect(screen.queryByText(/nicht erreichbar/)).toBeNull();
  });

  it("zeigt einen Speicherfehler im Dialog und markiert die Begründung", async () => {
    mockSet.mockRejectedValueOnce(
      new ApiError("reason required", 400, {
        code: "students.reason_required",
        errors: [{ field: "reason", reason: "required" }],
      }),
    );

    renderControl(false);

    fireEvent.click(screen.getByRole("button", { name: "Einschalten" }));
    const reason = screen.getByLabelText("Grund für die Änderung");
    fireEvent.change(reason, { target: { value: "Schutz nötig" } });
    fireEvent.click(screen.getByRole("button", { name: "Schutz einschalten" }));

    expect(
      await screen.findByText(
        catalogText(
          "students.reason_required",
          "die Einstellung zum Familienschutz",
        ),
      ),
    ).toBeVisible();
    await waitFor(() => expect(reason).toHaveAttribute("aria-invalid", "true"));
    expect(
      screen.getByRole("button", { name: "Schutz einschalten" }),
    ).toBeVisible();
  });
});
