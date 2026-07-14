import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  correctAdminChildData: vi.fn(),
}));

vi.mock("~/lib/enrollment-admin-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return { ...actual, correctAdminChildData: mocks.correctAdminChildData };
});

import { AdminChildDataCorrection } from "./admin-child-data-correction";

describe("AdminChildDataCorrection", () => {
  beforeEach(() => {
    mocks.correctAdminChildData.mockReset();
  });

  it("marks the reason as required and submits corrected enrollment data", async () => {
    mocks.correctAdminChildData.mockResolvedValue({});
    const onSaved = vi.fn();
    render(
      <AdminChildDataCorrection
        requestId="request-1"
        onSaved={onSaved}
        child={{
          id: "child-1",
          first_name: "Lina",
          last_name: "Falsch",
          date_of_birth: "2018-04-15",
          target_grade_level: 1,
          status: "approved",
          activation_mode: "scheduled",
          created_student_id: "student-1",
        }}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldedaten korrigieren" }),
    );
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Richtig" },
    });
    fireEvent.change(screen.getByLabelText("Geburtsdatum"), {
      target: { value: "2018-05-16" },
    });
    fireEvent.change(screen.getByLabelText("Ziel-Klassenstufe"), {
      target: { value: "2" },
    });
    const reason = screen.getByLabelText("Grund der Korrektur");
    expect(reason).toBeRequired();
    fireEvent.change(reason, {
      target: { value: " Nach Rücksprache korrigiert " },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Korrektur speichern" }),
    );

    await waitFor(() => {
      expect(mocks.correctAdminChildData).toHaveBeenCalledWith(
        "request-1",
        "child-1",
        {
          first_name: "Lina",
          last_name: "Richtig",
          date_of_birth: "2018-05-16",
          target_grade_level: 2,
          target_school_class: undefined,
          reason: "Nach Rücksprache korrigiert",
        },
      );
      expect(onSaved).toHaveBeenCalledOnce();
    });
  });
});
