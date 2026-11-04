import { describe, expect, it } from "vitest";
import {
  emptySchoolParentLetterDraft,
  schoolParentLetterFieldErrors,
  schoolParentLetterRequest,
} from "./school-parent-letters";

const draft = {
  schoolClass: "3b",
  title: " Wandertag ",
  body: " Bitte feste Schuhe mitgeben. ",
  kind: "info" as const,
};

describe("schoolParentLetterRequest", () => {
  it("sends an information letter to exactly the chosen class", () => {
    expect(schoolParentLetterRequest(draft)).toEqual({
      title: "Wandertag",
      body: "Bitte feste Schuhe mitgeben.",
      priority: "info",
      requires_acknowledgement: false,
      send_email: true,
      email_audience: "portal_only",
      targets: [{ target_type: "class", ref_text: "3b" }],
      delivery_mode: "standard",
    });
  });

  it("asks for a read confirmation as an Elternbrief", () => {
    const request = schoolParentLetterRequest({ ...draft, kind: "letter" });
    expect(request.delivery_mode).toBe("letter");
    expect(request.requires_acknowledgement).toBe(true);
  });

  it("asks for a signed consent as an Einverständnis", () => {
    const request = schoolParentLetterRequest({ ...draft, kind: "consent" });
    expect(request).toMatchObject({
      delivery_mode: "declaration",
      declaration_kind: "consent",
      declaration_signers: "any",
      declaration_requires_signature: true,
      requires_acknowledgement: false,
    });
  });
});

describe("school parent letter draft", () => {
  it("preselects the class when the Lehrkraft has exactly one", () => {
    expect(emptySchoolParentLetterDraft(["3b"]).schoolClass).toBe("3b");
    expect(emptySchoolParentLetterDraft(["3b", "4a"]).schoolClass).toBe("");
  });

  it("names every missing field", () => {
    expect(
      schoolParentLetterFieldErrors({
        schoolClass: "",
        title: " ",
        body: "",
        kind: "info",
      }),
    ).toEqual({
      school_class: "Bitte wählen Sie eine Klasse.",
      title: "Bitte geben Sie einen Betreff ein.",
      body: "Bitte schreiben Sie einen Text.",
    });
    expect(schoolParentLetterFieldErrors(draft)).toEqual({});
  });
});
