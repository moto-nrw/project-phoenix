import "@testing-library/jest-dom/vitest";
import { render, screen } from "@testing-library/react";
import { createRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useWeekendFollowsFriday } from "~/lib/tenant-context";
import { ToastProvider } from "~/contexts/ToastContext";
import { emptyForm } from "./form-model";
import { StepTermin } from "./step-termin";

// The kit date picker is a portal calendar; a plain input exposes whether a
// Saturday can be picked (#3921).
vi.mock("~/components/ui/date-picker", async (importOriginal) => ({
  ...(await importOriginal<typeof import("~/components/ui/date-picker")>()),
  ISODatePicker: ({
    id,
    disabledDay,
  }: {
    id?: string;
    disabledDay?: (date: Date) => boolean;
  }) => (
    <input
      id={id}
      data-testid={id}
      data-saturday-disabled={String(
        disabledDay?.(new Date("2026-09-05T00:00:00")) ?? false,
      )}
    />
  ),
}));

function renderStep() {
  render(
    <ToastProvider>
      <StepTermin
        form={{ ...emptyForm("2026-08-03"), type: "care" }}
        update={vi.fn()}
        fieldErrors={{}}
        rooms={[]}
        categories={[]}
        loadingRefs={false}
        expanded
        isSeriesFlow={false}
        isEditingSeries={false}
        quickPreset=""
        listKindTouched={createRef<boolean>() as React.RefObject<boolean>}
        canManageCategories={false}
      />
    </ToastProvider>,
  );
}

describe("StepTermin — Wochenende (#3921)", () => {
  afterEach(() => {
    vi.mocked(useWeekendFollowsFriday).mockReturnValue(false);
  });

  it("keeps Saturday out of the date picker by default", () => {
    renderStep();
    expect(screen.getByTestId("event_date")).toHaveAttribute(
      "data-saturday-disabled",
      "true",
    );
  });

  it("offers Saturday when the weekend follows Friday's plan", () => {
    vi.mocked(useWeekendFollowsFriday).mockReturnValue(true);
    renderStep();
    expect(screen.getByTestId("event_date")).toHaveAttribute(
      "data-saturday-disabled",
      "false",
    );
  });
});
