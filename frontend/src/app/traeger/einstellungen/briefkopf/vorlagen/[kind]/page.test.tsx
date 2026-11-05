import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
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
  logo: null,
  bankAccounts: [],
};

function renderPage() {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>
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
    expect(within(preview).getByText("92,50 €")).toBeInTheDocument();
    expect(screen.getByLabelText(/Betreff/)).toHaveValue(STANDARD.subject);
    expect(screen.getByText("{{familienname}}")).toBeInTheDocument();
    expect(screen.getByText("Standardvorlage")).toBeInTheDocument();
    expect(screen.getByText(/Noch keine eigene Version/)).toBeInTheDocument();
    expect(templatesApi.previewDocumentTemplate).toHaveBeenCalledWith(
      "rechnung",
      STANDARD,
    );
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
});
