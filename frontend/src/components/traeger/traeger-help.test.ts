import { describe, expect, it } from "vitest";
import { HELP_TOPICS } from "~/lib/help-topics";
import { getTraegerHelpTopicForPath } from "./traeger-help";

describe("getTraegerHelpTopicForPath", () => {
  it("opens the dashboard article on the dashboard (#4674)", () => {
    for (const path of ["/uebersicht", "/traeger/uebersicht"]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerDashboard,
      );
    }
  });

  it("opens the Stichtagszahlen article on its page (#4673)", () => {
    for (const path of ["/stichtagszahlen", "/traeger/stichtagszahlen"]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerKeyDateCounts,
      );
    }
  });

  it("opens the school view article on the view of a school (#5686)", () => {
    for (const path of ["/schulen/2", "/traeger/schulen/2"]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerSchoolView,
      );
    }
  });

  it("opens the article of the page Briefkopf on the Träger host and below /traeger (#4653)", () => {
    for (const path of [
      "/einstellungen/briefkopf",
      "/traeger/einstellungen/briefkopf",
    ]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerBriefkopf,
      );
    }
  });

  it.each([
    "/einstellungen/briefkopf/vorlagen",
    "/einstellungen/briefkopf/vorlagen/bestaetigung",
    "/traeger/einstellungen/briefkopf/vorlagen",
    "/traeger/einstellungen/briefkopf/vorlagen/bestaetigung",
  ])("opens the template article on %s", (path) => {
    expect(getTraegerHelpTopicForPath(path)).toBe(
      HELP_TOPICS.traegerDocumentTemplates,
    );
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

  it.each(["/traeger/ferien/5/plaetze", "/ferien/5/plaetze"])(
    "opens the holiday places and waitlist article on %s",
    (path) => {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerHolidayPlaces,
      );
    },
  );

  it("opens the article of the contribution rules on the page Abrechnungsvorgaben (#4664, #5685)", () => {
    for (const path of [
      "/einstellungen/abrechnung",
      "/traeger/einstellungen/abrechnung",
    ]) {
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

  it("opens the monthly billing article on a run and its letters (#4684)", () => {
    for (const path of [
      "/abrechnung/monatsbeitraege/5",
      "/traeger/abrechnung/monatsbeitraege/5/schreiben",
    ]) {
      expect(getTraegerHelpTopicForPath(path)).toBe(
        HELP_TOPICS.traegerCareMonths,
      );
    }
  });

  it("opens the articles of the members and the access log (#5685)", () => {
    expect(getTraegerHelpTopicForPath("/einstellungen/mitglieder")).toBe(
      HELP_TOPICS.traegerAccounts,
    );
    expect(
      getTraegerHelpTopicForPath("/traeger/einstellungen/zugriffsprotokoll"),
    ).toBe(HELP_TOPICS.traegerAccessLog);
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
