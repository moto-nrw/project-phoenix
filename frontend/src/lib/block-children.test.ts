import { describe, expect, it } from "vitest";
import {
  blockChildrenLabel,
  blockEndLine,
  blockTimeRange,
} from "./block-children";

const counts = {
  status: "active" as const,
  expectedStudentsCount: 0,
  presentStudentsCount: 0,
  currentStudentsCount: 0,
  plannedStudentsCount: 0,
};

describe("blockChildrenLabel (#3921)", () => {
  it("zählt laufend, wer gerade da ist, gegen die eigenen Kinder des Blocks", () => {
    expect(
      blockChildrenLabel({
        ...counts,
        expectedStudentsCount: 6,
        presentStudentsCount: 12,
        currentStudentsCount: 12,
        plannedStudentsCount: 18,
      }),
    ).toBe("12 von 18 da");
  });

  it("nennt Kinder, die gegangen sind, getrennt", () => {
    expect(
      blockChildrenLabel({
        ...counts,
        presentStudentsCount: 15,
        currentStudentsCount: 12,
        plannedStudentsCount: 18,
      }),
    ).toBe("12 von 18 da · 3 gegangen");
  });

  it("zeigt bei einem Block ohne eigene Kinder nie „von 0“", () => {
    const label = blockChildrenLabel({
      ...counts,
      presentStudentsCount: 41,
      currentStudentsCount: 9,
    });
    expect(label).toBe("9 da · 32 gegangen");
    expect(label).not.toContain("von 0");
  });

  it("zeigt nur „X da“, wenn mehr Kinder da sind als geplant", () => {
    expect(
      blockChildrenLabel({
        ...counts,
        presentStudentsCount: 11,
        currentStudentsCount: 10,
        plannedStudentsCount: 6,
      }),
    ).toBe("10 da · 1 gegangen");
  });

  it("zählt beendet, wer da war, und davor, wer erwartet wird", () => {
    expect(
      blockChildrenLabel({
        ...counts,
        status: "completed",
        presentStudentsCount: 1,
      }),
    ).toBe("1 Kind");
    expect(
      blockChildrenLabel({
        ...counts,
        status: "planned",
        expectedStudentsCount: 18,
      }),
    ).toBe("18 Kinder");
  });
});

describe("Uhrzeit eines spontanen Blocks (#3921)", () => {
  const block = { startTime: "15:16", endTime: "16:16" };

  it("zeigt bei einem laufenden spontanen Block kein ausgedachtes Ende", () => {
    const running = {
      ...block,
      status: "active" as const,
      isSpontaneous: true,
    };
    expect(blockTimeRange(running)).toBe("seit 15:16");
    expect(blockEndLine(running)).toBe("Ende offen");
  });

  it("zeigt sonst Beginn und Ende", () => {
    const planned = { ...block, status: "active" as const };
    expect(blockTimeRange(planned)).toBe("15:16–16:16");
    expect(blockEndLine(planned)).toBe("bis 16:16");
    const done = {
      ...block,
      status: "completed" as const,
      isSpontaneous: true,
    };
    expect(blockEndLine(done)).toBe("bis 16:16");
  });
});
