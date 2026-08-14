import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

const { captureExceptionMock } = vi.hoisted(() => ({
  captureExceptionMock: vi.fn(),
}));

vi.mock("@sentry/nextjs", () => ({
  captureException: captureExceptionMock,
}));

import { StudentDepartureErrorBoundary } from "./student-departure-error-boundary";

let departureEditorBroken = true;

function DepartureEditor() {
  if (departureEditorBroken) throw new Error("picker render failed");
  return <div>Erlaubte Heimwege</div>;
}

describe("StudentDepartureErrorBoundary", () => {
  beforeEach(() => {
    departureEditorBroken = true;
    vi.clearAllMocks();
  });

  it("keeps a recoverable error visible and reports the render failure", () => {
    const consoleError = vi
      .spyOn(console, "error")
      .mockImplementation(() => undefined);

    render(
      <StudentDepartureErrorBoundary>
        <DepartureEditor />
      </StudentDepartureErrorBoundary>,
    );

    expect(
      screen.getByText(
        "Die erlaubten Heimwege konnten nicht angezeigt werden. Bitte versuchen Sie es erneut.",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("button", { name: "Erneut versuchen" }),
    ).toBeVisible();
    expect(captureExceptionMock).toHaveBeenCalledWith(
      expect.objectContaining({ message: "picker render failed" }),
    );
    expect(consoleError).toHaveBeenCalledWith(
      "student_departure_render_failed",
      { error: "picker render failed", error_name: "Error" },
    );

    departureEditorBroken = false;
    fireEvent.click(screen.getByRole("button", { name: "Erneut versuchen" }));
    expect(screen.getByText("Erlaubte Heimwege")).toBeVisible();
    expect(captureExceptionMock).toHaveBeenCalledTimes(1);

    consoleError.mockRestore();
  });
});
