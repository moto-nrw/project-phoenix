import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createElement, type ComponentProps } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { Student } from "~/lib/api";
import type { AllowedDepartureModes } from "~/lib/student-helpers";

const { pickerShouldThrow, captureExceptionMock } = vi.hoisted(() => ({
  pickerShouldThrow: { current: false },
  captureExceptionMock: vi.fn(),
}));

vi.mock("@sentry/nextjs", () => ({ captureException: captureExceptionMock }));

vi.mock("./companion-picker", async (importOriginal) => {
  const original = await importOriginal<typeof import("./companion-picker")>();
  return {
    ...original,
    CompanionPicker: (
      props: ComponentProps<typeof original.CompanionPicker>,
    ) => {
      if (pickerShouldThrow.current) throw new Error("picker render failed");
      return createElement(original.CompanionPicker, props);
    },
  };
});

vi.mock("~/lib/hooks/use-student-photos-enabled", () => ({
  useStudentPhotosEnabled: () => ({ enabled: false, isLoading: false }),
}));

vi.mock("~/lib/hooks/use-student-enrollment-extra-fields", () => ({
  useStudentEnrollmentExtraFields: () => ({
    groups: [],
    loading: false,
    hasError: false,
  }),
}));

vi.mock("~/lib/hooks/use-companion-remote-refresh", () => ({
  useCompanionRemoteRefresh: () => ({
    companionsStale: false,
    refreshFromRemote: vi.fn(),
    withOwnWrite: <T,>(write: () => Promise<T>) => write(),
    markStale: vi.fn(),
  }),
}));

vi.mock("~/lib/student-companion-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-companion-api")>()),
  fetchStudentCompanions: vi.fn().mockResolvedValue([]),
}));

vi.mock("~/lib/student-api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/lib/student-api")>()),
  fetchStudentPrivacyConsent: vi.fn().mockResolvedValue(null),
  uploadStudentPhoto: vi.fn(),
  deleteStudentPhoto: vi.fn(),
}));

vi.mock("./student-photo-section", () => ({
  StudentPhotoSection: () => null,
}));

vi.mock("./student-common-form-sections", () => ({
  StudentCommonFormSections: () => <div data-testid="common-form-sections" />,
}));

import { StudentStammdatenTab } from "./student-stammdaten-tab";

const busEveryDay = {
  mon: ["bus"],
  tue: ["bus"],
  wed: ["bus"],
  thu: ["bus"],
  fri: ["bus"],
} satisfies AllowedDepartureModes;

function productionShapedStudent(): Student {
  return {
    id: "564",
    name: "Test Kind",
    first_name: "Test",
    second_name: "Kind",
    school_class: "3a",
    current_location: "class",
    allowed_departure_modes: busEveryDay,
    departure_days: {
      mon: "bus",
      tue: "bus",
      wed: "bus",
      thu: "bus",
      fri: "bus",
    },
    bus: true,
    bus_days: { mon: true, tue: true, wed: true, thu: true, fri: true },
    pickup_days: {},
    departure_companion_note: undefined,
  } as Student;
}

describe("StudentStammdatenTab departure integration", () => {
  beforeEach(() => {
    pickerShouldThrow.current = false;
    vi.clearAllMocks();
  });

  it("keeps the real form usable when accompanied is added beside bus", async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(
      <StudentStammdatenTab
        student={productionShapedStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );

    await waitFor(() =>
      expect(screen.getByLabelText("Montag: Bus")).toBeChecked(),
    );
    fireEvent.click(screen.getByLabelText("Montag: Anderes Kind"));

    expect(screen.getByLabelText("Montag: Anderes Kind")).toBeChecked();
    expect(
      screen.getByRole("button", { name: "Kind hinzufügen" }),
    ).toBeVisible();
    expect(onSave).not.toHaveBeenCalled();
    const note = screen.getByLabelText("Oder mit welcher Person?");
    fireEvent.change(note, { target: { value: "Nachbarin" } });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    await waitFor(() => expect(onSave).toHaveBeenCalledTimes(1));
    expect(onSave).toHaveBeenCalledWith(
      expect.objectContaining({
        allowed_departure_modes: expect.objectContaining({
          mon: ["bus", "accompanied"],
        }),
        departure_companion_note: "Nachbarin",
      }),
    );
  });

  it("shows a rejected save without unmounting the departure editor", async () => {
    const onSave = vi.fn().mockRejectedValue(new Error("Keine Berechtigung"));
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    render(
      <StudentStammdatenTab
        student={productionShapedStudent()}
        groups={[]}
        onSave={onSave}
      />,
    );
    await waitFor(() =>
      expect(screen.getByLabelText("Montag: Bus")).toBeChecked(),
    );

    fireEvent.click(screen.getByLabelText("Montag: Anderes Kind"));
    fireEvent.change(screen.getByLabelText("Oder mit welcher Person?"), {
      target: { value: "Nachbarin" },
    });
    fireEvent.click(screen.getByRole("button", { name: /Speichern/ }));

    expect(
      await screen.findByText(
        "Fehler beim Speichern. Bitte versuchen Sie es erneut.",
      ),
    ).toBeVisible();
    expect(screen.getByLabelText("Montag: Anderes Kind")).toBeChecked();
    expect(
      screen.getByRole("button", { name: "Kind hinzufügen" }),
    ).toBeVisible();
    expect(consoleError).toHaveBeenCalledWith("error saving student", {
      error: "Keine Berechtigung",
    });
    consoleError.mockRestore();
  });

  it("contains a picker render failure inside the composed student editor", async () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);
    render(
      <StudentStammdatenTab
        student={productionShapedStudent()}
        groups={[]}
        onSave={vi.fn()}
      />,
    );
    await waitFor(() =>
      expect(screen.getByLabelText("Montag: Bus")).toBeChecked(),
    );

    pickerShouldThrow.current = true;
    fireEvent.click(screen.getByLabelText("Montag: Anderes Kind"));

    expect(screen.getByRole("button", { name: /Speichern/ })).toBeVisible();
    expect(
      screen.getByText(
        "Die erlaubten Heimwege konnten nicht angezeigt werden. Bitte versuchen Sie es erneut.",
      ),
    ).toBeVisible();
    expect(captureExceptionMock).toHaveBeenCalledWith(
      expect.objectContaining({ message: "picker render failed" }),
    );
    expect(consoleError).toHaveBeenCalledWith(
      "student_departure_render_failed",
      { error: "picker render failed", error_name: "Error" },
    );
    consoleError.mockRestore();
  });
});
