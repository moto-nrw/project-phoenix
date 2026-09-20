import { readdirSync } from "node:fs";
import { relative, resolve, sep } from "node:path";
import { describe, expect, it } from "vitest";

import {
  getHelpTopics,
  helpGroupLabel,
  helpTopicMatchesRole,
  HELP_GROUP_LABELS,
  HELP_GROUPS,
  HELP_ROLES,
} from "~/components/help/help-content";
import {
  getHelpTopicForPath,
  getParentHelpTopicForPath,
  getSchoolHelpTopicForPath,
  HELP_TOPICS,
} from "./help-topics";

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
 * Pages without a matching help article yet. This list is shrink-only:
 * mapping a page requires removing it here, and adding an unmapped page fails.
 */
const UNMAPPED_HELP_PAGE_BASELINE: readonly string[] = [
  // Startseite und ihre Alt-Adresse: dazu gibt es noch keinen Artikel.
  "/dashboard",
  "/home",
  "/profile",
  "/reminders",
  // Tagesübersicht der Vertretungen; der Artikel deckt bisher nur den
  // Vertretungsplan der Leitung ab.
  "/substitutions",
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
    // Die Unterbereiche der Datenverwaltung haben seit dem Leitungs-Gerüst
    // eigene Artikel; nur was keinen hat, faellt noch auf den Ueberblick.
    ["/database/students/import", HELP_TOPICS.leadCreateStudent],
    ["/database/students/class-list", HELP_TOPICS.leadClassListEntries],
    ["/database/personal/import", HELP_TOPICS.leadInviteStaff],
    ["/database/absence-types", HELP_TOPICS.dataManagement],
    ["/admin/enrollments", HELP_TOPICS.enrollments],
    ["/admin/enrollments/42", HELP_TOPICS.enrollments],
    ["/enrollment-phases/42/review", HELP_TOPICS.enrollments],
    ["/care-offerings", HELP_TOPICS.enrollments],
    ["/enrollment-form", HELP_TOPICS.leadEnrollmentForm],
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
});

// Jede Oberkategorie hat eine eigene Seite mit Karten ihrer Themen. Das
// Fragezeichen einer App-Seite zeigt dorthin, wenn mehrere Anleitungen zu
// ihr passen.
describe("help groups", () => {
  it("lists every group exactly once", () => {
    expect([...HELP_GROUPS].sort()).toEqual(
      Object.keys(HELP_GROUP_LABELS).sort(),
    );
    expect(new Set(HELP_GROUPS).size).toBe(HELP_GROUPS.length);
  });

  // Eine Gruppenseite ohne Karten waere eine Sackgasse.
  it("keeps every group reachable with at least one article", () => {
    const topics = getHelpTopics("detailed", "fixed_groups", true);
    const empty = HELP_GROUPS.filter(
      (group) =>
        !topics.some(
          (topic) =>
            topic.group === group &&
            HELP_ROLES.some((role) => helpTopicMatchesRole(topic, role)),
        ),
    );
    expect(empty).toEqual([]);
  });

  // Die Kennung einer Gruppe darf nie wie ein Artikel aussehen: beide
  // teilen sich den Adressraum unter `/help`.
  it("keeps group ids apart from topic ids", () => {
    const topicIds = new Set<string>(Object.values(HELP_TOPICS));
    const collisions = HELP_GROUPS.filter((group) => topicIds.has(group));
    expect(collisions).toEqual([]);
  });
});

describe("getParentHelpTopicForPath", () => {
  it.each([
    ["/children", HELP_TOPICS.parentChildOverview],
    ["/children/42", HELP_TOPICS.parentChildOverview],
    ["/messages", HELP_TOPICS.parentMessages],
    ["/calendar", HELP_TOPICS.parentCalendar],
    ["/news", HELP_TOPICS.parentNews],
    ["/meal-plan", HELP_TOPICS.parentMealPlan],
    ["/settings", HELP_TOPICS.parentNotifications],
    ["/anmeldung", HELP_TOPICS.parentEnroll],
    ["/anmeldung/demo-school/1", HELP_TOPICS.parentEnroll],
    // Auf der Eltern-Subdomain fehlt das Praefix, im Tenant-Kontext
    // steht es davor. Beide Formen fuehren zum selben Artikel.
    ["/parents/children", HELP_TOPICS.parentChildOverview],
    ["/parents/anmeldung/demo-school/1", HELP_TOPICS.parentEnroll],
  ])("maps %s to %s", (pathname, expectedTopic) => {
    expect(getParentHelpTopicForPath(pathname)).toBe(expectedTopic);
  });

  // Beide Portale kennen `/messages`, `/calendar` und `/settings`, meinen
  // damit aber verschiedene Seiten. Die Tabellen duerfen sich nicht
  // gegenseitig treffen.
  it("keeps the two portals apart where their paths collide", () => {
    expect(getParentHelpTopicForPath("/messages")).toBe(
      HELP_TOPICS.parentMessages,
    );
    expect(getHelpTopicForPath("/messages")).toBe(HELP_TOPICS.parentMessage);

    expect(getParentHelpTopicForPath("/calendar")).toBe(
      HELP_TOPICS.parentCalendar,
    );
    expect(getHelpTopicForPath("/calendar")).toBe(HELP_TOPICS.mySchedule);

    expect(getParentHelpTopicForPath("/settings")).toBe(
      HELP_TOPICS.parentNotifications,
    );
    expect(getHelpTopicForPath("/settings")).toBe(HELP_TOPICS.settings);
  });

  // Die Startseite fasst nur zusammen, was anderswo steht.
  it("leaves the parent start page without an article", () => {
    expect(getParentHelpTopicForPath("/")).toBeNull();
    expect(getParentHelpTopicForPath("/parents")).toBeNull();
  });

  it("resolves every registered help topic in both presence modes", () => {
    const expectedTopics = Object.values(HELP_TOPICS).sort();

    // Jede registrierte ID hat eine Seite. Sichtbar sind sie alle, solange
    // die Arbeitsweise der OGS keinen Ablauf ausschliesst.
    expect(
      getHelpTopics("detailed")
        .map((topic) => topic.id)
        .sort(),
    ).toEqual(expectedTopics);
    expect(
      getHelpTopics("unknown", "unknown", null)
        .map((topic) => topic.id)
        .sort(),
    ).toEqual(expectedTopics);

    // Bei einfacher Anwesenheit fehlen genau die vier Themen, die es dann
    // in der OGS nicht gibt -- und sonst keines.
    expect(
      getHelpTopics("binary")
        .map((topic) => topic.id)
        .sort(),
    ).toEqual(
      expectedTopics.filter(
        (id) =>
          ![
            HELP_TOPICS.rooms,
            HELP_TOPICS.activeSupervision,
            HELP_TOPICS.manageActivity,
            HELP_TOPICS.changeLocation,
            HELP_TOPICS.dayPlan,
          ].includes(id as never),
      ),
    );
  });

  it("keeps the finished articles in the agreed caregiver order", () => {
    const caregiverTopics = getHelpTopics("detailed").filter((topic) =>
      helpTopicMatchesRole(topic, "caregiver"),
    );

    expect(caregiverTopics[0]).toMatchObject({
      id: HELP_TOPICS.acceptInvitation,
      group: "einstieg",
    });
    expect(caregiverTopics[0]?.steps).toHaveLength(9);
    expect(caregiverTopics[1]).toMatchObject({
      id: HELP_TOPICS.login,
      group: "einstieg",
    });
    expect(caregiverTopics[1]?.steps).toHaveLength(6);
    expect(caregiverTopics[2]).toMatchObject({
      id: HELP_TOPICS.installApp,
      group: "einstieg",
    });
    expect(caregiverTopics[2]?.instructionGroups).toHaveLength(2);
    expect(caregiverTopics[2]?.differences).toContain(
      "`Unsichere App blockiert` erscheint? moto selbst ist nicht unsicher. Aktualisieren Sie im Play Store Chrome und Google Play. Starten Sie das Gerät neu. Versuchen Sie es danach erneut.",
    );
    expect(caregiverTopics[3]).toMatchObject({
      id: HELP_TOPICS.appOverview,
      group: "einstieg",
    });
    // Am Computer, Wie die Seitenleiste geordnet ist, Auf dem Handy oder
    // Tablet, Hilfe öffnen.
    expect(caregiverTopics[3]?.instructionGroups).toHaveLength(4);
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
      "Steht dort `Noch kein Planungszeitraum`? Dann hat Ihre Leitung den Zeitraum noch nicht angelegt.",
    ]);
    expect(caregiverTopics[5]?.differences).toEqual([
      "Steht oben rechts `Nur ansehen`? Dann dürfen Sie den Plan lesen, aber nicht ändern.",
    ]);
    // Der Tagesplan steht in der App ganz oben im Tagesbetrieb und folgt in
    // der Hilfe auf den Betreuungsplan (#2383).
    expect(caregiverTopics[6]).toMatchObject({
      id: HELP_TOPICS.dayPlan,
      group: "tagesplanung",
    });
    expect(caregiverTopics[7]).toMatchObject({
      id: HELP_TOPICS.studentSearch,
      group: "kinder",
    });
    expect(caregiverTopics[7]?.instructionGroups).toHaveLength(3);
    expect(caregiverTopics[7]?.troubleshooting).toBe(
      HELP_TOPICS.missingChildOrGroup,
    );
    expect(caregiverTopics[8]).toMatchObject({
      id: HELP_TOPICS.editStudent,
    });
    expect(caregiverTopics[9]).toMatchObject({
      id: HELP_TOPICS.webAttendance,
    });
    expect(caregiverTopics[10]).toMatchObject({
      id: HELP_TOPICS.changeLocation,
    });
    expect(caregiverTopics[11]).toMatchObject({
      id: HELP_TOPICS.absences,
    });
    expect(caregiverTopics[12]).toMatchObject({
      id: HELP_TOPICS.dayLog,
    });
    expect(caregiverTopics[13]).toMatchObject({
      id: HELP_TOPICS.emergency,
    });
    expect(caregiverTopics[14]).toMatchObject({
      id: HELP_TOPICS.ownGroups,
    });
    expect(caregiverTopics[15]).toMatchObject({
      id: HELP_TOPICS.transferGroup,
    });
    expect(caregiverTopics).toHaveLength(38);
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
    const detailedArticle = getHelpTopics("detailed").find(
      (topic) => topic.id === HELP_TOPICS.studentSearch,
    );
    const binaryArticle = getHelpTopics("binary").find(
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
    const article = getHelpTopics("detailed").find(
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
    const article = getHelpTopics("detailed").find(
      (topic) => topic.id === HELP_TOPICS.editStudent,
    );

    expect(article?.instructionGroups?.map((group) => group.title)).toEqual([
      "Persönliche Angaben ändern",
      "Regelmäßige Betreuungszeiten ändern",
      "Nur einen Tag ändern",
    ]);
  });

  it("adapts attendance and location instructions to the attendance setting", () => {
    const detailedTopics = getHelpTopics("detailed");
    const binaryTopics = getHelpTopics("binary");
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
      "Ihre OGS erfasst die Räume der Kinder.",
    );
    expect(binaryLocation).toBeUndefined();
  });

  it("adapts the login result to the attendance and group settings", () => {
    const detailedLogin = getHelpTopics("detailed", "fixed_groups", true).find(
      (topic) => topic.id === HELP_TOPICS.login,
    );
    const openCareLogin = getHelpTopics("detailed", "open_care", true).find(
      (topic) => topic.id === HELP_TOPICS.login,
    );
    const binaryLogin = getHelpTopics("binary", "fixed_groups", true).find(
      (topic) => topic.id === HELP_TOPICS.login,
    );

    expect(detailedLogin?.result).toContain("Meine Gruppen");
    expect(openCareLogin?.result).toContain("Alle Kinder");
    expect(binaryLogin?.result).toContain("Alle Kinder");
  });

  it("documents the five next caregiver articles and their setting variants", () => {
    const detailedTopics = getHelpTopics("detailed", "fixed_groups", true);
    const binaryTopics = getHelpTopics("binary", "fixed_groups", true);
    const openCareTopics = getHelpTopics("detailed", "open_care", true);

    const article = (topics: typeof detailedTopics, id: string) =>
      topics.find((topic) => topic.id === id);

    expect(
      article(detailedTopics, HELP_TOPICS.absences)?.instructionGroups?.map(
        (group) => group.title,
      ),
    ).toEqual([
      "Ein Kind krankmelden",
      "Ein Kind entschuldigen",
      "Bei `Ab Uhrzeit`",
      "Die Meldung für heute aufheben",
      "Einen geplanten Tag entfernen",
      "Eine Entschuldigung `Ab Uhrzeit` entfernen",
    ]);
    expect(article(detailedTopics, HELP_TOPICS.dayLog)?.title).toBe(
      "Den heutigen Betreuungstag prüfen",
    );
    expect(article(detailedTopics, HELP_TOPICS.emergency)?.result).toContain(
      "Bei detaillierter Anwesenheit zeigt die Liste den aktuellen Ort oder Raum des Kindes.",
    );
    expect(article(binaryTopics, HELP_TOPICS.emergency)?.result).toContain(
      "Bei einfacher Anwesenheit zeigt die Liste `Anwesend` statt eines Raums.",
    );
    expect(article(detailedTopics, HELP_TOPICS.ownGroups)?.steps).toContain(
      "Öffnen Sie `Meine Gruppen` in der Seitenleiste.",
    );
    expect(article(openCareTopics, HELP_TOPICS.ownGroups)).toBeUndefined();
    expect(article(openCareTopics, HELP_TOPICS.transferGroup)).toBeUndefined();
  });

  it("finishes every caregiver article in every configuration variant", () => {
    for (const presenceMode of ["detailed", "binary"] as const) {
      for (const groupMode of ["fixed_groups", "open_care"] as const) {
        for (const nfcEnabled of [true, false]) {
          const caregiverTopics = getHelpTopics(
            presenceMode,
            groupMode,
            nfcEnabled,
          ).filter((topic) => helpTopicMatchesRole(topic, "caregiver"));

          // Was es in der OGS nicht gibt, steht auch nicht in der Liste:
          // ohne NFC entfallen sieben Themen, bei einfacher Anwesenheit
          // fuenf, bei offener Betreuung zwei. `Aktivitaet anlegen` faellt
          // unter die NFC- und die Anwesenheitsregel, wird also nur
          // einmal abgezogen.
          const expectedLength =
            38 -
            (nfcEnabled ? 0 : 7) -
            (presenceMode === "binary" ? (nfcEnabled ? 5 : 4) : 0) -
            (groupMode === "open_care" ? 2 : 0);
          expect(caregiverTopics).toHaveLength(expectedLength);
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

  it("adapts flows that survive simple attendance instead of leaving drafts", () => {
    const binaryTopics = getHelpTopics("binary", "fixed_groups", true);

    expect(
      binaryTopics.find((topic) => topic.id === HELP_TOPICS.nfcCheckIn)?.result,
    ).toContain("Anwesend");
  });

  it("finishes every lead article and keeps its links reachable", () => {
    const leadTopics = getHelpTopics("detailed", "fixed_groups", true).filter(
      (topic) => helpTopicMatchesRole(topic, "lead"),
    );
    const visible = new Set(leadTopics.map((topic) => topic.id));

    // 46 eigene Leitungs-Themen plus die geteilten Artikel, deren Ablauf
    // fuer Leitung und Betreuung derselbe ist -- die eigene Arbeitszeit,
    // der eigene Kalender, der Aufbau der Navigation, die Seiten des
    // Tagesbetriebs und der Umgang mit dem NFC-Tablet.
    expect(leadTopics).toHaveLength(75);
    expect(
      leadTopics.every(
        (topic) =>
          topic.steps.length > 0 || (topic.instructionGroups?.length ?? 0) > 0,
      ),
    ).toBe(true);

    // `related` laeuft durch denselben Rollenfilter wie die Seitenleiste: ein
    // Verweis auf einen reinen Betreuer-Artikel verschwindet in der
    // Leitungshilfe stillschweigend. Die Ueberschrift „Weitere Themen" haengt
    // aber an der ungefilterten Liste und stuende sonst leer da.
    const withoutReachableLink = leadTopics
      .filter((topic) => topic.related.length > 0)
      .filter((topic) => !topic.related.some((id) => visible.has(id)))
      .map((topic) => topic.id);
    expect(withoutReachableLink).toEqual([]);
  });

  it("keeps the parent structure complete and self-contained", () => {
    const parentTopics = getHelpTopics("detailed", "fixed_groups", true).filter(
      (topic) => helpTopicMatchesRole(topic, "parent"),
    );
    const visible = new Set(parentTopics.map((topic) => topic.id));

    expect(parentTopics).toHaveLength(20);
    expect(
      parentTopics.every(
        (topic) =>
          topic.steps.length > 0 || (topic.instructionGroups?.length ?? 0) > 0,
      ),
    ).toBe(true);
    expect([...new Set(parentTopics.map((topic) => topic.group))]).toEqual([
      "einstieg",
      "mein-kind",
      "nachrichten",
      "anmeldung",
      "probleme",
    ]);

    // Das Eltern-Portal ist eigenstaendig: kein Verweis darf in die
    // Betreuungs- oder Leitungshilfe zeigen, sonst faellt er im Rollenfilter
    // stillschweigend weg.
    const unreachable = parentTopics.flatMap((topic) =>
      topic.related
        .filter((id) => !visible.has(id))
        .map((id) => `${topic.id} -> ${id}`),
    );
    expect(unreachable).toEqual([]);
  });

  it("keeps the teacher structure complete and self-contained", () => {
    const teacherTopics = getHelpTopics(
      "detailed",
      "fixed_groups",
      true,
    ).filter((topic) => helpTopicMatchesRole(topic, "teacher"));
    const visible = new Set(teacherTopics.map((topic) => topic.id));

    expect(teacherTopics).toHaveLength(15);
    expect(
      teacherTopics.every(
        (topic) =>
          topic.steps.length > 0 || (topic.instructionGroups?.length ?? 0) > 0,
      ),
    ).toBe(true);
    expect([...new Set(teacherTopics.map((topic) => topic.group))]).toEqual([
      "einstieg",
      "klasse",
      "aufsicht",
      "nachrichten",
      "probleme",
    ]);

    // moto schule ist ein eigenes Portal: kein Verweis darf in die
    // Betreuungs-, Leitungs- oder Elternhilfe zeigen.
    const unreachable = teacherTopics.flatMap((topic) =>
      topic.related
        .filter((id) => !visible.has(id))
        .map((id) => `${topic.id} -> ${id}`),
    );
    expect(unreachable).toEqual([]);
  });

  it("hides every room-only article when attendance is simple", () => {
    const binaryIds = new Set(
      getHelpTopics("binary", "fixed_groups", true).map((t) => t.id),
    );

    for (const hidden of [
      HELP_TOPICS.rooms,
      HELP_TOPICS.activeSupervision,
      HELP_TOPICS.manageActivity,
      HELP_TOPICS.changeLocation,
    ]) {
      expect(binaryIds.has(hidden)).toBe(false);
    }
    // Der Ersatzweg bleibt: an- und abmelden geht auch ohne Raeume.
    expect(binaryIds.has(HELP_TOPICS.webAttendance)).toBe(true);
  });

  it("hides every group-only article when the school has open care", () => {
    const openIds = new Set(
      getHelpTopics("detailed", "open_care", true).map((t) => t.id),
    );

    expect(openIds.has(HELP_TOPICS.ownGroups)).toBe(false);
    expect(openIds.has(HELP_TOPICS.transferGroup)).toBe(false);
    // Bleibt: wer seine Gruppe vermisst, findet hier die Erklaerung.
    expect(openIds.has(HELP_TOPICS.missingChildOrGroup)).toBe(true);
  });

  it("drops group hints from articles that stay under open care", () => {
    const open = getHelpTopics("detailed", "open_care", true);

    const dayLog = open.find((t) => t.id === HELP_TOPICS.dayLog);
    const dayLogText = (dayLog?.instructionGroups ?? [])
      .flatMap((g) => g.steps)
      .join(" ");
    expect(dayLogText).not.toContain("Gruppe");

    expect(
      open.find((t) => t.id === HELP_TOPICS.parentRequests)?.requirements,
    ).not.toContain("Sie haben Zugriff auf die Gruppe des Kindes.");

    const search = open.find((t) => t.id === HELP_TOPICS.studentSearch);
    expect(
      (search?.instructionGroups ?? []).flatMap((g) => g.steps).join(" "),
    ).not.toContain("oder Gruppe");
  });

  it("names a topic group only after what it still contains", () => {
    expect(helpGroupLabel("gruppen", "detailed", "fixed_groups")).toBe(
      "Gruppen, Räume und Aufsicht",
    );
    expect(helpGroupLabel("gruppen", "binary", "fixed_groups")).toBe("Gruppen");
    expect(helpGroupLabel("gruppen", "detailed", "open_care")).toBe(
      "Räume und Aufsicht",
    );
  });

  it("drops room and supervision hints from articles that stay", () => {
    const binary = getHelpTopics("binary", "fixed_groups", true);

    expect(
      binary.find((t) => t.id === HELP_TOPICS.carePlan)?.steps.join(" "),
    ).not.toContain("Raum");
    expect(
      binary.find((t) => t.id === HELP_TOPICS.findStaff)?.result,
    ).not.toContain("Aufsicht");
  });

  it("hides every NFC-only article when the school says it has no NFC", () => {
    const noNfcIds = new Set(
      getHelpTopics("detailed", "fixed_groups", false).map((t) => t.id),
    );

    for (const hidden of [
      HELP_TOPICS.tabletLogin,
      HELP_TOPICS.tagAssignment,
      HELP_TOPICS.nfcWorkTime,
      HELP_TOPICS.nfcSupervision,
      HELP_TOPICS.nfcCheckIn,
      HELP_TOPICS.nfcProblem,
      HELP_TOPICS.manageActivity,
    ]) {
      expect(noNfcIds.has(hidden)).toBe(false);
    }
  });

  it("keeps NFC articles while the school has not said whether it uses NFC", () => {
    // Unbekannt ist kein Nein: wer nie gefragt wurde, darf das Thema finden.
    const unknownIds = new Set(
      getHelpTopics("unknown", "unknown", null).map((t) => t.id),
    );

    expect(unknownIds.has(HELP_TOPICS.tagAssignment)).toBe(true);
    expect(unknownIds.has(HELP_TOPICS.manageActivity)).toBe(true);
  });

  it("drops NFC hints from articles that stay when there is no NFC", () => {
    const noNfc = getHelpTopics("detailed", "fixed_groups", false);

    const loginProblem = noNfc.find(
      (topic) => topic.id === HELP_TOPICS.loginProblem,
    );
    expect(loginProblem?.differences?.join(" ")).not.toContain("Geräte-PIN");

    const missingMenu = noNfc.find(
      (topic) => topic.id === HELP_TOPICS.missingMenu,
    );
    // Die Arbeitsweise ist bekannt, also steht die zutreffende Zeile als
    // Tatsache da und nicht als eine von mehreren Moeglichkeiten.
    expect(missingMenu?.differences?.join(" ")).toContain(
      "Ihre OGS nutzt kein NFC",
    );
    expect(missingMenu?.differences?.join(" ")).not.toContain("Ohne NFC fehlt");
  });

  it("uses neutral articles when configuration context is unknown", () => {
    const topics = getHelpTopics("unknown", "unknown", null);

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

describe("getSchoolHelpTopicForPath", () => {
  it.each([
    ["/", HELP_TOPICS.teacherClassDay],
    ["/school", HELP_TOPICS.teacherClassDay],
    ["/klasse", HELP_TOPICS.teacherClassList],
    ["/school/klasse", HELP_TOPICS.teacherClassList],
    ["/aufsichten", HELP_TOPICS.teacherSupervision],
    ["/school/aufsichten", HELP_TOPICS.teacherSupervision],
    ["/nachrichten", HELP_TOPICS.teacherMessages],
    ["/nachrichten/42", HELP_TOPICS.teacherMessages],
    ["/school/nachrichten/42", HELP_TOPICS.teacherMessages],
    ["/tagesinformationen", HELP_TOPICS.teacherNotices],
    ["/school/tagesinformationen", HELP_TOPICS.teacherNotices],
    ["/einstellungen", HELP_TOPICS.teacherSettings],
    ["/school/einstellungen", HELP_TOPICS.teacherSettings],
  ])("maps %s to %s", (pathname, expectedTopic) => {
    expect(getSchoolHelpTopicForPath(pathname)).toBe(expectedTopic);
  });

  it("does not send an unrelated school page to a generic help topic", () => {
    expect(getSchoolHelpTopicForPath("/unbekannt")).toBeNull();
  });
});
