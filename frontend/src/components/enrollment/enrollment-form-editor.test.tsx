import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { StrictMode } from "react";

// useTenantMutate scopes its keys to the signed-in account (#5006).
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "1" }, expires: "2099-01-01" },
    status: "authenticated",
  }),
}));

vi.mock("vaul", async () =>
  (await import("~/test/enrollment-editors")).vaulMock(),
);

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

import { EnrollmentFormEditor } from "./enrollment-form-editor";
import { prepareFieldsForSave } from "./form-template-draft";
import { ApiError } from "~/lib/api-error";
import { catalogText } from "~/test/error-catalog-text";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import type { FormField, FormSchema } from "~/lib/enrollment-form-schema-api";
import {
  phase,
  schema,
  chooseOption,
  deferred,
  editStandardLegalBlock,
  resetEditorMocks,
} from "~/test/enrollment-editors";

beforeEach(() => {
  resetEditorMocks(mocks);
});

// The editor always sends the template's legal blocks on save (PR #1632).
// With no tenant legal texts configured (see the fetchPublicLegalTexts
// default above), the four standard blocks are passed through disabled.
const defaultLegalBlocksForSave = [
  expect.objectContaining({ key: "agb", enabled: false, source: "standard" }),
  expect.objectContaining({
    key: "data_processing",
    enabled: false,
    source: "standard",
  }),
  expect.objectContaining({
    key: "photo",
    enabled: false,
    source: "standard",
  }),
  expect.objectContaining({
    key: "email_contact",
    enabled: false,
    source: "standard",
  }),
];

describe("EnrollmentFormEditor", () => {
  it("uses tenant-aware links for public previews", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByRole("link", { name: "Vorschau" }),
    ).toHaveAttribute("href", "/demo/anmeldung/preview?base=1");
  });

  it("scrolls to an added question and focuses its text (#4328)", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    const scrollIntoView = vi.fn();
    const original = HTMLElement.prototype.scrollIntoView;
    HTMLElement.prototype.scrollIntoView = scrollIntoView;

    try {
      render(<EnrollmentFormEditor />);

      fireEvent.click(
        await screen.findByRole("button", { name: "Neue Vorlage" }),
      );
      fireEvent.click(
        screen.getByRole("button", { name: "Freie Zusatzfrage" }),
      );

      const questionInput = screen.getByLabelText("Frage im Elternformular");
      await waitFor(() => expect(questionInput).toHaveFocus());
      expect(scrollIntoView).toHaveBeenCalledTimes(1);
      expect(scrollIntoView.mock.contexts[0]).toBe(
        questionInput.closest("article"),
      );

      fireEvent.click(screen.getByRole("button", { name: "Infotext" }));
      await waitFor(() =>
        expect(
          screen.getByPlaceholderText("z. B. Wichtiger Hinweis"),
        ).toHaveFocus(),
      );
      expect(scrollIntoView).toHaveBeenCalledTimes(2);

      // Typing into the new row must not pull the view back again.
      fireEvent.change(questionInput, { target: { value: "Allergien" } });
      expect(scrollIntoView).toHaveBeenCalledTimes(2);
    } finally {
      HTMLElement.prototype.scrollIntoView = original;
    }
  });

  // #5027: the label placeholder no longer suggests a question about the
  // child for a question asked once per enrollment; the per-child switch
  // names those questions, and stays off until the school switches it on.
  it("points questions about the child to the per-child switch", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Kontaktformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Kontaktformular" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Freie Zusatzfrage" }));

    expect(screen.getByLabelText("Frage im Elternformular")).toHaveAttribute(
      "placeholder",
      "z. B. Wie haben Sie von uns erfahren?",
    );
    expect(
      screen.getByText(
        "Für Fragen zum Kind, etwa zu Allergien oder zum Schwimmen. Die Frage erscheint dann für jedes angemeldete Kind.",
      ),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Frage im Elternformular"), {
      target: { value: "Wie haben Sie von uns erfahren?" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalledWith(
        "Kontaktformular",
        expect.arrayContaining([
          expect.objectContaining({
            label: "Wie haben Sie von uns erfahren?",
            applies_to_child: false,
          }),
        ]),
        {},
        defaultLegalBlocksForSave,
      );
    });
  });

  it("shows an error when unsaved changes cannot be saved as a valid template", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Freie Zusatzfrage" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );

    const error = await screen.findByRole("alert");
    expect(error).toHaveTextContent(
      "Bitte geben Sie zuerst einen Namen für die Vorlage ein.",
    );
    expect(error).toHaveClass("bg-moto-red-soft");
    expect(
      screen.getByRole("button", { name: "Speichern und fortfahren" }),
    ).toBeDisabled();
  });

  it("creates, previews, updates, and deletes schemas", async () => {
    const initialSchema = schema();
    mocks.listSchemas.mockResolvedValue([initialSchema]);
    mocks.listPhases.mockResolvedValue([phase()]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Kontaktformular" }),
    );
    mocks.updateSchema.mockResolvedValue(schema({ name: "Regelformular" }));
    mocks.deleteSchema.mockResolvedValue(undefined);

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();
    expect(screen.getByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Neue Vorlage" }));
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Kontaktformular" },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "Freie Zusatzfrage" }));
    fireEvent.change(screen.getByLabelText("Frage im Elternformular"), {
      target: { value: "Lieblingsessen" },
    });
    await chooseOption("Typ", "Auswahl");
    fireEvent.change(screen.getByLabelText("Auswahloptionen"), {
      target: { value: "Pasta\nReis" },
    });
    fireEvent.click(screen.getByText("Pflichtfrage"));
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalledWith(
        "Kontaktformular",
        expect.arrayContaining([
          expect.objectContaining({
            key: "lieblingsessen",
            label: "Lieblingsessen",
            type: "select",
            required: true,
            options: [
              { label: "Pasta", value: "pasta" },
              { label: "Reis", value: "reis" },
            ],
          }),
        ]),
        {},
        defaultLegalBlocksForSave,
      );
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Prüfen" }));
    expect(screen.getByText("Zuletzt gespeichert")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Bearbeiten" }));
    fireEvent.change(screen.getByLabelText("Frage im Elternformular"), {
      target: { value: "Medizinische Hinweise" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );
    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalledWith(
        "schema-1",
        expect.arrayContaining([expect.objectContaining({ type: "textarea" })]),
        {},
        defaultLegalBlocksForSave,
        // Content-only edit: no rename name rides along on the save.
        undefined,
      );
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    // Zweistufige Löschbestätigung (ConfirmDeleteModal, #3110).
    fireEvent.click(screen.getByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    await waitFor(() => {
      expect(mocks.deleteSchema).toHaveBeenCalledWith("schema-1");
    });
  });

  it("resets the saving state after creating a schema under React StrictMode", async () => {
    const create = deferred<FormSchema>();
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockReturnValue(create.promise);

    render(
      <StrictMode>
        <EnrollmentFormEditor />
      </StrictMode>,
    );

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Kontaktformular" },
      },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    expect(
      await screen.findByRole("button", { name: "Speichert…" }),
    ).toBeDisabled();

    await act(async () => {
      create.resolve(schema({ id: "schema-new", name: "Kontaktformular" }));
    });

    await waitFor(() => {
      expect(mocks.toast.success).toHaveBeenCalledWith(
        "Die Formularvorlage „Kontaktformular“ ist angelegt.",
      );
    });
    expect(
      screen.queryByRole("button", { name: "Speichert…" }),
    ).not.toBeInTheDocument();
  });

  it("renames a schema via the actions menu", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.renameSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Umbenennen" }),
    );

    const input = await screen.findByLabelText("Name");
    fireEvent.change(input, { target: { value: "  Ferienprogramm  " } });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() => {
      // The trimmed name reaches the API; the lineage id (latest version)
      // identifies which schema to rename.
      expect(mocks.renameSchema).toHaveBeenCalledWith(
        "schema-1",
        "Ferienprogramm",
      );
    });
  });

  // #6652: the new name holds once the write is through; a failed reload of
  // the templates is a load error that says the change is saved.
  it("keeps a renamed schema when the reload fails", async () => {
    mocks.listSchemas
      .mockReset()
      .mockResolvedValueOnce([schema()])
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue([schema({ name: "Ferienprogramm" })]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.renameSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));

    render(<EnrollmentFormEditor />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Umbenennen" }),
    );
    fireEvent.change(await screen.findByLabelText("Name"), {
      target: { value: "Ferienprogramm" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige der Anmeldeformulare")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mocks.listSchemas).toHaveBeenCalledTimes(3));
    expect(mocks.renameSchema).toHaveBeenCalledTimes(1);
  });

  it("keeps the rename dialog open and shows the error when the name is taken", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.renameSchema.mockRejectedValue(
      new ApiError("schema name exists", 409, {
        code: "enrollment.schema_name_exists",
        errors: [{ field: "name", reason: "already exists" }],
      }),
    );

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Umbenennen" }),
    );

    fireEvent.change(await screen.findByLabelText("Name"), {
      target: { value: "Ferienprogramm" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("enrollment.schema_name_exists", "die Formularvorlage"),
      ),
    ).toBeInTheDocument();
    // Dialog stays open so the admin can correct the name.
    expect(screen.getByLabelText("Name")).toHaveAttribute(
      "aria-invalid",
      "true",
    );
    await waitFor(() => {
      expect(screen.getByLabelText("Name")).toHaveFocus();
    });
  });

  it("renames via the builder name field when saving an edited template", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.renameSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));
    mocks.updateSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    // The name input is now editable in the builder header (not only on
    // create). Changing ONLY the name and saving renames the lineage in
    // place; it must NOT publish a redundant identical version, matching
    // the standalone "Umbenennen" dialog's semantics.
    fireEvent.change(
      await screen.findByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Ferienprogramm" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.renameSchema).toHaveBeenCalledWith(
        "schema-1",
        "Ferienprogramm",
      );
    });
    expect(mocks.updateSchema).not.toHaveBeenCalled();
  });

  it("switches the parent's signature on for the whole form (#4786)", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema());

    render(<EnrollmentFormEditor />);
    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(
      await screen.findByText(
        "Eltern senden die Anmeldung ohne Unterschrift ab.",
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("switch", { name: "Unterschrift erforderlich" }),
    );
    expect(
      screen.getByText(
        /Ohne Unterschrift lässt sich die Anmeldung nicht absenden/,
      ),
    ).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalledWith(
        "schema-1",
        expect.anything(),
        expect.objectContaining({ signature: true }),
        expect.anything(),
        undefined,
      );
    });
  });

  it("keeps a saved template saved when the reload fails (#6652)", async () => {
    mocks.listSchemas
      .mockReset()
      .mockResolvedValueOnce([schema()])
      .mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      )
      .mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema());

    render(<EnrollmentFormEditor />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    fireEvent.click(
      await screen.findByRole("switch", { name: "Unterschrift erforderlich" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    expect(
      await screen.findByText(
        `${catalogText("general.unavailable", "die Anzeige der Anmeldeformulare")} ${SAVED_BUT_STALE_SUFFIX}`,
      ),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    await waitFor(() => expect(mocks.listSchemas).toHaveBeenCalledTimes(3));
    expect(mocks.updateSchema).toHaveBeenCalledTimes(1);
  });

  it("renames AND publishes in one atomic request when name and content both change", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema({ name: "Ferienprogramm" }));

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    // Change the name AND a base-field requirement: the new name rides along
    // on the single updateSchema request so the backend renames + publishes in
    // one transaction. No separate rename round-trip the publish could leave
    // half-applied on failure.
    fireEvent.change(
      await screen.findByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Ferienprogramm" } },
    );
    fireEvent.click(
      await screen.findByRole("switch", {
        name: "Telefonnummer verpflichtend",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalledWith(
        "schema-1",
        expect.anything(),
        expect.anything(),
        expect.anything(),
        "Ferienprogramm",
      );
    });
    // The combined save must NOT fire a separate rename request — that is the
    // non-atomic path the backend transaction replaced.
    expect(mocks.renameSchema).not.toHaveBeenCalled();
  });

  it("does not rename separately when the combined save fails", async () => {
    // Regression guard for the partial-save bug: a failed publish must not
    // leave the lineage renamed. Because the rename now rides inside the
    // single updateSchema transaction, the editor never calls renameSchema on
    // its own, so a rejected updateSchema commits nothing.
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockRejectedValue(
      new Error("Formularvorlage konnte nicht gespeichert werden"),
    );

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    fireEvent.change(
      await screen.findByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Ferienprogramm" } },
    );
    fireEvent.click(
      await screen.findByRole("switch", {
        name: "Telefonnummer verpflichtend",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalled();
    });
    expect(mocks.renameSchema).not.toHaveBeenCalled();
  });

  it("does not rename when the builder name is left unchanged", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema());

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    // Toggle a base-field requirement so there is something to save without
    // touching the name.
    fireEvent.click(
      await screen.findByRole("switch", {
        name: "Telefonnummer verpflichtend",
      }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalled();
    });
    expect(mocks.renameSchema).not.toHaveBeenCalled();
  });

  it("activates a standard legal block when an admin enters legal text", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Rechtstextformular" },
      },
    );
    editStandardLegalBlock(0);
    const legalTextAreas = await screen.findAllByLabelText(
      "Rechtstext / Erklärung",
    );
    fireEvent.change(legalTextAreas[0] as HTMLTextAreaElement, {
      target: { value: "AGB Text aus der Vorlage" },
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ key: string; enabled: boolean; text: string }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          enabled: true,
          text: "AGB Text aus der Vorlage",
        }),
      ]),
    );
  });

  it("uploads and saves an AGB PDF source for a form template", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "PDF-Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "PDF-Rechtstextformular" },
      },
    );
    editStandardLegalBlock(0);
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    const uploadInput = screen.getByLabelText("PDF hochladen");
    const file = new File(["%PDF-1.4"], "terms.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(uploadInput, { target: { files: [file] } });

    await waitFor(() => {
      expect(mocks.uploadEnrollmentLegalDocument).toHaveBeenCalledWith(file);
    });
    expect(await screen.findByText("PDF gespeichert")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{
        key: string;
        display_mode: string;
        document_url: string;
        enabled: boolean;
      }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          display_mode: "pdf",
          document_url: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
          enabled: true,
        }),
      ]),
    );
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
  });

  it("keeps legal block edits made while an AGB PDF upload is pending", async () => {
    const upload = deferred<string>();
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockReturnValue(upload.promise);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "PDF-Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "PDF-Rechtstextformular" },
      },
    );
    editStandardLegalBlock(0);
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    fireEvent.change(screen.getByLabelText("Titel"), {
      target: { value: "Individuelle Teilnahmebedingungen" },
    });

    await act(async () => {
      upload.resolve("/uploads/enrollment-form-legal-documents/1_terms.pdf");
    });
    expect(await screen.findByText("PDF gespeichert")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ key: string; title: string; document_url: string }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          title: "Individuelle Teilnahmebedingungen",
          document_url: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
        }),
      ]),
    );
  });

  it("clears stale AGB PDF URLs when saving the template in text mode", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Text-Rechtstextformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Text-Rechtstextformular" },
      },
    );
    editStandardLegalBlock(0);
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(screen.getByRole("button", { name: /Text eingeben/ }));
    fireEvent.change(screen.getByLabelText("Rechtstext / Erklärung"), {
      target: { value: "AGB als Text" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{
        key: string;
        display_mode: string;
        document_url: string;
        text: string;
      }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          display_mode: "text",
          document_url: "",
          text: "AGB als Text",
        }),
      ]),
    );
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      expect.any(Object),
    );
  });

  it("does not delete a draft AGB PDF while its template save is in flight", async () => {
    const save = deferred<FormSchema>();
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);
    mocks.createSchema.mockReturnValue(save.promise);

    const view = render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "PDF-Rechtstextformular" },
      },
    );
    editStandardLegalBlock(0);
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );
    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });

    view.unmount();
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();

    await act(async () => {
      save.resolve(
        schema({ id: "schema-new", name: "PDF-Rechtstextformular" }),
      );
    });
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
  });

  it("does not enable inherited PDF AGB when the active PDF source has no document", async () => {
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "Legacy AGB Text",
      agb_document_url: "",
      agb_display_mode: "pdf",
      dsgvo: "",
      email_contact: "",
      photo: "",
      terms_enabled: true,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [
        {
          key: "agb",
          kind: "terms",
          title: "AGB",
          label: "Ich akzeptiere die AGB.",
          text: "Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](/api/public/enrollment-legal-documents/legacy.pdf)",
          required: true,
          enabled: false,
          sort_order: 10,
          source: "standard",
        },
      ],
    });
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "PDF ohne Datei" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "PDF ohne Datei" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{
        key: string;
        enabled: boolean;
        text: string;
        display_mode: string;
        document_url: string;
      }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          enabled: false,
          text: "",
          display_mode: "pdf",
          document_url: "",
        }),
      ]),
    );
  });

  it("does not inherit stale AGB PDF URLs while settings are in text mode", async () => {
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "",
      agb_document_url: "/uploads/enrollment-legal-documents/1_stale.pdf",
      agb_display_mode: "text",
      dsgvo: "",
      email_contact: "",
      photo: "",
      terms_enabled: true,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [],
    });
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Text ohne Inhalt" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Text ohne Inhalt" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ key: string; enabled: boolean; document_url: string }>,
    ];
    expect(legalBlocks).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          key: "agb",
          enabled: false,
          document_url: "",
        }),
      ]),
    );
  });

  it("keeps generated PDF link text out of the editable AGB text source", async () => {
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](/api/public/enrollment-legal-documents/generated.pdf)",
      agb_document_url: "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      agb_display_mode: "pdf",
      dsgvo: "",
      email_contact: "",
      photo: "",
      terms_enabled: true,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [
        {
          key: "agb",
          kind: "terms",
          title: "AGB",
          label: "Ich akzeptiere die AGB.",
          text: "Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](/api/public/enrollment-legal-documents/generated.pdf)",
          required: true,
          enabled: true,
          sort_order: 10,
          source: "standard",
        },
      ],
    });
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    editStandardLegalBlock(0);
    fireEvent.click(screen.getByRole("button", { name: /Text eingeben/ }));

    expect(screen.getByLabelText("Rechtstext / Erklärung")).toHaveValue("");
  });

  it("removes an uploaded AGB PDF source from a form template draft", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_terms.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    editStandardLegalBlock(0);
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    const file = new File(["%PDF-1.4"], "terms.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: { files: [file] },
    });

    await screen.findByText("PDF gespeichert");
    fireEvent.click(screen.getByRole("button", { name: "Entfernen" }));

    await waitFor(() => {
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
        "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      );
    });
    expect(screen.queryByText("PDF gespeichert")).not.toBeInTheDocument();
  });

  it("cleans up uploaded AGB PDFs when a form template draft is discarded", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_draft.pdf",
    );
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    editStandardLegalBlock(0);
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );

    const file = new File(["%PDF-1.4"], "terms.pdf", {
      type: "application/pdf",
    });
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: { files: [file] },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Verwerfen" }));

    await waitFor(() => {
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
        "/uploads/enrollment-form-legal-documents/1_draft.pdf",
        expect.any(Object),
      );
    });
  });

  it("keeps failed draft AGB PDF cleanup tracked for a later retry", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.uploadEnrollmentLegalDocument.mockResolvedValue(
      "/uploads/enrollment-form-legal-documents/1_retry.pdf",
    );
    mocks.deleteEnrollmentLegalDocument
      .mockRejectedValueOnce(new Error("delete failed"))
      .mockResolvedValue(undefined);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Retry-Rechtstextformular" }),
    );

    const view = render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      {
        target: { value: "Retry-Rechtstextformular" },
      },
    );
    editStandardLegalBlock(0);
    fireEvent.click(
      screen.getByRole("button", { name: /PDF-Datei hochladen/ }),
    );
    fireEvent.change(screen.getByLabelText("PDF hochladen"), {
      target: {
        files: [
          new File(["%PDF-1.4"], "terms.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    await screen.findByText("PDF gespeichert");

    fireEvent.click(screen.getByRole("button", { name: /Text eingeben/ }));
    fireEvent.change(screen.getByLabelText("Rechtstext / Erklärung"), {
      target: { value: "AGB als Text" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    expect(mocks.showActionError).toHaveBeenCalledWith(expect.any(Error), {
      object: "eine ungespeicherte PDF-Datei",
    });
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledTimes(1);

    view.unmount();

    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledTimes(2);
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenLastCalledWith(
      "/uploads/enrollment-form-legal-documents/1_retry.pdf",
      { keepalive: true },
    );
  });

  it("shows standard legal blocks as inherited summaries until an admin edits an override", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );

    expect(
      screen.getAllByText("Aus Einstellungen übernommen").length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByLabelText("Rechtstext / Erklärung"),
    ).not.toBeInTheDocument();

    editStandardLegalBlock(0);

    expect(screen.getByLabelText("Rechtstext / Erklärung")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Einstellungen wieder verwenden" }),
    ).toBeInTheDocument();
  });

  it("keeps disabled inherited legal text available when editing a standard block", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.fetchPublicLegalTexts.mockResolvedValue({
      agb: "",
      agb_document_url: "",
      agb_display_mode: "text",
      dsgvo: "",
      email_contact: "",
      photo: "Foto-Rechtstext aus den Einstellungen",
      terms_enabled: false,
      dsgvo_enabled: false,
      email_contact_enabled: false,
      photo_enabled: false,
      blocks: [],
    });

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    expect(
      screen.getAllByText("Ist für diese Vorlage ausgeblendet.").length,
    ).toBeGreaterThan(0);

    editStandardLegalBlock(2);

    expect(screen.getByLabelText("Rechtstext / Erklärung")).toHaveValue(
      "Foto-Rechtstext aus den Einstellungen",
    );
  });

  it("uses right-side kit switches and icon menus for inherited standard legal blocks", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );

    expect(
      screen.queryByText("In dieser Vorlage anzeigen"),
    ).not.toBeInTheDocument();
    expect(screen.queryByText("Abweichend bearbeiten")).not.toBeInTheDocument();

    const displaySwitches = screen.getAllByRole("switch", {
      name: /in dieser Vorlage anzeigen/i,
    });
    const actionMenus = screen.getAllByRole("button", {
      name: /^Aktionen für (AGB \/ Teilnahmebedingungen|Datenschutzinformation|Fotoeinwilligung|E-Mail-Kontakt)$/,
    });

    // Kit-Switch (#5595); seine Form prüft switch.test.tsx.
    expect(displaySwitches[0]).toHaveAttribute("aria-checked", "false");
    fireEvent.click(displaySwitches[0]!);
    expect(displaySwitches[0]).toHaveAttribute("aria-checked", "true");
    // Nur das Kebab-Symbol; den Auslöser gestaltet OverflowMenu selbst.
    expect(actionMenus[0]).toHaveTextContent("");
  });

  it("restores missing standard legal blocks when editing an older schema", async () => {
    mocks.listSchemas.mockResolvedValue([
      schema({
        legal_blocks: [
          {
            key: "data_processing",
            kind: "privacy_notice",
            title: "Datenschutzinformation",
            label: "Ich habe die Datenschutzinformation gelesen.",
            text: "",
            required: true,
            enabled: false,
            sort_order: 20,
            source: "standard",
          },
          {
            key: "photo",
            kind: "consent",
            title: "Fotoeinwilligung",
            label: "Fotos dürfen verwendet werden.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 30,
            source: "standard",
          },
          {
            key: "email_contact",
            kind: "notice",
            title: "Kontakt per E-Mail",
            label: "Wir kontaktieren Sie per E-Mail.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 40,
            source: "standard",
          },
        ],
      }),
    ]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    expect(await screen.findByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(screen.getByText("AGB / Teilnahmebedingungen")).toBeInTheDocument();
    expect(screen.getAllByText("Aus Einstellungen übernommen")).toHaveLength(4);
  });

  it("keeps legal block keys hidden and generates unique keys for new custom consents", async () => {
    mocks.listSchemas.mockResolvedValue([
      schema({
        legal_blocks: [
          {
            key: "agb",
            kind: "terms",
            title: "AGB / Teilnahmebedingungen",
            label: "Ich akzeptiere die AGB.",
            text: "",
            required: true,
            enabled: false,
            sort_order: 10,
            source: "standard",
          },
          {
            key: "data_processing",
            kind: "privacy_notice",
            title: "Datenschutzinformation",
            label: "Ich habe die Datenschutzinformation gelesen.",
            text: "",
            required: true,
            enabled: false,
            sort_order: 20,
            source: "standard",
          },
          {
            key: "photo",
            kind: "consent",
            title: "Fotoeinwilligung",
            label: "Fotos dürfen verwendet werden.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 30,
            source: "standard",
          },
          {
            key: "email_contact",
            kind: "notice",
            title: "Kontakt per E-Mail",
            label: "Wir kontaktieren Sie per E-Mail.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 40,
            source: "standard",
          },
          {
            key: "custom_consent_6",
            kind: "consent",
            title: "Schwimmbad",
            label: "Mein Kind darf ins Schwimmbad gehen.",
            text: "",
            required: false,
            enabled: true,
            sort_order: 50,
            source: "custom",
          },
        ],
      }),
    ]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema({ name: "Regelformular" }));

    render(<EnrollmentFormEditor />);

    expect(await screen.findByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );

    expect(
      screen.queryByLabelText("Interner Schlüssel"),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Eigene Zustimmung hinzufügen" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.updateSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ key: string; source: string }>,
    ];
    expect(
      legalBlocks
        .filter((block) => block.source === "custom")
        .map((block) => block.key),
    ).toEqual(["custom_consent_6", "custom_consent_7"]);
  });

  it("renders legal block toggles with the shared styled checkbox treatment", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    editStandardLegalBlock(0);

    const displayToggles = screen.getAllByRole("checkbox", {
      name: "Im Formular anzeigen",
    });
    const requiredToggles = screen.getAllByRole("checkbox", {
      name: "Muss bestätigt werden",
    });

    // Kit-Checkbox (#5595): natives Kästchen, sichtbar über das gestaltete
    // Kästchen daneben, kein unstyled Browser-Kästchen.
    for (const toggle of [displayToggles[0], requiredToggles[0]]) {
      expect(toggle).toHaveAttribute("type", "checkbox");
      expect(toggle).toHaveClass("sr-only");
      expect(toggle?.nextElementSibling).toHaveAttribute("aria-hidden", "true");
    }
  });

  it("saves a required custom checkbox as terms instead of consent", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockResolvedValue(
      schema({ id: "schema-new", name: "Zustimmungsformular" }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Zustimmungsformular" } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Eigene Zustimmung hinzufügen" }),
    );

    // Der neue Custom-Block startet als freiwillige Einwilligung.
    expect(
      screen.getByText(
        "Freiwillige Einwilligung: Eltern können das Formular auch ohne Häkchen abschicken.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("checkbox", { name: "Muss bestätigt werden" }),
    );

    // Der Wechsel zur Pflicht-Bestätigung erklärt sich direkt am Schalter.
    expect(
      screen.getByText(
        "Pflicht-Bestätigung: Eltern müssen das Häkchen setzen, sonst können sie das Formular nicht abschicken. Das ist keine freiwillige Einwilligung.",
      ),
    ).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.createSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ kind: string; required: boolean; source: string }>,
    ];
    expect(legalBlocks.filter((block) => block.source === "custom")).toEqual([
      expect.objectContaining({ kind: "terms", required: true }),
    ]);
  });

  it("offers no required toggle for the standard photo consent block", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    // Blockreihenfolge: AGB, Datenschutz, Foto, E-Mail — Foto aufklappen.
    editStandardLegalBlock(2);

    expect(
      screen.getByText(
        "Freiwillige Einwilligung: Eltern können das Formular auch ohne Häkchen abschicken.",
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: "Muss bestätigt werden" }),
    ).not.toBeInTheDocument();
  });

  it("saves a legacy standard block without source as the standard block it shows", async () => {
    // Templates saved before legal blocks carried `source` come back without
    // it. The backend defaults an empty source by key (form_schema.go
    // validateSource), and the editor shows the block as a standard block, so
    // the save keeps it standard: the photo consent stays a voluntary consent.
    mocks.listSchemas.mockResolvedValue([
      schema({
        legal_blocks: [
          {
            key: "photo",
            kind: "consent",
            title: "Fotoeinwilligung",
            label: "Mein Kind darf fotografiert werden.",
            text: "",
            required: true,
            enabled: true,
            sort_order: 30,
          },
        ],
      }),
    ]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.updateSchema.mockResolvedValue(schema());

    render(<EnrollmentFormEditor />);

    expect(await screen.findByText("Regelformular")).toBeInTheDocument();
    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(
      await screen.findByRole("menuitem", { name: "Bearbeiten" }),
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Änderungen speichern" }),
    );

    await waitFor(() => {
      expect(mocks.updateSchema).toHaveBeenCalled();
    });
    const [, , , legalBlocks] = mocks.updateSchema.mock.calls[0] as [
      string,
      unknown,
      unknown,
      Array<{ key: string; kind: string; required: boolean; source: string }>,
    ];
    expect(legalBlocks.find((block) => block.key === "photo")).toEqual(
      expect.objectContaining({
        kind: "consent",
        required: false,
        source: "standard",
      }),
    );
  });

  it("shows a simple checkbox-or-notice mode instead of legal block kind names", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    editStandardLegalBlock(0);

    expect(
      screen.getAllByRole("button", { name: "Mit Checkbox" }).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByRole("button", { name: "Nur Hinweis" }).length,
    ).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Einwilligung" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Datenschutz-Bestätigung" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "AGB / Teilnahmebedingungen" }),
    ).not.toBeInTheDocument();
  });

  it("shows enabled legal blocks in the compact form preview", async () => {
    mocks.listSchemas.mockResolvedValue([
      schema({
        legal_blocks: [
          {
            key: "custom_photo_trip",
            kind: "consent",
            title: "Fotoausflug",
            label: "Mein Kind darf beim Ausflug fotografiert werden.",
            text: "Details",
            required: true,
            enabled: true,
            sort_order: 10,
            source: "custom",
          },
          {
            key: "disabled_consent",
            kind: "consent",
            title: "Ausgeblendet",
            label: "Diese Zustimmung ist deaktiviert.",
            text: "",
            required: false,
            enabled: false,
            sort_order: 20,
            source: "custom",
          },
        ],
      }),
    ]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);
    expect(await screen.findByText("Regelformular")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "Aktionen für Regelformular" }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Prüfen" }));

    expect(screen.getByText("Zustimmungen & Hinweise")).toBeInTheDocument();
    expect(screen.getByText("Fotoausflug")).toBeInTheDocument();
    expect(screen.getAllByText("Pflicht").length).toBeGreaterThan(0);
    expect(screen.queryByText("Ausgeblendet")).not.toBeInTheDocument();
  });

  it("shows a failed load in place with retry instead of an empty overview", async () => {
    mocks.listSchemas
      .mockRejectedValueOnce(
        new ApiError("boom", 500, {
          code: "general.server",
          instance: "req-7",
        }),
      )
      .mockResolvedValueOnce([]);
    mocks.listPhases.mockResolvedValue([]);
    const onTemplateCountChange = vi.fn();

    render(
      <EnrollmentFormEditor onTemplateCountChange={onTemplateCountChange} />,
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Übersicht der Anmeldeformulare"),
      ),
    ).toBeInTheDocument();
    expect(screen.getByText(/req-7/)).toBeInTheDocument();
    expect(screen.queryByText("Anmeldeformulare verwalten")).toBeNull();
    expect(onTemplateCountChange).toHaveBeenLastCalledWith(null, true);

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    expect(
      await screen.findByText("Anmeldeformulare verwalten"),
    ).toBeInTheDocument();
    expect(onTemplateCountChange).toHaveBeenLastCalledWith(0, false);
  });

  it("fails the load when the phases cannot be read", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockRejectedValueOnce(
      new ApiError("unavailable", 503, { code: "general.unavailable" }),
    );

    render(<EnrollmentFormEditor />);

    expect(
      await screen.findByText(
        catalogText(
          "general.unavailable",
          "die Übersicht der Anmeldeformulare",
        ),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("Anmeldeformulare verwalten")).toBeNull();
  });

  it("shows a failed save in the builder alert and marks the field from the API", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema.mockRejectedValueOnce(
      new ApiError("schema name exists", 409, {
        code: "enrollment.schema_name_exists",
        errors: [{ field: "name", reason: "already exists" }],
      }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    const nameInput = screen.getByPlaceholderText(
      "z. B. Ferienbetreuung Sommer 2026",
    );
    fireEvent.change(nameInput, { target: { value: "Fehlerformular" } });
    fireEvent.click(screen.getByRole("button", { name: "Freie Zusatzfrage" }));
    fireEvent.change(screen.getByLabelText("Frage im Elternformular"), {
      target: { value: "Hinweis" },
    });
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    expect(
      await screen.findByText(
        catalogText("enrollment.schema_name_exists", "die Formularvorlage"),
      ),
    ).toBeInTheDocument();
    expect(nameInput).toHaveAttribute("aria-invalid", "true");
    await waitFor(() => {
      expect(nameInput).toHaveFocus();
    });
    expect(mocks.toast.error).not.toHaveBeenCalled();
  });

  it("retries a failed save with the current draft", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.createSchema
      .mockRejectedValueOnce(
        new ApiError("boom", 500, {
          code: "general.server",
          instance: "req-1",
        }),
      )
      .mockResolvedValueOnce(
        schema({ id: "schema-new", name: "Zweiter Name" }),
      );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    const nameInput = screen.getByPlaceholderText(
      "z. B. Ferienbetreuung Sommer 2026",
    );
    fireEvent.change(nameInput, { target: { value: "Erster Name" } });
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    expect(
      await screen.findByText(
        catalogText("general.server", "die Formularvorlage"),
      ),
    ).toBeInTheDocument();

    fireEvent.change(nameInput, { target: { value: "Zweiter Name" } });
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));

    await waitFor(() => {
      expect(mocks.createSchema).toHaveBeenCalledTimes(2);
    });
    expect(mocks.createSchema.mock.calls[1]?.[0]).toBe("Zweiter Name");
  });

  it("marks and focuses the field a local check before saving names", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Prüfformular" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Freie Zusatzfrage" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );

    expect(
      await screen.findByText(
        "Bitte geben Sie für Frage 1 einen Fragetext ein.",
      ),
    ).toBeInTheDocument();
    const question = screen.getByLabelText("Frage im Elternformular");
    expect(question).toHaveAttribute("aria-invalid", "true");
    expect(question).toHaveAccessibleDescription(
      "Bitte füllen Sie dieses Feld aus.",
    );
    await waitFor(() => {
      expect(question).toHaveFocus();
    });
    expect(mocks.createSchema).not.toHaveBeenCalled();
  });

  it("keeps the delete dialog open with the reason when deleting fails", async () => {
    mocks.listSchemas.mockResolvedValue([schema()]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.deleteSchema.mockRejectedValueOnce(
      new ApiError("schema has phases", 409, {
        code: "enrollment.schema_has_phases",
      }),
    );

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", {
        name: "Aktionen für Regelformular",
      }),
    );
    fireEvent.click(await screen.findByRole("menuitem", { name: "Löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Ja, löschen" }));
    fireEvent.click(screen.getByRole("button", { name: "Endgültig löschen" }));

    expect(
      await screen.findByText(
        catalogText("enrollment.schema_has_phases", "die Formularvorlage"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Endgültig löschen" }),
    ).toBeInTheDocument();
    expect(mocks.toast.error).not.toHaveBeenCalled();
    expect(mocks.toast.success).not.toHaveBeenCalled();
  });

  it("keeps a half-filled pickup-time row visible while editing", async () => {
    // The allowed-time rows are committed up to the field (deduped, non-empty)
    // and echo back through field.allowed_times. The reset effect now also
    // watches field.allowed_times so a schema/template switch refreshes the
    // rows, but it must NOT reset on the row's own commit — otherwise a freshly
    // added empty row (or a row mid-edit) would be wiped on every keystroke.
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(
      screen.getByRole("button", {
        name: /^Abholzeiten\b/,
      }),
    );

    // Adding a row leaves an empty time input on screen (it is not yet a
    // committed allowed_time, so a naive reset would drop it).
    fireEvent.click(screen.getByRole("button", { name: "Zeit hinzufügen" }));
    const firstRow = await screen.findByLabelText("Auswahlzeit 1");
    expect(firstRow).toBeInTheDocument();
    expect((firstRow as HTMLInputElement).value).toBe("");

    fireEvent.change(firstRow, { target: { value: "14:45" } });
    expect(
      (screen.getByLabelText("Auswahlzeit 1") as HTMLInputElement).value,
    ).toBe("14:45");

    // A second empty row coexists with the committed first one.
    fireEvent.click(screen.getByRole("button", { name: "Zeit hinzufügen" }));
    expect(
      (screen.getByLabelText("Auswahlzeit 1") as HTMLInputElement).value,
    ).toBe("14:45");
    expect(
      (screen.getByLabelText("Auswahlzeit 2") as HTMLInputElement).value,
    ).toBe("");
  });

  // #6044: a school asked for pickup times in its form AND gave each offering
  // a Gehzeit. The answer replaces the offering time in the care plan, so
  // the two sources contradicted each other without any hint. The template
  // preview also listed „Gewünschtes Angebot“ with no word that the offerings
  // come from the phase.
  it("says where offerings and their Gehzeit come from", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);

    render(<EnrollmentFormEditor />);

    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    expect(
      screen.getByText(
        "Die Angebote legen Sie in der Anmeldephase fest. Eltern sehen sie über den Link der Phase.",
      ),
    ).toBeVisible();

    fireEvent.click(
      screen.getByRole("button", {
        name: /^Abholzeiten\b/,
      }),
    );

    expect(
      screen.getByText(
        "Haben Ihre Betreuungsangebote eine Gehzeit? Dann brauchen Sie diese Frage nicht. Eine Antwort hier ersetzt die Gehzeit des Angebots.",
      ),
    ).toBeVisible();
  });
});

describe("prepareFieldsForSave — fixed pickup times", () => {
  const pickupField = (overrides: Partial<FormField> = {}): FormField => ({
    key: "schedule_pickup",
    label: "Abholzeiten",
    type: "weekday_schedule",
    target: "schedule.pickup",
    applies_to_child: true,
    sort_order: 0,
    ...overrides,
  });

  it("keeps allowed_times on the rebuilt pickup target field", () => {
    const [out] = prepareFieldsForSave([
      pickupField({ allowed_times: ["14:45", "16:00"] }),
    ]);
    expect(out?.allowed_times).toEqual(["14:45", "16:00"]);
    expect(out?.target).toBe("schedule.pickup");
  });

  it("leaves allowed_times undefined when none configured", () => {
    const [out] = prepareFieldsForSave([pickupField()]);
    expect(out?.allowed_times).toBeUndefined();
  });
});

describe("information PDF attachments in the form editor", () => {
  const firstURL = "/uploads/enrollment-form-legal-documents/1_information.pdf";
  const secondURL = "/uploads/enrollment-form-legal-documents/1_pickup.pdf";
  const replacementURL =
    "/uploads/enrollment-form-legal-documents/1_replacement.pdf";

  async function newNotice() {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.deleteEnrollmentLegalDocument.mockResolvedValue(undefined);
    const view = render(<EnrollmentFormEditor />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.change(
      screen.getByPlaceholderText("z. B. Ferienbetreuung Sommer 2026"),
      { target: { value: "Informationen" } },
    );
    fireEvent.click(screen.getByRole("button", { name: "Infotext" }));
    fireEvent.change(screen.getByPlaceholderText("z. B. Wichtiger Hinweis"), {
      target: { value: "Schulstart" },
    });
    fireEvent.change(screen.getByLabelText("Infotext für Eltern"), {
      target: { value: "Bitte lesen Sie beide Informationen." },
    });
    return view;
  }

  async function upload(name: string, url: string, index: number) {
    mocks.uploadEnrollmentLegalDocument.mockResolvedValueOnce(url);
    const file = new File(["%PDF-1.4"], name, { type: "application/pdf" });
    fireEvent.change(screen.getByLabelText("PDF-Dateien auswählen"), {
      target: { files: [file] },
    });
    await waitFor(() =>
      expect(
        screen.getByRole("textbox", {
          name: `Bezeichnung für Eltern: PDF ${index}`,
        }),
      ).toHaveValue(name.replace(/\.pdf$/, "")),
    );
    return file;
  }

  it("adds both PDFs from one selection and saves their separate labels", async () => {
    await newNotice();
    expect(
      screen.getByRole("button", { name: "PDFs hinzufügen" }),
    ).toBeInTheDocument();
    const picker = screen.getByLabelText("PDF-Dateien auswählen");
    expect(picker).toHaveAttribute("multiple");
    expect(screen.getByLabelText("Ersatz-PDF auswählen")).not.toHaveAttribute(
      "multiple",
    );
    mocks.uploadEnrollmentLegalDocument
      .mockResolvedValueOnce(firstURL)
      .mockResolvedValueOnce(secondURL);
    const files = [
      new File(["%PDF first"], "Schulstart.pdf", { type: "application/pdf" }),
      new File(["%PDF second"], "Abholen.pdf", { type: "application/pdf" }),
    ];
    fireEvent.change(picker, { target: { files } });
    await waitFor(() =>
      expect(
        screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 2" }),
      ).toHaveValue("Abholen"),
    );
    expect(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 1" }),
    ).toHaveValue("Schulstart");
    expect(
      screen.getByRole("button", { name: "Weitere PDFs hinzufügen" }),
    ).toBeEnabled();
    expect(mocks.uploadEnrollmentLegalDocument.mock.calls).toEqual([
      [files[0]],
      [files[1]],
    ]);
    mocks.createSchema.mockResolvedValueOnce(schema({ name: "Informationen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );
    await waitFor(() => expect(mocks.createSchema).toHaveBeenCalledOnce());
    expect(mocks.createSchema.mock.calls[0]![1]).toEqual([
      expect.objectContaining({
        attachments: [
          { label: "Schulstart", document_url: firstURL },
          { label: "Abholen", document_url: secondURL },
        ],
      }),
    ]);
  });

  it("keeps successful PDFs when one file in a selection fails", async () => {
    await newNotice();
    const failure = new Error("upload failed");
    mocks.uploadEnrollmentLegalDocument
      .mockResolvedValueOnce(firstURL)
      .mockRejectedValueOnce(failure)
      .mockResolvedValueOnce(secondURL);
    fireEvent.change(screen.getByLabelText("PDF-Dateien auswählen"), {
      target: {
        files: [
          new File(["%PDF one"], "Schulstart.pdf", { type: "application/pdf" }),
          new File(["%PDF two"], "Fehler.pdf", { type: "application/pdf" }),
          new File(["%PDF three"], "Abholen.pdf", { type: "application/pdf" }),
        ],
      },
    });
    await waitFor(() =>
      expect(
        screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 2" }),
      ).toHaveValue("Abholen"),
    );
    expect(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 1" }),
    ).toHaveValue("Schulstart");
    expect(mocks.showActionError).toHaveBeenCalledWith(failure, {
      object: "die PDF-Datei",
      messageSuffix: "Datei: Fehler.pdf",
    });
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: "Weitere PDFs hinzufügen" }),
    ).toBeEnabled();
  });

  it("saves two named PDFs on an information field without adding consent", async () => {
    await newNotice();
    const first = await upload("Schulstart.pdf", firstURL, 1);
    const second = await upload("Abholen.pdf", secondURL, 2);
    fireEvent.change(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 2" }),
      { target: { value: "Abholen und Kontakt" } },
    );
    mocks.createSchema.mockResolvedValueOnce(schema({ name: "Informationen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );
    await waitFor(() => expect(mocks.createSchema).toHaveBeenCalledOnce());
    expect(mocks.uploadEnrollmentLegalDocument.mock.calls).toEqual([
      [first],
      [second],
    ]);
    expect(mocks.createSchema.mock.calls[0]![1]).toEqual([
      expect.objectContaining({
        type: "information",
        attachments: [
          { label: "Schulstart", document_url: firstURL },
          { label: "Abholen und Kontakt", document_url: secondURL },
        ],
      }),
    ]);
    expect(
      (mocks.createSchema.mock.calls[0]![1] as FormField[])[0]!.required,
    ).not.toBe(true);
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
  });

  it("preserves the name when replacing a PDF and cleans removed drafts on discard", async () => {
    await newNotice();
    await upload("Schulstart.pdf", firstURL, 1);
    await upload("Abholen.pdf", secondURL, 2);
    fireEvent.click(screen.getByRole("button", { name: "PDF 1 ersetzen" }));
    mocks.uploadEnrollmentLegalDocument.mockResolvedValueOnce(replacementURL);
    fireEvent.change(screen.getByLabelText("Ersatz-PDF auswählen"), {
      target: {
        files: [
          new File(["%PDF replaced"], "new.pdf", { type: "application/pdf" }),
        ],
      },
    });
    await waitFor(() =>
      expect(mocks.uploadEnrollmentLegalDocument).toHaveBeenCalledTimes(3),
    );
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "PDF 1 ersetzen" }),
      ).toBeEnabled(),
    );
    expect(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 1" }),
    ).toHaveValue("Schulstart");
    fireEvent.click(screen.getByRole("button", { name: "PDF 2 entfernen" }));
    expect(
      screen.queryByRole("textbox", { name: "Bezeichnung für Eltern: PDF 2" }),
    ).not.toBeInTheDocument();
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
    fireEvent.click(
      screen.getByRole("button", { name: "Zurück zur Übersicht" }),
    );
    fireEvent.click(await screen.findByRole("button", { name: "Verwerfen" }));
    await waitFor(() =>
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledTimes(3),
    );
    for (const url of [firstURL, secondURL, replacementURL]) {
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
        url,
        expect.any(Object),
      );
    }
    expect(mocks.createSchema).not.toHaveBeenCalled();
  });

  it("preserves both existing PDFs for saving after a replacement upload fails", async () => {
    await newNotice();
    await upload("Schulstart.pdf", firstURL, 1);
    await upload("Abholen.pdf", secondURL, 2);
    fireEvent.click(screen.getByRole("button", { name: "PDF 1 ersetzen" }));
    const failure = new Error("offline");
    mocks.uploadEnrollmentLegalDocument.mockRejectedValueOnce(failure);
    fireEvent.change(screen.getByLabelText("Ersatz-PDF auswählen"), {
      target: {
        files: [new File(["%PDF"], "new.pdf", { type: "application/pdf" })],
      },
    });
    await waitFor(() =>
      expect(mocks.showActionError).toHaveBeenCalledWith(failure, {
        object: "die PDF-Datei",
        messageSuffix: "Datei: new.pdf",
      }),
    );
    expect(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 1" }),
    ).toHaveValue("Schulstart");
    expect(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 2" }),
    ).toHaveValue("Abholen");
    expect(
      screen.getByRole("button", { name: "Weitere PDFs hinzufügen" }),
    ).toBeEnabled();
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalled();
    mocks.createSchema.mockResolvedValueOnce(schema({ name: "Informationen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );
    await waitFor(() => expect(mocks.createSchema).toHaveBeenCalledOnce());
    expect(mocks.createSchema.mock.calls[0]![1]).toEqual([
      expect.objectContaining({
        attachments: [
          { label: "Schulstart", document_url: firstURL },
          { label: "Abholen", document_url: secondURL },
        ],
      }),
    ]);
  });

  it("saves a replacement and removal while cleaning only unreferenced drafts", async () => {
    await newNotice();
    await upload("Schulstart.pdf", firstURL, 1);
    await upload("Abholen.pdf", secondURL, 2);
    fireEvent.click(screen.getByRole("button", { name: "PDF 1 ersetzen" }));
    mocks.uploadEnrollmentLegalDocument.mockResolvedValueOnce(replacementURL);
    fireEvent.change(screen.getByLabelText("Ersatz-PDF auswählen"), {
      target: {
        files: [
          new File(["%PDF replacement"], "replacement.pdf", {
            type: "application/pdf",
          }),
        ],
      },
    });
    await waitFor(() =>
      expect(
        screen.getByRole("button", { name: "PDF 2 entfernen" }),
      ).toBeEnabled(),
    );
    fireEvent.click(screen.getByRole("button", { name: "PDF 2 entfernen" }));
    mocks.createSchema.mockResolvedValueOnce(schema({ name: "Informationen" }));
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );
    await waitFor(() => expect(mocks.createSchema).toHaveBeenCalledOnce());
    expect(mocks.createSchema.mock.calls[0]![1]).toEqual([
      expect.objectContaining({
        attachments: [{ label: "Schulstart", document_url: replacementURL }],
      }),
    ]);
    await waitFor(() =>
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledTimes(2),
    );
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
      firstURL,
      expect.any(Object),
    );
    expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
      secondURL,
      expect.any(Object),
    );
    expect(mocks.deleteEnrollmentLegalDocument).not.toHaveBeenCalledWith(
      replacementURL,
      expect.anything(),
    );
  });

  it("prevents saving an unnamed PDF and keeps the error at its label field", async () => {
    await newNotice();
    await upload("Schulstart.pdf", firstURL, 1);
    fireEvent.change(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 1" }),
      { target: { value: " " } },
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    );
    expect(mocks.createSchema).not.toHaveBeenCalled();
    expect(
      screen.getByRole("textbox", { name: "Bezeichnung für Eltern: PDF 1" }),
    ).toHaveAttribute("aria-invalid", "true");
  });

  it("blocks saving during an upload and cleans a late result after unmount", async () => {
    const pending = deferred<string>();
    const view = await newNotice();
    mocks.uploadEnrollmentLegalDocument.mockReturnValueOnce(pending.promise);
    fireEvent.change(screen.getByLabelText("PDF-Dateien auswählen"), {
      target: {
        files: [
          new File(["%PDF"], "draft.pdf", { type: "application/pdf" }),
          new File(["%PDF"], "unstarted.pdf", { type: "application/pdf" }),
        ],
      },
    });
    expect(
      screen.getByRole("button", { name: "Formularvorlage erstellen" }),
    ).toBeDisabled();
    view.unmount();
    await act(async () => pending.resolve(firstURL));
    await waitFor(() =>
      expect(mocks.deleteEnrollmentLegalDocument).toHaveBeenCalledWith(
        firstURL,
        { keepalive: true },
      ),
    );
    expect(mocks.uploadEnrollmentLegalDocument).toHaveBeenCalledOnce();
    expect(mocks.createSchema).not.toHaveBeenCalled();
  });
});

describe("information PDF validation", () => {
  it("rejects wrong extensions and oversized PDFs before sending any upload", async () => {
    mocks.listSchemas.mockResolvedValue([]);
    mocks.listPhases.mockResolvedValue([]);
    mocks.showActionMessage.mockReset();
    render(<EnrollmentFormEditor />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Neue Vorlage" }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Infotext" }));
    const picker = screen.getByLabelText("PDF-Dateien auswählen");
    fireEvent.change(picker, {
      target: {
        files: [new File(["image"], "image.png", { type: "image/png" })],
      },
    });
    expect(mocks.showActionMessage).toHaveBeenLastCalledWith(
      "Bitte wählen Sie nur PDF-Dateien aus.",
    );
    const oversized = new File(["%PDF"], "large.pdf", {
      type: "application/pdf",
    });
    Object.defineProperty(oversized, "size", { value: 10 * 1024 * 1024 + 1 });
    fireEvent.change(picker, { target: { files: [oversized] } });
    expect(mocks.showActionMessage).toHaveBeenLastCalledWith(
      "Eine Datei ist größer als 10 MB. Bitte wählen Sie kleinere Dateien.",
    );
    expect(mocks.uploadEnrollmentLegalDocument).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("textbox", { name: "Bezeichnung für Eltern: PDF 1" }),
    ).not.toBeInTheDocument();
  });
});
