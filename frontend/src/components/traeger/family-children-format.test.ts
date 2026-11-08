import { describe, expect, it } from "vitest";
import type { FamilyChildren } from "~/lib/traeger-family-children-api";
import { checkReason } from "./family-children-format";

const family = (
  status: FamilyChildren["status"],
  validFrom: string,
  validUntil: string | null,
): FamilyChildren => ({
  recipientFirstName: "Eva",
  recipientLastName: "Beispiel",
  recipientEmail: "eva@example.org",
  children: [],
  count: {
    id: "1",
    contractId: "31",
    statedChildren: null,
    statedAt: null,
    confirmedChildren: 3,
    validFrom,
    validUntil,
    confirmedAt: "2026-09-01T08:00:00Z",
  },
  status,
});

describe("checkReason", () => {
  it("names the end of the current confirmation", () => {
    expect(
      checkReason(family("expired", "2026-01-01", "2026-08-31"), "2026-09-09"),
    ).toBe("Die Kinderzahl galt bis 31.08.2026. Bitte neu prüfen.");
    expect(
      checkReason(family("expiring", "2026-01-01", "2026-09-30"), "2026-09-09"),
    ).toBe("Die Kinderzahl gilt nur bis 30.09.2026.");
  });

  it("names the start of a later confirmation, never its open end", () => {
    expect(
      checkReason(family("expired", "2027-01-01", null), "2026-12-10"),
    ).toBe(
      "Die bisherige Kinderzahl ist abgelaufen. Die neue gilt erst ab 01.01.2027.",
    );
    expect(
      checkReason(family("expiring", "2027-01-01", null), "2026-11-10"),
    ).toBe(
      "Die bisherige Kinderzahl läuft bald ab. Die neue gilt erst ab 01.01.2027.",
    );
  });
});
