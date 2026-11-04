import { useTenantMutateMatching } from "~/lib/swr";
import { traegerKeys } from "~/lib/swr/portal-keys";
import { SWRConfig } from "~/lib/swr/testing";
vi.unmock("swr");
import {
  fireEvent,
  render as rtlRender,
  screen,
  waitFor,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import { ApiError } from "~/lib/api-error";
import type { ResolvedSetting, SettingsSchema } from "~/lib/settings-api";
import type { TraegerMasterData } from "~/lib/traeger-api";
import { catalogText } from "~/test/error-catalog-text";

const { api, session } = vi.hoisted(() => ({
  api: {
    fetchTraegerMasterData: vi.fn(),
    fetchTraegerSettingsSchema: vi.fn(),
    saveTraegerSettingValue: vi.fn(),
    clearTraegerSettingValue: vi.fn(),
    uploadTraegerLogo: vi.fn(),
    removeTraegerLogo: vi.fn(),
    saveTraegerContact: vi.fn(),
    createTraegerBankAccount: vi.fn(),
    updateTraegerBankAccount: vi.fn(),
    deleteTraegerBankAccount: vi.fn(),
    traegerLogoSrc: () => "/api/traeger/master-data/logo?v=1",
    TRAEGER_LOGO_MAX_BYTES: 1024 * 1024,
  },
  session: {
    permissions: ["traeger_overview:read", "traeger_settings:manage"],
  },
}));

vi.mock("~/lib/traeger-api", () => api);
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

import TraegerBriefkopfPage from "./page";

function setting(key: string, label: string, value: string): ResolvedSetting {
  return {
    key,
    label,
    description: "",
    type: key === "branding.primary_color" ? "color" : "textarea",
    default: "",
    value,
    is_default: value === "",
    writable: session.permissions.includes("traeger_settings:manage"),
    visible: true,
    sort_order: 1,
    access_policy: "shared",
    levels: "traeger",
    source: value === "" ? "default" : "traeger",
    validation: null,
    depends_on: null,
    options: null,
  };
}

function brandingSchema(): SettingsSchema {
  return {
    tabs: [
      {
        key: "branding",
        label: "branding",
        categories: [
          {
            key: "briefkopf",
            label: "briefkopf",
            items: [
              setting("branding.primary_color", "Farbe", "#217A78"),
              setting(
                "branding.letterhead_text",
                "Absender im Briefkopf",
                "Ganztag im Quartier e.V.\nAm Markt 4\n49143 Bissendorf",
              ),
              setting(
                "branding.footer_text",
                "Fußzeile",
                "Amtsgericht Osnabrück",
              ),
            ],
          },
        ],
      },
    ],
  };
}

const MASTER_DATA: TraegerMasterData = {
  contact: { name: "Petra Lindner", phone: "+49 5402 1234-0", email: "" },
  logo: null,
  bankAccounts: [
    {
      id: "7",
      label: "Elternbeiträge",
      accountHolder: "Ganztag im Quartier e.V.",
      iban: "DE89370400440532013000",
      bic: "COBADEFFXXX",
      bankName: "Commerzbank",
      isDefault: true,
    },
    {
      id: "8",
      label: "Ferienbetreuung",
      accountHolder: "Ganztag im Quartier e.V.",
      iban: "DE02120300000000202051",
      bic: "",
      bankName: "",
      isDefault: false,
    },
  ],
};

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerBriefkopfPage />
    </ToastProvider>,
  );
}

describe("TraegerBriefkopfPage", () => {
  beforeEach(() => {
    session.permissions = ["traeger_overview:read", "traeger_settings:manage"];
    for (const fn of Object.values(api)) {
      if (typeof fn === "function" && "mockReset" in fn) fn.mockReset();
    }
    api.fetchTraegerMasterData.mockResolvedValue(MASTER_DATA);
    api.fetchTraegerSettingsSchema.mockResolvedValue(brandingSchema());
  });

  it.each([503, 403])(
    "keeps loaded content only for recoverable schema refresh errors (%s)",
    async (status) => {
      render(
        <ToastProvider>
          <RefreshSchema />
          <TraegerBriefkopfPage />
        </ToastProvider>,
      );
      expect(
        await screen.findByRole("figure", { name: "Vorschau des Briefkopfs" }),
      ).toBeInTheDocument();
      const code =
        status === 503 ? "general.unavailable" : "general.permission";
      api.fetchTraegerSettingsSchema.mockRejectedValue(
        new ApiError("refused", status, { code }),
      );
      fireEvent.click(screen.getByRole("button", { name: "Refresh schema" }));
      // Anker: the current schema failure is shown after a complete initial load.
      expect(
        await screen.findByText(
          catalogText(code, "die Liste der Einstellungen"),
        ),
      ).toBeInTheDocument();
      if (status === 503)
        expect(
          screen.queryByRole("figure", { name: "Vorschau des Briefkopfs" }),
        ).toBeInTheDocument();
      else
        expect(
          screen.queryByRole("figure", { name: "Vorschau des Briefkopfs" }),
        ).not.toBeInTheDocument();
    },
  );

  it("previews the letterhead with sender, contact and the default bank account", async () => {
    renderPage();

    const preview = await screen.findByRole("figure", {
      name: "Vorschau des Briefkopfs",
    });
    expect(preview).toHaveTextContent("Ganztag im Quartier e.V.");
    expect(preview).toHaveTextContent("49143 Bissendorf");
    expect(preview).toHaveTextContent("Petra Lindner · +49 5402 1234-0");
    expect(preview).toHaveTextContent("IBAN DE89 3704 0044 0532 0130 00");
    expect(preview).toHaveTextContent("Amtsgericht Osnabrück");
    expect(preview).not.toHaveTextContent("Ferienbetreuung");
    expect(screen.getByText("Kein Logo")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Bankverbindung hinzufügen" }),
    ).toBeInTheDocument();
  });

  it("shows the letterhead read-only without the right Einstellungen", async () => {
    session.permissions = ["traeger_overview:read"];
    renderPage();

    expect(
      await screen.findByText(
        "Sie können die Angaben nur ansehen. Ändern kann, wer im Träger-Büro das Recht „Einstellungen“ hat.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByLabelText("Name")).toBeDisabled();
    expect(screen.getAllByText("Ferienbetreuung").length).toBeGreaterThan(0);
    expect(
      screen.queryByRole("button", { name: "Bankverbindung hinzufügen" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Logo hochladen" }),
    ).not.toBeInTheDocument();
  });

  it("adds a bank account and reads the master data again", async () => {
    api.createTraegerBankAccount.mockResolvedValue({
      ...MASTER_DATA.bankAccounts[1],
      id: "9",
    });
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Bankverbindung hinzufügen" }),
    );
    fireEvent.change(screen.getByLabelText(/Bezeichnung/), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText(/Kontoinhaber/), {
      target: { value: "Ganztag im Quartier e.V." },
    });
    fireEvent.change(screen.getByLabelText(/^IBAN/), {
      target: { value: "GB82 WEST 1234 5698 7654 32" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(api.createTraegerBankAccount).toHaveBeenCalledWith({
        label: "Mensa",
        accountHolder: "Ganztag im Quartier e.V.",
        iban: "GB82 WEST 1234 5698 7654 32",
        bic: "",
        bankName: "",
        isDefault: false,
      }),
    );
    await waitFor(() =>
      expect(api.fetchTraegerMasterData).toHaveBeenCalledTimes(2),
    );
  });

  it("shows a refused IBAN at its field", async () => {
    api.createTraegerBankAccount.mockRejectedValue(
      new ApiError("invalid", 400, {
        code: "traeger.master_data_invalid",
        errors: [{ field: "iban", reason: "is not a valid IBAN" }],
      }),
    );
    renderPage();

    fireEvent.click(
      await screen.findByRole("button", { name: "Bankverbindung hinzufügen" }),
    );
    fireEvent.change(screen.getByLabelText(/Bezeichnung/), {
      target: { value: "Mensa" },
    });
    fireEvent.change(screen.getByLabelText(/Kontoinhaber/), {
      target: { value: "Träger e.V." },
    });
    fireEvent.change(screen.getByLabelText(/^IBAN/), {
      target: { value: "DE00 1234" },
    });
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    expect(
      await screen.findByText(
        catalogText("traeger.master_data_invalid", "die Bankverbindung"),
      ),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByLabelText(/^IBAN/)).toHaveAttribute(
        "aria-invalid",
        "true",
      ),
    );
  });

  it("refuses a logo that is no PNG or JPG before uploading it", async () => {
    renderPage();
    await screen.findByRole("button", { name: "Logo hochladen" });

    const input = screen.getByLabelText("Logo-Datei auswählen");
    fireEvent.change(input, {
      target: {
        files: [new File(["GIF89a"], "logo.gif", { type: "image/gif" })],
      },
    });

    expect(
      await screen.findByText(
        "Bitte wählen Sie ein Bild im Format PNG oder JPG.",
      ),
    ).toBeInTheDocument();
    expect(api.uploadTraegerLogo).not.toHaveBeenCalled();

    const png = new File(["png"], "logo.png", { type: "image/png" });
    api.uploadTraegerLogo.mockResolvedValue({
      contentType: "image/png",
      width: 240,
      height: 80,
      sizeBytes: 3,
      updatedAt: "2026-09-09T10:00:00Z",
    });
    fireEvent.change(input, { target: { files: [png] } });
    await waitFor(() =>
      expect(api.uploadTraegerLogo).toHaveBeenCalledWith(png),
    );
  });

  it("shows no editable cards after a failed load and loads them on retry", async () => {
    api.fetchTraegerMasterData.mockRejectedValueOnce(
      new ApiError("unavailable", 503, { code: "general.unavailable" }),
    );
    renderPage();

    expect(
      await screen.findByText(
        catalogText("general.unavailable", "der Briefkopf"),
      ),
    ).toBeInTheDocument();
    expect(screen.queryByLabelText("Name")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Wiederholen" }));
    expect(await screen.findByLabelText("Name")).toHaveValue("Petra Lindner");
  });
});

function render(ui: React.ReactElement) {
  return rtlRender(
    <SWRConfig value={{ provider: () => new Map() }}>{ui}</SWRConfig>,
  );
}

function RefreshSchema() {
  const refresh = useTenantMutateMatching([traegerKeys.settings()]);
  return (
    <button
      onClick={() => {
        void refresh().catch(() => undefined);
      }}
    >
      Refresh schema
    </button>
  );
}
