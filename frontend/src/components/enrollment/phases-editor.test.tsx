import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

const careExit = vi.hoisted(() => ({
  permissions: [] as string[],
  preview: vi.fn(),
  confirm: vi.fn(),
}));

vi.mock("~/lib/care-exit-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  previewCareExit: careExit.preview,
  confirmCareExit: careExit.confirm,
}));

// useTenantMutate scopes its keys to the signed-in account (#5006).
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: { id: "1", permissions: careExit.permissions },
      expires: "2099-01-01",
    },
    status: "authenticated",
  }),
}));

vi.mock("vaul", async () => {
  const original = await (await import("~/test/enrollment-editors")).vaulMock();
  return {
    Drawer: {
      ...original.Drawer,
      Root: ({
        children,
        open,
        dismissible,
        onOpenChange,
      }: {
        children: React.ReactNode;
        open?: boolean;
        dismissible?: boolean;
        onOpenChange?: (open: boolean) => void;
      }) =>
        open === false ? null : (
          <div>
            <button
              type="button"
              disabled={dismissible === false}
              onClick={() => onOpenChange?.(false)}
            >
              Dismiss panel
            </button>
            {children}
          </div>
        ),
    },
  };
});

const mocks = await vi.hoisted(async () =>
  (await import("~/test/enrollment-editors")).createEditorMocks(),
);

// The date and datetime fields moved from native inputs to the kit pickers;
// these stubs keep them settable via fireEvent.change. Imported inside the
// factories because vi.mock is hoisted above the imports.
vi.mock("~/components/ui/date-picker", async () =>
  (await import("~/test/mocks/date-picker")).datePickerModuleMock(),
);

vi.mock("~/components/ui/date-time-picker", async (importOriginal) => {
  const { dateTimePickerMock } = await import("~/test/mocks/date-time-picker");
  return { ...(await importOriginal<object>()), ...dateTimePickerMock() };
});

vi.mock("next/navigation", () => ({
  usePathname: () => "/demo/enrollment-phases",
  useRouter: () => ({ replace: mocks.routerReplace }),
  useSearchParams: () => mocks.searchParams,
}));

vi.mock("~/lib/tenant-context", () => ({
  useTenant: () => ({
    tenantSlug: "demo",
    tenant: { gradeLevelMax: mocks.gradeLevelMax },
    routingMode: "path",
  }),
  useTenantSlugSafe: () => "demo",
  useTenantRoutingModeSafe: () => "path",
  useNFCEnabled: vi.fn(() => true),
}));

// Form, load and dialog errors run through the real hooks (#2515); only the
// toast surface and the toast path for actions without a form are doubles.
vi.mock("~/contexts/ToastContext", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useToast: () => mocks.toast,
  useApiErrorDisplay: () => ({
    show: mocks.showActionError,
    showMessage: mocks.showActionMessage,
  }),
}));

vi.mock("~/lib/swr", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  useSWRAuth: () => ({ data: [], error: undefined, isLoading: false }),
}));

vi.mock("./rollover-form", async () =>
  (await import("~/test/enrollment-editors")).rolloverFormMock(),
);

vi.mock("~/lib/enrollment-phase-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createPhase: mocks.createPhase,
    deletePhase: mocks.deletePhase,
    getPhaseDeleteImpact: mocks.getPhaseDeleteImpact,
    listPhaseExpiryWarnings: mocks.listPhaseExpiryWarnings,
    listPhases: mocks.listPhases,
    updatePhase: mocks.updatePhase,
  };
});

vi.mock("~/lib/enrollment-form-schema-api", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    createSchema: mocks.createSchema,
    deleteEnrollmentLegalDocument: mocks.deleteEnrollmentLegalDocument,
    deleteSchema: mocks.deleteSchema,
    fetchPublicLegalTexts: mocks.fetchPublicLegalTexts,
    listSchemas: mocks.listSchemas,
    renameSchema: mocks.renameSchema,
    updateSchema: mocks.updateSchema,
    uploadEnrollmentLegalDocument: mocks.uploadEnrollmentLegalDocument,
  };
});

vi.mock("~/lib/care-offering-api", () => ({
  cloneCareOffering: mocks.cloneCareOffering,
  createCareOffering: mocks.createCareOffering,
  deleteCareOffering: mocks.deleteCareOffering,
  listCareOfferings: mocks.listCareOfferings,
  updateCareOffering: mocks.updateCareOffering,
  // Real const consumed by CareOfferingForm's selection-rule dropdown.
  SELECTION_RULE_LABELS: {
    optional: "Optional (frei wählbar)",
    exactly_one: "Genau eines (Pflicht)",
    at_least_one: "Mindestens eines (Pflicht)",
    at_most_one: "Höchstens eines",
  },
}));

vi.mock("~/lib/care-offering-booking-stats", async (importOriginal) => {
  const actual = (await importOriginal()) as Record<string, unknown>;
  return {
    ...actual,
    fetchCareOfferingBookingStats: mocks.fetchCareOfferingBookingStats,
  };
});

vi.mock("~/lib/timetable-api", () => ({
  timetableService: {
    getTemplates: mocks.getTemplates,
  },
}));

vi.mock("~/lib/calendar-period-api", () => ({
  calendarPeriodService: {
    list: mocks.listCalendarPeriods,
  },
}));

vi.mock("~/lib/enrollment-admin-api", () => ({
  createLateInvite: mocks.createLateInvite,
  createManualApprovedEnrollment: mocks.createManualApprovedEnrollment,
  fetchManualEnrollmentBootstrap: mocks.fetchManualEnrollmentBootstrap,
}));

vi.mock("~/components/enrollment/enrollment-form", () => ({
  EnrollmentForm: (props: {
    lockChildStructure?: boolean;
    gradeLevelMax?: number;
    withoutSignature?: boolean;
  }) => {
    mocks.enrollmentFormProps.push(props);
    return (
      <div
        data-testid="manual-enrollment-form"
        data-lock-child-structure={String(props.lockChildStructure)}
      />
    );
  },
}));

import { PhasesEditor } from "./phases-editor";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import {
  phase,
  schema,
  inputByName,
  waitForInputByName,
  chooseOption,
  resetEditorMocks,
} from "~/test/enrollment-editors";

beforeEach(() => {
  resetEditorMocks(mocks);
  careExit.permissions = [];
  careExit.preview.mockReset();
  careExit.confirm.mockReset();
});

describe("PhasesEditor", () => {
  it("keeps the editor open while saving through outside dismissal", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    let finish!: (value: ReturnType<typeof phase>) => void;
    mocks.updatePhase.mockImplementation(
      () =>
        new Promise<ReturnType<typeof phase>>((resolve) => {
          finish = resolve;
        }),
    );
    render(<PhasesEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mocks.updatePhase).toHaveBeenCalled());
    expect(
      screen.getByRole("button", { name: "Dismiss panel" }),
    ).toBeDisabled();
    finish(phase());
    await waitFor(() =>
      expect(
        screen.queryByRole("button", { name: "Dismiss panel" }),
      ).not.toBeInTheDocument(),
    );
  });

  // #6598: Kürzt die Schule das Phasenende, nennt der Dialog nach dem
  // Speichern die schon betreuten Kinder mit beiden Enden.
  it("names the children whose care runs past a shortened end after saving", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue({
      ...phase({ service_end_date: "2027-06-30" }),
      careBeyondEnd: [
        {
          studentId: "7",
          firstName: "Mia",
          lastName: "Lang",
          schoolClass: "2b",
          careEnd: "2027-07-31",
          phaseEnd: "2027-06-30",
        },
      ],
    });
    render(<PhasesEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));

    const dialog = await screen.findByRole("dialog", {
      name: "Betreuung läuft länger als die Phase",
    });
    expect(within(dialog).getByText("Mia Lang")).toBeVisible();
    expect(within(dialog).getByText("Betreuung bis 31.07.2027")).toBeVisible();
    expect(
      within(dialog).getByText(/endet jetzt am 30\.06\.2027/),
    ).toBeVisible();
    // Die Sitzung dieser Suite hat kein users:delete.
    expect(
      within(dialog).getByText(/Die Betreuung beenden darf nur/),
    ).toBeVisible();
    // Anker: der Hinweis nennt das betroffene Kind und die zuständige Person.
    expect(
      within(dialog).queryByRole("button", { name: "Betreuung beenden" }),
    ).not.toBeInTheDocument();
    fireEvent.click(within(dialog).getByRole("button", { name: "Schließen" }));
    await waitFor(() =>
      expect(
        screen.queryByRole("dialog", {
          name: "Betreuung läuft länger als die Phase",
        }),
      ).not.toBeInTheDocument(),
    );
  });

  it.each([
    [
      "2027-06-30",
      "Soll die Betreuung mit der Phase enden? Dann beenden Sie sie zum 30.06.2027.",
    ],
    [
      "2026-09-01",
      "Soll die Betreuung mit der Phase enden? Das neue Ende ist schon vorbei. Die Betreuung kann frühestens heute enden.",
    ],
  ])(
    "names every child and explains the proposed end %s",
    async (phaseEnd, explanation) => {
      careExit.permissions = ["users:delete"];
      mocks.listPhases.mockResolvedValue([phase()]);
      mocks.listSchemas.mockResolvedValue([schema()]);
      mocks.updatePhase.mockResolvedValue({
        ...phase({ service_end_date: phaseEnd }),
        careBeyondEnd: [
          {
            studentId: "7",
            firstName: "Mia",
            lastName: "Lang",
            schoolClass: "2b",
            careEnd: "2027-07-31",
            phaseEnd,
          },
          {
            studentId: "8",
            firstName: "Ben",
            lastName: "Berg",
            schoolClass: "",
            careEnd: "2027-07-15",
            phaseEnd,
          },
        ],
      });
      render(<PhasesEditor />);
      fireEvent.click(
        await screen.findByRole("button", {
          name: "Aktionen für Schuljahr 2026/27",
        }),
      );
      fireEvent.click(
        await screen.findByRole("menuitem", { name: "Bearbeiten" }),
      );
      fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
      const dialog = await screen.findByRole("dialog", {
        name: "Betreuung läuft länger als die Phase",
      });
      expect(within(dialog).getByText("Mia Lang")).toBeVisible();
      expect(within(dialog).getByText("Klasse 2b")).toBeVisible();
      expect(
        within(dialog).getByText("Betreuung bis 31.07.2027"),
      ).toBeVisible();
      expect(within(dialog).getByText("Ben Berg")).toBeVisible();
      expect(
        within(dialog).getByText("Betreuung bis 15.07.2027"),
      ).toBeVisible();
      expect(
        within(dialog).getByText(
          /Bei 2 Kindern ist die Betreuung länger eingetragen/,
        ),
      ).toBeVisible();
      expect(
        within(dialog).getByText(
          /moto beendet sie nicht von selbst. Diese Liste sehen Sie nur jetzt/,
        ),
      ).toBeVisible();
      expect(within(dialog).getByText(explanation)).toBeVisible();
      expect(
        within(dialog).getByRole("button", { name: "Betreuung beenden" }),
      ).toBeEnabled();
      fireEvent.click(
        within(dialog).getByRole("button", { name: "Schließen" }),
      );
      // Anker: die Phasenliste bleibt nach dem Schließen verfügbar.
      expect(
        screen.getByRole("button", { name: "Aktionen für Schuljahr 2026/27" }),
      ).toBeVisible();
      expect(
        screen.queryByRole("dialog", {
          name: "Betreuung läuft länger als die Phase",
        }),
      ).not.toBeInTheDocument();
    },
  );

  it.each([
    ["one child", false, false],
    ["all children", true, false],
    ["a child changed after preview", false, true],
  ])(
    "can exclude blocked children: %s",
    async (_scenario, allBlocked, changedAfterPreview) => {
      careExit.permissions = ["users:delete"];
      const children = ["7", "8"].map((studentId) => ({
        studentId,
        firstName: `Kind ${studentId}`,
        lastName: "Lang",
        schoolClass: "2b",
        careEnd: "2027-07-31",
        phaseEnd: "2027-06-30",
      }));
      mocks.listPhases.mockResolvedValue([phase()]);
      mocks.listSchemas.mockResolvedValue([schema()]);
      mocks.updatePhase.mockResolvedValue({
        ...phase({ service_end_date: "2027-06-30" }),
        careBeyondEnd: children,
      });
      careExit.preview.mockImplementation(
        async (input: { studentIds: string[] }) => {
          const students = input.studentIds.map((studentId) => ({
            studentId,
            firstName: `Kind ${studentId}`,
            lastName: "Lang",
            blocker:
              (allBlocked || studentId === "7") &&
              (!changedAfterPreview || careExit.confirm.mock.calls.length > 0)
                ? "Dieses Kind ist bereits abgeschlossen."
                : "",
            sourceOfferings: [],
            weeklyPlans: [],
          }));
          return {
            token: `preview-${input.studentIds.join("-")}`,
            lastCareDay: "2027-06-30",
            blocked: students.some((student) => student.blocker),
            students,
          };
        },
      );
      if (changedAfterPreview) {
        careExit.confirm.mockRejectedValueOnce(
          new Error("Die Vorschau ist veraltet."),
        );
      }
      careExit.confirm.mockResolvedValue({ studentsEnded: 1 });
      render(<PhasesEditor />);
      fireEvent.click(
        await screen.findByRole("button", {
          name: "Aktionen für Schuljahr 2026/27",
        }),
      );
      fireEvent.click(
        await screen.findByRole("menuitem", { name: "Bearbeiten" }),
      );
      fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
      const notice = await screen.findByRole("dialog", {
        name: "Betreuung läuft länger als die Phase",
      });
      fireEvent.click(
        within(notice).getByRole("button", { name: "Betreuung beenden" }),
      );
      fireEvent.click(await screen.findByRole("combobox", { name: "Grund" }));
      fireEvent.click(screen.getByRole("option", { name: "Umzug" }));
      fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
      if (changedAfterPreview) {
        const confirm = await screen.findByRole("button", {
          name: "Betreuung beenden",
        });
        await waitFor(() => {
          expect(confirm).toBeEnabled();
          expect(confirm).not.toHaveAttribute("aria-disabled");
        });
        fireEvent.click(confirm);
      }
      expect(
        await screen.findByText("Die Betreuung wurde nicht beendet"),
      ).toBeVisible();
      const confirm = screen.getByRole("button", { name: "Betreuung beenden" });
      await waitFor(() =>
        expect(confirm).toHaveAttribute("aria-disabled", "true"),
      );
      const exclude = screen.getByRole("button", {
        name: "Aus Auswahl nehmen",
      });
      await waitFor(() => expect(exclude).toBeEnabled());
      fireEvent.click(exclude);
      if (!allBlocked) {
        const remaining = await screen.findByRole("dialog", {
          name: "Betreuung läuft länger als die Phase",
        });
        expect(within(remaining).getByText("Kind 8 Lang")).toBeVisible();
        // Anker: der Hinweis zeigt das verbleibende Kind.
        expect(
          within(remaining).queryByText("Kind 7 Lang"),
        ).not.toBeInTheDocument();
        fireEvent.click(
          within(remaining).getByRole("button", { name: "Betreuung beenden" }),
        );
        fireEvent.click(await screen.findByRole("combobox", { name: "Grund" }));
        fireEvent.click(screen.getByRole("option", { name: "Umzug" }));
        fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
        const retryConfirm = await screen.findByRole("button", {
          name: "Betreuung beenden",
        });
        await waitFor(() => {
          expect(retryConfirm).toBeEnabled();
          expect(retryConfirm).not.toHaveAttribute("aria-disabled");
        });
        fireEvent.click(retryConfirm);
        await waitFor(() =>
          expect(careExit.confirm).toHaveBeenLastCalledWith(
            "preview-8",
            expect.objectContaining({
              studentIds: ["8"],
              lastCareDay: "2027-06-30",
            }),
          ),
        );
      }
      expect(
        await screen.findByRole("button", {
          name: "Aktionen für Schuljahr 2026/27",
        }),
      ).toBeVisible();
      // Anker: die Phasenliste ist nach Abschluss oder leerer Auswahl verfügbar.
      expect(
        screen.queryByRole("dialog", {
          name: "Betreuung läuft länger als die Phase",
        }),
      ).not.toBeInTheDocument();
      expect(careExit.confirm).toHaveBeenCalledTimes(
        allBlocked ? 0 : changedAfterPreview ? 2 : 1,
      );
    },
  );

  it("confirms at most 500 children at a time and keeps the rest", async () => {
    careExit.permissions = ["users:delete"];
    const children = Array.from({ length: 501 }, (_, index) => ({
      studentId: String(index + 1),
      firstName: `Kind ${index + 1}`,
      lastName: "Lang",
      schoolClass: "2b",
      careEnd: "2027-07-31",
      phaseEnd: "2027-06-30",
    }));
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue({
      ...phase({ service_end_date: "2027-06-30" }),
      careBeyondEnd: children,
    });
    careExit.preview.mockImplementation(
      async (input: { studentIds: string[] }) => ({
        token: "preview-token",
        lastCareDay: "2027-06-30",
        blocked: false,
        students: input.studentIds.map((studentId) => ({
          studentId,
          firstName: `Kind ${studentId}`,
          lastName: "Lang",
          blocker: "",
          sourceOfferings: [],
          weeklyPlans: [],
        })),
      }),
    );
    careExit.confirm.mockResolvedValue({ studentsEnded: 500 });
    render(<PhasesEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
    const notice = await screen.findByRole("dialog", {
      name: "Betreuung läuft länger als die Phase",
    });
    expect(within(notice).getByText("Kind 501 Lang")).toBeVisible();
    fireEvent.click(
      within(notice).getByRole("button", { name: "Betreuung beenden" }),
    );
    expect(
      await screen.findByText(
        "500 Kinder sind ausgewählt. Alle bekommen denselben Tag und denselben Grund.",
      ),
    ).toBeVisible();

    // Abbrechen behält auch die noch nicht bestätigte Auswahl.
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    const resumed = await screen.findByRole("dialog", {
      name: "Betreuung läuft länger als die Phase",
    });
    expect(within(resumed).getByText("Kind 1 Lang")).toBeVisible();
    expect(within(resumed).getByText("Kind 501 Lang")).toBeVisible();
    expect(
      within(resumed).getByText(
        "Sie beenden zuerst die Betreuung der ersten 500 Kinder. Danach bleiben die übrigen Kinder in dieser Liste.",
      ),
    ).toBeVisible();

    for (const [index, batch] of [
      children.slice(0, 500),
      children.slice(500),
    ].entries()) {
      fireEvent.click(
        screen.getByRole("button", { name: "Betreuung beenden" }),
      );
      expect(await screen.findByLabelText("Letzter Betreuungstag")).toHaveValue(
        "2027-06-30",
      );
      fireEvent.click(screen.getByRole("combobox", { name: "Grund" }));
      fireEvent.click(screen.getByRole("option", { name: "Umzug" }));
      fireEvent.click(screen.getByRole("button", { name: "Weiter" }));
      const confirm = await screen.findByRole("button", {
        name: "Betreuung beenden",
      });
      await waitFor(() => {
        expect(confirm).toBeEnabled();
        expect(confirm).not.toHaveAttribute("aria-disabled");
      });
      expect(careExit.preview).toHaveBeenNthCalledWith(
        index + 1,
        expect.objectContaining({
          studentIds: batch.map((child) => child.studentId),
        }),
      );
      fireEvent.click(confirm);
      await waitFor(() =>
        expect(careExit.confirm).toHaveBeenCalledTimes(index + 1),
      );
      expect(careExit.confirm).toHaveBeenNthCalledWith(
        index + 1,
        "preview-token",
        expect.objectContaining({
          studentIds: batch.map((child) => child.studentId),
        }),
      );
      if (index === 0) {
        const remaining = await screen.findByRole("dialog", {
          name: "Betreuung läuft länger als die Phase",
        });
        expect(within(remaining).getByText("Kind 501 Lang")).toBeVisible();
        // Anker: der Hinweis zeigt das noch nicht bestätigte Kind.
        expect(
          within(remaining).queryByText("Kind 1 Lang"),
        ).not.toBeInTheDocument();
      }
    }
    // Anker: nach der letzten Bestätigung ist die Phasenliste geladen.
    expect(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    ).toBeVisible();
    expect(
      screen.queryByRole("dialog", {
        name: "Betreuung läuft länger als die Phase",
      }),
    ).not.toBeInTheDocument();
  });

  it("uses the button foreground color for the create action icon", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    const button = await screen.findByRole("button", {
      name: "Neue Anmeldephase",
    });
    const icon = button.querySelector("[data-moto-duotone-tone]");

    expect(icon).toBeInTheDocument();
    expect(icon).not.toHaveStyle({ color: "#3730A3" });
    expect(icon?.getAttribute("style")).toContain(
      "--moto-icon-secondary: currentColor",
    );
  });

  it("creates a phase, assigns schemas, toggles status, deletes, and completes rollover", async () => {
    mocks.searchParams = new URLSearchParams("assignForm=schema-1");
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.createPhase.mockResolvedValue(
      phase({ id: "12", name: "Sommerferien" }),
    );
    mocks.updatePhase.mockResolvedValue(phase({ is_active: false }));
    mocks.deletePhase.mockResolvedValue(undefined);

    render(<PhasesEditor />);

    expect(await screen.findByText("Schuljahr 2026/27")).toBeInTheDocument();
    // Die Aktion liegt jetzt im Seitenkopf und rendert dort responsiv zweimal
    // (mobile Kopfzeile und Desktop-Zeile).
    fireEvent.click(
      screen.getAllByRole("button", { name: "Neue Anmeldephase" })[0]!,
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "Sommerferien" },
    });
    await chooseOption("Typ", "Ferienbetreuung");
    fireEvent.click(
      document.querySelector("#schema-source-reuse") as HTMLInputElement,
    );
    await chooseOption("Formular auswählen", "Regelformular");
    fireEvent.click(screen.getByText("Begründung für Eltern sichtbar"));
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createPhase).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Sommerferien",
          kind: "holiday",
          form_schema_id: "schema-1",
          show_status_reason_to_parent: true,
        }),
      );
    });
    await waitFor(() => {
      expect(mocks.listPhases.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("validates date windows before saving", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Anmeldephase" }),
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "Falsches Fenster" },
    });
    fireEvent.change(inputByName("enrollment_open_at"), {
      target: { value: "2026-03-10T10:00" },
    });
    fireEvent.change(inputByName("enrollment_close_at"), {
      target: { value: "2026-03-09T10:00" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Schließung des Anmeldefensters muss nach der Öffnung liegen.",
    );
    expect(mocks.createPhase).not.toHaveBeenCalled();
  });

  it("does not save a phase without both service dates", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Anmeldephase" }),
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "Unvollständiger Zeitraum" },
    });
    fireEvent.change(inputByName("service_start_date"), {
      target: { value: "" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(
      await screen.findByText(
        "Bitte geben Sie Beginn und Ende des Betreuungszeitraums an.",
      ),
    ).toBeInTheDocument();
    // Der fehlende Beginn steht zusätzlich am Feld.
    expect(
      screen.getByText("Bitte wählen Sie den Beginn."),
    ).toBeInTheDocument();
    expect(mocks.createPhase).not.toHaveBeenCalled();
  });

  it("marks the phase name as required through the kit field (#5217)", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Anmeldephase" }),
    );
    const nameInput = await screen.findByLabelText("Name");
    expect(nameInput).toHaveAttribute("name", "name");
    expect(nameInput).toHaveAttribute("aria-required", "true");
    // The star is not part of the label text; the kit Input draws it,
    // hidden from screen readers, for a required field (input.test.tsx).
    const label = document.querySelector('label[for="phase-name"]');
    expect(label).toHaveTextContent(/^Name$/);
  });

  it("shows a German validation error when the phase name is blank", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Anmeldephase" }),
    );
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "   " },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Bitte geben Sie der Anmeldephase einen Namen.",
    );
    // Das Feld ist markiert und hat den Fokus (#2515).
    await waitFor(() => expect(inputByName("name")).toHaveFocus());
    expect(inputByName("name")).toHaveAttribute("aria-invalid", "true");
    // Kein Toast mehr neben dem Alert (Bauart 2 Regel 5, #3113).
    expect(mocks.toast.error).not.toHaveBeenCalled();
    expect(mocks.createPhase).not.toHaveBeenCalled();
  });

  it("shows a failed phase load in place instead of the empty state (#2515)", async () => {
    mocks.listPhases
      .mockRejectedValueOnce(
        new ApiError("phases down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Anmeldephasen"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText("Noch keine Anmeldephase angelegt"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText(/0 aktiv/)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("Schuljahr 2026/27")).toBeInTheDocument();
  });

  it("marks the name field when the backend refuses a taken phase name (#2515)", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.createPhase.mockRejectedValue(
      new ApiError("phase name already exists", 409, {
        code: "enrollment.phase_name_exists",
        errors: [{ field: "name", reason: "taken" }],
      }),
    );

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Anmeldephase" }),
    );
    fireEvent.change(await waitForInputByName("name"), {
      target: { value: "Schuljahr 2026/27" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    expect(
      await screen.findByText(
        catalogText("enrollment.phase_name_exists", "die Anmeldephase"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("phase name already exists")).toBeNull();
    await waitFor(() =>
      expect(inputByName("name")).toHaveAttribute("aria-invalid", "true"),
    );
    expect(mocks.toast.error).not.toHaveBeenCalled();
  });

  it("reports a failed status change through the toast path (#2515)", async () => {
    const refused = new ApiError("forbidden", 403, {
      code: "general.permission",
    });
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockRejectedValue(refused);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Deaktivieren" }),
    );

    await waitFor(() =>
      expect(mocks.showActionError).toHaveBeenCalledWith(
        refused,
        expect.objectContaining({ object: "die Anmeldephase" }),
      ),
    );
  });

  it("shows the reload error instead of the empty state after creating the first phase", async () => {
    mocks.listPhases
      .mockReset()
      .mockResolvedValueOnce([])
      .mockRejectedValueOnce(new ApiError("failed", 503))
      .mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.createPhase.mockResolvedValue(phase());
    render(<PhasesEditor />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Anmeldephase" }),
    );
    fireEvent.change(inputByName("name"), {
      target: { value: "Schuljahr 2026/27" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));
    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige der Anmeldephasen")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeVisible();
    expect(
      screen.queryByText("Noch keine Anmeldephase angelegt"),
    ).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByText("Schuljahr 2026/27")).toBeVisible();
    expect(mocks.createPhase).toHaveBeenCalledTimes(1);
  });

  // #6652: the status change holds once the write is through; a failed
  // reload is a load error that says the change is saved, no action error.
  it("keeps a status change when the reload fails", async () => {
    mocks.showActionError.mockClear();
    mocks.updatePhase.mockClear();
    mocks.listPhases
      .mockReset()
      .mockResolvedValueOnce([phase()])
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue([phase({ is_active: false })]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(phase({ is_active: false }));

    render(<PhasesEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Deaktivieren" }),
    );

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige der Anmeldephasen")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    expect(mocks.showActionError).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mocks.listPhases).toHaveBeenCalledTimes(3));
    expect(mocks.updatePhase).toHaveBeenCalledTimes(1);
  });

  it("keeps the confirmed inactive state when editing after a failed reload", async () => {
    mocks.showActionError.mockClear();
    mocks.updatePhase.mockClear();
    mocks.listPhases
      .mockReset()
      .mockResolvedValueOnce([phase()])
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue([phase({ is_active: false })]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(phase({ is_active: false }));

    render(<PhasesEditor />);
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Deaktivieren" }),
    );

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige der Anmeldephasen")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    expect(mocks.showActionError).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Schuljahr 2026/27" }),
    );
    expect(
      await screen.findByRole("menuitem", { name: "Aktivieren" }),
    ).toBeVisible();
    fireEvent.click(screen.getByRole("menuitem", { name: "Bearbeiten" }));
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));
    await waitFor(() => expect(mocks.updatePhase).toHaveBeenCalledTimes(2));
    expect(mocks.updatePhase.mock.calls[1]?.[1]).toMatchObject({
      is_active: false,
    });
  });

  it("keeps a failed delete in the open dialog (#2515)", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.getPhaseDeleteImpact.mockResolvedValue({
      requests: 0,
      care_offerings: 1,
      students_kept: 0,
    });
    mocks.deletePhase.mockRejectedValue(
      new ApiError("conflict", 409, { code: "general.business_rejection" }),
    );

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    const dialog = await screen.findByRole("dialog");
    await waitFor(() => expect(mocks.getPhaseDeleteImpact).toHaveBeenCalled());
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Löschen" }),
    );
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Endgültig löschen" }),
    );

    expect(
      await within(dialog).findByText(
        catalogText("general.business_rejection", "die Anmeldephase"),
      ),
    ).toBeInTheDocument();
    expect(mocks.toast.error).not.toHaveBeenCalled();
  });

  it("explains in the dialog why a phase with booked children stays (#6505)", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    // The preview showed no request; one arrived before the delete ran.
    mocks.getPhaseDeleteImpact.mockResolvedValue({
      requests: 0,
      care_offerings: 1,
      students_kept: 0,
    });
    mocks.deletePhase.mockRejectedValue(
      new ApiError("phase still has effective care offering bookings", 409, {
        code: "enrollment.phase_has_effective_bookings",
      }),
    );

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    const dialog = await screen.findByRole("dialog");
    await waitFor(() => expect(mocks.getPhaseDeleteImpact).toHaveBeenCalled());
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Löschen" }),
    );
    fireEvent.click(
      await within(dialog).findByRole("button", { name: "Endgültig löschen" }),
    );

    expect(
      await within(dialog).findByText(
        "Für diese Anmeldephase sind noch Kinder gebucht. Deshalb kann sie nicht gelöscht werden. Wählen Sie stattdessen „Deaktivieren“ oder beenden Sie zuerst die Buchungen.",
      ),
    ).toBeInTheDocument();
  });

  it("blocks deleting a phase with requests and names „Deaktivieren“ instead (#6836)", async () => {
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.getPhaseDeleteImpact.mockResolvedValue({
      requests: 2,
      care_offerings: 1,
      students_kept: 1,
    });

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    const dialog = await screen.findByRole("dialog");

    expect(
      await within(dialog).findByText(
        "Zu ihr gibt es 2 Anmeldungen. Wählen Sie stattdessen „Deaktivieren“.",
      ),
    ).toBeInTheDocument();
    expect(
      within(dialog).getByRole("button", { name: "Löschen" }),
    ).toBeDisabled();
    expect(mocks.deletePhase).not.toHaveBeenCalled();
  });

  it("uses tenant-aware phase detail links in the action menu", async () => {
    mocks.listPhases.mockResolvedValue([phase({ id: "12" })]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );

    const link = await screen.findByRole("menuitem", {
      name: /Anmeldungen ansehen/,
    });
    expect(link).toHaveAttribute("href", "/demo/admin/enrollments/phases/12");
    expect(
      screen.getByRole("menuitem", { name: "Formular ansehen" }),
    ).toHaveAttribute("href", "/demo/anmeldung/12");
  });

  it("opens late invite and manual enrollment actions from the phase action menu", async () => {
    mocks.listPhases.mockResolvedValue([phase({ id: "12" })]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );

    fireEvent.click(
      await screen.findByRole("menuitem", {
        name: "Nachzügler-Link erstellen",
      }),
    );
    expect(
      await screen.findByRole("dialog", {
        name: "Nachzügler-Link erstellen",
      }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Modal schließen" }));
    fireEvent.click(
      screen.getByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Manuelle Anmeldung" }),
    );

    // Die manuelle Anmeldung laeuft seit der Dialog-Diaet als SlideOver; im
    // Vaul-Mock traegt das Panel keine dialog-Rolle, deshalb der Titel.
    expect(
      await screen.findByRole("heading", {
        name: "Kind manuell über Anmeldung freigeben",
      }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(mocks.fetchManualEnrollmentBootstrap).toHaveBeenCalledWith("12");
    });
    const form = await screen.findByTestId("manual-enrollment-form");
    expect(form).toHaveAttribute("data-lock-child-structure", "true");
    expect(mocks.enrollmentFormProps.at(-1)).toMatchObject({
      lockChildStructure: true,
      gradeLevelMax: 13,
      withoutSignature: true,
    });
  });

  it("hides the untokenized public actions for a linked_parents phase but keeps the late-invite action (#1663)", async () => {
    mocks.listPhases.mockResolvedValue([
      phase({ id: "12", name: "Nur Konto", audience: "linked_parents" }),
    ]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Aktionen für Nur Konto" }),
    );

    // The copy-link entry is gone: the plain /anmeldung/{id} URL is rejected
    // by the backend without a late-invite token, so copying it would hand
    // out a 404.
    expect(
      screen.queryByRole("menuitem", { name: "Elternlink kopieren" }),
    ).not.toBeInTheDocument();
    // "Formular ansehen" (the untokenized public form) is hidden...
    expect(
      screen.queryByRole("menuitem", { name: "Formular ansehen" }),
    ).not.toBeInTheDocument();
    // ...while the tokenized "Nachzügler-Link erstellen" action stays available.
    expect(
      await screen.findByRole("menuitem", {
        name: "Nachzügler-Link erstellen",
      }),
    ).toBeInTheDocument();
  });

  it("keeps the public form actions for an open phase (#1663)", async () => {
    mocks.listPhases.mockResolvedValue([
      phase({ id: "12", name: "Offene Phase", audience: "open" }),
    ]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Aktionen für Offene Phase" }),
    );
    expect(
      await screen.findByRole("menuitem", { name: "Formular ansehen" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("menuitem", { name: "Elternlink kopieren" }),
    ).toBeInTheDocument();
  });

  // #6044: the public form of an inactive or closed phase shows parents an
  // error, so „Formular ansehen“ never showed a school its offerings before
  // opening the enrollment. Every phase offers the staff preview instead.
  it.each([
    { name: "Inaktive Phase", overrides: { is_active: false } },
    { name: "Nur Konto", overrides: { audience: "linked_parents" as const } },
  ])(
    "offers the preview with offerings for a phase like $name",
    async ({ name, overrides }) => {
      mocks.listPhases.mockResolvedValue([
        phase({ id: "12", name, ...overrides }),
      ]);
      mocks.listSchemas.mockResolvedValue([schema()]);

      render(<PhasesEditor />);

      fireEvent.click(
        await screen.findByRole("button", { name: `Aktionen für ${name}` }),
      );
      expect(
        await screen.findByRole("menuitem", { name: "Vorschau mit Angeboten" }),
      ).toHaveAttribute("href", "/demo/anmeldung/preview?phaseId=12");
    },
  );

  it("round-trips the Zielgruppe audience and eligible classes into the create payload (#1663)", async () => {
    mocks.listPhases.mockResolvedValue([]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.createPhase.mockResolvedValue(phase({ id: "20", name: "Nur Konto" }));

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Anmeldephase" }),
    );
    fireEvent.change(inputByName("name"), { target: { value: "Nur Konto" } });

    // The audience radios render with a default of "open".
    expect(
      document.querySelector("#phase-audience-open") as HTMLInputElement,
    ).toBeChecked();
    fireEvent.click(
      document.querySelector(
        "#phase-audience-linked_parents",
      ) as HTMLInputElement,
    );

    // Add an eligible class. Scope to the Zielgruppe fieldset so we don't hit
    // the separate SchoolClassListEditor under "Konkrete Klassen".
    const audienceFieldset = screen
      .getByText("Zielgruppe")
      .closest("fieldset") as HTMLElement;
    fireEvent.change(
      within(audienceFieldset).getByLabelText("Klasse hinzufügen"),
      { target: { value: "3b" } },
    );
    fireEvent.click(
      within(audienceFieldset).getByRole("button", { name: "Hinzufügen" }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Erstellen" }));

    await waitFor(() => {
      expect(mocks.createPhase).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "Nur Konto",
          audience: "linked_parents",
          eligible_school_classes: ["3b"],
        }),
      );
    });
  });

  it("keeps offered classes that are not eligible when saving a restricted phase (#1663)", async () => {
    // available_school_classes may legitimately be a superset of the eligible
    // list: late-invite recipients bypass the eligibility gate and get the
    // full offered list, so an unrelated edit must not drop those classes.
    const restricted = phase({
      id: "22",
      available_school_classes: ["2a", "2b", "3b"],
      eligible_school_classes: ["3b"],
      require_school_class: true,
    });
    mocks.listPhases.mockResolvedValue([restricted]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(restricted);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "22",
        expect.objectContaining({
          available_school_classes: ["2a", "2b", "3b"],
          eligible_school_classes: ["3b"],
          require_school_class: true,
        }),
      );
    });
  });

  it("saves a grade-level restriction without touching the class config (#1663)", async () => {
    // A phase for a whole grade needs no concrete classes — that is the case
    // the class list cannot express, so selecting a grade must not force a
    // class pick list or the mandatory toggle.
    const target = phase({ id: "26" });
    mocks.listPhases.mockResolvedValue([target]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(target);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    const gradeButtons = await screen.findAllByRole("button", { name: "3" });
    fireEvent.click(gradeButtons[0]!);
    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "26",
        expect.objectContaining({
          eligible_grade_levels: [3],
          eligible_school_classes: [],
          require_school_class: false,
        }),
      );
    });
  });

  it("refuses a class that contradicts the selected grade levels (#1663)", async () => {
    // Both restrictions apply together, so a grade-2 class under a grade-3
    // restriction can never be declared. The backend rejects the pair; the
    // editor names the offending class instead of surfacing that error.
    const conflicting = phase({
      id: "27",
      available_school_classes: ["2a"],
      eligible_school_classes: ["2a"],
      eligible_grade_levels: [3],
      require_school_class: true,
    });
    mocks.listPhases.mockResolvedValue([conflicting]);
    mocks.listSchemas.mockResolvedValue([schema()]);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    fireEvent.click(await screen.findByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(/passt nicht zu den gewählten Klassenstufen/),
    ).toBeInTheDocument();
    expect(mocks.updatePhase).not.toHaveBeenCalled();
  });

  it("adds an eligible class that is not yet offered to the offered list (#1663)", async () => {
    // The eligible list is edited free-form; the backend rejects a phase whose
    // eligible list is not a subset of the offered one, so the missing class
    // is added instead of the offered list being replaced.
    const restricted = phase({
      id: "23",
      available_school_classes: ["2a", "2b"],
      eligible_school_classes: [],
      require_school_class: false,
    });
    mocks.listPhases.mockResolvedValue([restricted]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(restricted);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    const audienceFieldset = (await screen.findByText("Zielgruppe")).closest(
      "fieldset",
    ) as HTMLElement;
    fireEvent.change(
      within(audienceFieldset).getByLabelText("Klasse hinzufügen"),
      { target: { value: "3b" } },
    );
    fireEvent.click(
      within(audienceFieldset).getByRole("button", { name: "Hinzufügen" }),
    );

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "23",
        expect.objectContaining({
          available_school_classes: ["2a", "2b", "3b"],
          eligible_school_classes: ["3b"],
          require_school_class: true,
        }),
      );
    });
  });

  it("defaults a legacy phase without eligibility fields to audience open on save (#1663)", async () => {
    // phase() intentionally omits audience/eligible_school_classes, mirroring
    // a phase created before #1663.
    const legacy = phase({ id: "21" });
    mocks.listPhases.mockResolvedValue([legacy]);
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.updatePhase.mockResolvedValue(legacy);

    render(<PhasesEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Schuljahr 2026/27",
      }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    await waitFor(() => {
      expect(
        document.querySelector("#phase-audience-open") as HTMLInputElement,
      ).toBeChecked();
    });

    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      expect(mocks.updatePhase).toHaveBeenCalledWith(
        "21",
        expect.objectContaining({
          audience: "open",
          eligible_school_classes: [],
        }),
      );
    });
  });
});

describe("PhasesEditor school year default", () => {
  it.each(schoolDateCases("2026-07-01"))(
    "defaults the school year from Berlin July in %s",
    async (zone, instant) => {
      mocks.listSchemas.mockResolvedValue([schema()]);
      setSchoolDateClock(zone, instant, "2026-07-01");
      mocks.listPhases.mockResolvedValue([phase()]);
      render(<PhasesEditor />);
      await screen.findByText("Schuljahr 2026/27");
      fireEvent.click(
        screen.getAllByRole("button", { name: "Neue Anmeldephase" })[0]!,
      );
      expect(inputByName("service_start_date")).toHaveValue("2026-09-01");
      expect(inputByName("service_end_date")).toHaveValue("2027-07-31");
    },
  );
});
