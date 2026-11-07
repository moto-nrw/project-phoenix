/**
 * Legal blocks of an enrollment form template: the four standard blocks the
 * tenant's legal texts fill, the template's own consents, and the rules both
 * the builder and the save path apply. The backend validates the same rules
 * in backend/modules/enrollment/form_schema.go (FormLegalBlock.Validate).
 */
import type {
  FormLegalBlock,
  LegalBlockDisplayMode,
  PublicLegalTexts,
} from "~/lib/enrollment-form-schema-api";
import { normalizeFieldKey } from "~/lib/enrollment-form-field-rules";

export const LEGAL_BLOCK_DISPLAY_MODE_TEXT =
  "text" satisfies LegalBlockDisplayMode;
export const LEGAL_BLOCK_DISPLAY_MODE_PDF =
  "pdf" satisfies LegalBlockDisplayMode;

export const STANDARD_LEGAL_BLOCKS: FormLegalBlock[] = [
  {
    key: "agb",
    kind: "terms",
    title: "AGB / Teilnahmebedingungen",
    label:
      "Ich akzeptiere die AGB / Teilnahmebedingungen / den Ganztag Info-Brief.",
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
    label:
      "Ich habe die Datenschutzinformation der Schule zur Kenntnis genommen.",
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
    label:
      "Mein Kind darf bei Schulveranstaltungen fotografiert werden. Diese Einwilligung ist freiwillig und jederzeit mit Wirkung für die Zukunft widerrufbar.",
    text: "",
    required: false,
    enabled: false,
    sort_order: 30,
    source: "standard",
  },
  {
    key: "email_contact",
    kind: "notice",
    title: "E-Mail-Kontakt",
    label:
      "Die Schule nutzt Ihre E-Mail-Adresse für Rückfragen und Status-Benachrichtigungen zu dieser Anmeldung.",
    text: "",
    required: false,
    enabled: false,
    sort_order: 40,
    source: "standard",
  },
];

export const STANDARD_LEGAL_BLOCK_KEYS: ReadonlySet<string> = new Set(
  STANDARD_LEGAL_BLOCKS.map((block) => block.key),
);

/**
 * A standard block is marked as one, or carries a standard key: templates
 * saved before `source` existed come back without it, and the backend
 * defaults an empty source by the key (validateSource). `standardKeys` is
 * the set the caller offers; a page without standard blocks passes none.
 */
export function isStandardLegalBlock(
  block: FormLegalBlock,
  standardKeys: ReadonlySet<string> = STANDARD_LEGAL_BLOCK_KEYS,
): boolean {
  return block.source === "standard" || standardKeys.has(block.key);
}

/**
 * The kind of an own checkbox block. Consents are always voluntary
 * (Art. 7 Abs. 4 DSGVO); a checkbox parents must tick is a confirmation
 * (terms). The backend rejects consent+required.
 */
export function customCheckboxKind(required: boolean): FormLegalBlock["kind"] {
  return required ? "terms" : "consent";
}

/** The kind a block gets when it switches from notice back to a checkbox. */
export function checkboxLegalBlockKindFor(
  block: FormLegalBlock,
): FormLegalBlock["kind"] {
  if (block.key === "agb") return "terms";
  if (block.key === "data_processing") return "privacy_notice";
  return customCheckboxKind(block.required);
}

export function legalBlockDisplayMode(
  block: Pick<FormLegalBlock, "display_mode">,
): LegalBlockDisplayMode {
  return block.display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF
    ? LEGAL_BLOCK_DISPLAY_MODE_PDF
    : LEGAL_BLOCK_DISPLAY_MODE_TEXT;
}

/** The public download address of a stored legal document. */
export function publicAGBDocumentURL(storedURL: string): string {
  const globalPrefix = "/uploads/enrollment-legal-documents/";
  if (storedURL.startsWith(globalPrefix)) {
    return `/api/public/enrollment-legal-documents/${storedURL.slice(globalPrefix.length)}`;
  }
  const formPrefix = "/uploads/enrollment-form-legal-documents/";
  if (storedURL.startsWith(formPrefix)) {
    return `/api/public/enrollment-form-legal-documents/${storedURL.slice(formPrefix.length)}`;
  }
  return storedURL;
}

/** The sentence parents read instead of the AGB text when it is a PDF. */
export function agbDocumentSentence(storedURL: string): string {
  return `Die AGB / Teilnahmebedingungen sind als PDF-Datei hinterlegt: [AGB-Dokument öffnen](${publicAGBDocumentURL(storedURL)})`;
}

/** The AGB PDF of a block in PDF mode, or "" when it shows its text. */
export function agbDocumentURL(
  block: Pick<FormLegalBlock, "key" | "display_mode" | "document_url">,
): string {
  if (
    block.key !== "agb" ||
    legalBlockDisplayMode(block) !== LEGAL_BLOCK_DISPLAY_MODE_PDF
  ) {
    return "";
  }
  return (block.document_url ?? "").trim();
}

export function legalBlockPreviewText(block: FormLegalBlock): string {
  if (
    block.key === "agb" &&
    legalBlockDisplayMode(block) === LEGAL_BLOCK_DISPLAY_MODE_PDF
  ) {
    const documentURL = agbDocumentURL(block);
    return documentURL === "" ? "" : agbDocumentSentence(documentURL);
  }
  return block.text;
}

export function hasStandardLegalBlockOverride(
  block: FormLegalBlock,
  standardBlock: FormLegalBlock,
): boolean {
  return (
    block.kind !== standardBlock.kind ||
    block.title !== standardBlock.title ||
    block.label !== standardBlock.label ||
    block.text !== standardBlock.text ||
    block.required !== standardBlock.required ||
    legalBlockDisplayMode(block) !== legalBlockDisplayMode(standardBlock) ||
    (block.document_url ?? "") !== (standardBlock.document_url ?? "") ||
    JSON.stringify(block.attachments ?? []) !==
      JSON.stringify(standardBlock.attachments ?? [])
  );
}

/** The standard blocks as the tenant's legal texts configure them. */
export function mergeStandardLegalBlocks(
  legalTexts: PublicLegalTexts | null,
): FormLegalBlock[] {
  const byKey = new Map(
    (legalTexts?.blocks ?? []).map((block) => [block.key, block]),
  );
  return STANDARD_LEGAL_BLOCKS.map((standard) => {
    const configured = byKey.get(standard.key);
    const inheritedText = standardLegalBlockText(standard.key, legalTexts);
    const inheritedEnabled = standardLegalBlockEnabled(
      standard.key,
      legalTexts,
    );
    if (!configured) {
      return {
        ...standard,
        text: inheritedText,
        enabled: inheritedEnabled,
        display_mode: standardLegalBlockDisplayMode(standard.key, legalTexts),
        document_url: standardLegalBlockDocumentURL(standard.key, legalTexts),
      };
    }
    return {
      ...standard,
      kind: configured.kind,
      title: configured.title,
      label: configured.label,
      text:
        standard.key === "agb"
          ? inheritedText
          : configured.text || inheritedText,
      required: configured.required,
      enabled: inheritedEnabled,
      sort_order: configured.sort_order ?? standard.sort_order,
      source: "standard",
      display_mode: standardLegalBlockDisplayMode(standard.key, legalTexts),
      document_url: standardLegalBlockDocumentURL(standard.key, legalTexts),
    };
  });
}

function standardLegalBlockText(
  key: string,
  legalTexts: PublicLegalTexts | null,
): string {
  if (!legalTexts) return "";
  switch (key) {
    case "agb":
      return standardAGBText(legalTexts);
    case "data_processing":
      return legalTexts.dsgvo;
    case "photo":
      return legalTexts.photo;
    case "email_contact":
      return legalTexts.email_contact;
    default:
      return "";
  }
}

function standardAGBText(legalTexts: PublicLegalTexts): string {
  if (legalTexts.agb_display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF) {
    return "";
  }
  return legalTexts.agb;
}

function standardLegalBlockDisplayMode(
  key: string,
  legalTexts: PublicLegalTexts | null,
): LegalBlockDisplayMode {
  if (key === "agb" && legalTexts?.agb_display_mode === "pdf") {
    return LEGAL_BLOCK_DISPLAY_MODE_PDF;
  }
  return LEGAL_BLOCK_DISPLAY_MODE_TEXT;
}

function standardLegalBlockDocumentURL(
  key: string,
  legalTexts: PublicLegalTexts | null,
): string {
  if (key !== "agb") return "";
  if (legalTexts?.agb_display_mode !== LEGAL_BLOCK_DISPLAY_MODE_PDF) return "";
  return (legalTexts?.agb_document_url ?? "").trim();
}

function standardLegalBlockEnabled(
  key: string,
  legalTexts: PublicLegalTexts | null,
): boolean {
  if (!legalTexts) return false;
  switch (key) {
    case "agb":
      if (!legalTexts.terms_enabled) return false;
      if (
        standardLegalBlockDisplayMode(key, legalTexts) ===
        LEGAL_BLOCK_DISPLAY_MODE_PDF
      ) {
        return standardLegalBlockDocumentURL("agb", legalTexts) !== "";
      }
      return standardAGBText(legalTexts).trim() !== "";
    case "data_processing":
      return legalTexts.dsgvo_enabled && legalTexts.dsgvo.trim() !== "";
    case "photo":
      return legalTexts.photo_enabled && legalTexts.photo.trim() !== "";
    case "email_contact":
      return (
        legalTexts.email_contact_enabled &&
        legalTexts.email_contact.trim() !== ""
      );
    default:
      return false;
  }
}

/**
 * A saved template's blocks over the standard blocks: a saved standard block
 * replaces its default, own blocks follow. The backend returns [] for
 * templates from before legal blocks; those show the standard blocks.
 */
export function legalBlocksForTemplate(
  savedBlocks: FormLegalBlock[] | undefined,
  standardBlocks: FormLegalBlock[],
): FormLegalBlock[] {
  if (!savedBlocks || savedBlocks.length === 0) return standardBlocks;
  const savedByKey = new Map(savedBlocks.map((block) => [block.key, block]));
  const mergedStandardBlocks = standardBlocks.map(
    (standard) => savedByKey.get(standard.key) ?? standard,
  );
  const standardKeys = new Set(standardBlocks.map((block) => block.key));
  const customBlocks = savedBlocks.filter(
    (block) => !isStandardLegalBlock(block, standardKeys),
  );
  return [...mergedStandardBlocks, ...customBlocks];
}

export function prepareLegalBlocksForSave(
  blocks: FormLegalBlock[],
): FormLegalBlock[] {
  return blocks.map((block, index) => {
    const displayMode = legalBlockDisplayMode(block);
    const documentURL =
      displayMode === LEGAL_BLOCK_DISPLAY_MODE_PDF
        ? (block.document_url ?? "").trim()
        : "";
    const standard = isStandardLegalBlock(block);
    // Legacy own blocks saved as consent+required become terms on the next
    // save; see customCheckboxKind.
    const kind =
      !standard && block.kind === "consent"
        ? customCheckboxKind(block.required)
        : block.kind;
    return {
      key: normalizeFieldKey(block.key) || `custom_consent_${index + 1}`,
      kind,
      title: block.title.trim(),
      label: block.label.trim(),
      text: block.text.trim(),
      required:
        kind === "notice" || kind === "consent"
          ? false
          : Boolean(block.required),
      enabled: Boolean(block.enabled),
      sort_order: index * 10 + 10,
      source: standard ? "standard" : "custom",
      display_mode: displayMode,
      document_url: documentURL,
      translations: block.translations,
      attachments: block.attachments?.map((attachment) => ({
        label: attachment.label.trim(),
        document_url: attachment.document_url,
      })),
    };
  });
}

/** The comparable shape of a block list for the unsaved-changes check. */
export function legalBlocksSignature(blocks: FormLegalBlock[]): string {
  return JSON.stringify(prepareLegalBlocksForSave(blocks));
}

/** The draft documents (uploaded, not saved yet) the blocks reference. */
export function draftDocumentURLsInLegalBlocks(
  blocks: FormLegalBlock[],
  draftURLs: ReadonlySet<string>,
): Set<string> {
  const urls = new Set<string>();
  for (const block of blocks) {
    for (const attachment of block.attachments ?? []) {
      if (draftURLs.has(attachment.document_url))
        urls.add(attachment.document_url);
    }
    const documentURL = (block.document_url ?? "").trim();
    if (
      block.display_mode === LEGAL_BLOCK_DISPLAY_MODE_PDF &&
      documentURL &&
      draftURLs.has(documentURL)
    ) {
      urls.add(documentURL);
    }
  }
  return urls;
}

export function nextCustomLegalBlockKey(blocks: FormLegalBlock[]): string {
  const usedKeys = new Set(blocks.map((block) => block.key));
  let nextIndex = Math.max(
    blocks.length + 1,
    ...blocks.map((block) => {
      const match = /^custom_consent_(\d+)$/.exec(block.key);
      return match ? Number(match[1]) + 1 : 0;
    }),
  );

  while (usedKeys.has(`custom_consent_${nextIndex}`)) {
    nextIndex += 1;
  }

  return `custom_consent_${nextIndex}`;
}
