import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({
  correctAdminChildData: vi.fn(),
}));

// The birthday field moved from a native input to the kit picker; this stub
// keeps it readable/settable as an input. Imported inside the factory because
// vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async (importOriginal) => {
  const { isoDatePickerMock } = await import("~/test/mocks/date-picker");
  return { ...(await importOriginal<object>()), ...isoDatePickerMock() };
});

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
    const initialChild = {
      id: "child-1",
      first_name: "Lina",
      last_name: "Falsch",
      date_of_birth: "2018-04-15",
      target_grade_level: 1,
      status: "approved" as const,
      activation_mode: "scheduled",
      created_student_id: "student-1",
    };
    const correctedChild = {
      ...initialChild,
      last_name: "Richtig",
      date_of_birth: "2018-05-16",
      target_grade_level: 2,
    };
    mocks.correctAdminChildData.mockResolvedValue({
      request: { children: [correctedChild] },
    });
    const onSaved = vi.fn();
    const { rerender } = render(
      <AdminChildDataCorrection
        requestId="request-1"
        onSaved={onSaved}
        child={initialChild}
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
      expect(onSaved).toHaveBeenCalledWith(correctedChild);
    });

    rerender(
      <AdminChildDataCorrection
        requestId="request-1"
        onSaved={onSaved}
        child={correctedChild}
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Anmeldedaten korrigieren" }),
    );

    expect(screen.getByLabelText("Nachname")).toHaveValue("Richtig");
    expect(screen.getByLabelText("Geburtsdatum")).toHaveValue("2018-05-16");
    expect(screen.getByLabelText("Ziel-Klassenstufe")).toHaveValue(2);
    expect(screen.getByLabelText("Grund der Korrektur")).toHaveValue("");
  });
});
