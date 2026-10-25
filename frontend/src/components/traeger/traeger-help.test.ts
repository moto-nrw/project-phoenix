import { describe, expect, it } from "vitest";
import { HELP_TOPICS } from "~/lib/help-topics";
import { traegerHelpHref } from "./traeger-help";

describe("traegerHelpHref", () => {
  it("opens the article of the page Briefkopf on the Träger host and below /traeger (#4653)", () => {
    for (const path of ["/briefkopf", "/traeger/briefkopf"]) {
      const href = new URL(traegerHelpHref(path), "http://localhost");
      expect(href.pathname).toBe(`/help/${HELP_TOPICS.traegerBriefkopf}`);
      expect(href.searchParams.get("return_to")).toBe("/briefkopf");
      expect(href.searchParams.get("role")).toBe("traeger");
    }
  });

  it("keeps the settings article and the sign-in article for the other pages", () => {
    expect(traegerHelpHref("/einstellungen")).toContain(
      `/help/${HELP_TOPICS.traegerSettings}`,
    );
    expect(traegerHelpHref("/ferien")).toContain(
      `/help/${HELP_TOPICS.traegerLogin}`,
    );
  });
});
