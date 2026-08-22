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
      sourceClassOptions={[]}
      sourceClassCounts={{}}
      sourceFilteredCount={0}
      sourceCountsPending={false}
      sourceCountsError={null}
      sourceRosterDiff={null}
      sourcePhaseKidsFromWarning={null}
      sourceOverlapWarnings={[]}
      changeSourceOfferings={changeSourceOfferings}
      toggleSourceGradeLevel={vi.fn()}
      toggleSourceSchoolClass={vi.fn()}
      changeSourceFilterMode={vi.fn()}
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

describe("StepPersonalKinder — Klassenfilter (#2482)", () => {
  const sourcedForm = (
    overrides: Partial<ReturnType<typeof emptyForm>> = {},
  ) => ({
    ...emptyForm("2026-08-03"),
    targetGroupType: "angebot" as const,
    sourceCareOfferingIds: ["42"],
    ...overrides,
  });

  it("offers the class filter only once a source is selected", () => {
    renderStep({ form: sourcedForm({ sourceCareOfferingIds: [] }) });
    expect(screen.queryByText("Kinder eingrenzen")).not.toBeInTheDocument();
  });

  it("shows classes with their child counts in the Klasse mode", () => {
    const toggleSourceSchoolClass = vi.fn();
    renderStep({
      form: sourcedForm({
        sourceFilterMode: "klasse",
        sourceSchoolClasses: ["1b"],
      }),
      sourceClassOptions: ["1a", "1b"],
      sourceClassCounts: { "1a": 4, "1b": 6 },
      sourceFilteredCount: 6,
      toggleSourceSchoolClass,
    });

    expect(
      screen.getByRole("checkbox", { name: "Klasse 1b (6)" }),
    ).toBeChecked();
    const other = screen.getByRole("checkbox", { name: "Klasse 1a (4)" });
    expect(other).not.toBeChecked();
    fireEvent.click(other);
    expect(toggleSourceSchoolClass).toHaveBeenCalledWith("1a");
  });

  it("hides the grade checkboxes while the Klasse mode is active", () => {
    renderStep({
      form: sourcedForm({ sourceFilterMode: "klasse" }),
      sourceGradeOptions: [1, 2],
      sourceClassOptions: ["1a"],
    });
    expect(
      screen.queryByRole("checkbox", { name: /^Jahrgang/ }),
    ).not.toBeInTheDocument();
  });

  it("switches the filter mode through the segmented control", () => {
    const changeSourceFilterMode = vi.fn();
    renderStep({
      form: sourcedForm(),
      changeSourceFilterMode,
    });
    fireEvent.click(screen.getByRole("button", { name: "Nach Klasse" }));
    expect(changeSourceFilterMode).toHaveBeenCalledWith("klasse");
  });

  it("says the count is still being determined instead of warning about zero", () => {
    renderStep({
      form: sourcedForm({ sourceFilterMode: "klasse" }),
      sourceFilteredCount: 0,
      sourceCountsPending: true,
    });
    expect(
      screen.getByText("Die Kinderzahl wird ermittelt ..."),
    ).toBeInTheDocument();
    expect(screen.queryByText(/erfasst keine Kinder/)).not.toBeInTheDocument();
  });

  it("says the children could not be loaded instead of holding the pending text", () => {
    renderStep({
      form: sourcedForm({ sourceFilterMode: "klasse" }),
      sourceFilteredCount: 0,
      sourceCountsPending: false,
      sourceCountsError:
        "Die Kinder der gewählten Angebote konnten nicht geladen werden.",
    });
    expect(
      screen.getByText(
        "Die Kinder der gewählten Angebote konnten nicht geladen werden.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Die Kinderzahl wird ermittelt ..."),
    ).not.toBeInTheDocument();
  });

  it("names the children joining and dropping out of a manual roster", () => {
    renderStep({
      form: sourcedForm({
        sourceFilterMode: "klasse",
        sourceSchoolClasses: ["1b"],
      }),
      sourceFilteredCount: 2,
      sourceRosterDiff: { added: ["Nele Braun"], removed: ["Ali Kaya"] },
    });
    expect(
      screen.getByText(
        "Das ändert sich gegenüber Ihrer bisherigen Kinderliste",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/Kommt neu dazu \(1\): Nele Braun/),
    ).toBeInTheDocument();
    expect(screen.getByText(/Fällt weg \(1\): Ali Kaya/)).toBeInTheDocument();
  });
});

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
        // Ein gespeicherter Jahrgangsfilter öffnet den Editor im
        // Jahrgangs-Modus (formFromSeries leitet das ab, #2482).
        sourceFilterMode: "jahrgang" as const,
      },
      offeringSources: [],
      sourceGradeOptions: [2],
      toggleSourceGradeLevel,
    });

    expect(screen.getByText("Kinder eingrenzen")).toBeInTheDocument();
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

  it("renders the phase-start warning supplied by the form", () => {
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
        "Die Betreuung aus „Schuljahr 2026/2027, 1. Halbjahr“ beginnt am 13.08.2026. Termine vor diesem Datum haben noch keine Kinder aus dem Angebot.",
    });

    expect(
      screen.getByText(
        /beginnt am 13\.08\.2026\. Termine vor diesem Datum haben noch keine Kinder/,
      ),
    ).toBeInTheDocument();
  });
});

describe("StepPersonalKinder — Maximale Teilnehmerzahl (#2233)", () => {
  it("renders the stored limit editable in the series flow", () => {
    const { update } = renderStep({
      form: { ...emptyForm("2026-08-03"), maxParticipants: "43" },
    });

    const input = screen.getByLabelText("Maximale Teilnehmerzahl");
    expect(input).toHaveValue(43);
    fireEvent.change(input, { target: { value: "20" } });
    expect(update).toHaveBeenCalledWith("maxParticipants", "20");
  });

  it("shows the unbegrenzt placeholder when no limit is stored", () => {
    renderStep({ form: { ...emptyForm("2026-08-03"), maxParticipants: "" } });

    expect(screen.getByLabelText("Maximale Teilnehmerzahl")).toHaveAttribute(
      "placeholder",
      "unbegrenzt",
    );
  });

  it("hides the field outside the series flow — a single occurrence has no capacity of its own", () => {
    renderStep({ isSeriesFlow: false });

    expect(
      screen.queryByLabelText("Maximale Teilnehmerzahl"),
    ).not.toBeInTheDocument();
  });

  it("renders the validation error", () => {
    renderStep({
      fieldErrors: {
        maxParticipants:
          "Bitte eine ganze Zahl größer als 0 angeben oder das Feld leer lassen.",
      },
    });

    expect(
      screen.getByText(
        "Bitte eine ganze Zahl größer als 0 angeben oder das Feld leer lassen.",
      ),
    ).toBeInTheDocument();
  });
});
