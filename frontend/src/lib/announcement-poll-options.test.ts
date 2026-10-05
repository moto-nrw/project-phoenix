import { describe, expect, it } from "vitest";

import {
  MAX_POLL_OPTION_LENGTH,
  MAX_POLL_OPTIONS,
  insertPastedOptions,
  splitPastedOptions,
} from "./announcement-poll-options";

describe("splitPastedOptions", () => {
  it("returns one trimmed answer per non-blank line", () => {
    expect(
      splitPastedOptions("  Di 14.10. 15:00 \r\n\nDi 14.10. 15:15\rMi 15.10."),
    ).toEqual(["Di 14.10. 15:00", "Di 14.10. 15:15", "Mi 15.10."]);
  });

  it("joins spreadsheet columns with a space", () => {
    expect(splitPastedOptions("Di 14.10.\t15:00\nDi 14.10.\t\t15:15")).toEqual([
      "Di 14.10. 15:00",
      "Di 14.10. 15:15",
    ]);
  });
});

describe("insertPastedOptions", () => {
  it("leaves single-line text to the browser", () => {
    expect(insertPastedOptions(["Ja", ""], 1, "Vielleicht")).toBeNull();
    expect(insertPastedOptions(["Ja", ""], 1, "Vielleicht\n\n")).toBeNull();
  });

  it("fills an empty row and inserts the rest below it", () => {
    expect(insertPastedOptions(["Ja", "", "Nein"], 1, "A\nB\nC")).toEqual({
      rows: ["Ja", "A", "B", "C", "Nein"],
      dropped: 0,
      tooLong: 0,
    });
  });

  it("keeps a filled row and inserts the lines after it", () => {
    expect(insertPastedOptions(["Ja", "Nein"], 0, "A\nB")).toEqual({
      rows: ["Ja", "A", "B", "Nein"],
      dropped: 0,
      tooLong: 0,
    });
  });

  it("drops the lines that exceed the limit and counts them", () => {
    const lines = Array.from(
      { length: MAX_POLL_OPTIONS + 3 },
      (_, i) => `Termin ${i + 1}`,
    );
    const result = insertPastedOptions(["Ja", ""], 1, lines.join("\n"));
    expect(result?.rows).toHaveLength(MAX_POLL_OPTIONS);
    expect(result?.rows[0]).toBe("Ja");
    expect(result?.rows.at(-1)).toBe(`Termin ${MAX_POLL_OPTIONS - 1}`);
    expect(result?.dropped).toBe(4);
    expect(result?.tooLong).toBe(0);
  });

  it("does not count blank rows against the limit", () => {
    const lines = Array.from(
      { length: MAX_POLL_OPTIONS },
      (_, i) => `Termin ${i + 1}`,
    );
    const result = insertPastedOptions(["", "", ""], 0, lines.join("\n"));
    expect(result?.dropped).toBe(0);
    expect(result?.tooLong).toBe(0);
    expect(result?.rows.filter((row) => row.trim())).toHaveLength(
      MAX_POLL_OPTIONS,
    );
  });

  it("drops overlong lines without using space needed by valid answers", () => {
    const result = insertPastedOptions(
      ["Ja", ""],
      1,
      `${"A".repeat(MAX_POLL_OPTION_LENGTH + 1)}\nNein\nVielleicht`,
    );

    expect(result).toEqual({
      rows: ["Ja", "Nein", "Vielleicht"],
      dropped: 0,
      tooLong: 1,
    });
  });
});
