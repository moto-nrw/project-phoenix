import { describe, expect, it } from "vitest";
import {
  ALL_CHILDREN_SCOPE,
  MY_GROUPS_SCOPE,
  activeChildScope,
  buildChildScopeTabs,
  roomIdFromScope,
  roomScope,
} from "./child-scope-tabs";

describe("buildChildScopeTabs (#1321)", () => {
  it("offers nothing without own groups and supervision", () => {
    expect(buildChildScopeTabs({ ownGroupIds: [], rooms: [] })).toBeNull();
  });

  it("names one group and one supervision as mine", () => {
    expect(
      buildChildScopeTabs({
        ownGroupIds: ["5"],
        rooms: [{ id: "42", name: "Werkraum" }],
      })?.map((tab) => tab.label),
    ).toEqual(["Alle", "Meine Gruppe", "Meine Aufsicht"]);
  });

  it("uses the plural for several groups and names each of several rooms", () => {
    expect(
      buildChildScopeTabs({
        ownGroupIds: ["5", "6"],
        rooms: [
          { id: "1", name: "Aula" },
          { id: "2", name: "Werkraum" },
        ],
      })?.map((tab) => tab.label),
    ).toEqual(["Alle", "Meine Gruppen", "Aufsicht Aula", "Aufsicht Werkraum"]);
  });

  it("offers the supervision alone when there is no own group", () => {
    expect(
      buildChildScopeTabs({
        ownGroupIds: [],
        rooms: [{ id: "42", name: "Werkraum" }],
      })?.map((tab) => tab.value),
    ).toEqual([ALL_CHILDREN_SCOPE, roomScope("42")]);
  });
});

describe("activeChildScope (#1321)", () => {
  const ownGroupIds = ["5", "6"];

  it("is Alle without group and room filter", () => {
    expect(
      activeChildScope({ selectedGroupIds: [], roomId: "", ownGroupIds }),
    ).toBe(ALL_CHILDREN_SCOPE);
  });

  it("is Meine Gruppen for exactly the own groups, in any order", () => {
    expect(
      activeChildScope({
        selectedGroupIds: ["6", "5"],
        roomId: "",
        ownGroupIds,
      }),
    ).toBe(MY_GROUPS_SCOPE);
  });

  it("marks no tab for another group selection", () => {
    expect(
      activeChildScope({ selectedGroupIds: ["5"], roomId: "", ownGroupIds }),
    ).toBe("");
    expect(
      activeChildScope({
        selectedGroupIds: ["7"],
        roomId: "",
        ownGroupIds: [],
      }),
    ).toBe("");
  });

  it("is the room tab for a room filter alone, none together with groups", () => {
    expect(
      activeChildScope({ selectedGroupIds: [], roomId: "42", ownGroupIds }),
    ).toBe(roomScope("42"));
    expect(
      activeChildScope({
        selectedGroupIds: ["5", "6"],
        roomId: "42",
        ownGroupIds,
      }),
    ).toBe("");
  });

  it("reads the room id back from its tab value", () => {
    expect(roomIdFromScope(roomScope("42"))).toBe("42");
    expect(roomIdFromScope(MY_GROUPS_SCOPE)).toBeNull();
  });
});
