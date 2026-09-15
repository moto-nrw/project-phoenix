import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("~/lib/hooks/use-media-query", () => ({
  BELOW_SM: "(max-width: 639px)",
  useMediaQuery: () => false,
}));

import { upcomingFirst, useHomeCardRows } from "./home-card-rows";

describe("upcomingFirst", () => {
  it("behält vergangene Einträge für den Restzähler und zeigt die neuesten", () => {
    const items = [
      { id: "1", endTime: "08:00" },
      { id: "2", endTime: "09:00" },
      { id: "3", endTime: "10:00" },
      { id: "4", endTime: "11:00" },
      { id: "5", endTime: "12:00" },
      { id: "6", endTime: "13:00" },
    ];

    const { items: past, allPast } = upcomingFirst(
      items,
      (item) => item.endTime,
      "16:00",
    );
    const { result } = renderHook(() =>
      useHomeCardRows(past, 3, { preferLatest: allPast }),
    );

    expect(result.current.shown.map((item) => item.id)).toEqual(["5", "6"]);
    expect(result.current.hidden).toBe(4);
  });
});
