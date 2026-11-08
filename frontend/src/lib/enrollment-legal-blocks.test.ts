import { describe, expect, it } from "vitest";
import type { FormLegalBlock } from "~/lib/enrollment-form-schema-api";
import {
  agbDocumentSentence,
  isStandardLegalBlock,
  legalBlocksForTemplate,
  prepareLegalBlocksForSave,
  publicAGBDocumentURL,
  STANDARD_LEGAL_BLOCKS,
} from "~/lib/enrollment-legal-blocks";

function block(overrides: Partial<FormLegalBlock>): FormLegalBlock {
  return {
    key: "custom_consent_1",
    kind: "consent",
    title: "Einwilligung",
    label: "Ich stimme zu.",
    text: "",
    required: false,
    enabled: true,
    sort_order: 10,
    ...overrides,
  };
}

describe("isStandardLegalBlock", () => {
  it("treats a block without source by its key, like the backend", () => {
    // backend/modules/enrollment/form_schema.go validateSource defaults an
    // empty source to standard for the four standard keys.
    expect(isStandardLegalBlock(block({ key: "photo" }))).toBe(true);
    expect(isStandardLegalBlock(block({ key: "custom_consent_1" }))).toBe(
      false,
    );
    expect(isStandardLegalBlock(block({ source: "standard" }))).toBe(true);
  });

  it("knows no standard key on a page without standard blocks", () => {
    expect(isStandardLegalBlock(block({ key: "photo" }), new Set())).toBe(
      false,
    );
  });
});

describe("prepareLegalBlocksForSave", () => {
  it("keeps a legacy standard block without source a standard consent", () => {
    const [saved] = prepareLegalBlocksForSave([
      block({ key: "photo", kind: "consent", required: true }),
    ]);
    expect(saved).toEqual(
      expect.objectContaining({
        key: "photo",
        kind: "consent",
        required: false,
        source: "standard",
      }),
    );
  });

  it("turns an own required consent into a confirmation", () => {
    const [saved] = prepareLegalBlocksForSave([
      block({ kind: "consent", required: true, source: "custom" }),
    ]);
    expect(saved).toEqual(
      expect.objectContaining({
        kind: "terms",
        required: true,
        source: "custom",
      }),
    );
  });
});

describe("legalBlocksForTemplate", () => {
  it("shows the standard blocks for a template without legal blocks", () => {
    expect(legalBlocksForTemplate([], STANDARD_LEGAL_BLOCKS)).toBe(
      STANDARD_LEGAL_BLOCKS,
    );
  });

  it("puts a legacy standard block in its standard place, not twice", () => {
    const legacyPhoto = block({ key: "photo", title: "Alt" });
    const merged = legalBlocksForTemplate(
      [legacyPhoto, block({ key: "custom_consent_1" })],
      STANDARD_LEGAL_BLOCKS,
    );
    expect(merged.map((entry) => entry.key)).toEqual([
      "agb",
      "data_processing",
      "photo",
      "email_contact",
      "custom_consent_1",
    ]);
    expect(merged[2]).toBe(legacyPhoto);
  });
});

describe("publicAGBDocumentURL", () => {
  it("maps stored uploads to their public routes", () => {
    expect(
      publicAGBDocumentURL("/uploads/enrollment-legal-documents/1_agb.pdf"),
    ).toBe("/api/public/enrollment-legal-documents/1_agb.pdf");
    expect(
      publicAGBDocumentURL(
        "/uploads/enrollment-form-legal-documents/1_terms.pdf",
      ),
    ).toBe("/api/public/enrollment-form-legal-documents/1_terms.pdf");
    expect(publicAGBDocumentURL("https://example.org/agb.pdf")).toBe(
      "https://example.org/agb.pdf",
    );
  });

  it("links the public route in the AGB sentence", () => {
    expect(
      agbDocumentSentence("/uploads/enrollment-legal-documents/1_agb.pdf"),
    ).toBe(
      "Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](/api/public/enrollment-legal-documents/1_agb.pdf)",
    );
  });
});
