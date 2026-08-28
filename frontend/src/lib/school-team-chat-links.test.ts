import { describe, expect, it } from "vitest";
import { schoolTeamChatDeepLink } from "./school-team-chat-links";

describe("schoolTeamChatDeepLink", () => {
  it("biegt Team-Chat-Links auf den Posteingang des Schul-Portals um", () => {
    expect(schoolTeamChatDeepLink("/team-chat/17")).toBe(
      "/school/nachrichten/17",
    );
    expect(schoolTeamChatDeepLink("/team-chat")).toBe("/school/nachrichten");
  });

  it("lässt fremde Links unangetastet", () => {
    expect(schoolTeamChatDeepLink("/school/aufsichten")).toBe(
      "/school/aufsichten",
    );
    expect(schoolTeamChatDeepLink("/team-chatter")).toBe("/team-chatter");
  });
});
