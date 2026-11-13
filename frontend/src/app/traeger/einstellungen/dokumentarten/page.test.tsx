import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { DocumentKindOverview } from "~/lib/traeger-document-kinds-api";

const { kindsApi, session } = vi.hoisted(() => ({
  kindsApi: {
    fetchDocumentKinds: vi.fn(),
    saveDocumentKind: vi.fn(),
  },
  session: { permissions: ["traeger_overview:read", "traeger_billing:manage"] },
}));

vi.mock("~/lib/traeger-document-kinds-api", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...kindsApi,
}));
vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));
vi.mock("next-auth/react", () => ({
  useSession: () => ({
    data: { user: { id: "A", permissions: session.permissions } },
    status: "authenticated",
  }),
}));

import TraegerDocumentKindsPage from "./page";

function overview(
  care: "invoice" | "fee_notice",
  legalRemedySet: boolean,
): DocumentKindOverview {
  return {
    services: [
      {
        service: "holiday",
        documentKind: "confirmation",
        choices: ["confirmation", "invoice", "fee_notice"],
        updatedAt: null,
      },
      {
        service: "care",
        documentKind: care,
        choices: ["invoice", "fee_notice"],
        updatedAt: care === "invoice" ? null : "2026-11-08T09:00:00Z",
      },
    ],
    legalRemedySet,
    feeNoticeTemplateVersion: legalRemedySet ? 2 : 0,
  };
}

function renderPage() {
  return render(
    <ToastProvider>
      <TraegerDocumentKindsPage />
    </ToastProvider>,
  );
}

describe("TraegerDocumentKindsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    session.permissions = ["traeger_overview:read", "traeger_billing:manage"];
  });

  it("shows the kinds of each service and saves a new choice", async () => {
    session.permissions.push("traeger_settings:manage");
    kindsApi.fetchDocumentKinds.mockResolvedValue(overview("invoice", false));
    kindsApi.saveDocumentKind.mockResolvedValue(overview("fee_notice", false));
    renderPage();

    const care = await screen.findByRole("radiogroup", {
      name: "Dokumentart für Monatsbeitrag",
    });
    expect(
      screen.getByRole("radiogroup", {
        name: "Dokumentart für Ferienbetreuung",
      }),
    ).toBeInTheDocument();
    // The monthly care has no confirmation letter.
    expect(care.querySelectorAll('input[type="radio"]')).toHaveLength(2);
    expect(
      screen.queryByRole("button", { name: "Speichern" }),
    ).not.toBeInTheDocument();
    expect(screen.getByText("Noch keine Rechtsbehelfsbelehrung")).toBeVisible();

    const notice = care.querySelector<HTMLInputElement>(
      'input[value="fee_notice"]',
    );
    expect(notice).not.toBeNull();
    fireEvent.click(notice!);
    fireEvent.click(screen.getByRole("button", { name: "Speichern" }));

    await waitFor(() =>
      expect(kindsApi.saveDocumentKind).toHaveBeenCalledWith(
        "care",
        "fee_notice",
      ),
    );
    // A fee notice without the legal remedy says what to do first.
    expect(
      await screen.findByText(
        "Ohne Belehrung können Sie keinen Gebührenbescheid freigeben. Tragen Sie sie zuerst in der Vorlage ein.",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Belehrung eintragen" }),
    ).toHaveAttribute(
      "href",
      expect.stringContaining("/einstellungen/briefkopf/vorlagen/bescheid"),
    );
  });

  it("explains the settings permission and offers viewing to billing-only users", async () => {
    kindsApi.fetchDocumentKinds.mockResolvedValue(
      overview("fee_notice", false),
    );
    renderPage();
    expect(
      await screen.findByText(
        "Ohne Belehrung können Sie keinen Gebührenbescheid freigeben. Bitten Sie eine Person mit der Rolle „Verwaltung“, sie einzutragen.",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Vorlage ansehen" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "Belehrung eintragen" }),
    ).not.toBeInTheDocument();
  });

  it("names the version of the legal remedy once it is written", async () => {
    kindsApi.fetchDocumentKinds.mockResolvedValue(overview("fee_notice", true));
    renderPage();
    expect(
      await screen.findByText(
        "Ihre Vorlage „Bescheid“ hat eine Rechtsbehelfsbelehrung (Version 2).",
      ),
    ).toBeVisible();
    expect(
      screen.getByRole("link", { name: "Vorlage ansehen" }),
    ).toBeInTheDocument();
  });

  it("loads nothing without the right Abrechnung", async () => {
    session.permissions = ["traeger_overview:read"];
    renderPage();
    expect(
      await screen.findByText(
        "Die Dokumentarten wählt die Rolle „Abrechnung“.",
      ),
    ).toBeVisible();
    expect(kindsApi.fetchDocumentKinds).not.toHaveBeenCalled();
  });
});
