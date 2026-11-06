import { describe, expect, it } from "vitest";
import type { ContractVersion } from "~/lib/traeger-care-contracts-api";
import {
  changedWeekdays,
  plannedVersion,
  versionPhase,
} from "./care-contract-versions";

function version(overrides: Partial<ContractVersion>): ContractVersion {
  return {
    number: 1,
    status: "wirksam",
    source: "abschluss",
    effectiveFrom: "2026-08-01",
    endsOn: null,
    validUntil: null,
    services: [],
    createdAt: "2026-07-20T08:00:00Z",
    ...overrides,
  };
}

describe("versionPhase", () => {
  it("places each version on the day by its dates, not by its status", () => {
    const today = "2026-09-09";
    expect(
      versionPhase(
        version({ status: "abgeloest", validUntil: "2027-02-28" }),
        today,
      ),
    ).toBe("current");
    expect(
      versionPhase(
        version({ status: "abgeloest", validUntil: "2026-08-31" }),
        today,
      ),
    ).toBe("past");
    expect(versionPhase(version({ effectiveFrom: "2027-03-01" }), today)).toBe(
      "planned",
    );
    expect(versionPhase(version({ status: "beantragt" }), today)).toBe(
      "pending",
    );
    expect(versionPhase(version({ status: "abgelehnt" }), today)).toBe(
      "rejected",
    );
  });

  it("names the next planned version", () => {
    const versions = [
      version({ number: 1, status: "abgeloest", validUntil: "2027-02-28" }),
      version({ number: 2, effectiveFrom: "2027-03-01" }),
    ];
    expect(plannedVersion(versions, "2026-09-09")?.number).toBe(2);
    expect(plannedVersion(versions, "2027-03-01")).toBeUndefined();
  });
});

describe("changedWeekdays", () => {
  it("marks a day with another module, another end or care added", () => {
    const monday = {
      weekday: "mon" as const,
      careOfferingId: "5",
      moduleName: "Ganztag",
      endsAt: "14:00",
      lastDay: null,
    };
    const before = version({ services: [monday] });
    const after = version({
      number: 2,
      services: [
        { ...monday, endsAt: "16:30" },
        { ...monday, weekday: "fri" },
      ],
    });
    expect([...changedWeekdays(after, before)]).toEqual(["mon", "fri"]);
    expect(changedWeekdays(before, undefined).size).toBe(0);
    expect(changedWeekdays(before, before).size).toBe(0);
  });
});
