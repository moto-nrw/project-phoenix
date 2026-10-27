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
import deMessages from "~/i18n/messages/de.json";
import { ApiError } from "~/lib/api-error";
import type { ParentDocument } from "~/lib/parent-api";
import { catalogText } from "~/test/error-catalog-text";

function render(ui: React.ReactElement) {
  return renderPlain(ui, { wrapper: ToastProvider });
}

const mocks = vi.hoisted(() => ({
  listMyDocuments: vi.fn(),
  downloadMyDocument: vi.fn(),
}));

vi.mock("~/lib/parent-api", () => ({
  listMyDocuments: mocks.listMyDocuments,
  downloadMyDocument: mocks.downloadMyDocument,
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

  it("shows each sent letter with its children, amount and sender", async () => {
    mocks.listMyDocuments.mockResolvedValue([letter]);

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

  it("downloads the stored PDF of a letter", async () => {
    mocks.listMyDocuments.mockResolvedValue([letter]);
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
    mocks.listMyDocuments.mockResolvedValue([letter]);
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

  it("explains an empty inbox and how letters arrive", async () => {
    mocks.listMyDocuments.mockResolvedValue([]);

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
