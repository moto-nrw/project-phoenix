import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";
import { StepPersonalKinder } from "./step-personal-kinder";
import { emptyForm } from "./form-model";

function renderStep(
  overrides: Partial<React.ComponentProps<typeof StepPersonalKinder>> = {},
) {
  const update = vi.fn();
  const changeSourceOfferings = vi.fn();
  render(
    <StepPersonalKinder
      form={{ ...emptyForm("2026-08-03"), targetGroupType: "angebot" }}
      update={update}
      changeTargetGroupType={vi.fn()}
      fieldErrors={{}}
      groups={[]}
      students={[]}
      staff={[]}
      loadingRefs={false}
      loadingStudents={false}
      studentLoadError={null}
      loadingStaff={false}
      staffLoadError={null}
      retryStudentLoad={vi.fn().mockResolvedValue(undefined)}
      retryStaffLoad={vi.fn().mockResolvedValue(undefined)}
      expanded
      isSeriesFlow
      gradeLevelMax={undefined}
      targetGradeOptions={[]}
      preservesGradeAboveTenantCap={false}
      studentBulkOptions={[]}
      targetClassOptions={[]}
      targetCohort={{ label: null, memberIds: [] }}
      missingTargetCohortCount={0}
      targetCohortButtonLabel=""
      addTargetCohort={vi.fn()}
      offeringSources={[]}
      offeringSourcesError={null}
      selectedOfferingSources={[]}
      sourcePhaseLockId={null}
      sourceGradeOptions={[]}
      sourceGradeCounts={{}}
      sourceFilteredCount={0}
      sourceOverlapWarnings={[]}
      changeSourceOfferings={changeSourceOfferings}
      toggleSourceGradeLevel={vi.fn()}
      conflictWarnings={[]}
      coverageWarnings={[]}
      coverageWarningCount={0}
      coverageCheckError={null}
      requiredStaffTouched={createRef<boolean>() as React.RefObject<boolean>}
      staffRosterTouched={createRef<boolean>() as React.RefObject<boolean>}
      activeRosterWeekday={1}
      setActiveRosterWeekday={vi.fn()}
      setPerWeekdayRoster={vi.fn()}
      setWeekdayRoster={vi.fn()}
      applyActiveWeekdayRosterToAll={vi.fn()}
      {...overrides}
    />,
  );
  return { update, changeSourceOfferings };
}

describe("StepPersonalKinder — Angebots-Quelle", () => {
  it("keeps a stored source removable when it is missing from the fetched list", () => {
    // A template whose saved sources are not in the calendar-period-filtered
    // list (deactivated offering, other period) must still be clearable —
    // otherwise the select is stuck disabled and the manual roster stays
    // hidden because a source is set.
    const { changeSourceOfferings } = renderStep({
      form: {
        ...emptyForm("2026-08-03"),
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["42"],
      },
      offeringSources: [],
    });

    const trigger = screen.getByRole("button", {
      name: "Angebote als Quelle",
    });
    expect(trigger).toBeEnabled();
    fireEvent.click(trigger);

    const entry = screen.getByRole("checkbox", {
      name: "Nicht mehr verfügbares Angebot (ID 42)",
    });
    expect(entry).toBeChecked();
    fireEvent.click(entry);
    expect(changeSourceOfferings).toHaveBeenCalledWith([]);
  });

  it("does not invent entries when every stored source is in the list", () => {
    renderStep({
      form: {
        ...emptyForm("2026-08-03"),
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["7"],
      },
      offeringSources: [
        {
          id: "7",
          name: "Betreuung bis 14:30",
          phaseId: "1",
          phaseName: "Phase 2026",
          totalCount: 12,
          gradeCounts: {},
          sourcedTemplates: [],
        },
      ],
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Angebote als Quelle" }),
    );
    expect(
      screen.queryByText(/Nicht mehr verfügbares Angebot/),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("checkbox", { name: /Betreuung bis 14:30/ }),
    ).toBeChecked();
  });
});
