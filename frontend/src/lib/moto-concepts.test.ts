import { describe, expect, it } from "vitest";
import { MOTO_DUOTONE_TONES } from "~/components/ui/moto-duotone-icon";
import { MOTO_CONCEPTS } from "./moto-concepts";

describe("MOTO_CONCEPTS", () => {
  it.each(["core", "status"] as const)(
    "keeps every %s concept on a unique primary color",
    (kind) => {
      const concepts = Object.values(MOTO_CONCEPTS).filter(
        (concept) => concept.kind === kind,
      );
      const colors = concepts.map(
        (concept) => MOTO_DUOTONE_TONES[concept.tone].primary,
      );

      expect(new Set(colors).size).toBe(colors.length);
    },
  );

  it("keeps every concept visually distinct via icon and tone", () => {
    const renderings = Object.values(MOTO_CONCEPTS).map((concept) =>
      [concept.icon.displayName, concept.tone].join("/"),
    );

    expect(new Set(renderings).size).toBe(renderings.length);
  });

  it("assigns distinct semantic tones to dashboard concepts", () => {
    expect(MOTO_CONCEPTS.dashboard.tone).toBe("neutral");
    expect(MOTO_CONCEPTS.children.tone).toBe("greenVivid");
    expect(MOTO_CONCEPTS.rooms.tone).toBe("navy");
    expect(MOTO_CONCEPTS.parents.tone).toBe("blue");
    expect(MOTO_CONCEPTS.timeTracking.tone).toBe("timeTracking");
    expect(MOTO_CONCEPTS.feedback.tone).toBe("coral");
    expect(MOTO_CONCEPTS.excused.tone).toBe("purple");
    expect(MOTO_CONCEPTS.schoolyard.tone).toBe("orange");
    expect(MOTO_CONCEPTS.utilization.tone).toBe("gold");
  });

  it("renders passkeys as a clean outline without a duotone fill", () => {
    expect(MOTO_CONCEPTS.passkeys.weight).toBe("regular");
  });
});
