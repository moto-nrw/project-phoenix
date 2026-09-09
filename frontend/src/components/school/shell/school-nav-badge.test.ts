import { describe, expect, it } from "vitest";

import { schoolNavBadge } from "./school-nav-badge";
import { SCHOOL_PRIMARY_NAV, SCHOOL_SECONDARY_NAV } from "./school-nav-items";

const item = (key: string) => {
  const found = [...SCHOOL_PRIMARY_NAV, ...SCHOOL_SECONDARY_NAV].find(
    (entry) => entry.key === key,
  );
  if (!found) throw new Error(`unknown nav item ${key}`);
  return found;
};

describe("schoolNavBadge", () => {
  const teamChat = { unreadCount: 4, available: true } as const;

  it("hängt den Team-Chat-Zähler an die Nachrichten", () => {
    expect(schoolNavBadge(item("messages"), { teamChat })).toEqual({
      count: 4,
      ariaLabel: "4 ungelesene Nachrichten",
    });
  });

  it("hängt offene Tagesinformationen an ihren Eintrag (#2208)", () => {
    expect(
      schoolNavBadge(item("notices"), {
        teamChat,
        notices: { pendingCount: 2 },
      }),
    ).toEqual({ count: 2, ariaLabel: "2 offene Tagesinformationen" });
  });

  it("zählt null, solange die Tagesinformationen nicht geladen sind", () => {
    expect(schoolNavBadge(item("notices"), { teamChat })).toEqual({
      count: 0,
      ariaLabel: "0 offene Tagesinformationen",
    });
  });

  it("kennt für Ziele ohne Zähler keine Zahl", () => {
    expect(schoolNavBadge(item("classDay"), { teamChat })).toBeNull();
    expect(schoolNavBadge(item("help"), { teamChat })).toBeNull();
  });
});
