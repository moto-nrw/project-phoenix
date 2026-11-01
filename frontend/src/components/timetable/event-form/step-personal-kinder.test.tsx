import "@testing-library/jest-dom/vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { createRef } from "react";
import { describe, expect, it, vi } from "vitest";

const {
  createExternal,
  getOfferingPreview,
  getOfferingSuggestion,
  getDraftOfferingPreview,
} = vi.hoisted(() => ({
  createExternal: vi.fn(),
  getOfferingPreview: vi.fn(),
  getOfferingSuggestion: vi.fn(),
  getDraftOfferingPreview: vi.fn(),
}));
vi.mock("~/lib/staff-api", () => ({
  staffService: { createExternal, inviteExternal: vi.fn() },
}));
vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getOfferingPreview,
    getOfferingSuggestion,
    getDraftOfferingPreview,
  },
}));

import { StepPersonalKinder } from "./step-personal-kinder";
import { emptyForm } from "./form-model";

function renderStep(
  overrides: Partial<React.ComponentProps<typeof StepPersonalKinder>> = {},
) {
  const update = vi.fn();
  const changeSourceOfferings = vi.fn();
  const element = (
    extra: Partial<React.ComponentProps<typeof StepPersonalKinder>> = {},
  ) => (
    <StepPersonalKinder
      form={{ ...emptyForm("2026-08-03"), targetGroupType: "angebot" }}
      update={update}
      changeTargetGroupType={vi.fn()}
      fieldErrors={{}}
      groups={[]}
      students={[]}
      staff={[]}
      addStaffOption={vi.fn()}
      loadingRefs={false}
      loadingStudents={false}
      studentLoadFailed={false}
      studentLoadError={null}
      loadingStaff={false}
      staffLoadFailed={false}
      staffLoadError={null}
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
      offeringPhasesOutsidePeriod={[]}
      canManageEnrollmentPhases={false}
      sourcePhaseLockId={null}
      sourceGradeOptions={[]}
      sourceGradeCounts={{}}
      sourceClassOptions={[]}
      sourceClassCounts={{}}
      sourceFilteredCount={0}
      sourceCountsPending={false}
      sourceCountsFailed={false}
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
      coverageCheckFailed={false}
      conflictCheckFailed={false}
      requiredStaffTouched={createRef<boolean>() as React.RefObject<boolean>}
      staffRosterTouched={createRef<boolean>() as React.RefObject<boolean>}
      activeRosterWeekday={1}
      setActiveRosterWeekday={vi.fn()}
      setPerWeekdayRoster={vi.fn()}
      setWeekdayRoster={vi.fn()}
      applyActiveWeekdayRosterToAll={vi.fn()}
      {...overrides}
      {...extra}
    />
  );
  const { rerender } = render(element());
  return {
    update,
    changeSourceOfferings,
    rerenderWith: (
      extra: Partial<React.ComponentProps<typeof StepPersonalKinder>>,
    ) => rerender(element(extra)),
  };
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
    // Anker: die Angebots-Ansicht ohne Quelle zeigt ihren Hinweis.
    expect(
      screen.getByText(/Mit Angeboten als Quelle übernimmt der Regeltermin/),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kinder eingrenzen")).not.toBeInTheDocument();
  });

  it("offers the class filter with a selected source", () => {
    renderStep({ form: sourcedForm() });
    expect(screen.getByText("Kinder eingrenzen")).toBeInTheDocument();
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

  // #5101: Sonnenhof speichert „Klasse 1b“; die Checkbox hieß „Klasse Klasse 1b (6)“.
  it("does not double the class word for classes stored as „Klasse 1b“", () => {
    renderStep({
      form: sourcedForm({ sourceFilterMode: "klasse" }),
      sourceClassOptions: ["Klasse 1b"],
      sourceClassCounts: { "klasse 1b": 6 },
    });

    expect(
      screen.getByRole("checkbox", { name: "Klasse 1b (6)" }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Klasse Klasse/)).not.toBeInTheDocument();
  });

  it("hides the grade checkboxes while the Klasse mode is active", () => {
    renderStep({
      form: sourcedForm({ sourceFilterMode: "klasse" }),
      sourceGradeOptions: [1, 2],
      sourceClassOptions: ["1a"],
    });
    // Anker: im Klasse-Modus steht die Klassen-Auswahl da.
    expect(
      screen.getByRole("checkbox", { name: /^Klasse 1a/ }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: /^Jahrgang/ }),
    ).not.toBeInTheDocument();
  });

  it("shows the grade checkboxes in the Jahrgang mode", () => {
    renderStep({
      form: sourcedForm({ sourceFilterMode: "jahrgang" }),
      sourceGradeOptions: [1, 2],
      sourceClassOptions: ["1a"],
    });
    expect(screen.getAllByRole("checkbox", { name: /^Jahrgang/ })).toHaveLength(
      2,
    );
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
    const retry = vi.fn();
    renderStep({
      form: sourcedForm({ sourceFilterMode: "klasse" }),
      sourceFilteredCount: 0,
      sourceCountsPending: false,
      sourceCountsFailed: true,
      sourceCountsError: {
        message:
          "Die Kinderzahl der gewählten Angebote ist gerade nicht erreichbar.",
        attempt: 1,
        retry: { label: "Wiederholen", onClick: retry },
      },
    });
    expect(
      screen.getByText(
        "Die Kinderzahl der gewählten Angebote ist gerade nicht erreichbar.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(
      screen.getByText("Die Klassen konnten nicht geladen werden."),
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

describe("StepPersonalKinder — Personal je Wochentag mit Angebot als Quelle (#3165)", () => {
  const sourcedPerWeekdayForm = () => ({
    ...emptyForm("2026-08-03"),
    targetGroupType: "angebot" as const,
    sourceCareOfferingIds: ["42"],
    weekdays: [1, 2],
    perWeekdayRoster: true,
    weekdayRosters: {
      1: { staffIds: [], primaryStaffId: "", studentIds: [] },
      2: { staffIds: [], primaryStaffId: "", studentIds: ["12"] },
    },
  });

  it("offers per-weekday staff but no per-weekday child list", () => {
    renderStep({ form: sourcedPerWeekdayForm() });

    expect(
      screen.getByRole("group", { name: "Wochentag für die Zuordnung" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Personal am Montag")).toBeInTheDocument();
    expect(screen.queryByText("Kinder am Montag")).not.toBeInTheDocument();
    expect(
      screen.getByText(/Die Kinderliste kommt aus den gewählten/),
    ).toBeInTheDocument();
  });

  it("does not flag a weekday as deviating because of hidden child lists", () => {
    renderStep({ form: sourcedPerWeekdayForm(), activeRosterWeekday: 2 });

    expect(screen.queryByText("abweichend")).not.toBeInTheDocument();
    expect(screen.getByText("wie üblich")).toBeInTheDocument();
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

    // Anker: das Nachbarfeld „Benötigtes Personal“ steht da.
    expect(screen.getByLabelText("Benötigtes Personal")).toBeInTheDocument();
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

describe("StepPersonalKinder — Lade- und Prüfhinweise (#2516)", () => {
  it("shows a failed roster load with its text and what the save keeps", () => {
    const retry = vi.fn();
    renderStep({
      form: emptyForm("2026-08-03"),
      staffLoadFailed: true,
      staffLoadError: {
        message: "Die Personalliste ist gerade nicht erreichbar.",
        attempt: 1,
        retry: { label: "Wiederholen", onClick: retry },
      },
    });

    expect(
      screen.getByText("Die Personalliste ist gerade nicht erreichbar."),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "Die Personalzuordnung bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(retry).toHaveBeenCalledOnce();
  });

  it("keeps the roster read-only while the load error text is still loading", () => {
    renderStep({
      form: emptyForm("2026-08-03"),
      studentLoadFailed: true,
      studentLoadError: null,
    });

    expect(
      screen.getByText(
        "Die Kinderzuordnung bleibt beim Speichern unverändert.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Kinder")).not.toBeInTheDocument();
  });

  it("names double bookings from the warning fields, not the server sentence", () => {
    renderStep({
      form: emptyForm("2026-08-03"),
      staff: [{ id: "11", name: "Ada Staff" }],
      conflictWarnings: [
        {
          kind: "staff",
          resourceId: "11",
          message: "Servertext",
          conflictingInstanceId: "99",
          conflictingTitle: "Mensa",
        },
      ],
    });

    expect(
      screen.getByText(
        "Hinweis: Ada Staff ist zur selben Zeit schon bei „Mensa“ eingeplant.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/Servertext/)).not.toBeInTheDocument();
  });

  it("says when the double-booking check could not run", () => {
    renderStep({ form: emptyForm("2026-08-03"), conflictCheckFailed: true });

    expect(
      screen.getByText(
        "Hinweis: Doppelte Einplanungen konnten nicht geprüft werden. Sie können trotzdem speichern.",
      ),
    ).toBeInTheDocument();
  });
});

// #4573: externe Personen direkt im Schritt „Personal“ eintragen.
describe("StepPersonalKinder — externe Person", () => {
  const plainForm = () => ({
    ...emptyForm("2026-08-03"),
    targetGroupType: "none" as const,
  });

  it("records and invites an external person and adds them to the roster", async () => {
    const created = {
      id: "88",
      name: "Lea Gast",
      firstName: "Lea",
      lastName: "Gast",
      hasRfid: false,
      isTeacher: true,
      isSupervising: false,
      supervisions: [],
      isExternal: true,
      externalAccountStatus: "invitation_pending" as const,
    };
    createExternal.mockResolvedValueOnce(created);
    const addStaffOption = vi.fn();
    const { update } = renderStep({
      form: { ...plainForm(), staffIds: ["1"] },
      staff: [{ id: "1", name: "Interne Kraft" }],
      canCreateExternalCaregiver: true,
      addStaffOption,
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Externe Person eintragen" }),
    );
    fireEvent.change(screen.getByLabelText("Vorname"), {
      target: { value: "Lea" },
    });
    fireEvent.change(screen.getByLabelText("Nachname"), {
      target: { value: "Gast" },
    });
    fireEvent.click(
      screen.getByRole("radio", { name: /Eigene Termine betreuen/ }),
    );
    fireEvent.change(screen.getByLabelText("E-Mail-Adresse"), {
      target: { value: "lea@example.test" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Eintragen" }));

    await waitFor(() => expect(addStaffOption).toHaveBeenCalledWith(created));
    expect(update).toHaveBeenCalledWith("staffIds", ["1", "88"]);
  });

  // ADR 0091: „Nur ansehen“ only where the planner may grant the role Gast.
  it.each([
    [true, true],
    [false, false],
  ])(
    "offers „Nur ansehen“ only with canGrantExternalView=%s",
    (canGrantExternalView, offered) => {
      renderStep({
        form: plainForm(),
        staff: [{ id: "1", name: "Interne Kraft" }],
        canCreateExternalCaregiver: true,
        canGrantExternalView,
      });

      fireEvent.click(
        screen.getByRole("button", { name: "Externe Person eintragen" }),
      );

      // Anker: die Zugangswahl ist offen.
      expect(screen.getByRole("radio", { name: /Kein Zugang/ })).toBeChecked();
      expect(
        screen.queryByRole("radio", { name: /Nur ansehen/ }) !== null,
      ).toBe(offered);
    },
  );

  it("offers no entry without permission to create users", () => {
    renderStep({
      form: plainForm(),
      staff: [{ id: "1", name: "Interne Kraft" }],
    });

    // Anker: die Personalliste ist geladen.
    expect(screen.getByText("Interne Kraft")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Externe Person eintragen" }),
    ).not.toBeInTheDocument();
  });
});

describe("StepPersonalKinder — Vorschau pro Wochentag (#5103)", () => {
  it("previews an existing series switched to an offering per weekday", async () => {
    getOfferingPreview.mockResolvedValue({
      from: "2026-10-28",
      weekdays: [
        {
          weekday: 1,
          added: [],
          removed: [{ studentId: "5", name: "Ben Kurz" }],
          keptCount: 3,
        },
      ],
      segments: [
        { templateId: "8", validFrom: "2026-09-01", validUntil: "2026-10-30" },
        { templateId: "9", validFrom: "2026-10-30" },
      ],
    });
    renderStep({
      form: {
        ...emptyForm("2026-08-03"),
        targetGroupType: "angebot",
        sourceCareOfferingIds: ["42"],
        sourceFilterMode: "klasse",
        sourceSchoolClasses: ["1b"],
      },
      sourcePreviewTemplateId: "9",
      sourcePreviewFrom: "2026-10-28",
    });

    expect(await screen.findByText("Montag")).toBeInTheDocument();
    expect(screen.getByText("Fällt weg (1): Ben Kurz")).toBeInTheDocument();
    expect(getOfferingPreview).toHaveBeenCalledWith(
      "9",
      { offeringIds: ["42"], gradeLevels: [], schoolClasses: ["1b"] },
      "2026-10-28",
    );
    expect(
      screen.getByText(/Die Termine vom 28.10.2026 bis 29.10.2026/),
    ).toBeInTheDocument();
  });
});

describe("StepPersonalKinder — Vorschlag „Angebot übernehmen“ (#5104)", () => {
  const classForm = (
    overrides: Partial<ReturnType<typeof emptyForm>> = {},
  ) => ({
    ...emptyForm("2026-09-14", "4"),
    listKind: "edge_hours" as const,
    targetGroupType: "klasse" as const,
    targetSchoolClasses: ["1b"],
    weekdays: [1, 3],
    ...overrides,
  });
  const cohort = { label: "Klasse 1b", memberIds: ["11", "12", "13"] };
  const randstunde = {
    offerings: [{ id: "7", name: "Randstunde" }],
    gradeLevels: [],
    schoolClasses: ["1b"],
    unbookedCount: 1,
    missingCount: 0,
    candidates: [{ id: "7", name: "Randstunde" }],
  };
  const suggestionLine =
    "Sollen nur die für „Randstunde“ angemeldeten Kinder kommen? Dann verbinden Sie den Termin mit dem Angebot.";

  it("suggests the offering whose bookings match Klasse 1b on its weekdays", async () => {
    getOfferingSuggestion.mockReset().mockResolvedValue(randstunde);
    renderStep({ form: classForm(), targetCohort: cohort });

    expect(await screen.findByText(suggestionLine)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Angebot übernehmen" }),
    ).toBeInTheDocument();
    expect(getOfferingSuggestion).toHaveBeenCalledWith({
      periodId: "4",
      type: "care",
      listKind: "edge_hours",
      endTime: "13:00",
      roster: { 1: ["11", "12", "13"], 3: ["11", "12", "13"] },
      schoolClasses: ["1b"],
      gradeLevels: [],
    });
    // The honest text stays above the suggestion.
    expect(
      screen.getByText(/Es kommen alle Kinder der Klasse 1b/),
    ).toBeInTheDocument();
  });

  it("takes over the offering with the Klassenfilter and previews against the old list", async () => {
    getOfferingSuggestion.mockReset().mockResolvedValue(randstunde);
    getDraftOfferingPreview.mockReset().mockResolvedValue({
      from: "2026-09-14",
      weekdays: [
        {
          weekday: 1,
          added: [],
          removed: [{ studentId: "13", name: "Ada Muster" }],
          keptCount: 2,
        },
      ],
      segments: [],
    });
    const changeTargetGroupType = vi.fn();
    const changeSourceFilterMode = vi.fn();
    const { update, changeSourceOfferings, rerenderWith } = renderStep({
      form: classForm(),
      targetCohort: cohort,
      changeTargetGroupType,
      changeSourceFilterMode,
    });

    fireEvent.click(
      await screen.findByRole("button", { name: "Angebot übernehmen" }),
    );

    expect(changeTargetGroupType).toHaveBeenCalledWith("angebot");
    expect(changeSourceOfferings).toHaveBeenCalledWith(["7"]);
    expect(changeSourceFilterMode).toHaveBeenCalledWith("klasse");
    expect(update).toHaveBeenCalledWith("sourceSchoolClasses", ["1b"]);

    rerenderWith({
      form: classForm({
        targetGroupType: "angebot",
        targetSchoolClasses: [],
        sourceCareOfferingIds: ["7"],
        sourceFilterMode: "klasse",
        sourceSchoolClasses: ["1b"],
      }),
      sourceFilteredCount: 2,
    });

    expect(await screen.findByText(/Fällt weg \(1\)/)).toBeInTheDocument();
    expect(getDraftOfferingPreview).toHaveBeenCalledWith(
      "4",
      { offeringIds: ["7"], gradeLevels: [], schoolClasses: ["1b"] },
      { 1: ["11", "12", "13"], 3: ["11", "12", "13"] },
      "2026-09-14",
    );
    expect(screen.queryByText(suggestionLine)).not.toBeInTheDocument();
  });

  it("shows only the honest text when no offering matches", async () => {
    getOfferingSuggestion.mockReset().mockResolvedValue(null);
    renderStep({ form: classForm(), targetCohort: cohort });

    await waitFor(() => expect(getOfferingSuggestion).toHaveBeenCalled());
    expect(
      screen.getByText(/Es kommen alle Kinder der Klasse 1b/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Angebot übernehmen" }),
    ).not.toBeInTheDocument();
  });

  it("does not ask with fewer than three children", async () => {
    getOfferingSuggestion.mockReset().mockResolvedValue(randstunde);
    renderStep({
      form: classForm(),
      targetCohort: { label: "Klasse 1b", memberIds: ["11", "12"] },
    });

    // Anchor: the step rendered its honest text.
    expect(
      screen.getByText(/Es kommen alle Kinder der Klasse 1b/),
    ).toBeInTheDocument();
    await new Promise((resolve) => setTimeout(resolve, 600));
    expect(getOfferingSuggestion).not.toHaveBeenCalled();
  });
});

describe("StepPersonalKinder — mehrere passende Angebote (#5138)", () => {
  const cohortForm = (overrides: Partial<ReturnType<typeof emptyForm>>) => ({
    ...emptyForm("2026-09-14", "4"),
    listKind: "edge_hours" as const,
    weekdays: [1, 3],
    ...overrides,
  });
  const cohort = { label: "Klasse 3a", memberIds: ["11", "12", "13"] };
  const roster = { 1: ["11", "12", "13"], 3: ["11", "12", "13"] };
  // A Klasse or Jahrgang target plans every child of the cohort; both
  // offerings then cover it fully and the backend marks the answer.
  const ambiguous = (filter: {
    schoolClasses: string[];
    gradeLevels: number[];
  }) => ({
    offerings: [{ id: "4", name: "Ganztag bis 14.30 Uhr" }],
    ...filter,
    unbookedCount: 1,
    missingCount: 0,
    candidates: [
      { id: "4", name: "Ganztag bis 14.30 Uhr" },
      { id: "7", name: "Randstunde" },
    ],
    ambiguous: true,
  });
  const clear = {
    offerings: [{ id: "7", name: "Randstunde" }],
    gradeLevels: [],
    schoolClasses: [],
    unbookedCount: 1,
    missingCount: 0,
    candidates: [{ id: "7", name: "Randstunde" }],
    ambiguous: false,
  };
  const choiceLine =
    "Sollen nur Kinder kommen, die für ein Angebot angemeldet sind? Dazu passen mehrere Angebote. Wählen Sie eins.";

  it.each([
    {
      target: "Klasse",
      form: { targetGroupType: "klasse" as const, targetSchoolClasses: ["3a"] },
      filter: { schoolClasses: ["3a"], gradeLevels: [] },
      mode: "klasse",
      field: ["sourceSchoolClasses", ["3a"]],
    },
    {
      target: "Jahrgang",
      form: { targetGroupType: "jahrgang" as const, targetGradeLevels: ["3"] },
      filter: { schoolClasses: [], gradeLevels: [3] },
      mode: "jahrgang",
      field: ["sourceGradeLevels", [3]],
    },
  ])(
    "offers every fitting offering of the $target as an equal choice and preselects none",
    async ({ form, filter, mode, field }) => {
      getOfferingSuggestion.mockReset().mockResolvedValue(ambiguous(filter));
      const changeTargetGroupType = vi.fn();
      const changeSourceFilterMode = vi.fn();
      const { update, changeSourceOfferings } = renderStep({
        form: cohortForm(form),
        targetCohort: cohort,
        changeTargetGroupType,
        changeSourceFilterMode,
      });

      expect(await screen.findByText(choiceLine)).toBeInTheDocument();
      expect(getOfferingSuggestion).toHaveBeenCalledWith({
        periodId: "4",
        type: "care",
        listKind: "edge_hours",
        endTime: "13:00",
        roster,
        ...filter,
      });
      const ganztag = screen.getByRole("button", {
        name: "„Ganztag bis 14.30 Uhr“ übernehmen",
      });
      const randstunde = screen.getByRole("button", {
        name: "„Randstunde“ übernehmen",
      });
      // Equal choices look the same.
      expect(ganztag.className).toBe(randstunde.className);
      expect(
        screen.queryByRole("button", { name: "Angebot übernehmen" }),
      ).not.toBeInTheDocument();
      expect(changeSourceOfferings).not.toHaveBeenCalled();

      fireEvent.click(randstunde);

      expect(changeTargetGroupType).toHaveBeenCalledWith("angebot");
      expect(changeSourceOfferings).toHaveBeenCalledWith(["7"]);
      expect(changeSourceFilterMode).toHaveBeenCalledWith(mode);
      expect(update).toHaveBeenCalledWith(...field);
    },
  );

  it.each([
    {
      target: "Gruppe",
      form: { targetGroupType: "gruppe" as const, educationGroupIds: ["g1"] },
      targetCohort: { label: "Gruppe Bären", memberIds: ["11", "12", "13"] },
    },
    {
      target: "Einzelkind",
      form: {
        targetGroupType: "none" as const,
        studentIds: ["11", "12", "13"],
      },
      targetCohort: { label: null, memberIds: [] },
    },
  ])(
    "keeps the single suggestion for a $target",
    async ({ form, targetCohort }) => {
      getOfferingSuggestion.mockReset().mockResolvedValue(clear);
      const { changeSourceOfferings } = renderStep({
        form: cohortForm(form),
        targetCohort,
      });

      expect(
        await screen.findByText(
          "Sollen nur die für „Randstunde“ angemeldeten Kinder kommen? Dann verbinden Sie den Termin mit dem Angebot.",
        ),
      ).toBeInTheDocument();
      expect(getOfferingSuggestion).toHaveBeenCalledWith({
        periodId: "4",
        type: "care",
        listKind: "edge_hours",
        endTime: "13:00",
        roster,
        schoolClasses: [],
        gradeLevels: [],
      });
      expect(screen.queryByText(choiceLine)).not.toBeInTheDocument();

      fireEvent.click(
        screen.getByRole("button", { name: "Angebot übernehmen" }),
      );

      expect(changeSourceOfferings).toHaveBeenCalledWith(["7"]);
    },
  );
});

describe("StepPersonalKinder — mehrere Angebote zusammen (#5139)", () => {
  it.each(["4", "8"])(
    "keeps Randstunde when choosing Ganztag %s",
    async (id) => {
      const randstunde = { id: "7", name: "Randstunde" };
      const ganztag = { id: "4", name: "Ganztag bis 14.30 Uhr" };
      const spaet = { id: "8", name: "Ganztag bis 16 Uhr" };
      getOfferingSuggestion.mockReset().mockResolvedValue({
        offerings: [ganztag, randstunde],
        combinations: [
          [ganztag, randstunde],
          [spaet, randstunde],
        ],
        candidates: [ganztag, randstunde, spaet],
        gradeLevels: [],
        schoolClasses: ["1a"],
        unbookedCount: 0,
        missingCount: 0,
        ambiguous: true,
      });
      const { changeSourceOfferings } = renderStep({
        form: {
          ...emptyForm("2026-09-14", "4"),
          listKind: "edge_hours" as const,
          targetGroupType: "klasse" as const,
          targetSchoolClasses: ["1a"],
          weekdays: [1, 3, 4],
        },
        targetCohort: { label: "Klasse 1a", memberIds: ["11", "12", "13"] },
      });

      const name = id === "4" ? ganztag.name : spaet.name;
      const choice = await screen.findByRole("button", {
        name: `„${name}“ und „Randstunde“ übernehmen`,
      });
      expect(changeSourceOfferings).not.toHaveBeenCalled();
      fireEvent.click(choice);
      expect(changeSourceOfferings).toHaveBeenCalledWith([id, "7"]);
    },
  );

  it("takes over Randstunde and Ganztag together for Klasse 1a", async () => {
    getOfferingSuggestion.mockReset().mockResolvedValue({
      offerings: [
        { id: "7", name: "Randstunde" },
        { id: "4", name: "Ganztag" },
      ],
      gradeLevels: [],
      schoolClasses: ["1a"],
      unbookedCount: 1,
      missingCount: 0,
      candidates: [
        { id: "7", name: "Randstunde" },
        { id: "4", name: "Ganztag" },
      ],
      ambiguous: false,
    });
    const changeTargetGroupType = vi.fn();
    const changeSourceFilterMode = vi.fn();
    const { update, changeSourceOfferings } = renderStep({
      form: {
        ...emptyForm("2026-09-14", "4"),
        listKind: "edge_hours" as const,
        targetGroupType: "klasse" as const,
        targetSchoolClasses: ["1a"],
        weekdays: [1, 3, 4],
      },
      targetCohort: { label: "Klasse 1a", memberIds: ["11", "12", "13"] },
      changeTargetGroupType,
      changeSourceFilterMode,
    });

    expect(
      await screen.findByText(
        "Sollen nur die für „Randstunde“ oder „Ganztag“ angemeldeten Kinder kommen? Dann verbinden Sie den Termin mit diesen Angeboten.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Angebote übernehmen" }),
    );

    expect(changeTargetGroupType).toHaveBeenCalledWith("angebot");
    expect(changeSourceOfferings).toHaveBeenCalledWith(["7", "4"]);
    expect(changeSourceFilterMode).toHaveBeenCalledWith("klasse");
    expect(update).toHaveBeenCalledWith("sourceSchoolClasses", ["1a"]);
  });
});

// #5344: offerings of a phase reaching beyond the period are missing from the
// choice; the step names the phase instead of leaving the list unexplained.
describe("StepPersonalKinder — Anmeldephase außerhalb des Zeitraums", () => {
  const phase = {
    phaseId: "41",
    phaseName: "Schuljahr 2026/27",
    serviceStart: "2026-08-01",
    serviceEnd: "2027-07-31",
    offeringCount: 1,
    periodStart: "2026-08-01",
    periodEnd: "2027-07-07",
  };

  it("names the phase whose offerings the choice leaves out", () => {
    renderStep({
      offeringPhasesOutsidePeriod: [phase],
      canManageEnrollmentPhases: true,
    });
    const notice = screen.getByTestId("phases-outside-period-notice");
    expect(notice).toHaveTextContent(
      "„Schuljahr 2026/27“ wird hier nicht geprüft",
    );
    expect(notice).toHaveTextContent(
      "Ihr Angebot steht hier deshalb nicht zur Auswahl.",
    );
    expect(
      screen.getByRole("link", { name: "Anmeldephasen öffnen" }),
    ).toBeInTheDocument();
  });

  it("says that phases can fall outside the period at either end", () => {
    renderStep({
      offeringPhasesOutsidePeriod: [
        {
          ...phase,
          phaseId: "42",
          serviceStart: "2026-07-20",
          serviceEnd: "2026-08-15",
        },
        phase,
      ],
    });

    expect(
      screen.getByTestId("phases-outside-period-notice"),
    ).toHaveTextContent(
      "Sie passen nicht vollständig in diesen Planungszeitraum",
    );
  });

  it("stays quiet when every phase fits the period", () => {
    renderStep();
    expect(screen.getByLabelText("Angebote als Quelle")).toBeInTheDocument();
    expect(
      screen.queryByTestId("phases-outside-period-notice"),
    ).not.toBeInTheDocument();
  });
});
