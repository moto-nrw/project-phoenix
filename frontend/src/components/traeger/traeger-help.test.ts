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

  it("opens the review article on the registrations of a holiday period (#4663)", () => {
    for (const path of [
      "/traeger/ferien/5/anmeldungen",
      "/ferien/5/anmeldungen/9",
    ]) {
      const href = new URL(traegerHelpHref(path), "http://localhost");
      expect(href.pathname).toBe(
        `/help/${HELP_TOPICS.traegerHolidayRegistrationReview}`,
      );
      expect(href.searchParams.get("role")).toBe("traeger");
    }
    expect(
      new URL(
        traegerHelpHref("/traeger/ferien/5/anmeldungen"),
        "http://localhost",
      ).searchParams.get("return_to"),
    ).toBe("/ferien/5/anmeldungen");
  });

  it("opens the article of the contribution rules on the page Beiträge (#4664)", () => {
    for (const path of ["/beitraege", "/traeger/beitraege"]) {
      const href = new URL(traegerHelpHref(path), "http://localhost");
      expect(href.pathname).toBe(
        `/help/${HELP_TOPICS.traegerHolidayContribution}`,
      );
      expect(href.searchParams.get("return_to")).toBe("/beitraege");
    }
  });

  it("opens the article of the billing runs on the page Abrechnung (#4668)", () => {
    for (const path of ["/abrechnung", "/traeger/abrechnung/5"]) {
      const href = new URL(traegerHelpHref(path), "http://localhost");
      expect(href.pathname).toBe(`/help/${HELP_TOPICS.traegerBillingRun}`);
      expect(href.searchParams.get("return_to")).toBe("/abrechnung");
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
