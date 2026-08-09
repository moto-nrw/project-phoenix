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
      sourcePhaseLockId={null}
      sourceGradeOptions={[]}
      sourceGradeCounts={{}}
      sourceFilteredCount={0}
      sourcePhaseKidsFromWarning={null}
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

  it("keeps the grade filter editable when the stored source is missing from the list", () => {
    // With a stored-but-unlisted source, hasOfferingSource still hides the
    // manual student picker — so the grade fieldset must render (gated on the
    // form selection, not on the resolved options) or no roster control is
    // reachable at all.
    const toggleSourceGradeLevel = vi.fn();
    renderStep({
      form: {
        ...emptyForm("2026-08-03"),
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["42"],
        sourceGradeLevels: [2],
      },
      offeringSources: [],
      sourceGradeOptions: [2],
      toggleSourceGradeLevel,
    });

    expect(screen.getByText("Nach Jahrgang filtern")).toBeInTheDocument();
    const grade = screen.getByRole("checkbox", { name: "Jahrgang 2 (0)" });
    expect(grade).toBeChecked();
    fireEvent.click(grade);
    expect(toggleSourceGradeLevel).toHaveBeenCalledWith(2);
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

  it("shows the phase-start warning when the series starts before the offering phase", () => {
    renderStep({
      form: {
        ...emptyForm("2026-08-10"),
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["7"],
        sourceGradeLevels: [1],
      },
      offeringSources: [
        {
          id: "7",
          name: "Ganztag bis 14:30",
          phaseId: "3",
          phaseName: "Schuljahr 2026/2027, 1. Halbjahr",
          phaseServiceStart: "2026-08-13",
          totalCount: 26,
          gradeCounts: { 1: 26 },
          sourcedTemplates: [],
        },
      ],
      sourceGradeOptions: [1],
      sourceGradeCounts: { 1: 26 },
      sourceFilteredCount: 26,
      sourcePhaseKidsFromWarning:
        "„Schuljahr 2026/2027, 1. Halbjahr“ startet am 13.08.2026. Termine vor diesem Datum haben noch keine Kinder aus dem Angebot.",
    });

    expect(
      screen.getByText(
        /startet am 13\.08\.2026\. Termine vor diesem Datum haben noch keine Kinder/,
      ),
    ).toBeInTheDocument();
  });
});
