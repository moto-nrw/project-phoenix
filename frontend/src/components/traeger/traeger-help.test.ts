import { describe, expect, it } from "vitest";
import { HELP_TOPICS } from "~/lib/help-topics";
import { getTraegerHelpTopicForPath } from "./traeger-help";

describe("getTraegerHelpTopicForPath", () => {
  it("opens the article of the page Briefkopf on the Träger host and below /traeger (#4653)", () => {
    for (const path of ["/briefkopf", "/traeger/briefkopf"]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerBriefkopf,
      );
    }
  });

  it("opens the review article on the registrations of a holiday period (#4663)", () => {
    for (const path of [
      "/traeger/ferien/5/anmeldungen",
      "/ferien/5/anmeldungen/9",
    ]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerHolidayRegistrationReview,
      );
    }
  });

  it("opens the form article on the form of a holiday period (#5198)", () => {
    for (const path of ["/traeger/ferien/5/formular", "/ferien/5/formular"]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerHolidayForm,
      );
    }
  });

  it("opens the article of the contribution rules on the page Beiträge (#4664)", () => {
    for (const path of ["/beitraege", "/traeger/beitraege"]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerHolidayContribution,
      );
    }
  });

  it("opens the article of the billing runs on the page Abrechnung (#4668)", () => {
    for (const path of ["/abrechnung", "/traeger/abrechnung/5"]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerBillingRun,
      );
    }
  });

  it("keeps the settings article and the sign-in article for the other pages", () => {
    expect(getTraegerHelpTopicForPath("/einstellungen")).toBe(
      HELP_TOPICS.traegerSettings,
    );
    expect(getTraegerHelpTopicForPath("/ferien")).toBe(
      HELP_TOPICS.traegerLogin,
    );
    expect(getTraegerHelpTopicForPath("/traeger")).toBe(
      HELP_TOPICS.traegerLogin,
    );
    expect(getTraegerHelpTopicForPath("/")).toBe(HELP_TOPICS.traegerLogin);
  });
});
