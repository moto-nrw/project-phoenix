import { readdirSync } from "node:fs";
import { relative, resolve, sep } from "node:path";
import { describe, expect, it } from "vitest";

import { getPrototypeTopics } from "~/components/help/prototype/prototype-data";
import { getHelpTopicForPath, HELP_TOPICS } from "./help-topics";

const PROTECTED_PAGES_ROOT = resolve(
  process.cwd(),
  "src/app/[tenant]/(protected)",
);

function collectPageFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name);
    if (entry.isDirectory()) return collectPageFiles(path);
    return entry.name === "page.tsx" ? [path] : [];
  });
}

function routePatternForPage(file: string): string {
  const pagePath = relative(PROTECTED_PAGES_ROOT, file).split(sep).join("/");
  const route = pagePath.slice(0, -"/page.tsx".length);
  return route ? `/${route}` : "/";
}

function examplePathForRoute(route: string): string {
  return route.replaceAll(/\[[^\]]+\]/g, "example");
}

/**
 * Pages without a matching prototype article yet. This list is shrink-only:
 * mapping a page requires removing it here, and adding an unmapped page fails.
 */
const UNMAPPED_HELP_PAGE_BASELINE: readonly string[] = [
  "/admin/change-requests",
  "/admin/guardian-approvals",
  "/betreuungsplan",
  "/calendar-periods",
  "/dashboard",
  "/dienstplan",
  "/eltern",
  "/eltern/bankverbindungen",
  "/info-displays",
  "/invitations",
  "/lists",
  "/meal-plan",
  "/parent-announcements",
  "/payroll",
  "/planung",
  "/profile",
  "/reminders",
  "/staff/[id]",
  "/staff/dienstplan",
  "/statistics",
  "/substitutions",
  "/timetables",
  "/vertretung",
  "/vertretungsplan",
];

describe("getHelpTopicForPath", () => {
  it.each([
    ["/students/search", HELP_TOPICS.studentSearch],
    ["/students/42", HELP_TOPICS.studentSearch],
    ["/students/42/room-history", HELP_TOPICS.studentSearch],
    ["/ogs-groups", HELP_TOPICS.ownGroups],
    ["/active-supervisions", HELP_TOPICS.activeSupervision],
    ["/absences", HELP_TOPICS.absences],
    ["/day-log", HELP_TOPICS.dayLog],
    ["/emergency", HELP_TOPICS.emergency],
    ["/database", HELP_TOPICS.dataManagement],
    ["/database/students/import", HELP_TOPICS.dataManagement],
    ["/admin/enrollments", HELP_TOPICS.enrollments],
    ["/admin/enrollments/42", HELP_TOPICS.enrollments],
    ["/enrollment-phases/42/review", HELP_TOPICS.enrollments],
    ["/care-offerings", HELP_TOPICS.enrollments],
    ["/enrollment-form", HELP_TOPICS.enrollments],
    ["/settings", HELP_TOPICS.settings],
    ["/calendar", HELP_TOPICS.mySchedule],
    ["/rooms", HELP_TOPICS.rooms],
    ["/rooms/42", HELP_TOPICS.rooms],
    ["/activities", HELP_TOPICS.manageActivity],
    ["/messages", HELP_TOPICS.parentMessage],
    ["/messages/42", HELP_TOPICS.parentMessage],
    ["/anfragen", HELP_TOPICS.parentRequests],
    ["/team-chat/42", HELP_TOPICS.teamChat],
    ["/staff", HELP_TOPICS.findStaff],
    ["/dateien", HELP_TOPICS.sharedFiles],
    ["/time-tracking", HELP_TOPICS.trackWorkTime],
  ])("maps %s to %s", (pathname, expectedTopic) => {
    expect(getHelpTopicForPath(pathname)).toBe(expectedTopic);
  });

  it("does not send an unrelated page to a generic help topic", () => {
    expect(getHelpTopicForPath("/dashboard")).toBeNull();
  });

  it("resolves every registered help topic in both prototype variants", () => {
    const expectedTopics = Object.values(HELP_TOPICS).sort();

    for (const presenceMode of ["detailed", "binary"] as const) {
      expect(
        getPrototypeTopics(presenceMode)
          .map((topic) => topic.id)
          .sort(),
      ).toEqual(expectedTopics);
    }
  });

  it("keeps the finished articles in the agreed caregiver order", () => {
    const caregiverTopics = getPrototypeTopics("detailed").filter(
      (topic) => topic.audience === "all" || topic.audience === "caregiver",
    );

    expect(caregiverTopics[0]).toMatchObject({
      id: HELP_TOPICS.acceptInvitation,
      group: "einstieg",
    });
    expect(caregiverTopics[0]?.status).toBeUndefined();
    expect(caregiverTopics[0]?.steps).toHaveLength(8);
    expect(caregiverTopics[1]).toMatchObject({
      id: HELP_TOPICS.login,
      group: "einstieg",
    });
    expect(caregiverTopics[1]?.status).toBeUndefined();
    expect(caregiverTopics[1]?.steps).toHaveLength(6);
    expect(caregiverTopics[2]).toMatchObject({
      id: HELP_TOPICS.installApp,
      group: "einstieg",
    });
    expect(caregiverTopics[2]?.status).toBeUndefined();
    expect(caregiverTopics[2]?.instructionGroups).toHaveLength(2);
    expect(caregiverTopics[2]?.differences).toContain(
      "`Unsichere App blockiert` erscheint? moto selbst ist nicht unsicher. Aktualisieren Sie im Play Store Chrome und Google Play. Starten Sie das Gerät neu. Versuchen Sie es danach erneut.",
    );
    expect(caregiverTopics[3]).toMatchObject({
      id: HELP_TOPICS.appOverview,
      group: "einstieg",
    });
    expect(caregiverTopics[3]?.instructionGroups).toHaveLength(3);
    expect(caregiverTopics[4]).toMatchObject({
      id: HELP_TOPICS.mySchedule,
      group: "tagesplanung",
    });
    expect(
      caregiverTopics[4]?.instructionGroups?.map((group) => group.title),
    ).toEqual([
      "Termine und Einsätze ansehen",
      "Die Farben verstehen",
      "Den Kalender abonnieren",
    ]);
    expect(caregiverTopics[4]?.instructionGroups?.[2]?.description).toBe(
      "Nutzen Sie das Abo, wenn Sie hauptsächlich Ihren persönlichen Kalender verwenden. Neue und geänderte Einträge aus moto erscheinen dort automatisch.",
    );
    expect(caregiverTopics[4]?.result).toContain(
      "Das Kalender-Abo übernimmt neue, geänderte und abgesagte Einträge automatisch.",
    );
    expect(caregiverTopics[5]).toMatchObject({
      id: HELP_TOPICS.carePlan,
      group: "tagesplanung",
    });
    expect(caregiverTopics[5]?.troubleshooting).toBeUndefined();
    expect(caregiverTopics[5]?.troubleshootingDetails).toEqual([
      "Der Tab `Betreuungsplan` fehlt? Bitten Sie Ihre Leitung, Ihren Zugang zu prüfen.",
    ]);
    expect(caregiverTopics[5]?.differences).toBeUndefined();
    expect(caregiverTopics[6]).toMatchObject({
      id: HELP_TOPICS.studentSearch,
      group: "kinder",
    });
    expect(caregiverTopics[6]?.status).toBeUndefined();
    expect(caregiverTopics[6]?.instructionGroups).toHaveLength(3);
    expect(caregiverTopics[6]?.troubleshooting).toBe(
      HELP_TOPICS.missingChildOrGroup,
    );
    expect(caregiverTopics[7]).toMatchObject({
      id: HELP_TOPICS.editStudent,
    });
    expect(caregiverTopics[8]).toMatchObject({
      id: HELP_TOPICS.webAttendance,
    });
    expect(caregiverTopics[9]).toMatchObject({
      id: HELP_TOPICS.changeLocation,
    });
    expect(caregiverTopics[10]).toMatchObject({
      id: HELP_TOPICS.absences,
    });
    expect(caregiverTopics[11]).toMatchObject({
      id: HELP_TOPICS.dayLog,
    });
    expect(caregiverTopics[12]).toMatchObject({
      id: HELP_TOPICS.emergency,
    });
    expect(caregiverTopics[13]).toMatchObject({
      id: HELP_TOPICS.ownGroups,
    });
    expect(caregiverTopics[14]).toMatchObject({
      id: HELP_TOPICS.transferGroup,
    });
    expect(caregiverTopics.slice(7, 15).every((topic) => !topic?.status)).toBe(
      true,
    );
    expect(caregiverTopics).toHaveLength(37);
    expect(caregiverTopics.every((topic) => !topic.status)).toBe(true);
    expect([...new Set(caregiverTopics.map((topic) => topic.group))]).toEqual([
      "einstieg",
      "tagesplanung",
      "kinder",
      "gruppen",
      "team",
      "arbeitszeit",
      "nfc",
      "probleme",
    ]);
  });

  it("adapts the child details article to the attendance setting", () => {
    const detailedArticle = getPrototypeTopics("detailed").find(
      (topic) => topic.id === HELP_TOPICS.studentSearch,
    );
    const binaryArticle = getPrototypeTopics("binary").find(
      (topic) => topic.id === HELP_TOPICS.studentSearch,
    );

    expect(detailedArticle?.instructionGroups?.[1]?.steps).toContain(
      "Prüfen Sie oben den Aufenthaltsort sowie die heutige Ankunft und Abholung.",
    );
    expect(binaryArticle?.instructionGroups?.[1]?.steps).toContain(
      "Prüfen Sie oben die Anwesenheit sowie die heutige Ankunft und Abholung.",
    );
    expect(detailedArticle?.result).toBe(
      "Sie sehen jetzt die Angaben des Kindes. Auf dem Handy wählen Sie `Zurück`. Am Computer nutzen Sie die Navigation oben.",
    );
    expect(binaryArticle?.differences).toContain(
      "Bei einfacher Anwesenheit sehen Sie keinen Raum. Sie sehen nur, ob das Kind da ist.",
    );
  });

  it("lists every possible tab in the child details article", () => {
    const article = getPrototypeTopics("detailed").find(
      (topic) => topic.id === HELP_TOPICS.studentSearch,
    );
    const details =
      article?.instructionGroups?.flatMap((group) => group.steps).join(" ") ??
      "";

    expect(article?.instructionGroups?.[2]).toMatchObject({
      title: "Den passenden Bereich wählen",
      ordered: false,
    });

    for (const tab of [
      "Stammdaten",
      "Nachrichten",
      "Erziehungsberechtigte",
      "Betreuungsplan",
      "Betreuungszeiten",
      "Anmeldungen",
      "Dokumente",
      "Änderungsprotokoll",
      "Historie",
    ]) {
      expect(details).toContain(`\`${tab}\``);
    }
  });

  it("documents all three ways to change child information", () => {
    const article = getPrototypeTopics("detailed").find(
      (topic) => topic.id === HELP_TOPICS.editStudent,
    );

    expect(article?.instructionGroups?.map((group) => group.title)).toEqual([
      "Persönliche Angaben ändern",
      "Regelmäßige Betreuungszeiten ändern",
      "Nur einen Tag ändern",
    ]);
  });

  it("adapts attendance and location instructions to the attendance setting", () => {
    const detailedTopics = getPrototypeTopics("detailed");
    const binaryTopics = getPrototypeTopics("binary");
    const detailedAttendance = detailedTopics.find(
      (topic) => topic.id === HELP_TOPICS.webAttendance,
    );
    const binaryAttendance = binaryTopics.find(
      (topic) => topic.id === HELP_TOPICS.webAttendance,
    );
    const detailedLocation = detailedTopics.find(
      (topic) => topic.id === HELP_TOPICS.changeLocation,
    );
    const binaryLocation = binaryTopics.find(
      (topic) => topic.id === HELP_TOPICS.changeLocation,
    );

    expect(detailedAttendance?.result).toContain("Unterwegs");
    expect(binaryAttendance?.result).toContain("Anwesend");
    expect(detailedLocation?.steps).toContain(
      "Wählen Sie unter `Zielraum` den neuen Raum.",
    );
    expect(detailedLocation?.requirements).toContain(
      "Ihre OGS erfasst, in welchem Raum sich Kinder aufhalten.",
    );
    expect(binaryLocation?.summary).toContain("keine Räume");
    expect(binaryLocation?.steps).toContain(
      "Wählen Sie `Anmelden` oder `Abmelden`.",
    );
  });

  it("adapts the login result to the attendance and group settings", () => {
    const detailedLogin = getPrototypeTopics(
      "detailed",
      "fixed_groups",
      true,
    ).find((topic) => topic.id === HELP_TOPICS.login);
    const openCareLogin = getPrototypeTopics(
      "detailed",
      "open_care",
      true,
    ).find((topic) => topic.id === HELP_TOPICS.login);
    const binaryLogin = getPrototypeTopics("binary", "fixed_groups", true).find(
      (topic) => topic.id === HELP_TOPICS.login,
    );

    expect(detailedLogin?.result).toContain("Meine Gruppen");
    expect(openCareLogin?.result).toContain("Alle Kinder");
    expect(binaryLogin?.result).toContain("Alle Kinder");
  });

  it("documents the five next caregiver articles and their setting variants", () => {
    const detailedTopics = getPrototypeTopics("detailed", "fixed_groups", true);
    const binaryTopics = getPrototypeTopics("binary", "fixed_groups", true);
    const openCareTopics = getPrototypeTopics("detailed", "open_care", true);

    const article = (topics: typeof detailedTopics, id: string) =>
      topics.find((topic) => topic.id === id);

    expect(
      article(detailedTopics, HELP_TOPICS.absences)?.instructionGroups,
    ).toHaveLength(2);
    expect(article(detailedTopics, HELP_TOPICS.dayLog)?.title).toBe(
      "Den heutigen Betreuungstag prüfen",
    );
    expect(
      article(detailedTopics, HELP_TOPICS.emergency)?.differences,
    ).toContain(
      "Bei detaillierter Anwesenheit zeigt die Liste den aktuellen Ort oder Raum des Kindes.",
    );
    expect(article(binaryTopics, HELP_TOPICS.emergency)?.differences).toContain(
      "Bei einfacher Anwesenheit zeigt die Liste `Anwesend` statt eines Raums.",
    );
    expect(article(detailedTopics, HELP_TOPICS.ownGroups)?.steps).toContain(
      "Öffnen Sie `Meine Gruppen` in der Seitenleiste.",
    );
    expect(article(openCareTopics, HELP_TOPICS.ownGroups)?.summary).toContain(
      "keine festen eigenen Gruppen",
    );
    expect(
      article(openCareTopics, HELP_TOPICS.transferGroup)?.summary,
    ).toContain("keine feste eigene Gruppe");
  });

  it("finishes every caregiver article in every configuration variant", () => {
    for (const presenceMode of ["detailed", "binary"] as const) {
      for (const groupMode of ["fixed_groups", "open_care"] as const) {
        for (const nfcEnabled of [true, false]) {
          const caregiverTopics = getPrototypeTopics(
            presenceMode,
            groupMode,
            nfcEnabled,
          ).filter(
            (topic) =>
              topic.audience === "all" || topic.audience === "caregiver",
          );

          expect(caregiverTopics).toHaveLength(37);
          expect(caregiverTopics.every((topic) => !topic.status)).toBe(true);
          expect(
            caregiverTopics.every(
              (topic) =>
                topic.steps.length > 0 ||
                (topic.instructionGroups?.length ?? 0) > 0,
            ),
          ).toBe(true);
        }
      }
    }
  });

  it("explains unavailable room and NFC flows instead of leaving drafts", () => {
    const binaryTopics = getPrototypeTopics("binary", "fixed_groups", true);
    const noNfcTopics = getPrototypeTopics("detailed", "fixed_groups", false);

    expect(
      binaryTopics.find((topic) => topic.id === HELP_TOPICS.rooms)?.result,
    ).toContain("nicht angezeigt");
    expect(
      binaryTopics.find((topic) => topic.id === HELP_TOPICS.nfcCheckIn)?.result,
    ).toContain("Anwesend");
    expect(
      noNfcTopics.find((topic) => topic.id === HELP_TOPICS.tagAssignment)
        ?.summary,
    ).toContain("NFC nicht nutzt");
    expect(
      noNfcTopics.find((topic) => topic.id === HELP_TOPICS.manageActivity)
        ?.result,
    ).toContain("nicht angezeigt");
  });

  it("uses neutral articles when configuration context is unknown", () => {
    const topics = getPrototypeTopics("unknown", "unknown", null);

    expect(topics.find((topic) => topic.id === HELP_TOPICS.login)?.result).toBe(
      "Nach der Anmeldung öffnet moto die für Sie vorgesehene Startseite.",
    );
    expect(
      topics.find((topic) => topic.id === HELP_TOPICS.rooms)?.summary,
    ).toContain("nur sichtbar");
    expect(
      topics.find((topic) => topic.id === HELP_TOPICS.ownGroups)?.summary,
    ).toContain("hängt von der Arbeitsweise");
    expect(
      topics.find((topic) => topic.id === HELP_TOPICS.nfcCheckIn)?.summary,
    ).toBe("Ob dieser Ablauf verfügbar ist, hängt von Ihrer OGS ab.");
    expect(
      topics.find((topic) => topic.id === HELP_TOPICS.manageActivity)?.summary,
    ).toBe("Der Bereich erscheint nur bei NFC und detaillierter Anwesenheit.");
  });

  it("keeps the protected-page help exceptions shrink-only", () => {
    const unmappedPages = collectPageFiles(PROTECTED_PAGES_ROOT)
      .map(routePatternForPage)
      .filter(
        (route) => getHelpTopicForPath(examplePathForRoute(route)) === null,
      )
      .sort();

    expect(unmappedPages).toEqual(UNMAPPED_HELP_PAGE_BASELINE);
  });
});
