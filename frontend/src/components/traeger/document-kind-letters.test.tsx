import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { ToastProvider } from "~/contexts/ToastContext";
import type { DocumentTemplatePreview } from "~/lib/traeger-document-templates";

vi.mock("~/lib/traeger-url", () => ({
  traegerPath: (path: string) => path.replace(/^\/traeger/, "") || "/",
}));

import { BillingLettersCard } from "./billing-letters-card";
import { DocumentTemplatePreviewBody } from "./document-template-preview-body";

// The letters of a released holiday run name the kind of document its
// release froze and lead to the template the letters are written with
// (#4688).
describe("BillingLettersCard by document kind", () => {
  it.each([
    [undefined, "Bestätigungsschreiben", "/vorlagen/bestaetigung"],
    ["invoice", "Rechnungen", "/vorlagen/rechnung"],
    ["fee_notice", "Gebührenbescheide", "/vorlagen/bescheid"],
  ])("%s: %s with its template", (documentKind, title, template) => {
    render(
      <ToastProvider>
        <BillingLettersCard
          runId="2"
          payments={[]}
          letters={null}
          onCreated={() => undefined}
          documentKind={documentKind}
        />
      </ToastProvider>,
    );
    expect(screen.getByRole("heading", { name: title })).toBeVisible();
    expect(screen.getByRole("link", { name: "Vorlage" })).toHaveAttribute(
      "href",
      expect.stringContaining(template),
    );
  });
});

// The preview prints every part of the legal remedy the PDF prints,
// a table too (#4688).
describe("DocumentTemplatePreviewBody legal remedy", () => {
  it("prints the heading, the text and a table of the legal remedy", () => {
    const preview: DocumentTemplatePreview = {
      subject: "Gebührenbescheid GB-1",
      salutation: "",
      blocks: [],
      closing: [],
      legalRemedy: [
        {
          heading: "",
          text: "Widerspruch binnen eines Monats.",
          bold: false,
          rows: [],
        },
        {
          heading: "",
          text: "",
          bold: false,
          rows: [{ label: "Gesamtbetrag", value: "92,50 €", bold: true }],
        },
      ],
    };
    render(<DocumentTemplatePreviewBody preview={preview} />);
    expect(screen.getByText("Rechtsbehelfsbelehrung")).toBeVisible();
    expect(screen.getByText("Widerspruch binnen eines Monats.")).toBeVisible();
    expect(screen.getByText("Gesamtbetrag")).toBeVisible();
    expect(screen.getByText("92,50 €")).toBeVisible();
  });
});
