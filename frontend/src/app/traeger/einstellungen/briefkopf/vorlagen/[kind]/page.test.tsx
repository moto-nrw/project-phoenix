import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  act,
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import { SAVED_BUT_STALE_SUFFIX } from "~/lib/hooks/use-refresh-after-write";
import { catalogText } from "~/test/error-catalog-text";
import type { TraegerMasterData } from "~/lib/traeger-api";
import type {
  DocumentTemplateDetail,
  DocumentTemplatePreview,
} from "~/lib/traeger-document-templates";

const { api, templatesApi, session, params } = vi.hoisted(() => ({
  api: {
    fetchTraegerMasterData: vi.fn(),
    fetchTraegerSettingsSchema: vi.fn(),
    traegerLogoSrc: () => "/api/traeger/master-data/logo?v=1",
  },
  templatesApi: {
    fetchDocumentTemplate: vi.fn(),
    previewDocumentTemplate: vi.fn(),
    saveDocumentTemplate: vi.fn(),
  },
  session: {
    permissions: ["traeger_overview:read", "traeger_settings:manage"],
  },
  params: { kind: "rechnung" },
}));

vi.mock("~/lib/traeger-api", () => api);
vi.mock("~/lib/traeger-document-templates", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("~/lib/traeger-document-templates")
  >()),
  ...templatesApi,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next/navigation", () => ({
  useParams: () => params,
  useRouter: () => ({ push: vi.fn() }),
  usePathname: () => `/einstellungen/briefkopf/vorlagen/${params.kind}`,
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: {
      user: {
        id: "A",
        scope: "org",
        orgId: 7,
        email: "buero@traeger.example",
        permissions: session.permissions,
      },
    },
    status: "authenticated",
  }),
}));

import TraegerVorlagePage from "./page";

const STANDARD = {
  subject: "Rechnung {{rechnungsnummer}}: {{zeitraum}}",
  salutation: "Sehr geehrte Familie {{familienname}},",
  body: "für {{kinder}} berechnen wir Ihnen:\n\n{{positionen}}",
  closing: "Mit freundlichen Grüßen\n{{traeger}}",
};

function detail(
  versions: DocumentTemplateDetail["versions"] = [],
): DocumentTemplateDetail {
  return {
    current: versions[0] ?? {
      kind: "rechnung",
      version: 0,
      standard: true,
      createdAt: null,
      ...STANDARD,
    },
    standardTemplate: STANDARD,
    placeholders: [
      { name: "familienname", label: "Familienname", table: false },
      { name: "positionen", label: "Tabelle der Beträge", table: true },
    ],
    versions,
  };
}

const PREVIEW: DocumentTemplatePreview = {
  subject: "Rechnung FE-2026-0001: Herbstferien 2026",
  salutation: "Sehr geehrte Familie Muster,",
  blocks: [
    {
      heading: "",
      text: "für Lina und Ben berechnen wir Ihnen:",
      bold: false,
      rows: [{ label: "Gesamtbetrag", value: "92,50 €", bold: true }],
    },
  ],
  closing: ["Mit freundlichen Grüßen", "Ihr Träger"],
};

const MASTER_DATA: TraegerMasterData = {
  contact: { name: "Petra Lindner", phone: "", email: "" },
  withdrawal: { phone: "", email: "" },
  logo: null,
  bankAccounts: [],
};

const BRANDING_SCHEMA = {
  tabs: [
    {
      key: "branding",
      categories: [
        {
          items: [
            {
              key: "branding.letterhead_text",
              value: "Ganztag im Quartier e.V.",
            },
            { key: "branding.footer_text", value: "Amtsgericht Osnabrück" },
            { key: "branding.primary_color", value: "#217A78" },
          ],
        },
      ],
    },
  ],
};

function renderPage() {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map(), shouldRetryOnError: false }}>
      <ToastProvider>
        <TraegerVorlagePage />
      </ToastProvider>
    </SWRConfig>,
  );
}

describe("TraegerVorlagePage", () => {
  beforeEach(() => {
    params.kind = "rechnung";
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    for (const fn of [...Object.values(api), ...Object.values(templatesApi)]) {
      if (typeof fn === "function" && "mockReset" in fn) fn.mockReset();
    }
    api.fetchTraegerMasterData.mockResolvedValue(MASTER_DATA);
    api.fetchTraegerSettingsSchema.mockResolvedValue({ tabs: [] });
    templatesApi.fetchDocumentTemplate.mockResolvedValue(detail());
    templatesApi.previewDocumentTemplate.mockResolvedValue(PREVIEW);
  });

  it("shows the standard template filled with sample data in the letterhead", async () => {
    renderPage();
    const preview = await screen.findByRole("figure", {
      name: "Vorschau des Briefs",
    });
    expect(
      within(preview).getByText("Rechnung FE-2026-0001: Herbstferien 2026"),
    ).toBeInTheDocument();
    expect(
      within(preview).getByRole("columnheader", { name: "Bezeichnung" }),
    ).toBeInTheDocument();
    const total = within(preview).getByRole("cell", { name: "92,50 €" });
    expect(total).toHaveClass("text-right", "tabular-nums");
    expect(total.closest("tr")).toHaveClass("font-semibold");
    expect(screen.getByLabelText(/Betreff/)).toHaveValue(STANDARD.subject);
    expect(screen.getByText("{{familienname}}")).toBeInTheDocument();
    expect(screen.getByText("Standardvorlage")).toBeInTheDocument();
    expect(screen.getByText(/Noch keine eigene Version/)).toBeInTheDocument();
    expect(templatesApi.previewDocumentTemplate).toHaveBeenCalledWith(
      "rechnung",
      STANDARD,
    );
  });

  it("waits for branding before showing the template preview", async () => {
    let finishBranding!: (value: typeof BRANDING_SCHEMA) => void;
    api.fetchTraegerSettingsSchema.mockReturnValue(
      new Promise<typeof BRANDING_SCHEMA>((resolve) => {
        finishBranding = resolve;
      }),
    );
    renderPage();
    await waitFor(() =>
      expect(templatesApi.previewDocumentTemplate).toHaveBeenCalled(),
    );
    // Anker: template data has loaded, but branding still keeps the page loading.
    expect(
      screen.getByRole("status", {
        name: "Der Bereich „Rechnung“ wird geladen…",
      }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("figure", { name: "Vorschau des Briefs" }),
    ).not.toBeInTheDocument();
    await act(async () => {
      finishBranding(BRANDING_SCHEMA);
    });
    const preview = await screen.findByRole("figure", {
      name: "Vorschau des Briefs",
    });
    expect(preview).toHaveTextContent("Ganztag im Quartier e.V.");
    expect(preview).toHaveTextContent("Amtsgericht Osnabrück");
  });

  it("shows a branding load error and loads the letterhead on retry", async () => {
    api.fetchTraegerSettingsSchema.mockRejectedValue(
      new ApiError("unavailable", 503, { code: "general.unavailable" }),
    );
    renderPage();
    // Anker: the schema failure is visible instead of an incomplete preview.
    expect(
      await screen.findByText(
        catalogText("general.unavailable", "die Liste der Einstellungen"),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("figure", { name: "Vorschau des Briefs" }),
    ).not.toBeInTheDocument();
    api.fetchTraegerSettingsSchema.mockResolvedValue(BRANDING_SCHEMA);
    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    const preview = await screen.findByRole("figure", {
      name: "Vorschau des Briefs",
    });
    expect(preview).toHaveTextContent("Ganztag im Quartier e.V.");
    expect(preview).toHaveTextContent("Amtsgericht Osnabrück");
    expect(api.fetchTraegerSettingsSchema).toHaveBeenCalledTimes(2);
  });

  it("previews and saves an edited template as a new version", async () => {
    renderPage();
    const subject = await screen.findByLabelText(/Betreff/);
    // Nothing changed yet: no save, and the preview is current.
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorschau zeigen" }),
    ).toBeDisabled();

    fireEvent.change(subject, { target: { value: "Ihre Rechnung" } });
    expect(
      screen.getByText(/Die Vorschau zeigt noch den Text vor Ihren Änderungen/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Vorschau zeigen" }));
    await waitFor(() =>
      expect(templatesApi.previewDocumentTemplate).toHaveBeenLastCalledWith(
        "rechnung",
        { ...STANDARD, subject: "Ihre Rechnung" },
      ),
    );

    templatesApi.saveDocumentTemplate.mockResolvedValue({
      kind: "rechnung",
      version: 1,
      standard: false,
      createdAt: "2026-09-09T10:00:00Z",
      ...STANDARD,
      subject: "Ihre Rechnung",
    });
    templatesApi.fetchDocumentTemplate.mockResolvedValue(
      detail([
        {
          kind: "rechnung",
          version: 1,
          standard: false,
          createdAt: "2026-09-09T10:00:00Z",
          ...STANDARD,
          subject: "Ihre Rechnung",
        },
      ]),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    expect(
      await screen.findByText(
        "Die Vorlage ist gespeichert. Neue Schreiben nutzen Version 1.",
      ),
    ).toBeInTheDocument();
    expect(templatesApi.saveDocumentTemplate).toHaveBeenCalledWith("rechnung", {
      ...STANDARD,
      subject: "Ihre Rechnung",
    });
    expect(
      await screen.findByText("Gilt für neue Schreiben"),
    ).toBeInTheDocument();
  });

  it("locks all draft changes until saving and reloading finish", async () => {
    const saved: DocumentTemplateDetail["current"] = {
      kind: "rechnung",
      version: 1,
      standard: false,
      createdAt: "2026-09-09T10:00:00Z",
      ...STANDARD,
      subject: "Gespeichert",
    };
    templatesApi.fetchDocumentTemplate.mockResolvedValue(detail([saved]));
    renderPage();
    const subject = await screen.findByLabelText(/Betreff/);
    fireEvent.change(subject, { target: { value: "Neue Rechnung" } });
    let finishSave!: (value: typeof saved) => void;
    templatesApi.saveDocumentTemplate.mockReturnValue(
      new Promise<typeof saved>((resolve) => {
        finishSave = resolve;
      }),
    );
    let finishReload!: (value: DocumentTemplateDetail) => void;
    templatesApi.fetchDocumentTemplate.mockReturnValue(
      new Promise<DocumentTemplateDetail>((resolve) => {
        finishReload = resolve;
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    const expectLocked = () => {
      for (const label of [/Betreff/, /Anrede/, /Brieftext/, /Grußformel/]) {
        expect(screen.getByLabelText(label)).toBeDisabled();
      }
      expect(
        screen.getByRole("button", { name: "Standardtext laden" }),
      ).toBeDisabled();
      expect(
        screen.getByRole("button", { name: "Text von Version 1 übernehmen" }),
      ).toBeDisabled();
      expect(
        screen.getByRole("button", { name: "Vorschau zeigen" }),
      ).toBeDisabled();
      expect(screen.getByRole("button", { name: "Abbrechen" })).toBeDisabled();
    };
    expectLocked();
    await act(async () => {
      finishSave({ ...saved, subject: "Neue Rechnung" });
    });
    expectLocked();
    expect(subject).toHaveValue("Neue Rechnung");
    await act(async () => {
      finishReload(detail([{ ...saved, subject: "Neue Rechnung" }]));
    });
    expect(subject).toBeEnabled();
    expect(subject).toHaveValue("Neue Rechnung");
  });

  it("ignores a preview that finishes after saving and reloading", async () => {
    renderPage();
    const subject = await screen.findByLabelText(/Betreff/);
    fireEvent.change(subject, { target: { value: "Alte Vorschau" } });
    let finishPreview!: (value: DocumentTemplatePreview) => void;
    templatesApi.previewDocumentTemplate.mockReturnValueOnce(
      new Promise<DocumentTemplatePreview>((resolve) => {
        finishPreview = resolve;
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Vorschau zeigen" }));
    fireEvent.change(subject, { target: { value: "Neue Rechnung" } });
    const saved: DocumentTemplateDetail["current"] = {
      kind: "rechnung",
      version: 1,
      standard: false,
      createdAt: "2026-09-09T10:00:00Z",
      ...STANDARD,
      subject: "Neue Rechnung",
    };
    templatesApi.saveDocumentTemplate.mockResolvedValue(saved);
    templatesApi.fetchDocumentTemplate.mockResolvedValue(detail([saved]));
    const savedPreview = { ...PREVIEW, subject: "Neue Rechnung" };
    templatesApi.previewDocumentTemplate.mockResolvedValue(savedPreview);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await screen.findByText("Gilt für neue Schreiben");
    await act(async () => {
      finishPreview({ ...PREVIEW, subject: "Alte Vorschau" });
    });
    const preview = screen.getByRole("figure", { name: "Vorschau des Briefs" });
    expect(within(preview).getByText("Neue Rechnung")).toBeInTheDocument();
    expect(screen.getByText("Gilt für neue Schreiben")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Vorschau zeigen" }),
    ).toBeDisabled();
    fireEvent.change(subject, { target: { value: "Weiter bearbeiten" } });
    fireEvent.click(screen.getByRole("button", { name: "Abbrechen" }));
    expect(subject).toHaveValue("Neue Rechnung");
  });

  it("names the unknown placeholders the backend refuses", async () => {
    renderPage();
    const body = await screen.findByLabelText(/Brieftext/);
    fireEvent.change(body, { target: { value: "Hallo {{vorname}}" } });
    templatesApi.previewDocumentTemplate.mockRejectedValue(
      new ApiError("refused", 400, {
        code: "traeger.document_template_unknown_placeholder",
        details: { placeholders: "vorname" },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "Vorschau zeigen" }));
    expect(
      await screen.findByText(/Diese Platzhalter kennt moto nicht: vorname/),
    ).toBeInTheDocument();
  });

  it("loads an earlier version into the editor", async () => {
    templatesApi.fetchDocumentTemplate.mockResolvedValue(
      detail([
        {
          kind: "rechnung",
          version: 2,
          standard: false,
          createdAt: "2026-09-09T10:00:00Z",
          ...STANDARD,
          subject: "Neu",
        },
        {
          kind: "rechnung",
          version: 1,
          standard: false,
          createdAt: "2026-09-01T10:00:00Z",
          ...STANDARD,
          subject: "Alt",
        },
      ]),
    );
    renderPage();
    expect(await screen.findByLabelText(/Betreff/)).toHaveValue("Neu");
    fireEvent.click(
      screen.getByRole("button", { name: "Text von Version 1 übernehmen" }),
    );
    expect(screen.getByLabelText(/Betreff/)).toHaveValue("Alt");
    expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
  });

  it("only shows the template to a role without the settings right", async () => {
    session.permissions = ["traeger_overview:read"];
    renderPage();
    expect(await screen.findByLabelText(/Betreff/)).toBeDisabled();
    expect(
      screen.getByText(/Sie können die Vorlage nur ansehen/),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Standardtext laden" }),
    ).not.toBeInTheDocument();
  });

  it("shows the whole letter with address field and numbers (#5691)", async () => {
    templatesApi.previewDocumentTemplate.mockResolvedValue({
      ...PREVIEW,
      recipient: ["Familie Muster", "Erika Muster"],
      facts: [
        { label: "Rechnungsnummer", value: "FE-2026-0001", bold: false },
        { label: "Debitorennummer", value: "10234", bold: false },
      ],
    });
    renderPage();
    const preview = await screen.findByRole("figure", {
      name: "Vorschau des Briefs",
    });
    expect(
      within(preview).getByRole("group", { name: "Anschrift" }),
    ).toHaveTextContent("Familie MusterErika Muster");
    const facts = within(preview).getByLabelText("Angaben zum Schreiben");
    expect(facts).toHaveTextContent("Debitorennummer10234");
    // The contact person moves into the facts, as in the PDF.
    expect(facts).toHaveTextContent("AnsprechpersonPetra Lindner");
  });

  it("saves the layout of the invoice table with the text (#5691)", async () => {
    renderPage();
    const group = await screen.findByRole("checkbox", {
      name: /Nach Kind ordnen/,
    });
    expect(group).not.toBeChecked();
    // The week is a mandatory detail of the invoice: no switch for it.
    expect(
      screen.queryByRole("checkbox", { name: /Woche zeigen/ }),
    ).not.toBeInTheDocument();
    fireEvent.click(group);
    fireEvent.click(
      screen.getByRole("checkbox", { name: /Zeitmodell zeigen/ }),
    );
    const invoice = {
      groupByChild: true,
      hideTimeModels: true,
    };
    fireEvent.click(screen.getByRole("button", { name: "Vorschau zeigen" }));
    await waitFor(() =>
      expect(templatesApi.previewDocumentTemplate).toHaveBeenLastCalledWith(
        "rechnung",
        { ...STANDARD, invoice },
      ),
    );
    templatesApi.saveDocumentTemplate.mockResolvedValue({
      kind: "rechnung",
      version: 1,
      standard: false,
      createdAt: "2026-09-09T10:00:00Z",
      ...STANDARD,
      invoice,
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
    await waitFor(() =>
      expect(templatesApi.saveDocumentTemplate).toHaveBeenCalledWith(
        "rechnung",
        { ...STANDARD, invoice },
      ),
    );
  });

  it("has no invoice table for a kind without amounts", async () => {
    params.kind = "kuendigung";
    templatesApi.fetchDocumentTemplate.mockResolvedValue({
      ...detail(),
      placeholders: [
        { name: "familienname", label: "Familienname", table: false },
      ],
    });
    renderPage();
    // Anker: the form has loaded.
    expect(await screen.findByLabelText(/Betreff/)).toBeInTheDocument();
    expect(
      screen.queryByRole("checkbox", { name: /Nach Kind ordnen/ }),
    ).not.toBeInTheDocument();
  });

  it("inserts a placeholder at the caret of the body", async () => {
    renderPage();
    const body = await screen.findByLabelText<HTMLTextAreaElement>(/Brieftext/);
    body.setSelectionRange(3, 3);
    fireEvent.blur(body);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Platzhalter „Familienname“ einfügen",
      }),
    );
    expect(body).toHaveValue(
      `${STANDARD.body.slice(0, 3)}{{familienname}}${STANDARD.body.slice(3)}`,
    );
    // The caret stays behind the placeholder; a table placeholder gets a
    // line of its own.
    fireEvent.click(
      screen.getByRole("button", {
        name: "Platzhalter „Tabelle der Beträge (Tabelle)“ einfügen",
      }),
    );
    expect(
      body.value.startsWith("für{{familienname}}\n{{positionen}}\n "),
    ).toBe(true);
  });

  it("puts a placeholder at the end while the body had no caret", async () => {
    renderPage();
    const body = await screen.findByLabelText<HTMLTextAreaElement>(/Brieftext/);
    fireEvent.click(
      screen.getByRole("button", {
        name: "Platzhalter „Familienname“ einfügen",
      }),
    );
    expect(body).toHaveValue(`${STANDARD.body}{{familienname}}`);
  });

  it("offers no placeholder buttons to a reader", async () => {
    session.permissions = ["traeger_overview:read"];
    renderPage();
    // Anker: the reference list still names the placeholder.
    expect(await screen.findByText("{{familienname}}")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Platzhalter .* einfügen/ }),
    ).not.toBeInTheDocument();
  });

  it("explains an unknown kind of document", () => {
    params.kind = "brief";
    renderPage();
    expect(
      screen.getByText(
        "Diese Vorlage gibt es nicht. Wählen Sie eine aus der Liste.",
      ),
    ).toBeInTheDocument();
    expect(templatesApi.fetchDocumentTemplate).not.toHaveBeenCalled();
  });
  // #6652: after a saved version a failed reload is a load error above the
  // template, not a page error that replaces the editor.
  describe("reload after a write (#6652)", () => {
    it.each(["before", "during"])(
      "preserves edits made %s a reload retry",
      async (when) => {
        renderPage();
        const subject = await screen.findByLabelText(/Betreff/);
        fireEvent.change(subject, {
          target: { value: "Gespeicherter Betreff" },
        });
        const saved: DocumentTemplateDetail["current"] = {
          ...detail().current,
          version: 1,
          standard: false,
          subject: "Gespeicherter Betreff",
        };
        templatesApi.saveDocumentTemplate.mockResolvedValue(saved);
        templatesApi.fetchDocumentTemplate.mockRejectedValueOnce(
          new ApiError("down", 503, { code: "general.unavailable" }),
        );
        fireEvent.click(screen.getByRole("button", { name: "Speichern" }));
        const retry = await screen.findByRole("button", {
          name: "Wiederholen",
        });
        await waitFor(() => expect(subject).toBeEnabled());
        fireEvent.change(subject, {
          target: { value: "Entwurf vor Wiederholen" },
        });
        let finishReload!: (value: DocumentTemplateDetail) => void;
        templatesApi.fetchDocumentTemplate.mockReturnValueOnce(
          new Promise<DocumentTemplateDetail>((resolve) => {
            finishReload = resolve;
          }),
        );
        fireEvent.click(retry);
        if (when === "during") {
          fireEvent.change(subject, {
            target: { value: "Noch neuerer Entwurf" },
          });
        }
        await act(async () => finishReload(detail([saved])));

        expect(subject).toHaveValue(
          when === "during"
            ? "Noch neuerer Entwurf"
            : "Entwurf vor Wiederholen",
        );
        expect(screen.getByRole("button", { name: "Speichern" })).toBeEnabled();
        expect(templatesApi.saveDocumentTemplate).toHaveBeenCalledTimes(1);
      },
    );

    it("keeps a saved template version when the reload fails", async () => {
      renderPage();
      const subject = await screen.findByLabelText(/Betreff/);
      fireEvent.change(subject, { target: { value: "Ihre Rechnung" } });
      const saved: DocumentTemplateDetail["current"] = {
        kind: "rechnung",
        version: 1,
        standard: false,
        createdAt: "2026-09-09T10:00:00Z",
        ...STANDARD,
        subject: "Ihre Rechnung",
      };
      templatesApi.saveDocumentTemplate.mockResolvedValue(saved);
      templatesApi.fetchDocumentTemplate.mockClear();
      templatesApi.fetchDocumentTemplate.mockRejectedValueOnce(
        new ApiError("down", 503, { code: "general.unavailable" }),
      );
      templatesApi.fetchDocumentTemplate.mockResolvedValue(detail([saved]));

      fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

      const reloadText = `${catalogText(
        "general.unavailable",
        "die Anzeige der Vorlage",
      )} ${SAVED_BUT_STALE_SUFFIX}`;
      expect(await screen.findByText(reloadText)).toBeInTheDocument();
      expect(templatesApi.saveDocumentTemplate).toHaveBeenCalledTimes(1);
      // Anker: the editor and its preview still show the saved text.
      expect(screen.getByLabelText(/Betreff/)).toHaveValue("Ihre Rechnung");
      expect(
        screen.getByRole("figure", { name: "Vorschau des Briefs" }),
      ).toBeInTheDocument();
      expect(
        screen.queryByText(catalogText("general.unavailable", "die Vorlage")),
      ).not.toBeInTheDocument();
      expect(screen.getByLabelText(/Betreff/)).toBeEnabled();
      // The saved version is the current one: no second save is offered,
      // which would create another version.
      expect(
        screen.queryByRole("button", { name: "Speichern" }),
      ).not.toBeInTheDocument();

      fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
      expect(
        await screen.findByText("Gilt für neue Schreiben"),
      ).toBeInTheDocument();
      expect(screen.queryByText(reloadText)).not.toBeInTheDocument();
      expect(templatesApi.fetchDocumentTemplate).toHaveBeenCalledTimes(2);
      expect(templatesApi.saveDocumentTemplate).toHaveBeenCalledTimes(1);
    });
  });
});
