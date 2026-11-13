import { describe, expect, it } from "vitest";
import type { TraegerDocumentRelease } from "~/lib/traeger-documents-api";
import {
  documentFileFacts,
  releaseAudienceLabel,
  revokeConsequence,
  schoolNamesOf,
  schoolReleaseAudienceLabel,
} from "./traeger-document-format";

const release = (
  overrides: Partial<TraegerDocumentRelease>,
): TraegerDocumentRelease => ({
  id: "1",
  audience: "parents",
  schoolId: null,
  groupId: null,
  groupName: "",
  releasedAt: "2026-09-09T10:00:00Z",
  ...overrides,
});

const schools = schoolNamesOf([{ id: "7", name: "OGS Nord" }]);

describe("releaseAudienceLabel", () => {
  it("names the people who see a release", () => {
    expect(releaseAudienceLabel(release({}), schools)).toBe(
      "Eltern an allen Schulen",
    );
    expect(releaseAudienceLabel(release({ schoolId: "7" }), schools)).toBe(
      "Eltern: OGS Nord",
    );
    expect(
      releaseAudienceLabel(
        release({ schoolId: "7", groupId: "3", groupName: "Sonnenblumen" }),
        schools,
      ),
    ).toBe("Eltern der Gruppe „Sonnenblumen“ (OGS Nord)");
    expect(
      releaseAudienceLabel(release({ audience: "schools" }), schools),
    ).toBe("OGS-Leitung aller Schulen");
    expect(
      releaseAudienceLabel(
        release({ audience: "schools", schoolId: "7" }),
        schools,
      ),
    ).toBe("OGS-Leitung: OGS Nord");
  });
});

describe("schoolReleaseAudienceLabel", () => {
  it("speaks from the school's view", () => {
    expect(schoolReleaseAudienceLabel(release({ schoolId: "7" }))).toBe(
      "Eltern Ihrer Schule",
    );
    expect(schoolReleaseAudienceLabel(release({ audience: "schools" }))).toBe(
      "Alle Schulen des Trägers",
    );
    expect(
      schoolReleaseAudienceLabel(
        release({ schoolId: "7", groupId: "3", groupName: "Sonnenblumen" }),
      ),
    ).toBe("Eltern der Gruppe „Sonnenblumen“");
  });
});

describe("documentFileFacts", () => {
  it("names the kind and the size", () => {
    expect(
      documentFileFacts({ contentType: "application/pdf", sizeBytes: 2048 }),
    ).toBe("PDF · 2 KB");
    expect(
      documentFileFacts({
        contentType:
          "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
        sizeBytes: 3 * 1024 * 1024,
      }),
    ).toBe("Excel · 3.0 MB");
  });
});

// Revoking one release ends only that one: who another active release
// covers keeps the document, and the confirmation says so (#4692 review).
describe("revokeConsequence", () => {
  const group = release({
    id: "2",
    schoolId: "7",
    groupId: "3",
    groupName: "Bären",
  });

  it("promises the end of access only when no other release remains", () => {
    expect(revokeConsequence(group, [group], schools, "Hausordnung")).toBe(
      "Eltern der Gruppe „Bären“ (OGS Nord) sehen „Hausordnung“ danach nicht mehr.",
    );
  });

  it("names that other releases keep their access", () => {
    const all = release({ id: "1" });
    expect(revokeConsequence(group, [all, group], schools, "Hausordnung")).toBe(
      "Die Freigabe für Eltern der Gruppe „Bären“ (OGS Nord) endet. Andere Freigaben dieses Dokuments bleiben: Wer darüber Zugang hat, sieht „Hausordnung“ weiter.",
    );
  });
});
