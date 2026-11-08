import {
  fireEvent,
  render as renderPlain,
  screen,
  waitFor,
} from "@testing-library/react";
import type React from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import deMessages from "~/i18n/messages/de.generated";
import { ApiError } from "~/lib/api-error";
import type { ParentDocument } from "~/lib/parent-api";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  listMyDocuments: vi.fn(),
  downloadMyDocument: vi.fn(),
  downloadMySharedDocument: vi.fn(),
}));

vi.mock("~/lib/parent-api", () => ({
  listMyDocuments: mocks.listMyDocuments,
  downloadMyDocument: mocks.downloadMyDocument,
  downloadMySharedDocument: mocks.downloadMySharedDocument,
}));

import { ParentDocumentsPage } from "./parent-documents-page";

const letter: ParentDocument = {
  id: "31",
  organization_id: "4",
  traeger_name: "Ganztag im Quartier e.V.",
  kind: "confirmation_letter",
  number: "FE-0007",
  period_name: "Herbstferien 2026",
  service_from: "2026-10-12",
  service_until: "2026-10-16",
  issued_on: "2026-10-01",
  total_cents: 4500,
  child_names: ["Felix", "Mia"],
  sent_at: "2026-10-02T09:30:00Z",
};

describe("ParentDocumentsPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("shows a document the Träger released for the parents (#4692)", async () => {
    const shared = {
      id: "9",
      organization_id: "4",
      traeger_name: "Ganztag im Quartier e.V.",
      title: "Hausordnung 2026/27",
      file_name: "hausordnung.pdf",
      content_type: "application/pdf",
      size_bytes: 2048,
      released_at: "2026-10-05T09:30:00Z",
    };
    mocks.listMyDocuments.mockResolvedValue({
      documents: [letter],
      shared: [shared],
      failedSource: null,
      error: null,
    });

    render(<ParentDocumentsPage />);

    const headings = await screen.findAllByRole("heading", { level: 2 });
    expect(headings.map((heading) => heading.textContent)).toEqual([
      "Hausordnung 2026/27",
      "Bestätigungsschreiben Herbstferien 2026",
    ]);
    expect(
      screen.getByText(
        "Vom Träger für Eltern freigegeben: Ganztag im Quartier e.V.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("PDF · 2 KB")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));
    await waitFor(() =>
      expect(mocks.downloadMySharedDocument).toHaveBeenCalledWith(shared),
    );
  });

  it("shows each sent letter with its children, amount and sender", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [letter],
      shared: [],
      failedSource: null,
      error: null,
    });

    render(<ParentDocumentsPage />);

    expect(
      await screen.findByRole("heading", {
        name: "Bestätigungsschreiben Herbstferien 2026",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Von Ganztag im Quartier e.V."),
    ).toBeInTheDocument();
    expect(screen.getByText("Felix, Mia")).toBeInTheDocument();
    expect(screen.getByText(/45,00\s€/)).toBeInTheDocument();
    expect(screen.getByText("FE-0007")).toBeInTheDocument();
    expect(screen.getByText("02.10.2026")).toBeInTheDocument();
  });

  it("names the letter of a Storno and a Gutschrift (#4690)", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [
        { ...letter, id: "8", kind: "cancellation_letter", number: "FE-0008" },
        { ...letter, id: "9", kind: "credit_letter", number: "FE-0009" },
      ],
      shared: [],
      failedSource: null,
      error: null,
    });

    render(<ParentDocumentsPage />);

    expect(
      await screen.findByRole("heading", { name: "Storno Herbstferien 2026" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Gutschrift Herbstferien 2026" }),
    ).toBeInTheDocument();
  });

  it("names the letter of a monthly invoice and of its Storno (#4684)", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [
        {
          ...letter,
          id: "10",
          kind: "care_invoice_letter",
          number: "MB-2027-0001",
          period_name: "Monatsbeitrag Januar 2027",
        },
        {
          ...letter,
          id: "11",
          kind: "care_cancellation_letter",
          number: "MB-2027-0009",
          period_name: "Monatsbeitrag Januar 2027",
        },
      ],
      shared: [],
      failedSource: null,
      error: null,
    });

    render(<ParentDocumentsPage />);

    expect(
      await screen.findByRole("heading", {
        name: "Rechnung Monatsbeitrag Januar 2027",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("heading", { name: "Storno Monatsbeitrag Januar 2027" }),
    ).toBeInTheDocument();
  });

  it("downloads the stored PDF of a letter", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [letter],
      shared: [],
      failedSource: null,
      error: null,
    });
    mocks.downloadMyDocument.mockResolvedValue(undefined);

    render(<ParentDocumentsPage />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Als PDF laden" }),
    );

    await waitFor(() =>
      expect(mocks.downloadMyDocument).toHaveBeenCalledWith(letter),
    );
  });

  it("names a failed download in a toast", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [letter],
      shared: [],
      failedSource: null,
      error: null,
    });
    mocks.downloadMyDocument.mockRejectedValue(
      new ApiError("diag", 404, {
        code: "parent_billing.parent_document_not_found",
      }),
    );

    render(<ParentDocumentsPage />);
    fireEvent.click(
      await screen.findByRole("button", { name: "Als PDF laden" }),
    );

    expect(
      await screen.findByText(
        deMessages.errorCatalog.codes.parent_billing.parent_document_not_found,
      ),
    ).toBeInTheDocument();
  });

  it("keeps a loaded letter visible and downloadable beside the failed contract source", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [letter],
      shared: [],
      failedSource: "contracts",
      error: new ApiError("diag", 500, { code: "general.internal_error" }),
    });
    render(<ParentDocumentsPage />);
    expect(
      await screen.findByRole("heading", {
        name: "Bestätigungsschreiben Herbstferien 2026",
      }),
    ).toBeInTheDocument();
    expect(await screen.findByRole("alert")).toHaveTextContent(
      /Betreuungsverträge/,
    );
    fireEvent.click(screen.getByRole("button", { name: "Als PDF laden" }));
    await waitFor(() =>
      expect(mocks.downloadMyDocument).toHaveBeenCalledWith(letter),
    );
  });

  it("names the documents as a whole when several sources fail (#4692 review)", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [],
      shared: [],
      failedSource: "several",
      error: new ApiError("diag", 500, { code: "general.internal_error" }),
    });
    render(<ParentDocumentsPage />);
    const alert = await screen.findByRole("alert");
    expect(alert).toHaveTextContent(/Die Dokumente/);
    expect(alert).not.toHaveTextContent(/Briefe|Betreuungsverträge/);
  });

  it("keeps letters and contracts with identical IDs as distinct rows", async () => {
    const errors = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      mocks.listMyDocuments.mockResolvedValue({
        documents: [letter, { ...letter, kind: "care_contract" }],
        shared: [],
        failedSource: null,
        error: null,
      });
      render(<ParentDocumentsPage />);
      expect(
        await screen.findByRole("heading", { name: "Betreuungsvertrag" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("heading", {
          name: "Bestätigungsschreiben Herbstferien 2026",
        }),
      ).toBeInTheDocument();
      expect(errors).not.toHaveBeenCalled();
    } finally {
      errors.mockRestore();
    }
  });

  it.each([true, false])(
    "offers signing only with per-contract eligibility: %s",
    async (canSign) => {
      const contract: ParentDocument = {
        ...letter,
        kind: "care_contract",
        contract_status: "versendet",
        can_sign: canSign,
      };
      mocks.listMyDocuments.mockResolvedValue({
        documents: [contract],
        shared: [],
        failedSource: null,
        error: null,
      });
      render(<ParentDocumentsPage />);
      expect(
        await screen.findByRole("heading", { name: "Betreuungsvertrag" }),
      ).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Als PDF laden" }),
      ).toBeInTheDocument();
      if (canSign) {
        expect(
          screen.getByRole("link", {
            name: deMessages.parentDocuments.signNow,
          }),
        ).toBeInTheDocument();
      } else {
        expect(
          screen.queryByRole("link", {
            name: deMessages.parentDocuments.signNow,
          }),
        ).not.toBeInTheDocument();
      }
    },
  );

  it("distinguishes a concluded contract without a signature", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [
        {
          ...letter,
          kind: "care_contract",
          contract_status: "aktiv",
          signed_at: null,
          can_sign: false,
        },
      ],
      shared: [],
      failedSource: null,
      error: null,
    });
    render(<ParentDocumentsPage />);
    expect(
      await screen.findByRole("heading", { name: "Betreuungsvertrag" }),
    ).toBeInTheDocument();
    expect(screen.getByText("Abgeschlossen")).toBeInTheDocument();
    expect(screen.queryByText("Unterschrieben")).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: deMessages.parentDocuments.signNow }),
    ).not.toBeInTheDocument();
  });

  it("marks a replaced contract version and names from when the new version applies (#4681)", async () => {
    const contract: ParentDocument = {
      ...letter,
      id: "74",
      kind: "care_contract",
      number: "Nr. 74",
      period_name: "OGS Ganztag · OGS Sonnenhof",
      service_from: "2027-03-01",
      total_cents: 0,
      child_names: ["Lina"],
    };
    mocks.listMyDocuments.mockResolvedValue({
      documents: [
        {
          ...contract,
          contract_status: "aktiv",
          replaced: true,
          can_end_contract: true,
        },
        {
          ...contract,
          id: "75",
          contract_status: "aktiv",
          replaced: false,
          can_end_contract: false,
        },
      ],
      shared: [],
      failedSource: null,
      error: null,
    });
    render(<ParentDocumentsPage />);

    expect(
      await screen.findAllByRole("heading", { name: "Betreuungsvertrag" }),
    ).toHaveLength(2);
    expect(screen.getByText("Ersetzt")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Diese Fassung gilt nicht mehr. Ab 01.03.2027 gilt eine neue Fassung. Sie enthält Ihre bestätigten Änderungen. Die neue Fassung bekommen Sie vom Träger.",
      ),
    ).toBeInTheDocument();
    // The button only where the relationship may end the contract (#4682);
    // the other contract says why it has none.
    expect(
      screen.getAllByRole("link", { name: "Verträge hier kündigen" }),
    ).toHaveLength(1);
    expect(
      screen.getByText(deMessages.parentDocuments.cancelNotPermitted),
    ).toBeInTheDocument();
    // Only the replaced version carries the mark.
    expect(screen.getAllByText("Ersetzt")).toHaveLength(1);
  });

  it("explains an empty inbox and how letters arrive", async () => {
    mocks.listMyDocuments.mockResolvedValue({
      documents: [],
      shared: [],
      failedSource: null,
      error: null,
    });

    render(<ParentDocumentsPage />);

    expect(
      await screen.findByText(deMessages.parentDocuments.emptyTitle),
    ).toBeInTheDocument();
    expect(
      screen.getByText(deMessages.parentDocuments.emptyDescription),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Als PDF laden" }),
    ).not.toBeInTheDocument();
  });

  it("tells an account without the right to ask the school", async () => {
    mocks.listMyDocuments.mockRejectedValue(
      new ApiError("diag", 403, { code: "care.guardian_permission_denied" }),
    );

    render(<ParentDocumentsPage />);

    expect(
      await screen.findByText(
        catalogText(
          "care.guardian_permission_denied",
          deMessages.parentDocuments.errorObject,
        ),
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByText(deMessages.parentDocuments.emptyTitle),
    ).not.toBeInTheDocument();
  });
});
