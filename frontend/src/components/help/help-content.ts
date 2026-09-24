import type { HelpIconName } from "~/components/help/help-icons";
import {
  HELP_TOPICS,
  type HelpRole,
  type HelpTopicId,
} from "~/lib/help-topics";

// Die Rolle ist ein Navigationsbegriff, kein Inhalt: sie liegt in
// `help-topics.ts`, damit die Seitenleiste eine Hilfe-Adresse bauen kann,
// ohne die Artikeltexte zu laden. Hier nur weitergereicht, damit die
// bestehenden Importe aus dieser Datei weiter stimmen.
export type { HelpRole };
export type HelpPresenceMode = "detailed" | "binary" | "unknown";
export type HelpGroupMode = "fixed_groups" | "open_care" | "unknown";
export type HelpTopicGroup =
  // Betreuungskraefte
  | "einstieg"
  | "tagesplanung"
  | "kinder"
  | "gruppen"
  | "team"
  | "arbeitszeit"
  | "nfc"
  | "probleme"
  // OGS-Leitung. „anmeldeverwaltung" heisst absichtlich nicht „anmeldung":
  // so heisst die Eltern-Gruppe, in der sich Eltern selbst anmelden.
  | "einrichten"
  | "kinderdaten"
  | "elternarbeit"
  | "anmeldeverwaltung"
  | "personal"
  | "planung"
  | "auswertung"
  | "konfiguration"
  // Eltern
  | "mein-kind"
  | "nachrichten"
  | "anmeldung"
  // Lehrkraefte
  | "klasse"
  | "aufsicht";

export const HELP_ROLES = [
  "caregiver",
  "lead",
  "parent",
  "teacher",
] as const satisfies readonly HelpRole[];

export interface HelpTopic {
  readonly id: HelpTopicId;
  readonly title: string;
  readonly question: string;
  readonly summary: string;
  readonly group: HelpTopicGroup;
  /**
   * Wer den Artikel in Seitenleiste und Suche sieht. Eine Liste, wo mehrere
   * Rollen denselben Ablauf haben -- eine Leitung meldet sich genauso an wie
   * eine Betreuungskraft. Zwei fast gleiche Artikel waeren sonst die Folge,
   * und einer davon liefe irgendwann hinterher. `"all"` heisst jede Rolle.
   */
  readonly audience: "all" | HelpRole | readonly HelpRole[];
  readonly icon: HelpIconName;
  readonly requirements?: readonly string[];
  readonly steps: readonly string[];
  readonly instructionGroups?: readonly {
    readonly title: string;
    readonly description?: string;
    readonly steps: readonly string[];
    readonly ordered?: boolean;
  }[];
  readonly result?: string;
  readonly differences?: readonly string[];
  /**
   * Abschnitt „Tipp": optionale Hinweise, die den Ablauf weder aendern noch
   * voraussetzen -- „das geht auch noch". Nicht zu verwechseln mit
   * `differences` („Wenn es anders aussieht": der Bildschirm weicht ab) und
   * `troubleshootingDetails` („Wenn es nicht klappt": etwas fehlt).
   */
  readonly notes?: readonly string[];
  readonly troubleshooting?: HelpTopicId;
  readonly troubleshootingDetails?: readonly string[];
  readonly image?: string;
  readonly imageAlt?: string;
  readonly related: readonly HelpTopicId[];
}

/** Sieht `role` diesen Artikel? Einzige Stelle, die `audience` auswertet. */
export function helpTopicMatchesRole(
  topic: HelpTopic,
  role: HelpRole,
): boolean {
  const { audience } = topic;
  if (audience === "all") return true;
  if (Array.isArray(audience)) return audience.includes(role);
  return audience === role;
}

function loginResult(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): string {
  if (presenceMode === "binary" || groupMode === "open_care") {
    return "Nach der Anmeldung öffnet moto den Bereich `Alle Kinder`.";
  }
  if (presenceMode === "detailed" && groupMode === "fixed_groups") {
    return "Nach der Anmeldung öffnet moto den Bereich `Meine Gruppen`.";
  }
  return "Nach der Anmeldung öffnet moto die für Sie vorgesehene Startseite.";
}

function invitationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.acceptInvitation,
    title: "Einladung annehmen und Konto einrichten",
    question: "Wie nehme ich meine Einladung an?",
    summary:
      "Öffnen Sie Ihre Einladungs-Mail und legen Sie Ihr persönliches Passwort fest.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "EnvelopeOpen",
    requirements: [
      "Ihre Einladungs-Mail von moto",
      "Zugriff auf das E-Mail-Postfach, an das die Einladung gesendet wurde",
    ],
    steps: [
      "Öffnen Sie die Einladungs-Mail von moto.",
      "Wählen Sie in der E-Mail `Einladung annehmen`.",
      "Prüfen Sie oben die Zeile `Einladung für ... als ...`. Stimmt die Rolle?",
      "Prüfen Sie unter `Gültig bis`, wie lange die Einladung gilt.",
      "Ergänzen Sie `Vorname` und `Nachname`, falls die Felder leer sind.",
      "Geben Sie unter `Passwort` ein persönliches Passwort ein.",
      "Geben Sie dasselbe Passwort unter `Passwort bestätigen` erneut ein.",
      "Prüfen Sie, ob alle Passwortanforderungen grün markiert sind.",
      "Wählen Sie `Einladung akzeptieren`.",
    ],
    result:
      "Ihr moto-Konto ist eingerichtet und Sie können sich jetzt anmelden. Bewahren Sie Ihre E-Mail-Adresse und Ihr Passwort sicher auf und geben Sie beides nicht weiter.",
    differences: [
      "Steht dort `Schule hinzufügen`? Dann haben Sie schon ein moto-Konto. Melden Sie sich an und wählen Sie `Einladung annehmen`. Ihr Passwort bleibt gleich.",
    ],
    troubleshootingDetails: [
      "Sie finden keine Einladungs-Mail? Prüfen Sie auch Ihren Spam-Ordner. Bitten Sie sonst Ihre Leitung um eine neue Einladung.",
      "Die Einladung ist abgelaufen oder wurde schon verwendet? Bitten Sie Ihre Leitung um eine neue Einladung.",
      "Die Einladung ist nicht für Sie oder die Rolle ist falsch? Nehmen Sie sie nicht an und wenden Sie sich an Ihre Leitung.",
    ],
    related: [HELP_TOPICS.login, HELP_TOPICS.installApp],
  };
}

function loginTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): HelpTopic {
  return {
    id: HELP_TOPICS.login,
    title: "Bei moto anmelden",
    question: "Wie melde ich mich bei moto an?",
    summary:
      "Melden Sie sich über die moto-Seite Ihrer OGS mit Ihrem persönlichen Konto an.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "UserCircle",
    requirements: [
      "Den Link zur moto-Seite Ihrer OGS",
      "Ein moto-Konto. Nehmen Sie vorher Ihre Einladung an.",
      "Ihre E-Mail-Adresse und Ihr Passwort für moto",
      "Zugriff auf Ihr E-Mail-Postfach, falls moto einen Sicherheitscode abfragt",
    ],
    steps: [
      "Öffnen Sie die moto-Seite Ihrer OGS.",
      "Geben Sie unter `E-Mail-Adresse` Ihre persönliche E-Mail-Adresse ein.",
      "Geben Sie unter `Passwort` Ihr Passwort ein.",
      "Wählen Sie `Anmelden`.",
      "Zeigt moto die `Code-Eingabe`? Öffnen Sie die E-Mail von moto.",
      "Geben Sie den sechsstelligen Code ein. moto prüft ihn automatisch.",
    ],
    result: loginResult(presenceMode, groupMode),
    differences: [
      "Richten Sie die Sicherheitsabfrage erstmals ein? Wählen Sie `Code an meine E-Mail senden`.",
      "Geben Sie danach den sechsstelligen Code ein.",
      "Wenn Sie bereits einen Passkey eingerichtet haben, können Sie `Mit Passkey anmelden` wählen.",
    ],
    notes: [
      "Nutzen mehrere Personen das Gerät? Melden Sie sich bei moto ab, wenn Sie fertig sind.",
    ],
    troubleshooting: HELP_TOPICS.loginProblem,
    related: [
      HELP_TOPICS.acceptInvitation,
      HELP_TOPICS.installApp,
      HELP_TOPICS.loginProblem,
    ],
  };
}

function installAppTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.installApp,
    title: "moto als App auf dem Handy oder Tablet hinzufügen",
    question: "Wie füge ich moto auf meinem Handy oder Tablet hinzu?",
    summary:
      "Fügen Sie moto zum Startbildschirm hinzu. Sie brauchen dafür keinen App Store.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "Smartphone",
    requirements: [
      "Ein Handy oder Tablet",
      "Den Link zur moto-Seite Ihrer OGS",
      "Safari auf dem iPhone oder iPad oder Chrome auf einem Android-Gerät",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Auf dem iPhone oder iPad",
        steps: [
          "Öffnen Sie die moto-Seite Ihrer OGS in Safari.",
          "Melden Sie sich bei moto an.",
          "Tippen Sie auf das Teilen-Symbol. Wählen Sie bei Bedarf zuerst `Mehr`.",
          "Wählen Sie `Zum Home-Bildschirm`.",
          "Schalten Sie `Als Web-App öffnen` ein. Wählen Sie danach `Hinzufügen`.",
        ],
      },
      {
        title: "Auf einem Android-Handy oder Android-Tablet",
        steps: [
          "Öffnen Sie die moto-Seite Ihrer OGS in Chrome.",
          "Melden Sie sich bei moto an.",
          "Ab dem zweiten Besuch erscheint der Hinweis `moto als App nutzen`.",
          "Wählen Sie `App installieren`.",
          "Wählen Sie im nächsten Fenster `Installieren`.",
        ],
      },
    ],
    result:
      "Auf Ihrem Startbildschirm erscheint ein neues moto-Symbol. Öffnen Sie moto künftig über dieses Symbol.",
    differences: [
      "Der Hinweis erscheint auf Android nicht? Öffnen Sie in Chrome die drei Punkte. Wählen Sie `App installieren` oder `Zum Startbildschirm hinzufügen`.",
      "Nutzen Sie Samsung Internet? Wählen Sie im Hinweis `In Chrome öffnen`. Führen Sie die Schritte danach in Chrome aus.",
      "`Unsichere App blockiert` erscheint? moto selbst ist nicht unsicher. Aktualisieren Sie im Play Store Chrome und Google Play. Starten Sie das Gerät neu. Versuchen Sie es danach erneut.",
      "Arbeiten Sie für mehrere Einrichtungen? Fügen Sie jede Einrichtung einzeln zum Startbildschirm hinzu.",
    ],
    related: [
      HELP_TOPICS.login,
      HELP_TOPICS.mySchedule,
      HELP_TOPICS.leadGoLive,
    ],
  };
}

function appOverviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.appOverview,
    title: "Sich in moto zurechtfinden",
    question: "Wie ist moto aufgebaut?",
    summary:
      "Über die Navigation öffnen Sie die Bereiche, die Sie für Ihren Arbeitstag brauchen.",
    group: "einstieg",
    audience: ["caregiver", "lead"],
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "Am Computer",
        steps: [
          "Nutzen Sie die Seitenleiste links, um einen Bereich zu öffnen.",
          "Wählen Sie einen Eintrag mit Pfeil, um die zugehörigen Bereiche ein- oder auszublenden.",
          "Öffnen Sie oben rechts Ihr Profil, um Ihre Angaben anzusehen oder sich abzumelden.",
        ],
      },
      {
        // Beschreibt den Zweck jeder Gruppe, nicht ihre Eintraege. Welche
        // Seiten in einer Gruppe stehen, haengt von Rolle und Einstellungen
        // ab -- eine Aufzaehlung waere in der Haelfte der OGS falsch.
        // Quelle der Ordnung: STAFF_NAV_GROUPS in lib/staff-navigation.ts.
        title: "Wie die Seitenleiste geordnet ist",
        description:
          "Ganz oben steht die `Startseite`. Darunter bündeln Gruppen, was zusammengehört.",
        ordered: false,
        steps: [
          "`Tagesbetrieb`: alles für den laufenden Betreuungstag. Diese Gruppe ist schon aufgeklappt.",
          "`Eltern`: was Sie mit den Familien austauschen.",
          "`Team`: Ihre Arbeitszeit, Ihr Kalender und Ihre Kolleginnen und Kollegen.",
          "`Planung`: die Pläne Ihrer OGS.",
          "`Verwaltung`: Stammdaten, Auswertungen und Dateien.",
          "In jeder Gruppe steht oben, was im Alltag am häufigsten gebraucht wird.",
          "Sie sehen nur Gruppen, in denen etwas für Sie dabei ist.",
          "Ganz unten stehen `Notfall` und `Hilfe`. Wer die OGS verwaltet, findet dort auch `Einstellungen`.",
        ],
      },
      {
        title: "Auf dem Handy oder Tablet",
        steps: [
          "Nutzen Sie die Leiste unten, um häufige Bereiche zu öffnen.",
          "Wählen Sie `Mehr`, um die übrigen Bereiche zu sehen.",
          "Öffnen Sie oben rechts Ihr Profil, um Ihre Angaben anzusehen oder sich abzumelden.",
        ],
      },
      {
        title: "Hilfe öffnen",
        steps: [
          "Sehen Sie oben neben dem Seitennamen ein Fragezeichen? Wählen Sie es, um die passende Anleitung zu öffnen.",
          "Oder wählen Sie `Hilfe`: am Computer unten in der Seitenleiste, auf dem Handy unter `Mehr`.",
        ],
      },
    ],
    result:
      "Sie wissen, wo Sie die wichtigsten Bereiche und die passende Hilfe finden.",
    differences: [
      "Welche Bereiche Sie sehen, hängt von Ihrer Rolle und den Einstellungen Ihrer OGS ab.",
    ],
    related: [
      HELP_TOPICS.login,
      HELP_TOPICS.mySchedule,
      HELP_TOPICS.studentSearch,
    ],
  };
}

function studentSearchTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): HelpTopic {
  const currentStatusStep =
    presenceMode === "detailed"
      ? "Prüfen Sie oben den Aufenthaltsort sowie die heutige Ankunft und Abholung."
      : presenceMode === "binary"
        ? "Prüfen Sie oben die Anwesenheit sowie die heutige Ankunft und Abholung."
        : "Prüfen Sie oben die Anwesenheit sowie die heutige Ankunft und Abholung.";

  const presenceDifference =
    presenceMode === "binary"
      ? "Bei einfacher Anwesenheit sehen Sie keinen Raum. Sie sehen nur, ob das Kind da ist."
      : null;

  return {
    id: HELP_TOPICS.studentSearch,
    title: "Ein Kind finden und Angaben ansehen",
    question: "Wie finde ich ein Kind und seine Angaben?",
    summary: "Finden Sie ein Kind. Öffnen Sie danach die benötigten Angaben.",
    group: "kinder",
    audience: ["caregiver", "lead"],
    icon: "Search",
    steps: [],
    instructionGroups: [
      {
        title: "Ein Kind finden",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Geben Sie in `Name suchen...` den Namen ein.",
          "Sie können auch nur einen Namensteil eingeben.",
          // Der Filter traegt kein sichtbares Wort, nur ein Symbol mit drei
          // Reglern (FilterButton, aria-label "Filter").
          "Zu viele Treffer? Wählen Sie rechts neben der Suche das Filtersymbol.",
          // Ohne feste Gruppen gibt es auch keinen Gruppenfilter.
          groupMode === "open_care"
            ? "Wählen Sie zum Beispiel eine Klasse."
            : "Wählen Sie zum Beispiel eine Klasse oder Gruppe.",
          // Der Klick auf den eigenen Menüeintrag leert Suche und Filter
          // (#3374). Am Handy heißt der Eintrag unten "Suchen".
          "Neue Suche? Wählen Sie im Menü `Alle Kinder` oder unten `Suchen`. Suche und Filter sind dann leer.",
        ],
      },
      {
        title: "Angaben ansehen",
        steps: ["Wählen Sie die Karte des Kindes.", currentStatusStep],
      },
      {
        title: "Den passenden Bereich wählen",
        ordered: false,
        steps: [
          "Wählen Sie `Stammdaten` für persönliche Angaben.",
          "Wählen Sie `Nachrichten` für den Nachrichtenaustausch mit den Eltern.",
          "Wählen Sie `Erziehungsberechtigte` für Kontaktangaben.",
          "Wählen Sie `Betreuungsplan` für den geplanten Tages- oder Wochenablauf.",
          "Wählen Sie `Betreuungszeiten` für regelmäßige Ankunfts- und Abholzeiten.",
          // Live geprueft: mit Betreuer-Rechten fehlt der Reiter ganz. Er
          // bleibt trotzdem in der Liste -- die Leitung liest denselben
          // Artikel und sieht ihn (canViewEnrollments = `config:manage`).
          "Wählen Sie `Anmeldungen` für Angaben aus der Halbjahresanmeldung. Diesen Reiter sieht nur die Leitung.",
          "Wählen Sie `Dokumente` für Dateien zum Kind.",
          "Wählen Sie `Änderungsprotokoll` für Änderungen an den Angaben.",
          "Wählen Sie `Historie` für Anwesenheit, Feedback und weitere Verläufe.",
        ],
      },
    ],
    result:
      "Sie sehen jetzt die Angaben des Kindes. Auf dem Handy wählen Sie `Zurück`. Am Computer nutzen Sie die Navigation oben.",
    differences: [
      "Haben Sie einen anderen Tag gewählt? Dann sehen Sie die geplante Anwesenheit. Ein aktueller Aufenthaltsort wird nicht gezeigt.",
      "Einige Bereiche brauchen zusätzliche Rechte: `Betreuungsplan`, `Dokumente` und `Änderungsprotokoll`. Fehlt ein Bereich? Fragen Sie Ihre Leitung.",
      ...(presenceDifference ? [presenceDifference] : []),
    ],
    troubleshooting: HELP_TOPICS.missingChildOrGroup,
    related: [
      HELP_TOPICS.editStudent,
      HELP_TOPICS.webAttendance,
      HELP_TOPICS.absences,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}

function editStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.editStudent,
    title: "Angaben oder Betreuungszeiten eines Kindes ändern",
    question: "Wie ändere ich Angaben oder Betreuungszeiten eines Kindes?",
    summary:
      "Ändern Sie persönliche Angaben, den Wochenplan oder einen einzelnen Tag.",
    group: "kinder",
    audience: "caregiver",
    icon: "FileText",
    steps: [],
    instructionGroups: [
      {
        title: "Persönliche Angaben ändern",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie das Kind und wählen Sie `Stammdaten`.",
          "Wählen Sie `Bearbeiten`.",
          "Ändern Sie die benötigten Angaben.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Regelmäßige Betreuungszeiten ändern",
        description:
          "Der Wochenplan gilt ab sofort für alle kommenden Wochen. Schon eingetragene Ausnahmen bleiben bestehen.",
        steps: [
          "Öffnen Sie beim Kind den Reiter `Betreuungszeiten`.",
          "Wählen Sie oben rechts `Wochenplan`.",
          "Setzen Sie bei den Betreuungstagen einen Haken.",
          "Tragen Sie je Tag `Ankunft` und `Abholung` ein.",
          "Kommt das Kind an einem Wochentag nicht? Lassen Sie `Abholung` leer.",
          "Öffnen Sie `Notizen`. Tragen Sie den Hinweis unter `Notiz zum Tag (jede Woche)` ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        // Live geprueft: jede Tageskarte hat oben rechts den Knopf
        // `Ausnahme`. Ankunft und Abholung haben je drei Auswahlfelder.
        title: "Nur einen Tag ändern",
        description:
          "Eine Ausnahme gilt nur an diesem Tag. Die festen Zeiten der Woche bleiben unverändert.",
        steps: [
          "Öffnen Sie beim Kind den Reiter `Betreuungszeiten`.",
          "Wählen Sie bei Bedarf `Vorherige Woche` oder `Nächste Woche`.",
          "Wählen Sie auf der Karte des Tages `Ausnahme`.",
          "Wählen Sie unter `Ankunft` `Regulär`, `Andere Zeit` oder `Kommt nicht`.",
          "Wählen Sie unter `Abholung` `Regulär`, `Andere Zeit` oder `Keine Abholung`.",
          "Tragen Sie bei Bedarf unter `Hinweise nur für diesen Tag` etwas ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "Die gespeicherten Angaben gelten sofort. Eine Notiz ohne Abholzeit steht auf der Kinderkarte unter `Kommt heute nicht`. Eine Ausnahme ändert den regelmäßigen Wochenplan nicht.",
    differences: [
      "Die Kennzeichnung `Für Eltern sichtbar` zeigt Angaben, die Eltern sehen.",
      "Kommen die Betreuungstage aus Buchungen? Dann lassen sich die Tage nicht auswählen. Ändern Sie nur Zeiten an gebuchten Tagen.",
      "Fehlt `Bearbeiten` oder `Wochenplan`? Fragen Sie Ihre Leitung nach den nötigen Rechten.",
    ],
    notes: [
      "Steht unter dem Feld `Ankunft` die Auswahl `Nach Schulstunde`? Wählen Sie zum Beispiel `5. Stunde`. moto trägt die passende Uhrzeit ein.",
      // #3371: Knopf erscheint nur mit gepflegter Vorgabe unter einem leeren
      // Feld an einem Betreuungstag (care-weekly-plan-editor.tsx).
      "Steht unter einem leeren Feld zum Beispiel `16:00 Uhr eintragen`? Ein Klick trägt die übliche Zeit Ihrer Schule ein.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.carePlan,
      HELP_TOPICS.dayLog,
    ],
  };
}

function webAttendanceTopic(presenceMode: HelpPresenceMode): HelpTopic {
  const result =
    presenceMode === "detailed"
      ? "Nach dem Anmelden steht das Kind zunächst unter `Unterwegs`. Ein Raum wird erst bei der nächsten Zuordnung eingetragen. Nach dem Abmelden steht das Kind unter `Zuhause`."
      : presenceMode === "binary"
        ? "Nach dem Anmelden steht das Kind auf `Anwesend`. Nach dem Abmelden steht es auf `Abwesend`."
        : "moto zeigt nach der Aktion den neuen Anwesenheitsstatus des Kindes.";

  return {
    id: HELP_TOPICS.webAttendance,
    title: "Ein Kind an- oder abmelden",
    question: "Wie melde ich ein Kind an oder ab?",
    summary: "Tragen Sie ein, ob ein Kind heute in der OGS ist.",
    group: "kinder",
    audience: "caregiver",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "Über die Angaben des Kindes",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Anmelden` oder `Abmelden`.",
          "Prüfen Sie den Namen im Fenster.",
          "Bestätigen Sie mit `Anmelden` oder `Geht nach Hause`.",
        ],
      },
      {
        title: "Direkt in der Kinderliste",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Wählen Sie `An- & Abmelden`.",
          "Wählen Sie die Karte des Kindes.",
          "Prüfen Sie den neuen Status auf der Karte.",
          "Wählen Sie danach `Fertig`.",
        ],
      },
    ],
    result,
    notes: [
      "Möchten Sie mehrere Kinder ändern? Wählen Sie `Mehrere`.",
      // Live geprueft: in `Direkt` schreibt jeder Tipp sofort, ohne Rueckfrage.
      "In `Direkt` meldet jeder Tipp sofort an oder ab. Auf der Karte steht `Tippen zum Anmelden`.",
    ],
    differences: [
      "moto zeigt nur die Aktion, die zum aktuellen Status passt.",
      "Fehlt `Anmelden` oder `Abmelden`? Fragen Sie Ihre Leitung nach dem Recht zum An- und Abmelden.",
    ],
    troubleshooting: HELP_TOPICS.attendanceProblem,
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.changeLocation,
      HELP_TOPICS.absences,
    ],
  };
}

function changeLocationTopic(presenceMode: HelpPresenceMode): HelpTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.changeLocation,
      title: "Den Aufenthaltsort eines Kindes ändern",
      question: "Wie ändere ich den Aufenthaltsort eines Kindes?",
      summary:
        "Bei einfacher Anwesenheit erfasst moto keine Räume oder Aufenthaltsorte.",
      group: "kinder",
      audience: "caregiver",
      icon: "Eye",
      steps: [
        "Öffnen Sie das Kind unter `Alle Kinder`.",
        "Wählen Sie `Anmelden` oder `Abmelden`.",
      ],
      result:
        "moto zeigt nur, ob das Kind anwesend oder abwesend ist. Ein Raum wird nicht gespeichert.",
      related: [HELP_TOPICS.webAttendance, HELP_TOPICS.studentSearch],
    };
  }
  if (presenceMode === "unknown") {
    return {
      id: HELP_TOPICS.changeLocation,
      title: "Den Aufenthaltsort eines Kindes ändern",
      question: "Wie ändere ich den Aufenthaltsort eines Kindes?",
      summary:
        "Ob moto Aufenthaltsorte erfasst, hängt von der Arbeitsweise Ihrer OGS ab.",
      group: "kinder",
      audience: "caregiver",
      icon: "Eye",
      steps: [
        "Prüfen Sie, ob der Bereich `Räume` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie den aktuellen Raum des Kindes und wählen Sie einen neuen Zielraum.",
        "Fehlt der Bereich? Dann erfasst moto nur, ob das Kind anwesend ist.",
      ],
      result:
        "moto zeigt den neuen Raum nur bei einer OGS mit detaillierter Anwesenheit.",
      related: [HELP_TOPICS.webAttendance, HELP_TOPICS.rooms],
    };
  }

  return {
    id: HELP_TOPICS.changeLocation,
    title: "Ein Kind in einen anderen Raum verschieben",
    question: "Wie verschiebe ich ein Kind in einen anderen Raum?",
    summary: "Ändern Sie den Raum eines anwesenden Kindes.",
    group: "kinder",
    audience: "caregiver",
    icon: "MapPin",
    requirements: [
      "Ihre OGS erfasst die Räume der Kinder.",
      "Das Kind ist anwesend.",
      "Sie beaufsichtigen den aktuellen Raum oder den Zielraum.",
      "Im Zielraum läuft eine Aufsicht, oder er ist ein offener Raum.",
    ],
    steps: [
      "Öffnen Sie `Räume`.",
      "Öffnen Sie den aktuellen Raum des Kindes.",
      "Wählen Sie das Kind unter `Kinder im Raum` aus.",
      "Wählen Sie unter `Zielraum` den neuen Raum.",
      "Wählen Sie `In Raum setzen`.",
    ],
    result: "moto zeigt sofort den neuen Raum. Der bisherige Raumbesuch endet.",
    notes: ["Sie können mehrere Kinder auswählen und gemeinsam verschieben."],
    differences: [
      "Hat das Kind noch keinen Raum? Wählen Sie auf `Räume` die Karte `Unterwegs`. Wählen Sie dort das Kind und den Zielraum.",
      "Steht hinter dem Zielraum `(offener Raum)`? Dort braucht es keine Aufsicht. Das Kind nutzt den Raum dann, ohne an einem Angebot teilzunehmen.",
      "Fehlt der Zielraum? Prüfen Sie dort `Aktuelle Aufsicht`.",
      "Beaufsichtigen Sie keinen der beiden Räume? Bitten Sie eine zuständige Aufsicht. Oder fragen Sie Ihre Leitung.",
    ],
    troubleshooting: HELP_TOPICS.missingChildOrGroup,
    related: [
      HELP_TOPICS.webAttendance,
      HELP_TOPICS.rooms,
      HELP_TOPICS.activeSupervision,
    ],
  };
}

function absencesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.absences,
    title: "Ein Kind krank oder entschuldigt melden",
    question: "Wie melde ich ein Kind krank oder entschuldigt?",
    summary:
      "Tragen Sie eine Krankmeldung oder Entschuldigung für die passenden Tage ein.",
    group: "kinder",
    audience: "caregiver",
    icon: "BellRing",
    steps: [],
    instructionGroups: [
      {
        title: "Ein Kind krankmelden",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Krank melden`.",
          "Wählen Sie `Einzelne Tage` oder `Zeitraum`.",
          "Wählen Sie die Tage der Krankmeldung aus.",
          "Tragen Sie bei Bedarf einen Grund ein.",
          "Wählen Sie `Krankmelden`.",
        ],
      },
      {
        title: "Ein Kind entschuldigen",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Entschuldigen`.",
          "Wählen Sie `Ganzer Tag` oder `Ab Uhrzeit`.",
          "Bei `Ganzer Tag`: Wählen Sie `Einzelne Tage` oder `Zeitraum`. Wählen Sie danach die Tage aus.",
          "Bei `Ab Uhrzeit`: Wählen Sie einen Tag. Tragen Sie danach die passende Uhrzeit ein.",
          "Tragen Sie bei Bedarf einen Grund ein.",
          "Wählen Sie `Entschuldigen`.",
        ],
      },
      {
        title: "Bei `Ab Uhrzeit`",
        steps: [
          "`Ab Uhrzeit` gilt für genau einen Tag.",
          "Spätere Betreuungszeiten sind an diesem Tag entschuldigt.",
          "Ist keine Abholzeit eingetragen, übernimmt moto die gewählte Uhrzeit.",
        ],
        ordered: false,
      },
      {
        title: "Die Meldung für heute aufheben",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Gesund melden` oder `Entschuldigung aufheben`.",
          "Bestätigen Sie im Fenster mit `Gesundmelden` oder `Entschuldigung aufheben`.",
        ],
      },
      {
        title: "Einen geplanten Tag entfernen",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Krank melden` oder `Entschuldigen`.",
          "Suchen Sie den Tag unter `Bereits krank` oder `Bereits entschuldigt`.",
          "Öffnen Sie beim Tag das Menü mit den drei Punkten.",
          "Wählen Sie `Entfernen`.",
          "Wählen Sie `Entfernen bestätigen`. Wählen Sie danach `Eintrag entfernen`.",
        ],
      },
      {
        // Live geprueft: die Liste `Bereits ab Uhrzeit entschuldigt` erscheint,
        // sobald `Ab Uhrzeit` gewaehlt ist. Ein Tag muss dafuer nicht gewaehlt sein.
        title: "Eine Entschuldigung `Ab Uhrzeit` entfernen",
        steps: [
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Entschuldigen`.",
          "Wählen Sie `Ab Uhrzeit`.",
          "Öffnen Sie unter `Bereits ab Uhrzeit entschuldigt` beim Tag das Menü mit den drei Punkten.",
          "Wählen Sie `Entfernen`. Für eine andere Uhrzeit wählen Sie `Bearbeiten`.",
          "Wählen Sie `Entfernen bestätigen`. Wählen Sie danach `Teilentschuldigung entfernen`.",
        ],
      },
    ],
    result:
      "Die Krankmeldung oder Entschuldigung ist gespeichert. Das Kind erscheint am gewählten Tag mit dem passenden Status.",
    notes: [
      "Die heutige Meldung aufheben ändert geplante Tage in der Zukunft nicht.",
    ],
    differences: [
      "Fehlen `Krank melden` und `Entschuldigen`? Fragen Sie Ihre Leitung, ob Ihre Rolle Abwesenheiten bearbeiten darf.",
      "Steht dort `Gesund melden`? Dann ist das Kind heute krank gemeldet. Heben Sie erst die heutige Meldung auf.",
      "Fehlt `Ab Uhrzeit`? Dann dürfen Sie nur ganze Tage entschuldigen.",
      "Steht beim Tag `Automatisch (Abholzeit)`? Dann kommt die Entschuldigung aus der früheren Abholzeit. Ändern Sie dafür die Abholzeit des Tages.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.dayLog,
      HELP_TOPICS.webAttendance,
    ],
  };
}

function dayLogTopic(groupMode: HelpGroupMode): HelpTopic {
  // Ohne feste Gruppen zeigt die Tagesauswertung eine gemeinsame Liste.
  const openCare = groupMode === "open_care";
  return {
    id: HELP_TOPICS.dayLog,
    title: "Den heutigen Betreuungstag prüfen",
    question: "Wie prüfe ich den heutigen Betreuungstag?",
    summary: "Prüfen Sie für heute, welche Kinder anwesend oder abwesend sind.",
    group: "kinder",
    audience: ["caregiver", "lead"],
    icon: "CalendarCheck",
    requirements: ["Ihre OGS hat das `Anwesenheitsprotokoll` eingeschaltet."],
    steps: [],
    instructionGroups: [
      {
        title: "Den Betreuungstag prüfen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf und öffnen Sie `Tagesauswertung`.",
          openCare
            ? "Prüfen Sie oben die Zahlen für alle Kinder."
            : "Prüfen Sie oben die Zahlen für alle Gruppen.",
          ...(openCare ? [] : ["Wählen Sie bei einer Gruppe `Details`."]),
          "Prüfen Sie die Kinder in den einzelnen Bereichen.",
          "Wählen Sie ein Kind, um seine Angaben zu öffnen.",
        ],
      },
      {
        title: "Die Auswertung ausgeben",
        steps: [
          ...(openCare
            ? ["Bleiben Sie für alle Kinder in der Übersicht."]
            : [
                "Bleiben Sie für alle Gruppen in der Übersicht.",
                "Für eine einzelne Gruppe öffnen Sie deren `Details`.",
              ]),
          "Wählen Sie `Drucken`, `PDF` oder `Excel`.",
        ],
      },
    ],
    result:
      "Sie sehen für heute, welches Kind da ist. Die Liste ist danach geordnet. Kinder ohne Meldung stehen unter `Unentschuldigt abwesend`. Vergangene Tage können Sie hier nicht auswählen.",
    differences: [
      "Fehlt `Tagesauswertung`? Bitten Sie Ihre Leitung, das `Anwesenheitsprotokoll` unter `Einstellungen` und `Datenschutz` einzuschalten.",
      "Steht dort `Anwesenheitsprotokoll ist ausgeschaltet`? Dann ist es für Ihre Schule nicht eingeschaltet.",
    ],
    related: [
      HELP_TOPICS.absences,
      HELP_TOPICS.emergency,
      HELP_TOPICS.ownGroups,
      HELP_TOPICS.leadAbsenceReport,
      HELP_TOPICS.leadStatistics,
    ],
  };
}

function emergencyTopic(presenceMode: HelpPresenceMode): HelpTopic {
  const locationDescription =
    presenceMode === "detailed"
      ? "Bei detaillierter Anwesenheit zeigt die Liste den aktuellen Ort oder Raum des Kindes."
      : presenceMode === "binary"
        ? "Bei einfacher Anwesenheit zeigt die Liste `Anwesend` statt eines Raums."
        : "Je nach Arbeitsweise Ihrer OGS zeigt die Liste einen Raum oder nur `Anwesend`.";

  return {
    id: HELP_TOPICS.emergency,
    title: "Eine Notfallliste drucken",
    question: "Wie drucke ich eine Notfallliste?",
    summary:
      "Nutzen Sie die Liste zum Beispiel bei einem geplanten Feueralarm. Damit prüfen Sie am Sammelplatz, ob alle anwesenden Kinder da sind.",
    group: "kinder",
    audience: ["caregiver", "lead"],
    icon: "ShieldAlert",
    requirements: ["Ihr Konto hat das Recht `Benutzerinformationen ansehen`."],
    steps: [],
    instructionGroups: [
      {
        title: "Die Notfallliste drucken",
        description: "Ihr Gerät ist mit einem Drucker verbunden.",
        steps: [
          "Öffnen Sie unten in der Seitenleiste `Notfall`.",
          "Wählen Sie `Notfallliste drucken`.",
          "Wählen Sie im Druckfenster den passenden Drucker.",
          "Prüfen Sie die Einstellungen im Druckfenster.",
          "Starten Sie den Druck.",
        ],
      },
      {
        title: "Die Notfallliste als PDF speichern",
        steps: [
          "Öffnen Sie unten in der Seitenleiste `Notfall`.",
          "Wählen Sie `PDF herunterladen`.",
        ],
      },
    ],
    result: [
      "Die Liste enthält alle Kinder, die beim Erstellen anwesend sind, mit ihrer Klasse und wichtigen Kontakten.",
      locationDescription,
      "Je nach Einstellung enthält die Liste auch `Gesundheitsinfos`.",
      "`Nicht hinterlegt` bedeutet: Es fehlen Gesundheitsinfos, eine Allergie ist trotzdem möglich.",
    ].join(" "),
    notes: [
      "Erstellen Sie im Notfall eine neue Liste. Ein älterer Ausdruck kann veraltet sein.",
      "Lassen Sie den Ausdruck nicht offen liegen.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.dayLog,
      HELP_TOPICS.ownGroups,
    ],
  };
}

function ownGroupsTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): HelpTopic {
  if (groupMode === "open_care") {
    return {
      id: HELP_TOPICS.ownGroups,
      title: "Meine Gruppen ansehen",
      question: "Wie sehe ich meine Gruppen an?",
      summary:
        "Bei offener Betreuung gibt es keine festen eigenen Gruppen in moto.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "Users",
      steps: [
        "Öffnen Sie `Alle Kinder`.",
        "Nutzen Sie die Suche oder die Filter für die benötigten Kinder.",
      ],
      result:
        "Sie sehen die Kinder der OGS gemeinsam. Der Bereich `Meine Gruppen` wird nicht angezeigt.",
      related: [HELP_TOPICS.studentSearch, HELP_TOPICS.webAttendance],
    };
  }
  if (groupMode === "unknown") {
    return {
      id: HELP_TOPICS.ownGroups,
      title: "Meine Gruppen ansehen",
      question: "Wie sehe ich meine Gruppen an?",
      summary:
        "Ob Sie eigene Gruppen sehen, hängt von der Arbeitsweise Ihrer OGS ab.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "Users",
      steps: [
        "Prüfen Sie, ob `Meine Gruppen` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie dort die gewünschte Gruppe.",
        "Fehlt der Bereich? Öffnen Sie `Alle Kinder` und nutzen Sie Suche und Filter.",
      ],
      result:
        "Sie sehen die Kinder entweder in festen eigenen Gruppen oder in einer gemeinsamen Kinderliste.",
      related: [HELP_TOPICS.studentSearch, HELP_TOPICS.missingChildOrGroup],
    };
  }

  const statusResult =
    presenceMode === "detailed"
      ? "Die Karten zeigen die geplanten Zeiten und den aktuellen Aufenthaltsort der Kinder."
      : "Die Karten zeigen die geplanten Zeiten und die Anwesenheit der Kinder.";

  return {
    id: HELP_TOPICS.ownGroups,
    title: "Meine Gruppen ansehen",
    question: "Wie sehe ich meine Gruppen an?",
    summary: "Öffnen Sie Ihre festen Gruppen und die zugehörigen Kinder.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "Users",
    steps: [
      "Öffnen Sie `Meine Gruppen` in der Seitenleiste.",
      "Wählen Sie die gewünschte Gruppe.",
      "Prüfen Sie die Zahlen für `krank` und `entschuldigt`.",
      "Suchen Sie bei Bedarf über `Name suchen...` nach einem Kind.",
      // Live geprueft: die Seite hat keinen Knopf `Filter`, sondern einen
      // Umschalter fuer die Reihenfolge und eine Auswahl fuer den Ort.
      "Ordnen Sie die Liste nach `Alphabetisch`, `Nächste Ankunft` oder `Nächste Abholung`.",
      "Grenzen Sie über `Alle Orte` auf einen Aufenthaltsort ein.",
      "Wählen Sie die Karte eines Kindes, um seine Angaben zu öffnen.",
    ],
    result: statusResult,
    differences: [
      "In der Seitenleiste steht hinter jeder Gruppe, wie viele Kinder gerade da sind.",
      "Eine vorübergehend übernommene Gruppe erscheint ebenfalls unter `Meine Gruppen`.",
      "Fehlt eine Gruppe? Bitten Sie Ihre Leitung, Ihre Gruppenzuordnung zu prüfen.",
    ],
    troubleshooting: HELP_TOPICS.missingChildOrGroup,
    related: [
      HELP_TOPICS.transferGroup,
      HELP_TOPICS.activeSupervision,
      HELP_TOPICS.studentSearch,
    ],
  };
}

function transferGroupTopic(groupMode: HelpGroupMode): HelpTopic {
  if (groupMode === "open_care") {
    return {
      id: HELP_TOPICS.transferGroup,
      title: "Meine Gruppe vorübergehend übergeben",
      question: "Wie übergebe ich meine Gruppe vorübergehend?",
      summary:
        "Bei offener Betreuung gibt es keine feste eigene Gruppe zum Übergeben.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "ArrowsLeftRight",
      steps: [
        "Sie müssen keine Gruppe übergeben.",
        "Öffnen Sie `Alle Kinder`, um mit den Kindern der OGS zu arbeiten.",
      ],
      result:
        "Alle Betreuungskräfte arbeiten ohne feste Gruppenzuordnung mit der gemeinsamen Kinderliste.",
      related: [HELP_TOPICS.ownGroups, HELP_TOPICS.studentSearch],
    };
  }
  if (groupMode === "unknown") {
    return {
      id: HELP_TOPICS.transferGroup,
      title: "Meine Gruppe vorübergehend übergeben",
      question: "Wie übergebe ich meine Gruppe vorübergehend?",
      summary: "Eine Übergabe ist nur bei festen Gruppen möglich.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "ArrowsLeftRight",
      steps: [
        "Prüfen Sie, ob `Meine Gruppen` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie Ihre Gruppe und das Menü mit den drei Punkten.",
        "Fehlt der Bereich? Dann müssen Sie keine feste Gruppe übergeben.",
      ],
      result: "Eine andere Betreuungskraft erhält Zugriff bis zum Tagesende.",
      related: [HELP_TOPICS.ownGroups, HELP_TOPICS.missingChildOrGroup],
    };
  }

  return {
    id: HELP_TOPICS.transferGroup,
    title: "Meine Gruppe vorübergehend übergeben",
    question: "Wie übergebe ich meine Gruppe vorübergehend?",
    summary:
      "Geben Sie einer anderen Betreuungskraft bis zum Tagesende Zugriff auf Ihre Gruppe.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "ArrowsLeftRight",
    requirements: ["Sie sind dieser Gruppe fest zugeordnet."],
    steps: [],
    instructionGroups: [
      {
        title: "Gruppe übergeben",
        steps: [
          "Öffnen Sie Ihre Gruppe unter `Meine Gruppen`.",
          "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
          "Wählen Sie `Gruppe übergeben`.",
          "Wählen Sie unter `Übergeben an:` im Feld `Fachkraft auswählen...` die Person aus.",
          "Wählen Sie `Übergeben`.",
        ],
      },
      {
        title: "Eine Übergabe früher beenden",
        description:
          "Eine Übergabe gilt nur für heute. Sie können sie auch vorher beenden.",
        steps: [
          "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
          "Wählen Sie `Gruppe übergeben`.",
          "Wählen Sie unter `Aktuell übergeben an:` neben der Person `Zurücknehmen`.",
          "Bestätigen Sie mit `Übergabe zurücknehmen`.",
        ],
      },
    ],
    result:
      "Die andere Betreuungskraft sieht die Gruppe bis zum Ende des Tages unter `Meine Gruppen`. Sie behalten selbst den Zugriff.",
    notes: [
      "Sie können die Gruppe an mehrere Betreuungskräfte übergeben. Wiederholen Sie dafür die Schritte.",
      "Alle Übergaben stehen im Fenster unter `Aktuell übergeben an:`.",
    ],
    troubleshootingDetails: [
      "Fehlt der Eintrag `Gruppe übergeben` im Menü? Dann betreuen Sie die Gruppe nur als Vertretung. Eine Vertretung kann die Gruppe nicht weitergeben.",
      "Steht dort `Keine pädagogische Fachkraft verfügbar`? Dann gibt es niemanden zum Übergeben. Fragen Sie Ihre Leitung.",
    ],
    related: [HELP_TOPICS.ownGroups, HELP_TOPICS.missingChildOrGroup],
  };
}

function myScheduleTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.mySchedule,
    title: "Meine Termine und Einsätze ansehen",
    question: "Wo sehe ich meine Termine und Einsätze?",
    summary:
      "Prüfen Sie persönliche Termine, geplante Schichten und Ihre Einsätze im Betreuungsplan.",
    group: "tagesplanung",
    audience: ["caregiver", "lead"],
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Termine und Einsätze ansehen",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Mein Kalender`.",
          "Bleiben Sie im Tab `Meine Termine`.",
          "Wählen Sie oben `Tag`, `Woche` oder `Monat`.",
          "Nutzen Sie die Pfeile oder `Heute`, um zum gewünschten Zeitraum zu wechseln.",
          "Wählen Sie einen Eintrag, um Uhrzeit, Ort und weitere Angaben zu sehen.",
        ],
      },
      {
        title: "Die Farben verstehen",
        ordered: false,
        steps: [
          "Grün zeigt Termine und Einladungen.",
          "Blau zeigt Ihre Einsätze im Betreuungsplan.",
          "Orange zeigt Ihre Schichten aus dem Dienstplan.",
        ],
      },
      {
        title: "Den Kalender abonnieren",
        description:
          "Nutzen Sie das Abo, wenn Sie hauptsächlich Ihren persönlichen Kalender verwenden. Neue und geänderte Einträge aus moto erscheinen dort automatisch.",
        steps: [
          "Gehen Sie unter dem Kalender zum Bereich `Kalender abonnieren`.",
          "Wählen Sie `Abo-Link anzeigen`.",
          "Wählen Sie `Im Kalender abonnieren`. Oder kopieren Sie den Link in Ihren persönlichen Kalender.",
          "Der Link ist persönlich und wird nur einmal angezeigt. Geben Sie ihn nicht weiter.",
        ],
      },
    ],
    result:
      "Sie haben einen Überblick über Ihre Termine und Einsätze. Das Kalender-Abo übernimmt neue, geänderte und abgesagte Einträge automatisch.",
    notes: [
      "Mit `Sa/So` blenden Sie das Wochenende ein. Eine Zahl zeigt, dass dort Einträge liegen.",
      "Bei einer Einladung wählen Sie im geöffneten Termin `Zusagen` oder `Absagen`.",
    ],
    differences: [
      "Das Kalender-Abo ist nur zum Lesen. Änderungen in Ihrem persönlichen Kalender werden nicht an moto übertragen.",
    ],
    related: [HELP_TOPICS.carePlan, HELP_TOPICS.trackWorkTime],
  };
}

function carePlanTopic(presenceMode: HelpPresenceMode): HelpTopic {
  return {
    id: HELP_TOPICS.carePlan,
    title: "Den Betreuungsplan ansehen",
    question: "Wie sehe ich den Betreuungsplan an?",
    summary:
      "Prüfen Sie, welche Betreuung für die Schule geplant ist und wo Sie selbst eingesetzt sind.",
    group: "tagesplanung",
    audience: ["caregiver", "lead"],
    icon: "Table",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Mein Kalender`.",
      "Wählen Sie den Tab `Betreuungsplan`.",
      "Wählen Sie oben `Tag` oder `Woche`.",
      "Nutzen Sie die Pfeile oder `Heute`, um zum passenden Tag zu wechseln.",
      // Ohne Raumzuordnung steht im Block auch kein Raum.
      presenceMode === "binary"
        ? "Wählen Sie einen Block, um Zeit, Betreuungsteam und Kinder zu sehen."
        : "Wählen Sie einen Block, um Zeit, Raum, Betreuungsteam und Kinder zu sehen.",
    ],
    result:
      "Sie sehen, was die OGS geplant hat und wo Sie eingesetzt sind. Als Betreuungskraft können Sie den Plan nicht verändern.",
    differences: [
      // Live geprueft: die Kopfzeile des Reiters traegt das Kennzeichen.
      "Steht oben rechts `Nur ansehen`? Dann dürfen Sie den Plan lesen, aber nicht ändern.",
    ],
    troubleshootingDetails: [
      "Der Tab `Betreuungsplan` fehlt? Bitten Sie Ihre Leitung, Ihren Zugang zu prüfen.",
      "Steht dort `Noch kein Planungszeitraum`? Dann hat Ihre Leitung den Zeitraum noch nicht angelegt.",
    ],
    related: [HELP_TOPICS.mySchedule, HELP_TOPICS.activeSupervision],
  };
}

/**
 * Der Tagesplan (#2383) ist in der echten App der oberste Eintrag im
 * Tagesbetrieb und hatte bisher keine Hilfeseite. Inhalt belegt aus
 * `components/timetable/tagesplan-view.tsx`: chronologische Blockliste,
 * Jetzt-Linie, Tippen auf einen laufenden Block oeffnet dessen Kinderliste,
 * ein startbarer Block startet ueber `Starten` (#3622). Bei einfacher Anwesenheit greift
 * der BinaryModeGuard, deshalb steht das Thema in DETAILED_ONLY_TOPIC_IDS.
 */
function dayPlanTopic(presenceMode: HelpPresenceMode): HelpTopic {
  return {
    id: HELP_TOPICS.dayPlan,
    title: "Den Betreuungstag im Tagesplan verfolgen",
    question: "Wo sehe ich den Betreuungstag von heute?",
    summary:
      "Der Tagesplan zeigt die Betreuungsblöcke des Tages der Reihe nach.",
    group: "tagesplanung",
    audience: "caregiver",
    icon: "ClockAfternoon",
    requirements: [
      "Ihre OGS hat den Betreuungsplan eingeschaltet.",
      "Sie dürfen den Betreuungsplan sehen.",
    ],
    steps: [
      "Öffnen Sie `Tagesplan` in der Seitenleiste.",
      "Sie sehen die Blöcke des Tages von früh nach spät.",
      "Eine Linie zeigt, wo Sie gerade im Tag stehen.",
      "Tippen Sie auf einen laufenden Block. Das öffnet seine Kinderliste.",
      "Hat Ihr Block noch nicht begonnen? Wählen Sie `Starten`.",
    ],
    result:
      "Nach `Starten` öffnet moto die Kinderliste des Blocks in `Aktuelle Aufsicht`.",
    differences: [
      "`Läuft` heißt: der Block ist gerade aktiv. `Beendet` und `Nicht gestartet` können Sie nur ansehen.",
      "`Fällt aus` zeigt zusätzlich den Grund.",
      "Welche Blöcke Sie sehen, legt Ihre OGS fest. Manche sehen den ganzen Tag der Schule, andere nur die eigene Einteilung.",
      "Ob Sie nur eigene oder alle Blöcke starten dürfen, legt Ihre OGS fest. Wer einen fremden Block startet, wird nicht zur Aufsicht.",
      ...(presenceMode === "unknown"
        ? ["Bei einfacher Anwesenheit gibt es den Tagesplan nicht."]
        : []),
    ],
    notes: ["Mit den Pfeilen oben rechts sehen Sie einen anderen Tag."],
    troubleshootingDetails: [
      // Live geprueft: der haeufigste Leerzustand einer Betreuungskraft.
      "Steht dort `Heute ist keine Betreuung geplant`? Dann sind Sie für diesen Tag nicht eingeteilt. Die Einteilung macht Ihre Leitung.",
      "Steht dort `Der Betreuungsplan ist ausgeschaltet`? Dann nutzt Ihre OGS diese Funktion nicht.",
      "Steht dort `Kein Zugriff auf den Betreuungsplan`? Bitten Sie Ihre Leitung, Ihren Zugang zu prüfen.",
    ],
    related: [HELP_TOPICS.carePlan, HELP_TOPICS.activeSupervision],
  };
}

function roomsTopic(presenceMode: HelpPresenceMode): HelpTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.rooms,
      title: "Räume und ihre aktuelle Belegung ansehen",
      question: "Wo sehe ich Räume und ihre aktuelle Belegung?",
      summary: "Bei einfacher Anwesenheit ordnet moto Kinder keinen Räumen zu.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "DoorOpen",
      steps: [
        "Öffnen Sie `Alle Kinder`.",
        "Prüfen Sie dort, welche Kinder anwesend oder abwesend sind.",
      ],
      result:
        "Der Bereich `Räume` wird nicht angezeigt. moto speichert dann keinen Aufenthaltsort.",
      related: [HELP_TOPICS.webAttendance, HELP_TOPICS.studentSearch],
    };
  }
  if (presenceMode === "unknown") {
    return {
      id: HELP_TOPICS.rooms,
      title: "Räume und ihre aktuelle Belegung ansehen",
      question: "Wo sehe ich Räume und ihre aktuelle Belegung?",
      summary:
        "Der Bereich ist nur sichtbar, wenn Ihre OGS Aufenthaltsorte erfasst.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "DoorOpen",
      steps: [
        "Prüfen Sie, ob `Räume` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie dort einen Raum und prüfen Sie die Belegung.",
        "Fehlt der Bereich? Öffnen Sie `Alle Kinder`, um die Anwesenheit zu prüfen.",
      ],
      result:
        "Raumbelegungen werden nur bei detaillierter Anwesenheit angezeigt.",
      related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.webAttendance],
    };
  }

  return {
    id: HELP_TOPICS.rooms,
    title: "Räume und ihre aktuelle Belegung ansehen",
    question: "Wo sehe ich Räume und ihre aktuelle Belegung?",
    summary:
      "Prüfen Sie, welche Räume frei oder belegt sind und wer sich dort befindet.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "DoorOpen",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Raum und seine Belegung ansehen",
        steps: [
          "Öffnen Sie `Räume` in der Seitenleiste.",
          "Suchen Sie den Raum über `Raum suchen...`.",
          // Live geprueft: kein Knopf `Filter`, sondern ein Umschalter
          // `Alle` / `Belegt` / `Frei` und eine Auswahl `Alle Gebäude`.
          "Oder wählen Sie oben `Belegt` oder `Frei`.",
          "Grenzen Sie über `Alle Gebäude` auf ein Gebäude ein.",
          "Wählen Sie einen Raum aus.",
          "Prüfen Sie die aktuelle Aktivität, die Aufsicht und die Kinder im Raum.",
          "Wählen Sie bei Bedarf ein Kind, um seine Angaben zu öffnen.",
        ],
      },
      {
        title: "Kinder ohne Raum in einen Raum setzen",
        description:
          "Führen Sie selbst die Aufsicht, stehen nur Ihre Räume zur Wahl. Mit dem Recht für alle Räume steht die ganze Liste da. Dann stehen auch offene Räume in der Liste, mit dem Zusatz `(offener Raum)`. So bleibt ein Kind nach seinem Angebot im Raum, ohne an einem Angebot teilzunehmen.",
        steps: [
          "Wählen Sie oben die Karte `Unterwegs`.",
          "Wählen Sie die gewünschten Kinder aus.",
          "Wählen Sie unter `Zielraum` den Raum aus.",
          "Wählen Sie `In Raum setzen`.",
        ],
      },
    ],
    result:
      "Sie sehen die aktuelle Belegung des Raums. Unter `Unterwegs` stehen anwesende Kinder, die noch keinem Raum zugeordnet sind. Die Anzeige ändert sich, sobald Kinder den Raum wechseln.",
    troubleshootingDetails: [
      "Steht unter `Zielraum` `Keine aktiven Räume`? Dann läuft gerade keine Aufsicht, in die Sie Kinder setzen können.",
      "Fehlt unter `Unterwegs` die Auswahl der Kinder? Dann erfasst Ihre OGS die Anwesenheit nicht über die App. Fragen Sie Ihre Leitung.",
    ],
    related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.changeLocation],
  };
}

function activeSupervisionTopic(presenceMode: HelpPresenceMode): HelpTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.activeSupervision,
      title: "Eine Aufsicht starten, führen und beenden",
      question: "Wie arbeite ich mit einer laufenden Aufsicht?",
      summary:
        "Bei einfacher Anwesenheit führt moto keine Aufsichten für einzelne Räume oder Aktivitäten.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "Eye",
      steps: [
        "Öffnen Sie `Alle Kinder`.",
        "Nutzen Sie `An- & Abmelden`, um die Anwesenheit zu erfassen.",
      ],
      result:
        "Der Bereich `Aktuelle Aufsicht` wird nicht angezeigt. moto speichert nur, ob ein Kind anwesend ist.",
      related: [HELP_TOPICS.webAttendance, HELP_TOPICS.rooms],
    };
  }
  if (presenceMode === "unknown") {
    return {
      id: HELP_TOPICS.activeSupervision,
      title: "Eine Aufsicht starten, führen und beenden",
      question: "Wie arbeite ich mit einer laufenden Aufsicht?",
      summary:
        "Der Bereich ist nur sichtbar, wenn Ihre OGS Räume und Aktivitäten erfasst.",
      group: "gruppen",
      audience: ["caregiver", "lead"],
      icon: "Eye",
      steps: [
        "Prüfen Sie, ob `Aktuelle Aufsicht` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Öffnen Sie dort einen Raum oder geplanten Termin.",
        "Fehlt der Bereich? Erfassen Sie die Anwesenheit unter `Alle Kinder`.",
      ],
      result: "Diese Aufsicht gibt es nur bei detaillierter Anwesenheit.",
      related: [HELP_TOPICS.rooms, HELP_TOPICS.webAttendance],
    };
  }

  return {
    id: HELP_TOPICS.activeSupervision,
    title: "Eine Aufsicht starten, führen und beenden",
    question: "Wie arbeite ich mit einer laufenden Aufsicht?",
    summary:
      "Übernehmen Sie einen Raum oder einen geplanten Termin und halten Sie die Kinderliste aktuell.",
    group: "gruppen",
    audience: ["caregiver", "lead"],
    icon: "Eye",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Aufsicht starten",
        steps: [
          "Öffnen Sie `Aktuelle Aufsicht`.",
          "Wählen Sie unter `Als Nächstes` einen geplanten Termin.",
          "Starten Sie ihn, sobald die Schaltfläche freigegeben ist.",
          "Oder wählen Sie bei einem freien Raum `Beaufsichtigen`.",
          // Live geprueft: der gruene Streifen steht immer oben auf der
          // Seite, auch ohne geplanten Block und ohne NFC.
          "Oder wählen Sie `Spontane Aktivität starten`, wenn nichts geplant ist.",
          "Prüfen Sie Raum, Aktivität und Betreuungsteam.",
        ],
      },
      {
        title: "Die Kinderliste führen",
        steps: [
          "Suchen Sie ein Kind in der laufenden Aufsicht.",
          "Wählen Sie `Hinzufügen`, wenn das Kind anwesend ist und zur Aufsicht kommen soll.",
          "Prüfen Sie geplante Abholzeiten und Hinweise in der Liste.",
          "Wechselt ein Kind den Raum oder geht nach Hause? Ändern Sie seinen Aufenthaltsort.",
        ],
      },
      {
        title: "Die Aufsicht beenden",
        steps: [
          "Wählen Sie `Beenden`.",
          "Prüfen Sie im Fenster, ob noch Kinder anwesend sind.",
          "Bestätigen Sie das Ende der Aufsicht.",
        ],
      },
    ],
    result:
      "Die Aufsicht ist abgeschlossen. Ein selbst beendeter Termin kann für kurze Zeit über `Rückgängig` wieder geöffnet werden.",
    differences: [
      "Bei einem laufenden Termin können weitere Betreuungskräfte der Aufsicht beitreten.",
      "Ein geplanter Termin hat feste Zeiten. Vorher sind Start oder Ende möglicherweise gesperrt.",
      "Steht am Termin `Nur für Eingeplante`? Dann dürfen hier nur eingeplante Kräfte starten. Ihre OGS kann das Starten für das ganze Team freigeben.",
      "Sie starten einen Termin, für den Sie nicht eingeplant sind? Dann werden Sie nicht zur Aufsicht. Wer beenden darf, legt Ihre OGS fest.",
      "Fehlt `Beenden`? Dann dürfen hier nur eingeplante Kräfte beenden. Ihre OGS kann das Beenden für das ganze Team freigeben.",
      "Welche freien Räume Sie übernehmen können, legt Ihre OGS fest.",
    ],
    troubleshooting: HELP_TOPICS.attendanceProblem,
    related: [HELP_TOPICS.rooms, HELP_TOPICS.ownGroups],
  };
}

function manageActivityTopic(
  presenceMode: HelpPresenceMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  if (nfcEnabled === false || presenceMode === "binary") {
    return {
      id: HELP_TOPICS.manageActivity,
      title: "Eine Aktivität anlegen oder ändern",
      question: "Wie lege ich eine Aktivität an oder ändere sie?",
      summary:
        nfcEnabled === false
          ? "Der Bereich ist nicht verfügbar, weil Ihre OGS NFC nicht nutzt."
          : "Bei einfacher Anwesenheit gibt es keine Aktivitäten mit eigener Kinderzuordnung.",
      group: "gruppen",
      audience: "caregiver",
      icon: "Sparkles",
      steps: [
        "Nutzen Sie den `Betreuungsplan`, um geplante Angebote anzusehen.",
        // Live geprueft: der Katalog `Aktivitäten` haengt an NFC, der
        // spontane Start in `Aktuelle Aufsicht` nicht.
        "Für heute können Sie in `Aktuelle Aufsicht` eine `Spontane Aktivität starten`.",
        "Fragen Sie Ihre Leitung, wenn ein Angebot dauerhaft geändert werden soll.",
      ],
      result:
        "Der Menüpunkt `Aktivitäten` wird in diesem Modus nicht angezeigt.",
      related: [HELP_TOPICS.carePlan, HELP_TOPICS.activeSupervision],
    };
  }
  if (nfcEnabled === null || presenceMode === "unknown") {
    return {
      id: HELP_TOPICS.manageActivity,
      title: "Eine Aktivität anlegen oder ändern",
      question: "Wie lege ich eine Aktivität an oder ändere sie?",
      summary:
        "Der Bereich erscheint nur bei NFC und detaillierter Anwesenheit.",
      group: "gruppen",
      audience: "caregiver",
      icon: "Sparkles",
      steps: [
        "Prüfen Sie, ob `Aktivitäten` in der Seitenleiste steht.",
        "Ist der Bereich vorhanden? Wählen Sie `Aktivität erstellen`.",
        "Fehlt der Bereich? Fragen Sie Ihre Leitung nach der Arbeitsweise Ihrer OGS.",
      ],
      result:
        "Nur verfügbare Aktivitäten können am NFC-Tablet ausgewählt werden.",
      related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.carePlan],
    };
  }

  return {
    id: HELP_TOPICS.manageActivity,
    title: "Eine Aktivität anlegen oder ändern",
    question: "Wie lege ich eine Aktivität an oder ändere sie?",
    summary:
      "Legen Sie eine Aktivität für NFC-Tablets an oder ändern Sie eine eigene Aktivität.",
    group: "gruppen",
    audience: "caregiver",
    icon: "Sparkles",
    steps: [
      "Öffnen Sie `Aktivitäten`.",
      "Wählen Sie die Schaltfläche `Aktivität erstellen` mit dem Plus-Symbol.",
      "Geben Sie einen kurzen Namen ein.",
      "Wählen Sie eine Kategorie und bei Bedarf eine maximale Anzahl an Kindern.",
      "Wählen Sie `Aktivität erstellen`.",
      "Öffnen Sie eine eigene Aktivität und wählen Sie `Bearbeiten`, wenn Sie Angaben ändern möchten.",
    ],
    result:
      "Die Aktivität ist sofort am NFC-Tablet verfügbar. Suche und Filter helfen in der Liste.",
    differences: [
      "Eine Aktivität kann am Tablet mehrfach gewählt werden. Termine mit Uhrzeit stehen im Betreuungsplan.",
      "Aktivitäten anderer Personen können Sie ansehen, aber nicht immer bearbeiten.",
    ],
    related: [HELP_TOPICS.activeSupervision, HELP_TOPICS.carePlan],
  };
}

function parentMessageTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentMessage,
    title: "Eltern eine Nachricht schreiben",
    question: "Wie schreibe ich Eltern eine Nachricht?",
    summary:
      "Starten Sie eine Unterhaltung mit einer Bezugsperson oder antworten Sie auf eine Nachricht.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "MessageSquareText",
    steps: [],
    instructionGroups: [
      {
        title: "Eine neue Unterhaltung beginnen",
        steps: [
          "Öffnen Sie unter `Eltern` den Bereich `Nachrichten`.",
          "Wählen Sie `Neue Nachricht`.",
          "Suchen Sie das Kind und wählen Sie die passende Bezugsperson.",
          "Schreiben Sie Ihre Nachricht in das Textfeld.",
          "Wählen Sie `Senden`.",
        ],
      },
      {
        title: "Auf eine Nachricht antworten",
        steps: [
          "Öffnen Sie die Unterhaltung im Posteingang.",
          "Lesen Sie den bisherigen Verlauf.",
          "Schreiben Sie Ihre Antwort in `Nachricht an die Eltern schreiben…`.",
          "Wählen Sie `Senden`.",
        ],
      },
    ],
    result:
      "Die Bezugsperson sieht die Nachricht in der Eltern-App. Sie wird dort im Namen der OGS angezeigt.",
    differences: [
      "Sehen Sie `Nachrichten` nicht? Vielleicht ist die Funktion ausgeschaltet.",
      "Möglicherweise fehlt Ihnen auch der Zugriff auf die betroffenen Kinder.",
    ],
    notes: [
      "Schreiben Sie persönliche Angaben nur in die Unterhaltung der richtigen Bezugsperson.",
      "Mit `Nur ungelesen` sehen Sie nur neue Unterhaltungen.",
      "Über `Zum Kinderprofil` wechseln Sie direkt zu den Angaben des Kindes.",
    ],
    related: [HELP_TOPICS.parentRequests, HELP_TOPICS.studentSearch],
  };
}

function parentRequestsTopic(groupMode: HelpGroupMode): HelpTopic {
  return {
    id: HELP_TOPICS.parentRequests,
    title: "Anfragen von Eltern prüfen und bearbeiten",
    question: "Wie prüfe und bearbeite ich Anfragen von Eltern?",
    summary:
      "Vergleichen Sie die bisherigen Angaben mit dem Elternwunsch. Tragen Sie danach Ihre Entscheidung ein.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "ListChecks",
    requirements: [
      "Ihr Konto darf Kinderdaten bearbeiten.",
      // Ohne feste Gruppen gibt es keine Gruppenzuordnung, an der ein
      // Zugriff haengen koennte.
      ...(groupMode === "open_care"
        ? []
        : ["Sie haben Zugriff auf die Gruppe des Kindes."]),
    ],
    steps: [
      "Öffnen Sie `Anfragen` in der Seitenleiste.",
      "Wählen Sie den Tab `Eltern`.",
      "Suchen Sie nach dem Namen des Kindes oder wählen Sie unter `Anfrageart` einen Filter.",
      "Öffnen Sie die Anfrage und vergleichen Sie `Aktuell` mit `Gewünscht`.",
      "Wählen Sie `Freigeben`, wenn die Änderung übernommen werden soll.",
      "Oder tragen Sie eine kurze Begründung ein und wählen Sie `Ablehnen`.",
    ],
    result:
      "moto übernimmt die freigegebene Änderung. Die Bezugsperson sieht die Entscheidung in der Eltern-App.",
    differences: [
      "Bei einer geänderten Abholzeit zeigt moto vorher betroffene Termine an.",
      "Anmeldungsänderungen öffnen Sie über `Prüfen`. Dafür brauchen Sie ein zusätzliches Recht.",
      "Fehlt eine Anfrage? Sie sehen nur Kinder, auf die Sie Zugriff haben.",
    ],
    notes: [
      "Über `Historie` sehen Sie bereits entschiedene oder zurückgezogene Anfragen.",
    ],
    troubleshootingDetails: [
      // Live geprueft: ohne das Recht steht `Anfragen` nicht in der
      // Seitenleiste, und der Aufruf der Adresse landet auf der Startseite.
      "Fehlt `Anfragen` in der Seitenleiste? Dann darf Ihre Rolle keine Elternanfragen bearbeiten. Fragen Sie Ihre Leitung.",
    ],
    related: [
      HELP_TOPICS.parentMessage,
      HELP_TOPICS.editStudent,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}

function teamChatTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teamChat,
    title: "Den Team-Chat nutzen",
    question: "Wie nutze ich den Team-Chat?",
    summary:
      "Schreiben Sie einer Betreuungskraft oder Lehrkraft Ihrer Schule eine Nachricht.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "ChatsCircle",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Team-Chat`.",
      "Wählen Sie `Neue Nachricht`.",
      "Suchen Sie die Person und wählen Sie sie aus.",
      "Schreiben Sie Ihre Nachricht.",
      "Wählen Sie `Senden`.",
    ],
    result:
      "Die Nachricht erscheint in Ihrer Unterhaltung mit dieser Person. Lehrkräfte lesen sie in moto schule.",
    differences: [
      "Nachrichten im Team-Chat können nicht nachträglich geändert oder gelöscht werden.",
      "Der Bereich fehlt? Dann hat Ihre OGS den Team-Chat nicht eingeschaltet.",
    ],
    notes: [
      "Eltern können den Team-Chat nicht sehen. Für Eltern nutzen Sie `Nachrichten` im Bereich `Eltern`.",
      "Mit `Nur ungelesen` sehen Sie nur Unterhaltungen mit neuen Nachrichten.",
    ],
    related: [HELP_TOPICS.findStaff, HELP_TOPICS.parentMessage],
  };
}

function findStaffTopic(presenceMode: HelpPresenceMode): HelpTopic {
  return {
    id: HELP_TOPICS.findStaff,
    title: "Eine Person aus dem Team finden",
    question: "Wie finde ich eine Person aus dem Team?",
    summary: "Suchen Sie eine Person und sehen Sie, ob sie gerade da ist.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "UserRoundSearch",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Mitarbeiter`.",
      "Geben Sie den Namen in das Suchfeld ein.",
      // Live geprueft: die Auswahl traegt ihren aktuellen Wert als
      // Beschriftung, anfangs `Alle`. Ein Knopf `Filter` existiert nicht.
      "Öffnen Sie bei Bedarf rechts neben der Suche die Auswahl `Alle`.",
      "Wählen Sie zum Beispiel `Anwesend`, `Homeoffice` oder `Krank/Urlaub`.",
      "Prüfen Sie auf der Karte, ob die Person gerade da ist.",
    ],
    // Ohne Aufsichten steht auf der Karte auch keine.
    result:
      presenceMode === "binary"
        ? "Sie sehen, ob die Person gerade arbeitet."
        : "Sie sehen, ob die Person gerade arbeitet. Manchmal steht dort auch ihre Aufsicht.",
    differences: [
      "Persönliche Personalunterlagen und Arbeitszeiten sind besonders geschützt. Ohne zusätzliches Recht sehen Sie diese Angaben nicht.",
      "Möchten Sie der Person schreiben? Öffnen Sie den `Team-Chat`.",
    ],
    related: [HELP_TOPICS.teamChat, HELP_TOPICS.activeSupervision],
  };
}

function sharedFilesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.sharedFiles,
    title: "Eine gemeinsame Datei öffnen oder hochladen",
    question: "Wie öffne oder teile ich eine gemeinsame Datei?",
    summary: "Nutzen Sie die gemeinsame Dateiablage Ihrer OGS.",
    group: "team",
    audience: ["caregiver", "lead"],
    icon: "FolderOpen",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Datei öffnen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf und öffnen Sie `Dateien`.",
          "Wählen Sie links unter `Ordner` den passenden aus.",
          "Suchen Sie die Datei in der Liste.",
          // Live geprueft: die Aktionen liegen im Zeilenmenue und heissen
          // `Öffnen` und `Herunterladen`.
          "Öffnen Sie am Ende der Zeile das Menü mit den drei Punkten.",
          "Wählen Sie `Öffnen` oder `Herunterladen`.",
        ],
      },
      {
        title: "Eine Datei hochladen",
        steps: [
          "Öffnen Sie den passenden Ordner.",
          "Wählen Sie `Dateien auswählen`.",
          "Wählen Sie die Datei auf Ihrem Gerät aus.",
          "Warten Sie auf die Bestätigung.",
        ],
      },
    ],
    result:
      "Die Datei steht den berechtigten Personen im Ordner zur Verfügung.",
    differences: [
      "Welche Ordner Sie sehen, hängt von Ihrer Rolle ab.",
      "Fehlt `Dateien auswählen`? Dann dürfen Sie in diesem Ordner nur lesen.",
    ],
    notes: [
      "Speichern Sie persönliche Angaben nur in einem dafür vorgesehenen und geschützten Ordner.",
      "Legen Sie keine zweite Datei mit fast gleichem Namen an. Prüfen Sie zuerst, ob bereits eine aktuelle Fassung vorhanden ist.",
    ],
    related: [HELP_TOPICS.missingMenu, HELP_TOPICS.leadMissingMenu],
  };
}

function trackWorkTimeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.trackWorkTime,
    title: "Arbeitszeit und Pausen erfassen",
    question: "Wie erfasse ich meine Arbeitszeit und Pausen?",
    summary:
      "Stempeln Sie sich zu Arbeitsbeginn ein. Erfassen Sie Pausen und das Arbeitsende.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "Timer",
    steps: [],
    instructionGroups: [
      {
        title: "Arbeitszeit starten",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
          "Wählen Sie unter `Stempeluhr` den Arbeitsort `In der OGS` oder `Homeoffice`.",
          "Wählen Sie `Einstempeln`.",
        ],
      },
      {
        title: "Eine Pause erfassen",
        steps: [
          "Wählen Sie `Pause starten`.",
          "Wählen Sie die geplante Pausenlänge.",
          "Wählen Sie `Starten`.",
          "Wählen Sie `Pause beenden`, wenn Sie früher weiterarbeiten.",
        ],
      },
      {
        title: "Arbeitszeit beenden",
        steps: [
          "Beenden Sie zuerst eine laufende Pause.",
          "Wählen Sie `Ausstempeln`.",
        ],
      },
    ],
    result:
      "Ihre Arbeitszeit erscheint in der Wochenübersicht und in der Tabelle `Zeiterfassung`.",
    notes: [
      // Live geprueft: `Einstempeln` ist ein runder Knopf mit dem Wort
      // darunter. Ohne gewaehlten Arbeitsort steht dort `Bitte Status wählen`.
      "`Einstempeln` steht unter dem runden Knopf in der Mitte.",
      "Oben steht Ihr Stand: `Ist`, `Soll` und `Saldo`.",
    ],
    differences: [
      "Steht dort `Bitte Status wählen`? Dann fehlt noch der Arbeitsort. Wählen Sie zuerst `In der OGS` oder `Homeoffice`.",
      "Eine geplante Schicht steht oberhalb der Stempeluhr. Sie startet die Zeiterfassung nicht automatisch.",
      "Bei einer deutlichen Abweichung von Ihrer geplanten Schicht kann moto nach einem Grund fragen.",
      "Nach der gewählten Pausenlänge läuft die Arbeitszeit automatisch weiter.",
    ],
    related: [HELP_TOPICS.correctWorkTime, HELP_TOPICS.nfcWorkTime],
  };
}

function correctWorkTimeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.correctWorkTime,
    title: "Eigene Arbeitszeit prüfen und korrigieren",
    question: "Wie prüfe oder korrigiere ich meine Arbeitszeit?",
    summary:
      "Prüfen Sie Ihre erfassten Zeiten und berichtigen Sie einen vorhandenen Eintrag.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "Clock3",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
      // Live geprueft: der Umschalter sitzt in der Kopfkarte, nicht an der
      // Tabelle. Die Tabelle heisst selbst `Zeiterfassung`.
      "Wechseln Sie oben rechts zwischen `Woche` und `Monat`.",
      "Suchen Sie in der Tabelle `Zeiterfassung` den betroffenen Tag.",
      "Wählen Sie am Ende der Zeile das Stift-Symbol.",
      "Ändern Sie Beginn, Ende oder Pause.",
      "Tragen Sie einen Grund für die Änderung ein.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die korrigierte Zeit und der neue Saldo werden angezeigt. Die Änderung bleibt in der Historie nachvollziehbar.",
    differences: [
      "Einen vollständig fehlenden Arbeitstag trägt nur nach, wer die Zeiten des Teams verwalten darf.",
      "Ein abgeschlossener Monat kann nachträgliche Änderungen anzeigen, sein übertragener Saldo bleibt jedoch festgeschrieben.",
    ],
    notes: [
      "Klappen Sie einen geänderten Tag auf, um die Änderungshistorie zu sehen.",
    ],
    related: [HELP_TOPICS.trackWorkTime, HELP_TOPICS.vacation],
  };
}

function vacationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.vacation,
    title: "Urlaub beantragen und den Stand prüfen",
    question: "Wie beantrage ich Urlaub und prüfe den Stand?",
    summary:
      "Senden Sie einen Urlaubsantrag und verfolgen Sie die Entscheidung.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "SunHorizon",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
      "Suchen Sie die Karte `Urlaub`.",
      "Wählen Sie `Urlaub beantragen`.",
      "Wählen Sie den Zeitraum und bei Bedarf einen halben Tag.",
      "Ergänzen Sie bei Bedarf eine Notiz.",
      "Prüfen Sie Ihren Resturlaub und mögliche Überschneidungen.",
      "Senden Sie den Antrag ab.",
    ],
    result:
      "Der Antrag erscheint unter `Meine Anträge`. Dort sehen Sie, ob er offen, genehmigt, abgelehnt oder zur Rückfrage zurückgegeben wurde.",
    differences: [
      "Krankheit, Fortbildung und andere Abwesenheiten melden Sie über `Abwesend` in der Stempeluhr. Dafür stellen Sie keinen Urlaubsantrag.",
    ],
    notes: [
      "Einen offenen oder zukünftigen genehmigten Antrag können Sie stornieren.",
      "Bei einer `Rückfrage` schreiben Sie eine Antwort und wählen `Antwort senden & erneut einreichen`.",
    ],
    related: [HELP_TOPICS.ownAbsence, HELP_TOPICS.correctWorkTime],
  };
}

function ownAbsenceTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.ownAbsence,
    title: "Eine eigene Abwesenheit eintragen",
    question: "Wie trage ich eine eigene Abwesenheit ein?",
    summary:
      "Melden Sie Krankheit, Fortbildung oder eine andere Abwesenheit für den passenden Zeitraum.",
    group: "arbeitszeit",
    audience: ["caregiver", "lead"],
    icon: "BellRing",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf und öffnen Sie `Zeiterfassung`.",
      "Wählen Sie in der `Stempeluhr` oben `Abwesend`.",
      "Wählen Sie den runden Knopf `Abwesenheit melden`.",
      "Wählen Sie unter `Art der Abwesenheit` den passenden Eintrag.",
      "Tragen Sie `Von` und `Bis` ein.",
      "Schalten Sie bei Bedarf `Halber Tag` ein.",
      "Tragen Sie bei Bedarf eine `Bemerkung` ein.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die Abwesenheit ist eingetragen und erscheint in Ihrer Zeiterfassung.",
    differences: [
      "Ihre OGS kann zusätzliche Abwesenheitsarten anbieten.",
      "Urlaub beantragen Sie in der Karte `Urlaub` und nicht über diese Aktion.",
      "Freizeitausgleich trägt nur ein, wer die Zeiten des Teams verwalten darf.",
    ],
    notes: [
      "Ihre Abwesenheit steht danach in der Tabelle `Zeiterfassung` unter `Status`. Über das Stift-Symbol am Ende der Zeile öffnen Sie den Tag.",
    ],
    related: [HELP_TOPICS.vacation, HELP_TOPICS.trackWorkTime],
  };
}

function unavailableNfcTopic(
  id: HelpTopicId,
  title: string,
  question: string,
  icon: HelpIconName,
  related: readonly HelpTopicId[],
  group: HelpTopicGroup = "nfc",
): HelpTopic {
  return {
    id,
    title,
    question,
    summary:
      "Dieser Ablauf ist nicht verfügbar, weil Ihre OGS NFC nicht nutzt.",
    group,
    audience: ["caregiver", "lead"],
    icon,
    steps: [
      "Nutzen Sie die entsprechenden Funktionen in der moto-App.",
      "Erwarten Sie ein NFC-Tablet? Nur das moto-Team kann NFC einschalten.",
    ],
    result: "In Ihrer OGS werden Anwesenheit und Arbeitszeit ohne NFC erfasst.",
    related,
  };
}

function unknownNfcTopic(
  id: HelpTopicId,
  title: string,
  question: string,
  icon: HelpIconName,
  related: readonly HelpTopicId[],
  group: HelpTopicGroup = "nfc",
): HelpTopic {
  return {
    id,
    title,
    question,
    summary: "Ob dieser Ablauf verfügbar ist, hängt von Ihrer OGS ab.",
    group,
    audience: ["caregiver", "lead"],
    icon,
    steps: [
      "Prüfen Sie, ob in Ihrer OGS ein Tablet mit moto steht.",
      "Steht dort keines? Nutzen Sie die entsprechenden Funktionen in der moto-App.",
      "Ein- und ausschalten kann NFC nur das moto-Team.",
    ],
    result:
      "Die vollständigen Schritte gelten nur für eine OGS mit eingeschalteter NFC-Nutzung.",
    related,
  };
}

function tabletLoginTopic(nfcEnabled: boolean | null): HelpTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.tabletLogin,
      "Mit der PIN am Tablet anmelden",
      "Wie melde ich mich mit meiner PIN am Tablet an?",
      "Password",
      [HELP_TOPICS.login, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.tabletLogin,
      "Mit der PIN am Tablet anmelden",
      "Wie melde ich mich mit meiner PIN am Tablet an?",
      "Password",
      [HELP_TOPICS.login, HELP_TOPICS.nfcProblem],
    );
  }

  return {
    id: HELP_TOPICS.tabletLogin,
    title: "Mit der PIN am Tablet anmelden",
    question: "Wie melde ich mich mit meiner PIN am Tablet an?",
    summary:
      "Öffnen Sie mit der Geräte-PIN den geschützten Bereich des Tablets.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Password",
    requirements: ["Die vierstellige Geräte-PIN Ihrer OGS"],
    steps: [
      "Wählen Sie auf dem Startbildschirm `Anmelden`.",
      "Geben Sie die vierstellige PIN über das Zahlenfeld ein.",
      "Warten Sie nach der vierten Ziffer. Das Tablet prüft die PIN automatisch.",
    ],
    result: "Bei richtiger PIN öffnet sich das `Menü`.",
    differences: [
      "Es gibt keine Schaltfläche zum Bestätigen.",
      "Bei einer falschen Eingabe wählen Sie `C` und geben die PIN erneut ein.",
      "Die Geräte-PIN ist nicht Ihr persönliches Passwort für die moto-App.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.nfcSupervision, HELP_TOPICS.tagAssignment],
  };
}

function tagAssignmentTopic(nfcEnabled: boolean | null): HelpTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.tagAssignment,
      "Ein Armband zuweisen oder ändern",
      "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
      "Watch",
      [HELP_TOPICS.studentSearch, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.tagAssignment,
      "Ein Armband zuweisen oder ändern",
      "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
      "Watch",
      [HELP_TOPICS.studentSearch, HELP_TOPICS.nfcProblem],
    );
  }

  return {
    id: HELP_TOPICS.tagAssignment,
    title: "Ein Armband zuweisen oder ändern",
    question: "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
    summary:
      "Verbinden Sie ein NFC-Armband mit dem richtigen Kind oder Teammitglied.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Watch",
    requirements: [
      "Die Person ist in moto angelegt und steht dort nur einmal.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Ein freies Armband zuweisen",
        steps: [
          "Melden Sie sich am Tablet an.",
          "Wählen Sie im `Menü` `Armband identifizieren`.",
          "Wählen Sie `Scan starten` und halten Sie das Armband an den NFC-Sensor.",
          "Wählen Sie bei einem freien Armband `Person auswählen`.",
          "Suchen Sie die richtige Person und wählen Sie sie aus.",
          "Wählen Sie `Armband zuweisen`.",
        ],
      },
      {
        title: "Eine Zuweisung ändern",
        steps: [
          "Scannen Sie das Armband erneut unter `Armband identifizieren`.",
          "Wählen Sie `Anderer Person zuweisen`, um die Person zu wechseln.",
          "Oder wählen Sie `Armband freigeben`, damit das Armband wieder frei ist.",
          "Bestätigen Sie die Rückfrage.",
        ],
      },
    ],
    result:
      "Nach der Bestätigung ist das Armband der ausgewählten Person zugeordnet oder wieder frei.",
    differences: [
      "Auf dem Armband selbst stehen keine Namen oder anderen persönlichen Angaben.",
      "Finden Sie eine Person nicht? Prüfen Sie ihren Status und ihre OGS-Zuordnung in moto.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.nfcCheckIn, HELP_TOPICS.nfcWorkTime],
  };
}

function nfcWorkTimeTopic(nfcEnabled: boolean | null): HelpTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.nfcWorkTime,
      "Die eigene Arbeitszeit mit dem Armband erfassen",
      "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
      "Timer",
      [HELP_TOPICS.trackWorkTime, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcWorkTime,
      "Die eigene Arbeitszeit mit dem Armband erfassen",
      "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
      "Timer",
      [HELP_TOPICS.trackWorkTime, HELP_TOPICS.nfcProblem],
    );
  }

  return {
    id: HELP_TOPICS.nfcWorkTime,
    title: "Die eigene Arbeitszeit mit dem Armband erfassen",
    question: "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
    summary: "Stempeln Sie sich am NFC-Tablet ein, aus oder in eine Pause.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Timer",
    requirements: ["Ihr persönliches, aktives NFC-Armband"],
    steps: [
      "Öffnen Sie im `Menü` den Bereich `Mitarbeiter-Stempeln`.",
      "Wählen Sie `Armband scannen` und halten Sie Ihr Armband an den NFC-Sensor.",
      "Wählen Sie beim Einstempeln `Vor Ort` oder `Homeoffice`.",
      "Wählen Sie `Einstempeln`.",
      "Scannen Sie das Armband erneut, um `Pause starten`, `Pause beenden` oder `Ausstempeln` zu wählen.",
      "Geben Sie bei einer Abweichung vom Dienstplan den verlangten Grund ein.",
    ],
    result:
      "Der neue Status erscheint am Tablet. Die Buchung steht auch in Ihrer `Zeiterfassung` in der moto-App.",
    differences: [
      "Kinderarmbänder und nicht zugewiesene Armbänder funktionieren für die persönliche Arbeitszeit nicht.",
      "Eine laufende Pause wird als `In Pause` angezeigt.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.trackWorkTime, HELP_TOPICS.tagAssignment],
  };
}

function nfcSupervisionTopic(
  presenceMode: HelpPresenceMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  if (nfcEnabled === null || presenceMode === "unknown") {
    return unknownNfcTopic(
      HELP_TOPICS.nfcSupervision,
      "Eine Aufsicht am Tablet starten und beenden",
      "Wie starte oder beende ich eine Aufsicht am Tablet?",
      "TabletSmartphone",
      [HELP_TOPICS.activeSupervision, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcSupervision,
      "Eine Aufsicht am Tablet starten und beenden",
      "Wie starte oder beende ich eine Aufsicht am Tablet?",
      "TabletSmartphone",
      [HELP_TOPICS.activeSupervision, HELP_TOPICS.nfcProblem],
    );
  }
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.nfcSupervision,
      title: "Eine Aufsicht am Tablet starten und beenden",
      question: "Wie starte oder beende ich eine Aufsicht am Tablet?",
      // Das Tablet fragt auch bei einfacher Anwesenheit nach Aktivitaet und
      // Raum, weil es die Einstellung noch nicht liest (PyrePortal kennt
      // `presence_mode` nur als Typ). Der Server verwirft die Auswahl:
      // `scanBinary` bekommt das Kommando mit der RoomID gar nicht erst.
      // Der Ablauf funktioniert also -- er hat nur einen Zwischenschritt
      // ohne Wirkung. Frueher stand hier, das Tablet sei unzuverlaessig und
      // man solle die App nehmen; das legte ein funktionierendes Geraet still.
      summary:
        "Am Tablet wählen Sie zuerst Aktivität und Raum. Bei einfacher Anwesenheit ändert diese Auswahl nichts.",
      group: "nfc",
      audience: ["caregiver", "lead"],
      icon: "TabletSmartphone",
      steps: [
        "Melden Sie sich am Tablet mit Ihrer PIN an.",
        "Wählen Sie eine Aktivität und einen Raum.",
        "Starten Sie die Aufsicht. So kommen Sie zum Scannen.",
        "Scannen Sie danach die Armbänder wie gewohnt.",
      ],
      result:
        "Die Kinder sind am Tablet an- und abgemeldet. moto hält nur fest, ob ein Kind da ist.",
      differences: [
        "Aktivität und Raum sind nur ein Zwischenschritt. moto speichert sie bei einfacher Anwesenheit nicht.",
      ],
      related: [HELP_TOPICS.nfcCheckIn, HELP_TOPICS.tabletLogin],
    };
  }

  return {
    id: HELP_TOPICS.nfcSupervision,
    title: "Eine Aufsicht am Tablet starten und beenden",
    question: "Wie starte oder beende ich eine Aufsicht am Tablet?",
    summary:
      "Wählen Sie Aktivität, Betreuungsteam und Raum. Starten Sie danach die Aufsicht.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "TabletSmartphone",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Aufsicht starten",
        steps: [
          "Melden Sie sich am Tablet an.",
          "Wählen Sie im `Menü` `Aufsicht starten`.",
          "Wählen Sie unter `Was machen wir?` die Aktivität und danach `Weiter`.",
          "Wählen Sie unter `Wer ist dabei?` mindestens eine Betreuungskraft und danach `Weiter`.",
          "Wählen Sie unter `Wo machen wir das?` den Raum und danach `Weiter`.",
          "Prüfen Sie die Zusammenfassung und wählen Sie `Aufsicht starten`.",
        ],
      },
      {
        title: "Eine Aufsicht beenden",
        steps: [
          "Wählen Sie oben rechts `Anmelden` und geben Sie die Geräte-PIN ein.",
          "Wählen Sie im `Menü` `Aufsicht beenden`.",
          "Bestätigen Sie mit `Ja, beenden`.",
          "Wählen Sie `Abmelden`, um das Menü wieder zu sperren.",
        ],
      },
    ],
    result:
      "Das Tablet zeigt Aktivität, Raum und Kinderzahl. Beim Beenden werden die Kinder abgemeldet.",
    notes: [
      "Unter `Letzte Aufsichten` starten Sie eine frühere Aufsicht neu. Aktivität, Team und Raum werden übernommen.",
      "Unter `Team anpassen` ändern Sie während der Aufsicht das Betreuungsteam.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.nfcCheckIn, HELP_TOPICS.activeSupervision],
  };
}

function nfcCheckInTopic(
  presenceMode: HelpPresenceMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  if (nfcEnabled === null || presenceMode === "unknown") {
    return unknownNfcTopic(
      HELP_TOPICS.nfcCheckIn,
      "Kinder mit dem Armband ein- und auschecken",
      "Wie checken sich Kinder mit dem Armband ein und aus?",
      "Scan",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcCheckIn,
      "Kinder mit dem Armband ein- und auschecken",
      "Wie checken sich Kinder mit dem Armband ein und aus?",
      "Scan",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.nfcProblem],
    );
  }

  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.nfcCheckIn,
      title: "Kinder mit dem Armband ein- und auschecken",
      question: "Wie checken sich Kinder mit dem Armband ein und aus?",
      // Der Server schaltet bei einfacher Anwesenheit nur die Anwesenheit um
      // (`scanBinary`) und antwortet mit "Willkommen, ..." bzw. "Tschuess,
      // ...". Raum und taegliche Abmeldung sind in der Antwort leer bzw.
      // aus. Das Scannen selbst funktioniert also unveraendert.
      summary:
        "Kinder melden sich mit ihrem Armband an und ab. moto hält fest, ob ein Kind da ist.",
      group: "nfc",
      audience: ["caregiver", "lead"],
      icon: "Scan",
      requirements: [
        "Das Armband ist dem Kind zugewiesen.",
        "Am Tablet ist der Scan-Bildschirm geöffnet.",
      ],
      steps: [],
      instructionGroups: [
        {
          title: "Anmelden",
          steps: [
            "Das Kind hält sein Armband an den NFC-Sensor.",
            "moto bestätigt mit dem Namen des Kindes.",
          ],
        },
        {
          title: "Abmelden",
          steps: [
            "Das angemeldete Kind hält sein Armband erneut an den Sensor.",
            "moto meldet das Kind ab und bestätigt mit seinem Namen.",
          ],
        },
      ],
      result:
        "moto zeigt `Anwesend` oder `Abwesend`. Räume werden nicht erfasst.",
      differences: [
        "Die Frage `Wohin geht ...?` erscheint nicht. Das zweite Scannen meldet das Kind direkt ab.",
        "Eine Auswahl von Raum oder Aktivität am Tablet ändert nichts an der Anwesenheit.",
      ],
      troubleshooting: HELP_TOPICS.nfcProblem,
      related: [HELP_TOPICS.tagAssignment, HELP_TOPICS.webAttendance],
    };
  }

  return {
    id: HELP_TOPICS.nfcCheckIn,
    title: "Kinder mit dem Armband ein- und auschecken",
    question: "Wie checken sich Kinder mit dem Armband ein und aus?",
    summary:
      "Kinder melden sich mit ihrem Armband bei einer Aufsicht an. Danach wählen sie ihr nächstes Ziel.",
    group: "nfc",
    audience: ["caregiver", "lead"],
    icon: "Scan",
    requirements: [
      "Das Armband ist dem Kind zugewiesen.",
      "Am Tablet läuft eine Aufsicht.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Einchecken",
        steps: [
          "Das Kind hält sein Armband an den NFC-Sensor.",
          "Prüfen Sie die grüne Bestätigung mit Name, Abholzeit und Raum.",
          "Warten Sie, bis sich die Anzeige automatisch schließt.",
        ],
      },
      {
        title: "Auschecken oder den Ort wechseln",
        steps: [
          "Das bereits eingecheckte Kind hält sein Armband erneut an den Sensor.",
          "Das Kind wählt unter `Wohin geht ...?` das passende Ziel.",
          "Wählen Sie `Raumwechsel` für einen anderen betreuten Raum. Dort hält das Kind sein Armband erneut an.",
          "Offene Räume stehen mit ihrem Namen da, zum Beispiel `Turnhalle`. Ein Tipp trägt das Kind sofort dort ein.",
          "Wählen Sie `nach Hause`, wenn das Kind die OGS verlässt.",
          "Je nach Einstellung können auch `Schulhof` oder `Toilette` erscheinen.",
        ],
      },
    ],
    result:
      "moto aktualisiert den Aufenthaltsort. Bei einem Ortswechsel bleibt das Kind angemeldet. Erst nach `nach Hause` ist es abgemeldet.",
    differences: [
      "Welche Ziele angezeigt werden, legt Ihre OGS fest.",
      "Ein offener Raum erscheint nur, wenn die Leitung ihn unter `Räume` als `Offener Raum` freigegeben hat. Dort braucht es kein Tablet und keine Aufsicht.",
      "Eine tägliche Abmeldezeit kann verhindern, dass `nach Hause` zu früh gewählt wird.",
      "Nach dem Abmelden kann ein freiwilliges Tages-Feedback erscheinen.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.tagAssignment, HELP_TOPICS.nfcSupervision],
  };
}

// Auf "Ein Menuepunkt fehlt" ist die Arbeitsweise der OGS meistens schon
// bekannt. Dann gehoert die zutreffende Zeile als Tatsache hin, nicht als
// eine von vier Moeglichkeiten -- sonst raetselt die Person auf genau der
// Seite weiter, die ihr die Antwort geben sollte.
function missingMenuDifferences(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): readonly string[] {
  const lines: string[] = [];

  if (presenceMode === "binary") {
    // Vollstaendige Liste aus sidebar.tsx BINARY_HIDDEN_HREFS (/rooms,
    // /activities, /tagesplan) plus dem separat gegateten Aufsicht-Akkordeon.
    lines.push(
      "In Ihrer OGS gibt es `Räume`, `Aktivitäten`, `Aktuelle Aufsicht` und `Tagesplan` nicht. Ihre OGS hält nur fest, ob ein Kind da ist.",
    );
  } else if (presenceMode === "unknown") {
    lines.push(
      "Bei einfacher Anwesenheit fehlen `Räume`, `Aktivitäten`, `Aktuelle Aufsicht` und `Tagesplan`.",
    );
  }

  if (groupMode === "open_care") {
    lines.push(
      "In Ihrer OGS gibt es `Meine Gruppen` nicht. Ihre OGS arbeitet mit offener Betreuung.",
    );
  } else if (groupMode === "unknown") {
    lines.push("Bei offener Betreuung fehlt `Meine Gruppen`.");
  }

  if (nfcEnabled === false) {
    if (presenceMode !== "binary") {
      lines.push(
        "In Ihrer OGS gibt es den Bereich `Aktivitäten` nicht. Ihre OGS nutzt kein NFC.",
      );
    }
  } else if (nfcEnabled === null) {
    lines.push("Ohne NFC fehlt der Bereich `Aktivitäten`.");
  }

  lines.push(
    "Team-Chat, Eltern-Nachrichten und einige Auswertungen können von der OGS ein- oder ausgeschaltet werden.",
  );
  return lines;
}

function missingMenuTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  return {
    id: HELP_TOPICS.missingMenu,
    title: "Ein Menüpunkt fehlt",
    question: "Was kann ich tun, wenn ein Menüpunkt fehlt?",
    summary:
      "Ein Bereich kann wegen Ihrer Rolle, einer Einstellung oder der gewählten Arbeitsweise der OGS fehlen.",
    group: "probleme",
    audience: "caregiver",
    icon: "EyeSlash",
    steps: [
      "Öffnen Sie auf dem Handy unten `Mehr`. Dort stehen alle Bereiche.",
      "Klappen Sie am Computer in der Seitenleiste die passende Gruppe auf.",
      "Laden Sie die Seite neu.",
      "Melden Sie sich ab und wieder an, wenn Ihre Rolle oder Rechte gerade geändert wurden. `Abmelden` steht am Computer im Menü oben rechts, auf dem Handy unter `Mehr`.",
      "Fragen Sie Ihre Leitung, ob die Funktion für Ihre OGS eingeschaltet ist.",
      "Bitten Sie Ihre Leitung, Ihre Rolle und Rechte zu prüfen.",
    ],
    result:
      "Ihre Leitung prüft die Einstellungen und Ihren Zugang. Danach kann sie die Ursache klären.",
    differences: missingMenuDifferences(presenceMode, groupMode, nfcEnabled),
    related: [HELP_TOPICS.missingChildOrGroup, HELP_TOPICS.loginProblem],
  };
}

function missingChildOrGroupTopic(groupMode: HelpGroupMode): HelpTopic {
  return {
    id: HELP_TOPICS.missingChildOrGroup,
    title: "Ein Kind oder eine Gruppe fehlt",
    question: "Was kann ich tun, wenn ein Kind oder eine Gruppe fehlt?",
    summary:
      "Prüfen Sie Suche, Filter, gewählten Tag und Ihre Gruppenzuordnung.",
    group: "probleme",
    audience: "caregiver",
    icon: "Search",
    steps: [
      "Löschen Sie den Text im Suchfeld.",
      // Jede Liste hat eigene Filter: ein Symbol, ein Umschalter oder eine
      // Auswahl. Einen Knopf namens `Filter` gibt es nirgends.
      "Setzen Sie die Filter über der Liste zurück.",
      "Prüfen Sie, ob oben der richtige Tag gewählt ist.",
      "Suchen Sie das Kind unter `Alle Kinder`.",
      ...(groupMode === "fixed_groups"
        ? [
            "Fehlt das Kind nur unter `Meine Gruppen`? Prüfen Sie, ob Sie die richtige Gruppe geöffnet haben.",
            "Bitten Sie Ihre Leitung, die Gruppe des Kindes und Ihre Gruppenzuordnung zu prüfen.",
          ]
        : groupMode === "open_care"
          ? [
              "Bei offener Betreuung gibt es keine festen eigenen Gruppen. Arbeiten Sie über `Alle Kinder`.",
            ]
          : [
              "Prüfen Sie, ob Ihre OGS mit festen Gruppen oder einer gemeinsamen Kinderliste arbeitet.",
            ]),
    ],
    result:
      "Das Kind bleibt unsichtbar? Dann muss Ihre Leitung Daten und Zugriff prüfen.",
    differences: [
      "Ein Kind kann an einem anderen Tag anders eingeplant sein.",
      "Vorübergehend übernommene Gruppen gelten nur bis zum Ende des Tages.",
      "Gelöschte oder noch nicht übernommene Kinder erscheinen nicht in der normalen Kinderliste.",
    ],
    related: [HELP_TOPICS.studentSearch, HELP_TOPICS.ownGroups],
  };
}

function attendanceProblemTopic(presenceMode: HelpPresenceMode): HelpTopic {
  return {
    id: HELP_TOPICS.attendanceProblem,
    title: "Ich kann ein Kind nicht an- oder abmelden",
    question: "Warum kann ich ein Kind nicht an- oder abmelden?",
    summary: "Prüfen Sie den aktuellen Status, Ihre Auswahl und Ihre Rechte.",
    group: "probleme",
    audience: "caregiver",
    icon: "ShieldAlert",
    steps: [
      "Öffnen Sie das Kind und prüfen Sie den aktuellen Status.",
      "Laden Sie die Seite neu, damit Sie den neuesten Stand sehen.",
      "Bei einem abwesenden Kind zeigt moto `Anmelden`.",
      "Bei einem anwesenden Kind zeigt moto `Abmelden`.",
      ...(presenceMode === "detailed"
        ? [
            "Möchten Sie den Raum ändern? Prüfen Sie, ob im Zielraum eine Aufsicht läuft.",
            "Öffnen Sie bei Bedarf `Aktuelle Aufsicht`.",
            "Prüfen Sie, ob Sie den aktuellen Raum oder den Zielraum beaufsichtigen.",
          ]
        : []),
      "Versuchen Sie die Aktion erneut.",
      "Fehlt die Aktion weiterhin? Bitten Sie Ihre Leitung, Ihr Recht zum An- und Abmelden zu prüfen.",
    ],
    result:
      "Stimmen Status und Rechte, ist die Aktion verfügbar. Sonst prüft Ihre Leitung den Zugang.",
    differences: [
      "Eine Krankmeldung oder Entschuldigung ist nicht dasselbe wie An- oder Abmelden.",
      "Bei einer gleichzeitigen Änderung durch eine andere Person kann moto den neueren Status anzeigen.",
    ],
    related: [HELP_TOPICS.webAttendance, HELP_TOPICS.activeSupervision],
  };
}

function nfcProblemTopic(nfcEnabled: boolean | null): HelpTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.nfcProblem,
      "Das NFC-Tablet funktioniert nicht",
      "Was kann ich tun, wenn das NFC-Tablet nicht funktioniert?",
      "TabletSmartphone",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.trackWorkTime],
      "probleme",
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcProblem,
      "Das NFC-Tablet funktioniert nicht",
      "Was kann ich tun, wenn das NFC-Tablet nicht funktioniert?",
      "TabletSmartphone",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.trackWorkTime],
      "probleme",
    );
  }

  return {
    id: HELP_TOPICS.nfcProblem,
    title: "Das NFC-Tablet funktioniert nicht",
    question: "Was kann ich tun, wenn das NFC-Tablet nicht funktioniert?",
    summary:
      "Prüfen Sie Strom, Verbindung, Lesegerät und Armband nacheinander.",
    group: "probleme",
    audience: ["caregiver", "lead"],
    icon: "TabletSmartphone",
    steps: [
      "Prüfen Sie, ob Tablet und NFC-Sensor Strom haben.",
      "Prüfen Sie, ob das Tablet mit dem Internet verbunden ist.",
      "Legen Sie das Armband flach und für einen kurzen Moment auf den NFC-Sensor.",
      "Testen Sie ein zweites, sicher funktionierendes Armband.",
      "Melden Sie sich mit der Geräte-PIN an und öffnen Sie `Armband identifizieren`.",
      "Prüfen Sie dort, ob das Armband erkannt und der richtigen Person zugewiesen ist.",
      "Starten Sie das Tablet neu, wenn weiterhin kein Armband erkannt wird.",
      "Hilft nichts davon? Geben Sie die Fehlermeldung an Ihre Leitung oder an das moto-Team weiter.",
    ],
    result:
      "Der Vergleich grenzt die Ursache ein. Das Problem liegt am Armband oder am Tablet.",
    differences: [
      "Wird nur ein Armband nicht erkannt, prüfen oder erneuern Sie dessen Zuweisung.",
      "Werden keine Armbänder erkannt, prüfen Sie besonders NFC-Sensor, Kabel und Gerätestatus.",
      "Bei einem Ausfall können berechtigte Personen Anwesenheit und Arbeitszeit vorübergehend in der moto-App erfassen.",
    ],
    related: [HELP_TOPICS.tagAssignment, HELP_TOPICS.nfcCheckIn],
  };
}

function loginProblemTopic(nfcEnabled: boolean | null): HelpTopic {
  return {
    id: HELP_TOPICS.loginProblem,
    title: "Ich kann mich nicht anmelden",
    question: "Was kann ich tun, wenn die Anmeldung nicht klappt?",
    summary:
      "Prüfen Sie die Seite Ihrer OGS, Ihre E-Mail-Adresse und Ihr Passwort.",
    group: "probleme",
    audience: ["caregiver", "lead"],
    icon: "KeyRound",
    steps: [
      "Prüfen Sie, ob Sie die moto-Seite Ihrer OGS geöffnet haben.",
      "Geben Sie Ihre E-Mail-Adresse erneut vollständig ein.",
      "Prüfen Sie Groß- und Kleinschreibung sowie versehentliche Leerzeichen im Passwort.",
      "Wählen Sie `Passwort vergessen?`, wenn Sie Ihr Passwort nicht mehr wissen.",
      "Öffnen Sie die E-Mail von moto und folgen Sie dem Link zum neuen Passwort.",
      "Fragt moto nach einem Sicherheitscode? Prüfen Sie Ihr E-Mail-Postfach und den Spam-Ordner.",
      "Versuchen Sie die Anmeldung mit dem neuen Passwort erneut.",
      "Klappt die Anmeldung weiterhin nicht? Wenden Sie sich an Ihre Leitung.",
      "Bitten Sie um eine Prüfung von Konto und Einladung.",
    ],
    result:
      "Mit einem aktiven Konto und den richtigen Zugangsdaten gelangen Sie wieder in moto.",
    differences: [
      "Eine abgelaufene oder bereits verwendete Einladung kann nicht noch einmal angenommen werden.",
      // Ohne NFC gibt es kein Tablet und keine Geraete-PIN. Der Hinweis
      // schickt dann auf die Suche nach etwas, das es nicht gibt.
      ...(nfcEnabled === false
        ? []
        : [
            "Die vierstellige Geräte-PIN des NFC-Tablets ist nicht Ihr persönliches moto-Passwort.",
          ]),
      "Ein Sicherheitscode kann nur für kurze Zeit verwendet werden. Fordern Sie bei Bedarf einen neuen Code an.",
    ],
    related: [HELP_TOPICS.login, HELP_TOPICS.acceptInvitation],
  };
}

/**
 * Die Themen des laufenden Betriebs. Die Reihenfolge einer Gruppe in
 * Seitenleiste und Gruppenseite folgt dieser Liste, deshalb stehen die zwei
 * Leitungsthemen zum NFC-Gerät hier und nicht in `leadTopics`: ohne
 * gesetzte `OGS Geräte-PIN` kommt am Tablet niemand weiter, und stünden sie
 * am Ende, müsste die Leitung zuerst nach unten springen. Die Betreuung
 * sieht beide nicht.
 */
function caregiverTopics(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): readonly HelpTopic[] {
  return [
    myScheduleTopic(),
    carePlanTopic(presenceMode),
    dayPlanTopic(presenceMode),
    studentSearchTopic(presenceMode, groupMode),
    editStudentTopic(),
    webAttendanceTopic(presenceMode),
    changeLocationTopic(presenceMode),
    absencesTopic(),
    dayLogTopic(groupMode),
    emergencyTopic(presenceMode),
    ownGroupsTopic(presenceMode, groupMode),
    transferGroupTopic(groupMode),
    roomsTopic(presenceMode),
    activeSupervisionTopic(presenceMode),
    manageActivityTopic(presenceMode, nfcEnabled),
    parentMessageTopic(),
    parentRequestsTopic(groupMode),
    teamChatTopic(),
    findStaffTopic(presenceMode),
    sharedFilesTopic(),
    trackWorkTimeTopic(),
    correctWorkTimeTopic(),
    vacationTopic(),
    ownAbsenceTopic(),

    // Nur für die Leitung, und bewusst vor den fünf Themen zum täglichen
    // Umgang: Gerät prüfen und PIN setzen kommen davor.
    devicesTopic(),
    nfcSettingsTopic(presenceMode),
    tabletLoginTopic(nfcEnabled),
    tagAssignmentTopic(nfcEnabled),
    nfcWorkTimeTopic(nfcEnabled),
    nfcSupervisionTopic(presenceMode, nfcEnabled),
    nfcCheckInTopic(presenceMode, nfcEnabled),
    missingMenuTopic(presenceMode, groupMode, nfcEnabled),
    missingChildOrGroupTopic(groupMode),
    attendanceProblemTopic(presenceMode),
    nfcProblemTopic(nfcEnabled),
    loginProblemTopic(nfcEnabled),
  ];
}

// OGS-Leitung und Verwaltung. Geruest nach der Informationsarchitektur
// (#2229, Abschnitt 7): Einrichtung zuerst, danach die wiederkehrende
// Verwaltung. Die Ablaeufe sind noch nicht Schritt fuer Schritt gegen die App
// geprueft, deshalb bleiben es Entwuerfe.
//
// Was eine Leitung genauso erledigt wie eine Betreuungskraft, steht nicht
// hier: `login`, `installApp`, `studentSearch`, `parentMessage`,
// `parentRequests`, `dayLog` und `sharedFiles` tragen beide Rollen in
// `audience` und erscheinen dadurch in beiden Seitenleisten.
/**
 * Raumkatalog unter `/database/rooms`. Die Seite selbst haengt an keiner
 * Einstellung: sie steht weder in `BINARY_HIDDEN_HREFS` noch in
 * `NFC_ONLY_HREFS` (sidebar.tsx), und jede Gruppe bekommt ueber
 * `groups.config.tsx` einen Raum zugewiesen -- auch bei einfacher Anwesenheit.
 *
 * Zwei Felder wirken aber nur bei detaillierter Anwesenheit: `scanBinary`
 * (backend/modules/devicescan/internal/application/scan.go) schaltet nur die
 * Anwesenheit um und legt keinen Raumbesuch an, und `checkRoomCapacity`
 * (checkin.go) zaehlt genau diese offenen Raumbesuche.
 */
function roomsCatalogTopic(presenceMode: HelpPresenceMode): HelpTopic {
  const tracksRooms = presenceMode !== "binary";
  return {
    id: HELP_TOPICS.leadRooms,
    title: "Räume anlegen",
    question: "Wie lege ich die Räume der OGS an?",
    summary: "Legen Sie jeden Raum einmal an. Gruppen brauchen einen Raum.",
    group: "einrichten",
    audience: "lead",
    icon: "Building2",
    // Kein `requirements`: wer die Leitungshilfe liest, hat die Rechte der
    // Leitung. Der Fall „Bereich fehlt trotzdem" steht bei den Problemen,
    // wo er zu einer Handlung fuehrt.
    steps: [],
    instructionGroups: [
      {
        title: "Einen Raum anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Räume`.",
          "Wählen Sie oben rechts `+ Raum`.",
          "Tragen Sie bei `Raumname` den Namen des Raums ein.",
          "Wählen Sie eine `Kategorie`.",
          "Tragen Sie bei Bedarf `Gebäude` und `Etage` ein.",
          "Wählen Sie bei Bedarf eine `Farbe`.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Einen Raum ändern",
        steps: [
          "Öffnen Sie `Räume` und wählen Sie den Raum aus der Liste.",
          "Ändern Sie die Angaben unter `Stammdaten`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Einen Raum löschen",
        steps: [
          "Öffnen Sie `Räume` und wählen Sie den Raum aus der Liste.",
          "Wählen Sie oben rechts `Löschen`.",
          "Bestätigen Sie mit `Ja, löschen`.",
        ],
      },
    ],
    result:
      "Der Raum steht sofort zur Auswahl. Sie können ihn einer Gruppe zuweisen.",
    notes: [
      "Die Farbe erscheint später an jedem Ort-Schild dieses Raums.",
      "Oben rechts fassen Sie die Liste unter `Gruppieren` nach `Gebäude`, `Etage` oder `Keine` zusammen.",
      ...(tracksRooms
        ? [
            "`Maximale Belegung` begrenzt, wie viele Kinder gleichzeitig im Raum sind.",
            "Mit `Offener Raum` dürfen Kinder den Raum jederzeit als Ziel wählen. Eine Aufsicht ersetzt das nicht.",
          ]
        : [
            "`Maximale Belegung` und `Offener Raum` wirken bei Ihnen nicht. Ihre OGS hält nur fest, ob ein Kind da ist.",
          ]),
    ],
    differences: [
      "Auf dem Handy steht statt `+ Raum` ein rundes Plus unten rechts.",
      "`Schulhof`, `WC` und `Toilette` legt moto selbst an. Ihre Namen sind fest, und `Löschen` fehlt.",
      "Beim `WC` und bei der `Toilette` fehlen zusätzlich `Farbe` und `Offener Raum`.",
    ],
    troubleshootingDetails: [
      "Fehlt `Datenverwaltung` in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
      "Lässt sich ein Raum nicht löschen? Dann ist er gerade belegt. Oder ein Betreuungsangebot braucht ihn.",
    ],
    related: [
      HELP_TOPICS.leadGroups,
      HELP_TOPICS.leadActivities,
      HELP_TOPICS.dataManagement,
    ],
  };
}

/**
 * Gruppenkatalog unter `/database/groups`. An keine Einstellung gebunden: der
 * `OpenCareModeGuard` schuetzt nur `/ogs-groups`, und die Seitenleiste blendet
 * bei offener Betreuung nur das Akkordeon `Meine Gruppen` aus
 * (sidebar.tsx:1391). Der Katalog selbst bleibt in jeder Arbeitsweise stehen.
 */
function groupsCatalogTopic(groupMode: HelpGroupMode): HelpTopic {
  const openCare = groupMode === "open_care";
  return {
    id: HELP_TOPICS.leadGroups,
    title: "Gruppen anlegen",
    question: "Wie lege ich die Gruppen der OGS an?",
    summary: openCare
      ? "Legen Sie Gruppen an, wenn Sie Kinder darin ordnen möchten."
      : "Legen Sie jede Gruppe an und geben Sie ihr Raum und Leitung.",
    group: "einrichten",
    audience: "lead",
    icon: "Users",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Gruppe anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Gruppen`.",
          "Wählen Sie oben rechts `+ Gruppe`.",
          "Tragen Sie bei `Gruppenname` den Namen der Gruppe ein.",
          "Wählen Sie bei Bedarf einen `Gruppenraum`.",
          "Wählen Sie bei Bedarf eine `Gruppenleitung`. Mehrere sind möglich.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Eine Gruppe ändern",
        steps: [
          "Öffnen Sie `Gruppen` und wählen Sie die Gruppe aus der Liste.",
          "Ändern Sie die Angaben unter `Stammdaten`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Eine Gruppe löschen",
        steps: [
          "Öffnen Sie `Gruppen` und wählen Sie die Gruppe aus der Liste.",
          "Wählen Sie oben rechts `Löschen`.",
          "Bestätigen Sie mit `Ja, löschen`.",
        ],
      },
    ],
    result:
      "Die Gruppe steht sofort zur Auswahl. Sie können Kinder darin eintragen.",
    notes: [
      "Ein Kind kommt über sein Feld `OGS Gruppe` in die Gruppe, nicht hier.",
      "Über `Alle Räume` filtern Sie die Liste auf die Gruppen eines Raums.",
      ...(openCare
        ? [
            "Ihre OGS arbeitet ohne feste Gruppen. Ihr Team sieht deshalb keine `Meine Gruppen`.",
          ]
        : []),
    ],
    differences: [
      "Auf dem Handy steht statt `+ Gruppe` ein rundes Plus unten rechts.",
      "Bei `Gruppenraum` steht `Kein Gruppenraum`, solange kein Raum gewählt ist.",
    ],
    troubleshootingDetails: [
      "Fehlt `Datenverwaltung` in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
      "Fehlt eine Person bei `Gruppenleitung`? Dann ist sie noch nicht als Betreuungskraft angelegt.",
    ],
    related: [
      HELP_TOPICS.leadRooms,
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadCreateStudent,
    ],
  };
}

/**
 * Aktivitaetenkatalog unter `/database/activities`. Nur mit NFC: die Route
 * liegt hinter `NfcModeGuard`, und der Eintrag steht in `NFC_ONLY_HREFS`
 * (dashboard/sidebar.tsx). Deshalb auch in NFC_ONLY_TOPIC_IDS.
 */
function activitiesCatalogTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadActivities,
    title: "Aktivitäten anlegen",
    question: "Wie lege ich Aktivitäten an?",
    summary:
      "Aktivitäten sind Angebote neben der Gruppe, zum Beispiel eine AG.",
    group: "einrichten",
    audience: "lead",
    icon: "Sparkles",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Aktivität anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Aktivitäten`.",
          "Wählen Sie oben rechts `+ Aktivität`.",
          "Tragen Sie bei `Name` die Bezeichnung des Angebots ein.",
          "Wählen Sie eine `Kategorie`.",
          "Tragen Sie bei Bedarf `Maximale Teilnehmer` ein.",
          "Wählen Sie bei Bedarf einen `Hauptbetreuer`.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Eine Aktivität ändern",
        steps: [
          "Öffnen Sie `Aktivitäten` und wählen Sie die Aktivität aus der Liste.",
          "Ändern Sie die Angaben unter `Stammdaten`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Eine Aktivität löschen",
        steps: [
          "Öffnen Sie `Aktivitäten` und wählen Sie die Aktivität aus der Liste.",
          "Wählen Sie oben rechts `Löschen`.",
          "Bestätigen Sie mit `Ja, löschen`.",
        ],
      },
    ],
    result:
      "Die Aktivität steht sofort zur Auswahl. Ihr Team kann sie am Tablet starten.",
    notes: [
      "`Maximale Teilnehmer` steht zunächst auf 20. Leeren Sie das Feld für keine Begrenzung.",
      "Über `Kategorie` filtern Sie die Liste auf eine Art von Angebot.",
    ],
    differences: [
      "Auf dem Handy steht statt `+ Aktivität` ein rundes Plus unten rechts.",
      "`Schulhof Freispiel` und `WC` legt moto selbst an. Ihre Namen sind fest, und `Löschen` fehlt.",
    ],
    troubleshootingDetails: [
      "Fehlt `Datenverwaltung` in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
      "Fehlt die passende `Kategorie`? Legen Sie sie in der `Datenverwaltung` unter `Terminkategorien` an.",
    ],
    related: [
      HELP_TOPICS.leadRooms,
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.dataManagement,
    ],
  };
}

/**
 * Kinder anlegen: einzeln im Slide-over `Neues Kind` auf
 * `/database/students`, viele auf einmal unter `/database/students/import`.
 * Beides an keine Einstellung gebunden.
 *
 * Live geprueft: `Neues Kind` ist ein einziges Fenster. Pflicht sind nur
 * `Vorname`, `Nachname` und `Klasse`; darunter folgen die Abschnitte
 * `Erziehungsberechtigte`, `Betreuungszeiten`, `Gesundheitsinformationen`,
 * `Betreuernotizen`, `Elternnotizen`, `Datenschutz` und `Erlaubte Heimwege`
 * -- alle vor `Erstellen`, alle freiwillig. Deshalb tragen die Schritte nur
 * den Weg, nicht jedes Feld.
 *
 * Die Weiche `Nur Klassenliste` erscheint nur mit `users:create`
 * (students/page.tsx:638), fuer die Leitung also immer. Der Einstieg
 * `Importieren` steht nur auf breiten Bildschirmen (`!isMobile`,
 * page.tsx:723).
 */
function createStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCreateStudent,
    title: "Kinder anlegen",
    question: "Wie lege ich Kinder an?",
    summary: "Einzeln über ein Fenster. Viele auf einmal über eine Vorlage.",
    group: "einrichten",
    audience: "lead",
    icon: "Student",
    steps: [],
    instructionGroups: [
      {
        title: "Ein Kind einzeln anlegen",
        description:
          "Alles steht in einem Fenster. Pflicht sind nur drei Felder, alles andere können Sie später nachtragen.",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Wählen Sie oben rechts `+ Kinder`.",
          "Bleiben Sie oben auf `Mit OGS-Betreuung`.",
          "Tragen Sie `Vorname`, `Nachname` und `Klasse` ein.",
          "Füllen Sie weiter unten aus, was Sie schon wissen.",
          "Wählen Sie unten `Erstellen`.",
        ],
      },
      {
        title: "Viele Kinder aus einer Liste übernehmen",
        description:
          "Für ein ganzes Schuljahr ist die Vorlage schneller als das Fenster.",
        steps: [
          "Öffnen Sie `Kinderdaten` und wählen Sie oben `Importieren`.",
          "Wählen Sie bei `Format wählen` `Excel (.xlsx)` oder `CSV (Komma-getrennt)`.",
          "Wählen Sie `Vorlage herunterladen`.",
          "Tragen Sie die Kinder ein. Beim Geburtstag geht zum Beispiel `14.03.2018`.",
          "Bleiben Sie unter `Was soll der Import tun?` bei `Nur neue anlegen`.",
          "Ziehen Sie die Datei unter `Schritt 3: Datei hochladen` in das Feld.",
          "Prüfen Sie die `Datenvorschau`.",
          "Wählen Sie unten den grünen Knopf, zum Beispiel `12 Kinder importieren`.",
        ],
      },
    ],
    result:
      "Ein einzeln angelegtes Kind steht sofort unter `Kinderdaten`. Nach einem Import meldet moto, wie viele Kinder angelegt und wie viele aktualisiert wurden.",
    notes: [
      "Im Fenster stehen weiter unten `Erziehungsberechtigte`, `Betreuungszeiten`, `Gesundheitsinformationen`, Notizen, `Datenschutz` und `Erlaubte Heimwege`.",
      "Bei einem Geschwisterkind wählen Sie `Vorhandene/n suchen`. Dann teilen beide dieselben Erziehungsberechtigten.",
      "Ein Auge neben einer Angabe heißt: Eltern sehen sie.",
      "In der Vorlage erklärt das Blatt `Hinweise` jede Spalte.",
      "`Nur bestehende aktualisieren` und `Beides` brauchen Sie erst, wenn Sie Angaben nachträglich ändern. Leere Zellen ändern nie etwas.",
      "Hat Ihre Schule ein Kinderkontingent, steht es oben in `Kinderdaten`. Zum Beispiel `Kinderkontingent: 48 von 50 belegt`. Es zählen aktive Kinder und Kinder, deren Betreuung später beginnt. Das Info-Symbol daneben erklärt es auch.",
    ],
    differences: [
      "`Vorname`, `Nachname` und `Klasse` tragen einen roten Stern. Sie sind Pflicht.",
      "Oben im Fenster können Sie auf `Nur Klassenliste` wechseln. Das ist für Kinder ohne OGS-Betreuung.",
      "Auf dem Handy steht statt `+ Kinder` ein rundes Plus unten rechts. `Importieren` fehlt dort ganz.",
    ],
    troubleshootingDetails: [
      "Fehlt die passende `Gruppe`? Legen Sie sie zuerst in der `Datenverwaltung` unter `Gruppen` an.",
      "Der grüne Knopf ist grau? Dann hat die Vorschau noch Fehler. Beheben Sie sie in der Datei und laden Sie erneut hoch.",
      "moto erkennt eine Zeile an Vorname, Nachname und Klasse. Bei einem Klassenwechsel an der Spalte `RFID-Karte` oder am Geburtstag.",
      "moto meldet, das Kinderkontingent ist voll? Dann ist die Kontingentzahl erreicht. Wie viel belegt ist, steht oben in `Kinderdaten`. Ihre Eingaben bleiben im Fenster. Für weitere Kinder melden Sie sich beim moto-Team.",
    ],
    related: [
      HELP_TOPICS.leadCareTimes,
      HELP_TOPICS.leadGroups,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}

/**
 * Betreuungszeiten. Drei Wege, alle an keine der drei Einstellungen gebunden:
 * beim Anlegen (`CareWeeklyPlanModal` im Slide-over `Neues Kind`), beim
 * einzelnen Kind (`CareScheduleManager`, Knopf `Wochenplan`) und fuer mehrere
 * Kinder ueber `Auswählen` -> `Ankunftszeiten` / `Gehzeiten`
 * (students-master-detail.tsx:532).
 */
function careTimesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCareTimes,
    title: "Betreuungszeiten eintragen",
    question: "Wie trage ich die Betreuungszeiten eines Kindes ein?",
    summary: "Legen Sie fest, an welchen Tagen ein Kind betreut wird.",
    group: "einrichten",
    audience: "lead",
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Den Wochenplan eines Kindes eintragen",
        steps: [
          "Öffnen Sie das Kind und wählen Sie den Bereich `Betreuungszeiten`.",
          "Wählen Sie `Wochenplan`.",
          "Wählen Sie die Betreuungstage.",
          "Tragen Sie `Ankunft` und `Abholung` ein.",
          "Kommt das Kind an einem Wochentag nicht? Lassen Sie `Abholung` leer.",
          "Öffnen Sie `Notizen`. Tragen Sie den Hinweis unter `Notiz zum Tag (jede Woche)` ein.",
          "Wählen Sie `Wochenplan speichern`.",
        ],
      },
      {
        title: "Zeiten für mehrere Kinder auf einmal setzen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Wählen Sie oben rechts `Auswählen`.",
          "Haken Sie die Kinder an.",
          "Wählen Sie `Ankunftszeiten` oder `Gehzeiten`.",
          "Tragen Sie die Zeiten ein und wählen Sie `Speichern`.",
          "Wählen Sie `Fertig`.",
        ],
      },
      {
        title: "Einen einzelnen Tag ändern",
        steps: [
          "Öffnen Sie beim Kind den Bereich `Betreuungszeiten`.",
          "Wählen Sie beim gewünschten Tag `Ausnahme`.",
          "Ändern Sie `Ankunft` oder `Abholung`.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "Die Zeiten gelten sofort. Eine Notiz ohne Abholzeit steht auf der Kinderkarte unter `Kommt heute nicht`. Eine Ausnahme ändert den Wochenplan nicht.",
    notes: [
      "Ohne eigene Zeit gilt die Klassenzeit des Kindes.",
      // #3373: Ankunft nicht vor Abholung (comesOnlyIfLessonCancelled in
      // student-time-status.ts) warnt nicht mehr als überfällig.
      "Endet der Unterricht erst zur Abholzeit? Dann steht auf der Kinderkarte `Nur bei Unterrichtsausfall`. Das Kind gilt nicht als verspätet. Kommt es doch, checken Sie es wie gewohnt ein.",
      "Den Wochenplan können Sie schon beim Anlegen des Kindes mitgeben.",
      // #3372: Die Auswahl erscheint nur mit gepflegten Schulstunden
      // (school-period-select.tsx) und kopiert die Uhrzeit ins Zeitfeld.
      "Schneller geht es mit `Nach Schulstunde` unter dem Feld `Ankunft`: Wählen Sie zum Beispiel `5. Stunde`. moto trägt die passende Uhrzeit ein.",
      "Dieselbe Auswahl gibt es für eine ganze Klasse: unter `Kinderdaten` im Menü der Klasse bei `Ankunftszeit bearbeiten`.",
      "Die Auswahl erscheint, wenn unter `Einstellungen` bei `Schulstunden` Uhrzeiten stehen. Jede Schule pflegt ihre eigenen Zeiten.",
      "Ändern Sie später eine Schulstunde, bleiben gespeicherte Zeiten unverändert.",
      // #3371: Textfeld mit Ziffernmaske statt nativem Zeitfeld, Knopf nur
      // mit gepflegter Vorgabe (care-weekly-plan-editor.tsx).
      "Tippen Sie nur Ziffern, zum Beispiel 1600 für 16:00 Uhr.",
      "Steht unter einem leeren Feld zum Beispiel `16:00 Uhr eintragen`? Ein Klick trägt die übliche Zeit Ihrer Schule ein.",
      "Die üblichen Zeiten stehen unter `Einstellungen` bei `Betreuungszeiten`. Jede Schule pflegt ihre eigenen Zeiten.",
    ],
    differences: [
      // Einstellung `enrollment.bookings_authoritative`, Vorgabe aus. Der
      // Satz beginnt bei dem, was zu sehen ist -- den Namen der Einstellung
      // kennt die Leserin nicht (care-weekly-plan-modal.tsx:271).
      "Lassen sich die Betreuungstage nicht anhaken? Dann bestimmen bei Ihnen die gebuchten Betreuungsangebote, wann ein Kind da ist.",
      "An einem Tag ohne Buchung sind `Ankunft` und `Abholung` grau. `Notiz zum Tag (jede Woche)` bleibt frei.",
    ],
    troubleshootingDetails: [
      "Fehlt `Wochenplan`? Dann fehlen Ihnen die Schreibrechte für dieses Kind.",
      "Die Tage kommen aus den Buchungen und das soll anders sein? Das kann nur das moto-Team umstellen.",
    ],
    related: [
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadCarePlan,
    ],
  };
}

/**
 * Uebersichtsseite `/database`. Die Kacheln blenden sich nach Rechten,
 * NFC und Planungsbereich aus (page.tsx:312-336) -- deshalb steht in der
 * Anleitung keine feste Liste, sondern nur die Grenze zum Tagesbetrieb.
 */
function dataManagementTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.dataManagement,
    title: "Die Datenverwaltung im Überblick",
    question: "Wo pflege ich die Stammdaten der OGS?",
    summary:
      "In der `Datenverwaltung` legen Sie an, was die OGS dauerhaft braucht.",
    group: "einrichten",
    audience: "lead",
    icon: "Database",
    steps: [
      "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
      "Wählen Sie `Datenverwaltung`.",
      "Wählen Sie die Kachel des Bereichs, den Sie pflegen möchten.",
    ],
    result:
      "Jede Kachel führt in ihre Liste. Oben steht, wie viele Einträge es gibt.",
    notes: [
      "In der Seitenleiste erreichen Sie jeden Bereich auch direkt unter `Datenverwaltung`.",
      "Die Kacheln `Terminkategorien`, `Planungsspuren`, `Schichtarten` und `Abwesenheitsarten` sind kurze Stammdaten-Listen.",
    ],
    differences: [
      "Sie sehen weniger Kacheln als eine Kollegin? Jede Kachel hängt an einem eigenen Recht.",
      "Ohne NFC fehlen `Aktivitäten` und `Geräte`.",
      "Ohne Planungsbereich fehlen `Planungsspuren` und `Schichtarten`.",
    ],
    troubleshootingDetails: [
      "Suchen Sie ein Kind für heute? Das steht nicht hier, sondern unter `Alle Kinder` im `Tagesbetrieb`.",
    ],
    related: [
      HELP_TOPICS.leadRooms,
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadExports,
    ],
  };
}

/**
 * Einstiegs-Checkliste. Verweist nur, statt Schritte zu wiederholen
 * (Informationsarchitektur Abschnitt 7). Alle drei Einstellungen wirken hier:
 * ohne NFC entfaellt die Geraetevorbereitung, bei einfacher Anwesenheit gibt
 * es im Testlauf keine Aufsicht, und bei offener Betreuung keine Gruppe.
 */
function goLiveTopic(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): HelpTopic {
  const usesGroups = groupMode !== "open_care";
  const tracksRooms = presenceMode !== "binary";
  const usesNfc = nfcEnabled !== false;
  return {
    id: HELP_TOPICS.leadGoLive,
    title: "moto für den ersten Betreuungstag vorbereiten",
    question: "Was muss ich vor dem Start erledigen?",
    summary: "Arbeiten Sie diese Liste einmal von oben nach unten ab.",
    group: "einrichten",
    audience: "lead",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "In dieser Reihenfolge einrichten",
        description:
          "Jeder Punkt hat eine eigene Anleitung. Die Reihenfolge zählt, weil ein Schritt den vorigen braucht.",
        steps: [
          "Legen Sie die `Räume` an.",
          "Legen Sie Ihr Team an und laden Sie es ein.",
          ...(usesGroups
            ? [
                "Legen Sie die `Gruppen` an. Jede Gruppe bekommt einen Raum und eine Gruppenleitung.",
              ]
            : []),
          "Übernehmen Sie die Kinder aus einer Liste. Oder legen Sie sie einzeln an.",
          "Tragen Sie die Betreuungszeiten der Kinder ein.",
          "Tragen Sie `Schuljahr und Ferien` ein.",
          ...(usesNfc
            ? ["Richten Sie die NFC-Geräte und die Armbänder ein."]
            : []),
          "Prüfen Sie zum Schluss die `Einstellungen` Ihrer OGS.",
        ],
      },
      {
        title: "Vor dem ersten Tag einmal durchspielen",
        steps: [
          "Melden Sie sich mit einem Leitungskonto an.",
          ...(usesGroups
            ? ["Öffnen Sie eine Gruppe und prüfen Sie die zugeordneten Kinder."]
            : ["Prüfen Sie unter `Alle Kinder` drei Kinder."]),
          ...(tracksRooms
            ? ["Starten Sie eine Aufsicht und beenden Sie sie wieder."]
            : ["Melden Sie ein Kind an und wieder ab."]),
          "Melden Sie ein Kind krank und heben Sie die Meldung wieder auf.",
          "Weisen Sie Ihr Team kurz ein.",
        ],
        ordered: false,
      },
    ],
    result:
      "Sind alle Punkte erledigt, kann Ihr Team am ersten Tag ohne Vorbereitung starten.",
    notes: [
      "Sie müssen nicht alles an einem Tag schaffen. Die Liste bleibt gültig.",
      "Laden Sie Ihr Team früh ein. Als `Gruppenleitung` wählbar ist nur, wer die Einladung schon angenommen hat.",
    ],
    troubleshootingDetails: [
      "Ein Punkt fehlt Ihnen in der Seitenleiste? Dann fehlen Ihnen die Leitungsrechte. Fragen Sie Ihre Leitung.",
    ],
    related: [
      HELP_TOPICS.leadRooms,
      ...(usesGroups ? [HELP_TOPICS.leadGroups] : []),
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadCareTimes,
      ...(usesNfc ? [HELP_TOPICS.leadTabletSetup] : []),
      HELP_TOPICS.settings,
    ],
  };
}

/**
 * Die Kindakte unter `/students/[id]`. Welche Reiter erscheinen, haengt am
 * Zugriff auf das Kind (FULL_ACCESS_BASE_TABS gegen LIMITED_ACCESS_BASE_TABS,
 * students/[id]/page.tsx:139) und daran, ob die Anmeldung ueber moto laeuft.
 * An keine der drei Arbeitsweisen gebunden.
 */
function manageStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadManageStudent,
    title: "Angaben eines Kindes verwalten",
    question: "Wo pflege ich alle Angaben zu einem Kind?",
    summary: "Die Kindakte hat für jeden Bereich einen eigenen Reiter.",
    group: "kinderdaten",
    audience: "lead",
    icon: "UserRoundSearch",
    steps: [],
    instructionGroups: [
      {
        title: "Die Kindakte öffnen",
        steps: [
          "Öffnen Sie `Alle Kinder` und suchen Sie das Kind.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie oben den passenden Reiter.",
        ],
      },
      {
        title: "Was in welchem Reiter steht",
        steps: [
          "`Stammdaten`: Name, Klasse, Gruppe, Adresse und Notizen.",
          "`Erziehungsberechtigte`: wer das Kind abholen darf und wer erreichbar ist.",
          "`Betreuungszeiten`: der Wochenplan und einzelne Ausnahmen.",
          "`Betreuungsplan`: die geplante Betreuung Woche für Woche.",
          "`Nachrichten`: die Unterhaltungen mit den Bezugspersonen.",
          "`Dokumente`: hochgeladene Dateien zu diesem Kind.",
          "`Änderungsprotokoll`: wer wann was geändert hat.",
          "`Historie`: die vergangenen Betreuungstage.",
        ],
        ordered: false,
      },
      {
        title: "Stammdaten ändern",
        steps: [
          "Wählen Sie den Reiter `Stammdaten`.",
          "Wählen Sie `Bearbeiten`.",
          "Ändern Sie die benötigten Angaben.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result: "Geänderte Angaben gelten sofort für alle im Team.",
    notes: ["Wer eine Angabe geändert hat, steht im `Änderungsprotokoll`."],
    differences: [
      "Sie sehen weniger Reiter? Dann haben Sie nur eingeschränkten Zugriff auf dieses Kind.",
      "Der Reiter `Anmeldungen` erscheint nur, wenn die Anmeldung über moto läuft.",
    ],
    troubleshootingDetails: [
      "Fehlt `Bearbeiten`? Dann fehlen Ihnen die Schreibrechte für dieses Kind.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.leadCareTimes,
      HELP_TOPICS.leadInviteGuardians,
    ],
  };
}

/**
 * Das Einladen ins Elternportal. Der Weg des Teams laeuft ueber das Kind:
 * Reiter `Erziehungsberechtigte`, dann `Einladen` bei der Person
 * (student-guardian-manager.tsx). Ohne E-Mail-Adresse tut der Knopf nichts --
 * `handleInviteGuardian` bricht bei `!guardian.email` still ab.
 *
 * Die Seite `Elternzugänge` (`/admin/guardian-approvals`) ist NICHT die
 * Warteschlange fuer Einladungen des Teams. Sie sammelt nur, was Eltern
 * selbst anstossen, gesteuert von `guardians.parent_invite_mode`. Beides
 * steht hier zusammen, weil es dieselbe Frage beantwortet: wer bekommt
 * Zugang zu einem Kind. Deshalb zeigt auch das Fragezeichen dieser Seite
 * hierher.
 *
 * Eine eingeschraenkte `Portalrolle` (`Notfallkontakt`, `Nur Abholung`,
 * `Sozialdienst`) hat keinen Portalzugriff. Eine Einladung stuft sie hoch
 * und fragt vorher `Vollen Zugriff gewähren?`.
 */
function inviteGuardiansTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadInviteGuardians,
    title: "Eltern ins Elternportal einladen",
    question: "Wie lade ich Eltern ein?",
    summary:
      "Sie laden jede Person beim Kind ein. Dafür brauchen Sie ihre E-Mail-Adresse.",
    group: "elternarbeit",
    audience: "lead",
    icon: "KeyRound",
    requirements: [
      "Das Kind ist in moto angelegt.",
      "Sie kennen die E-Mail-Adresse der Person.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Eine eingetragene Person einladen",
        description:
          "Meistens stehen die Eltern schon beim Kind. Dann sind es vier Schritte.",
        steps: [
          "Öffnen Sie in der Seitenleiste `Alle Kinder`.",
          "Wählen Sie das Kind aus der Liste.",
          "Wechseln Sie auf den Reiter `Erziehungsberechtigte`.",
          "Wählen Sie rechts neben der Person `Einladen`.",
        ],
      },
      {
        title: "Eine Person zuerst eintragen",
        description:
          "Steht die Person noch nicht beim Kind, tragen Sie sie vorher ein. Pflicht sind nur zwei Felder.",
        steps: [
          "Wählen Sie im Reiter `Erziehungsberechtigte` oben rechts `+ Hinzufügen`.",
          "Tragen Sie `Vorname` und `Nachname` ein.",
          "Tragen Sie unter `Kontaktdaten` die `E-Mail` ein. Ohne sie geht keine Einladung raus.",
          "Lassen Sie `Portalrolle` auf `Erziehungsberechtigte/r`.",
          "Wählen Sie unten `Hinzufügen`.",
          "Wählen Sie danach bei der Person `Einladen`.",
        ],
      },
      {
        title: "Eine Anfrage von Eltern freigeben",
        description:
          "Eltern können weitere Bezugspersonen einladen, zum Beispiel eine Großmutter. Je nach Einstellung landet das erst bei Ihnen.",
        steps: [
          "Klappen Sie in der Seitenleiste `Eltern` auf.",
          "Öffnen Sie `Elternzugänge`.",
          "Prüfen Sie bei der Anfrage Name und Kind.",
          "Wählen Sie `Freigeben`.",
        ],
      },
    ],
    result:
      "Die Person bekommt eine E-Mail mit einem Link. Sobald sie ihr Passwort festlegt, sieht sie ihr Kind im Elternportal.",
    notes: [
      "Hinter dem Namen steht, wie weit die Person ist: `Kein Konto`, `Einladung offen` oder `Konto aktiv`.",
      "Bei `Einladung offen` heißt der Knopf `Erneut einladen`. Damit geht die E-Mail noch einmal raus.",
      "Eine Person mit mehreren Kindern laden Sie nur einmal ein. Sie sieht danach alle ihre Kinder.",
    ],
    differences: [
      "Steht bei der Person `Konto aktiv, kein Portalzugriff`, heißt der Knopf `Zugriff gewähren`.",
      "Bei einer eingeschränkten `Portalrolle` wie `Notfallkontakt` oder `Nur Abholung` fragt moto zuerst `Vollen Zugriff gewähren?`. Erst `Zugriff gewähren` erteilt den Zugang.",
      "Unter `Elternzugänge` steht `Einladungen gehen ohne Freigabe raus`? Dann laden Eltern direkt ein. Über `Zu den Einstellungen` ändern Sie das.",
      "Dort steht `Eltern können derzeit niemanden einladen`? Dann dürfen nur Sie einladen.",
    ],
    troubleshootingDetails: [
      "`Einladen` bewirkt nichts? Dann fehlt die `E-Mail`. Tragen Sie sie über `Bearbeiten` nach.",
      "Die Person ist schon bei einem anderen Kind eingetragen? Wählen Sie `Vorhandene/n suchen` statt `+ Hinzufügen`. Sonst entstehen zwei Einträge für dieselbe Person.",
      "Fehlt `Elternzugänge` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
      "Eine Anfrage passt nicht? Wählen Sie `Ablehnen` und bestätigen Sie mit `Ablehnen`. Es wird kein Zugang gewährt.",
    ],
    related: [
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadParentVisibility,
      HELP_TOPICS.parentMessage,
    ],
  };
}

/**
 * Regulaerer Austritt, Assistent `CareExitModal` in zwei Schritten (#2487).
 * Bewusst getrennt vom Loeschen: hier bleiben alle Daten erhalten.
 */
function endCareTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEndCare,
    title: "Die Betreuung eines Kindes beenden",
    question: "Wie beende ich die Betreuung eines Kindes?",
    summary: "Sie setzen einen letzten Betreuungstag. Die Daten bleiben.",
    group: "kinderdaten",
    audience: "lead",
    icon: "LogOut",
    steps: [],
    instructionGroups: [
      {
        title: "Ein einzelnes Kind abmelden",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Wählen Sie das Kind aus der Liste.",
          "Wählen Sie oben rechts `Betreuung beenden`.",
          "Tragen Sie bei `Letzter Betreuungstag` den Tag ein.",
          "Wählen Sie einen Grund: `Umzug`, `Kein Betreuungsbedarf mehr` oder `Anderer Grund`.",
          "Wählen Sie `Weiter`.",
          "Prüfen Sie die Vorschau und wählen Sie `Betreuung beenden`.",
        ],
      },
      {
        title: "Mehrere Kinder auf einmal abmelden",
        steps: [
          "Wählen Sie oben rechts `Auswählen`.",
          "Haken Sie die Kinder an.",
          "Wählen Sie `Betreuung beenden`.",
          "Gehen Sie die beiden Schritte wie oben durch.",
        ],
      },
    ],
    result:
      "Ab dem Tag nach dem letzten Betreuungstag ist das Kind nicht mehr in Betreuung.",
    notes: [
      "Bei `Anderer Grund` tragen Sie zusätzlich eine kurze Erklärung ein.",
      "Beendete Kinder finden Sie über das Menü mit den drei Punkten unter `Beendete Betreuungen`.",
    ],
    differences: [
      "Ist schon ein Ende geplant, heißt der Knopf `Ende ändern`. Daneben steht `Ende stornieren`.",
      "Ist die Betreuung schon beendet, steht dort `Wieder aufnehmen`.",
    ],
    troubleshootingDetails: [
      "Die Vorschau blockiert das Beenden? Dann sprechen die geprüften Angaben dagegen. Lesen Sie den Hinweis in der Vorschau.",
      "Fehlt `Wieder aufnehmen`? Dann lief die Betreuung mit der Anmeldephase aus. Das lässt sich nicht zurücknehmen.",
    ],
    related: [
      HELP_TOPICS.leadDeleteStudent,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadGradeTransition,
    ],
  };
}

/**
 * Endgueltiges Loeschen, `StudentDeletionModal`. Textbestaetigung statt
 * Zwei-Klick-Tor: der Name des Kindes muss abgetippt werden (Bauart 2).
 */
function deleteStudentTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDeleteStudent,
    title: "Ein Kind dauerhaft löschen",
    question: "Wie lösche ich ein Kind endgültig?",
    summary: "Löschen entfernt die Daten des Kindes. Das ist endgültig.",
    group: "kinderdaten",
    audience: "lead",
    icon: "Trash2",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
      "Wählen Sie das Kind aus der Liste.",
      "Wählen Sie oben rechts `Löschen`.",
      "Lesen Sie, welche Daten entfernt und welche gelöst werden.",
      "Wählen Sie einen Grund für das Löschen.",
      "Haken Sie die Bestätigung an.",
      "Tippen Sie den Namen des Kindes in das Feld.",
      "Wählen Sie `Kind endgültig löschen`.",
    ],
    result:
      "Das Kind ist entfernt. Dieser Schritt lässt sich nicht zurücknehmen.",
    notes: [
      "Als Grund stehen `Testdaten`, `Fehlerhafte Erfassung`, `Doppelter Datensatz` und `Datenschutz-/Löschanfrage` zur Wahl.",
      "Ist die Betreuung bereits beendet, kommt `Aufbewahrungsfrist abgelaufen` dazu.",
    ],
    differences: [
      "Ein geplanter letzter Betreuungstag wird nicht abgewartet. Das Kind wird sofort gelöscht.",
    ],
    troubleshootingDetails: [
      "`Kind endgültig löschen` bleibt grau? Dann fehlt der Grund, das Häkchen oder der getippte Name.",
      "Soll das Kind nur aufhören? Nutzen Sie `Betreuung beenden`. Dabei bleiben alle Daten erhalten.",
    ],
    related: [
      HELP_TOPICS.leadEndCare,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.dataManagement,
    ],
  };
}

/**
 * Jahrgangswechsel unter `/database/grade-transitions`. Nur am Computer: die
 * Seite rendert ihren Inhalt erst ab `lg` (page.tsx:69) und zeigt darunter
 * einen Hinweis. Braucht `grade_transitions:read`, das Anwenden zusaetzlich
 * `grade_transitions:apply`.
 */
function gradeTransitionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadGradeTransition,
    title: "Kinder in den nächsten Jahrgang übernehmen",
    question: "Wie übernehme ich die Kinder in das neue Schuljahr?",
    summary: "moto schlägt für jede Klasse die nächste vor. Sie prüfen nur.",
    group: "kinderdaten",
    audience: "lead",
    icon: "ArrowUpRight",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Jahrgangswechsel`.",
      "Wählen Sie `Neuer Jahrgangswechsel`.",
      "Tragen Sie bei `Schuljahr (neu)` das neue Schuljahr ein, zum Beispiel `2026-2027`.",
      "Tragen Sie bei Bedarf eine `Notiz (optional)` ein.",
      "Prüfen Sie die vorgeschlagenen Klassen und passen Sie sie an.",
      "Wählen Sie `Weiter zur Vorschau`.",
      "Prüfen Sie die Vorschau.",
      "Wählen Sie `Jahrgangswechsel anwenden`. Bestätigen Sie mit `Ja, anwenden`.",
    ],
    result:
      "Alle Kinder stehen in der neuen Klasse. Abgänge sind aus der Betreuung heraus.",
    notes: [
      "moto schlägt jede Klasse automatisch vor, zum Beispiel `1a` in `2a`.",
      "Ein Entwurf bleibt liegen. Über `Zurück zum Entwurf` arbeiten Sie später weiter.",
    ],
    differences: [
      "Auf dem Handy erscheint nur ein Hinweis. Der Jahrgangswechsel läuft am Computer.",
      "Ohne das Recht zum Anwenden sehen Sie die Vorschau, aber keinen Knopf zum Anwenden.",
    ],
    troubleshootingDetails: [
      "Die Vorschau meldet geänderte Daten? Dann hat jemand zwischendurch Klassen geändert. Prüfen Sie erneut und bestätigen Sie dann.",
      "Fehlt `Jahrgangswechsel`? Dann fehlt Ihnen das Recht dafür.",
    ],
    related: [
      HELP_TOPICS.leadEndCare,
      HELP_TOPICS.leadManageStudent,
      HELP_TOPICS.leadCalendarPeriods,
    ],
  };
}

/**
 * Klassenlisteneintraege unter `/database/students/class-list` (#2382):
 * Kinder ohne OGS-Betreuung, damit Klassenlisten vollstaendig sind.
 */
function classListEntriesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadClassListEntries,
    title: "Kinder ohne OGS-Betreuung erfassen",
    question: "Wie erfasse ich Kinder, die nicht in der OGS sind?",
    summary: "Für vollständige Klassenlisten erfassen Sie auch diese Kinder.",
    group: "kinderdaten",
    audience: "lead",
    icon: "ClipboardList",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Eintrag anlegen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Kinderdaten`.",
          "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
          "Wählen Sie `Klassenliste`.",
          "Wählen Sie oben rechts `+ Eintrag`.",
          "Tragen Sie Vorname, Nachname und Klasse ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Viele Einträge auf einmal",
        steps: [
          "Öffnen Sie `Klassenliste`.",
          "Wählen Sie oben `Sammelimport`.",
          "Folgen Sie den Schritten auf der Seite.",
        ],
      },
    ],
    result:
      "Das Kind erscheint auf den Klassenlisten, aber nicht in der Betreuung.",
    notes: [
      "Sie können ein Kind auch beim Anlegen über `Nur Klassenliste` erfassen.",
      "Schreiben Sie die Klasse genau wie bei den regulären Kindern.",
    ],
    differences: [
      "Ein Eintrag trägt `Mögliche Dublette`? Dann gibt es ein reguläres Kind mit demselben Namen in derselben Klasse.",
      "Ein Eintrag trägt `In moto angelegt`? Dann ist es ein reguläres Kind. Gepflegt wird es unter `Kinderdaten`.",
    ],
    troubleshootingDetails: [
      "Kommt das Kind doch in die Betreuung? Ordnen Sie den Eintrag dem regulären Kind zu. Der Eintrag verschwindet dann aus der Klassenliste.",
    ],
    related: [
      HELP_TOPICS.leadCreateStudent,
      HELP_TOPICS.leadDayLists,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}

/**
 * Mitteilung, Elternbrief und Umfrage liegen auf derselben Seite
 * `/parent-announcements`; `KIND_ITEMS` schaltet zwischen ihnen um
 * (page.tsx:136). Deshalb teilen sich die drei Artikel denselben Weg dorthin
 * und unterscheiden sich nur in dem, was die Art ausmacht.
 */
function parentAnnouncementSteps(
  kindTab: string,
  actionLabel: string,
  titleField: string,
): readonly string[] {
  return [
    "Klappen Sie in der Seitenleiste `Eltern` auf.",
    "Öffnen Sie `Mitteilungen`.",
    `Wählen Sie oben \`${kindTab}\`.`,
    `Wählen Sie oben rechts \`+ ${actionLabel}\`.`,
    `Tragen Sie bei \`${titleField}\` ein, worum es geht.`,
    "Tragen Sie bei `Text` die Nachricht ein.",
    "Wählen Sie unten `Weiter`.",
    "Wählen Sie in `Schritt 2 von 2` unter `Wer soll diese Mitteilung erhalten?` die Empfänger.",
  ];
}

function parentAnnouncementTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentAnnouncement,
    title: "Eine Elternmitteilung veröffentlichen",
    question: "Wie erreiche ich alle Eltern auf einmal?",
    summary:
      "Eine Mitteilung erscheint im Elternportal der gewählten Familien.",
    group: "elternarbeit",
    audience: "lead",
    icon: "Megaphone",
    steps: [
      ...parentAnnouncementSteps("Mitteilungen", "Mitteilung", "Titel"),
      "Wählen Sie `Veröffentlichen`.",
    ],
    result:
      "Die Mitteilung steht sofort im Elternportal der erreichten Eltern.",
    notes: [
      "Mit `Als Entwurf speichern` legen Sie die Mitteilung zurück, ohne sie zu senden.",
      "Sie können mehrere Empfänger kombinieren: `Ganze Schule`, `Offene Anmeldungen`, `Klassen`, `Gruppen`, `AGs / Betreuung` oder einzelne Kinder.",
      "Jedes Elternteil bekommt die Mitteilung höchstens einmal.",
      "Mit `Dateien anhängen` geben Sie Dateien mit. Sie liegen im Elternportal und gehen nicht per E-Mail mit.",
    ],
    differences: [
      "Eine Mitteilung trägt `Entwurf`, `Veröffentlicht` oder `Abgelaufen`.",
      "Unter `Priorität` stehen `Info` und `Wichtig` zur Wahl.",
      "Mit `Ablaufdatum (optional)` blenden Sie die Mitteilung später wieder aus.",
    ],
    troubleshootingDetails: [
      "`Veröffentlichen` bleibt grau? Dann fehlt noch ein Empfänger.",
      "Soll die Nachricht auch per E-Mail gehen? Schalten Sie `Eltern zusätzlich per E-Mail benachrichtigen` ein.",
      "Sollen Eltern den Erhalt bestätigen? Schalten Sie `Lesebestätigung erforderlich` ein.",
    ],
    related: [
      HELP_TOPICS.leadParentLetter,
      HELP_TOPICS.leadParentSurvey,
      HELP_TOPICS.parentMessage,
    ],
  };
}

/**
 * Elternbrief: Mitteilung, die zusaetzlich per E-Mail geht und von den Eltern
 * bestaetigt wird (page.tsx:171).
 */
function parentLetterTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentLetter,
    title: "Einen Elternbrief versenden",
    question: "Wie versende ich einen Elternbrief?",
    summary: "Ein Elternbrief geht zusätzlich per E-Mail und wird bestätigt.",
    group: "elternarbeit",
    audience: "lead",
    icon: "FileText",
    steps: [
      ...parentAnnouncementSteps("Elternbriefe", "Elternbrief", "Titel"),
      "Wählen Sie unter `Wer erhält die E-Mail?`, wer sie bekommt.",
      "Wählen Sie `Veröffentlichen`.",
    ],
    result:
      "Der Brief steht im Elternportal. Die erreichten Eltern bekommen zusätzlich eine E-Mail.",
    notes: [
      "Unter `Wer erhält die E-Mail?` wählen Sie `Nur mit Portalzugang` oder `Alle Bezugspersonen`.",
      "Die Eltern bestätigen den Brief. Eine Bestätigung pro Kind genügt.",
      "Der Kasten im Fenster zählt auf, was beim Veröffentlichen automatisch passiert.",
    ],
    differences: [
      "Nach dem Veröffentlichen lässt sich ein Brief nicht mehr ändern.",
    ],
    troubleshootingDetails: [
      "Reicht eine Nachricht ohne E-Mail und ohne Bestätigung? Nehmen Sie stattdessen eine Mitteilung. Dort sind beides Schalter.",
    ],
    related: [
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadParentSurvey,
      HELP_TOPICS.leadInviteGuardians,
    ],
  };
}

/**
 * Umfrage: Mitteilung mit Antwortmoeglichkeiten (#1371). Im Formular heisst
 * das Titelfeld deshalb `Frage` statt `Titel` (page.tsx:1363).
 */
function parentSurveyTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentSurvey,
    title: "Eine Umfrage erstellen",
    question: "Wie frage ich die Eltern nach ihrer Rückmeldung?",
    summary: "Eine Umfrage sammelt die Antworten der Eltern an einem Ort.",
    group: "elternarbeit",
    audience: "lead",
    icon: "ListChecks",
    steps: [
      ...parentAnnouncementSteps("Umfragen", "Umfrage", "Frage"),
      "Tragen Sie unter `Antwortmöglichkeiten` die Antworten ein. `Ja` und `Nein` stehen schon da, `+ Antwort hinzufügen` ergänzt weitere.",
      "Tragen Sie bei Bedarf ein Datum bei `Antwortfrist (optional)` ein.",
      "Wählen Sie `Veröffentlichen`.",
    ],
    result: "Die Eltern sehen die Frage in moto und können antworten.",
    notes: [
      "Mit `Mehrfachauswahl erlauben` dürfen Eltern mehrere Antworten wählen.",
      "Die Ergebnisse sehen Sie später beim Öffnen der Umfrage.",
      "Zwei bis zehn Antworten sind möglich. Eltern antworten für jedes Kind einzeln.",
    ],
    differences: ["Das erste Feld heißt hier `Frage`, nicht `Titel`."],
    troubleshootingDetails: [
      "Brauchen Sie keine Antwort? Nehmen Sie stattdessen eine Mitteilung.",
    ],
    related: [
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadParentLetter,
      HELP_TOPICS.parentRequests,
    ],
  };
}

/**
 * Essensplan unter `/meal-plan`. Wochenweise, mit ausdruecklichem Speichern:
 * Aenderungen sind bis dahin nur Entwurf (page.tsx:799).
 */
function mealPlanTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadMealPlan,
    title: "Einen Essensplan veröffentlichen",
    question: "Wie zeige ich den Eltern den Essensplan?",
    summary:
      "Tragen Sie die Gerichte je Tag ein. Eltern sehen den Plan sofort.",
    group: "elternarbeit",
    audience: "lead",
    icon: "UtensilsCrossed",
    steps: [
      "Klappen Sie in der Seitenleiste `Eltern` auf.",
      "Öffnen Sie `Essensplan`.",
      "Blättern Sie mit den Pfeilen zur gewünschten Woche.",
      "Tragen Sie bei einem Tag unter `Gericht eintragen...` das Gericht ein.",
      "Tragen Sie bei Bedarf einen `Hinweis (optional)` dazu ein.",
      "Mit `+ Gericht` ergänzen Sie ein zweites Gericht für denselben Tag.",
      "Wählen Sie `Speichern`.",
    ],
    result: "Der Plan erscheint sofort im Elternportal.",
    notes: [
      "Pro Tag sind mehrere Gerichte möglich.",
      "Mit `Vorwoche übernehmen` holen Sie die Gerichte der Vorwoche in diese Woche.",
      "Der heutige Tag trägt das Kennzeichen `Heute`.",
    ],
    differences: [
      "Ihre Änderungen zählen erst mit `Speichern`. Vorher steht `Verwerfen` daneben.",
      "Beim Wechseln der Woche mit offenen Änderungen fragt moto nach.",
    ],
    troubleshootingDetails: [
      "Fehlt `Essensplan` in der Seitenleiste? Dann ist die Funktion für Ihre OGS ausgeschaltet.",
    ],
    related: [
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadParentVisibility,
      HELP_TOPICS.settings,
    ],
  };
}

/**
 * Bankverbindungen unter `/eltern/bankverbindungen`. Reine Leseansicht mit
 * Export; die IBAN pflegen die Eltern selbst im Portal. Sichtbar nur mit
 * `guardians:financial` (sidebar.tsx, PARENT_SUB_PAGES-Filter).
 */
function bankDetailsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadBankDetails,
    title: "Bankverbindungen einsehen",
    question: "Wo finde ich die Bankverbindungen der Familien?",
    summary: "Sie sehen je Kind die hinterlegte IBAN und können sie ausgeben.",
    group: "elternarbeit",
    audience: "lead",
    icon: "Landmark",
    steps: [
      "Klappen Sie in der Seitenleiste `Eltern` auf.",
      "Öffnen Sie `Bankverbindungen`.",
      "Wählen Sie `Alle Kinder` oder `Ohne IBAN`.",
      "Wählen Sie das Format: `PDF`, `Excel` oder `Word`.",
      "Wählen Sie `Herunterladen`.",
    ],
    result: "Die Liste wird als Datei erstellt und heruntergeladen.",
    notes: [
      "Die IBAN tragen Sie beim Kind ein, im Reiter `Erziehungsberechtigte`.",
      "Die Datei enthält die vollständigen IBANs und wird protokolliert. Geben Sie sie nicht per E-Mail weiter.",
      "`Ohne IBAN` zeigt, bei welchen Kindern noch etwas fehlt.",
    ],
    differences: [
      "Fehlt bei einem Kind die IBAN, steht dort `Nicht zugeordnet`.",
    ],
    troubleshootingDetails: [
      "Fehlt `Bankverbindungen` in der Seitenleiste? Dafür braucht es ein eigenes Recht. Fragen Sie Ihre Leitung.",
      "`Herunterladen` bleibt grau? Dann ist die Liste leer.",
    ],
    related: [
      HELP_TOPICS.leadInviteGuardians,
      HELP_TOPICS.leadExports,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}

/**
 * Anmeldung vorbereiten: Phase, Angebote und Formular. Die drei Seiten liegen
 * im Akkordeon `Anmeldungen` der Gruppe `Eltern`; die Uebersicht selbst
 * (`/admin/enrollments`) rendert ihren Inhalt erst ab `lg` (page.tsx:44).
 */
function enrollmentSetupTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentSetup,
    title: "Eine Anmeldung vorbereiten",
    question: "Wie öffne ich die Anmeldung für das nächste Schuljahr?",
    summary: "Drei Schritte: Phase anlegen, Angebote pflegen, Formular prüfen.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "CalendarPlus",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Anmeldephase anlegen",
        description:
          "Die Phase kommt zuerst. Betreuungsangebote gehören immer zu einer Phase, und ohne sie bleibt die Seite `Betreuungsangebote` leer.",
        steps: [
          "Klappen Sie in der Seitenleiste `Eltern` auf.",
          "Öffnen Sie `Anmeldungen` und danach `Anmeldephasen`.",
          "Wählen Sie `Neue Anmeldephase`.",
          "Tragen Sie bei `Name` ein, worum es geht, zum Beispiel `Schuljahr 2026/27`.",
          "Wählen Sie den `Typ`.",
          "Tragen Sie `Beginn` und Ende der Betreuung ein.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Angebote und Formular vorbereiten",
        description:
          "Beides gehört zu der Phase, die Sie gerade angelegt haben. Wählen Sie sie oben auf der Seite aus, falls Sie mehrere haben.",
        steps: [
          "Öffnen Sie `Betreuungsangebote` und tragen Sie ein, was buchbar sein soll.",
          "Öffnen Sie `Anmeldeformulare`. Dort steht schon ein `Basisformular`. Für die meisten OGS reicht es.",
          "Sehen Sie es sich mit `Vorschau` an.",
          "Öffnen Sie `Überblick` und wählen Sie `Formular ansehen`.",
        ],
      },
      {
        title: "Die Anmeldung öffnen",
        steps: [
          "Öffnen Sie `Anmeldephasen`.",
          "Öffnen Sie bei der Phase das Menü mit den drei Punkten.",
          "Wählen Sie `Aktivieren`.",
          "Kopieren Sie den Link der Phase und geben Sie ihn an die Eltern.",
        ],
      },
    ],
    result:
      "Eltern können sich über den Link anmelden. Eingänge sehen Sie im `Überblick`.",
    notes: [
      "Über `Elternansicht öffnen` sehen Sie das Formular so wie die Eltern.",
      "Das `Basisformular` fragt Elternteil, Kind, Klassenstufe und das gewünschte Betreuungsangebot ab. Brauchen Sie mehr, legen Sie eine eigene Vorlage an.",
      "Eine Phase lässt sich mit einem Kalenderzeitraum verknüpfen. Dann passen die Daten automatisch.",
    ],
    differences: [
      "Auf dem Handy erscheint nur ein Hinweis. Die Anmeldungen laufen am Computer.",
      "Eine Phase trägt `Aktiv` oder nichts. Nur eine aktive Phase nimmt Anmeldungen an.",
    ],
    troubleshootingDetails: [
      "Der Überblick sagt `Noch keine Anmeldephase`? Dann legen Sie zuerst eine Phase an.",
      "Auf `Betreuungsangebote` steht `Erst eine Anmeldephase anlegen`? Dann fehlt die Phase noch. Angebote gibt es nur innerhalb einer Phase.",
      "Oben steht `Anschlussphase fehlt`? Dann laufen die Buchungen aus. Wählen Sie `Anschlussphase erstellen`.",
      "Fehlt `Anmeldungen` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadEnrollmentForm,
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadEnrollmentExport,
      HELP_TOPICS.leadCalendarPeriods,
    ],
  };
}

/**
 * Der Formular-Editor unter `/enrollment-form`. Zwei Dinge sind hier
 * folgenreich und stehen deshalb gross im Artikel:
 *
 * 1. Das `Basisformular` existiert immer und ist ein Systemformular. Die
 *    Seite raet selbst davon ab, ohne Not eine Vorlage anzulegen. Wer das
 *    nicht weiss, baut eine ueberfluessige Vorlage.
 * 2. `Stammdaten-Vorschläge` schreiben die Antwort in die Daten des Kindes
 *    (FIELD_TARGET_LABELS, enrollment-form-editor.tsx:139), eine
 *    `Freie Zusatzfrage` bleibt bei der Anmeldung. Gleiche Frage, anderes
 *    Ergebnis -- und im Formular sehen beide gleich aus.
 *
 * Die Vorlage wirkt erst, wenn eine Anmeldephase sie auswaehlt: dort im
 * Abschnitt `Formular` die Option `Eigene Vorlage wählen`
 * (phases-editor.tsx:1565).
 */
function enrollmentFormTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentForm,
    title: "Ein eigenes Anmeldeformular anlegen",
    question: "Wie frage ich bei der Anmeldung mehr ab?",
    summary:
      "Nur nötig, wenn das Basisformular nicht reicht. Sie legen dann eine Vorlage an.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "FileText",
    requirements: [
      "Sie haben das `Basisformular` angesehen und etwas vermisst.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Erst prüfen, ob Sie eine Vorlage brauchen",
        description:
          "Das `Basisformular` ist immer da und fragt Elternteil, Kind, Klassenstufe und das gewünschte Betreuungsangebot ab. Für die meisten OGS reicht das.",
        steps: [
          "Klappen Sie in der Seitenleiste `Eltern` auf.",
          "Öffnen Sie `Anmeldungen` und danach `Anmeldeformulare`.",
          "Wählen Sie beim `Basisformular` die `Vorschau`.",
          "Fehlt nichts? Dann sind Sie fertig. Legen Sie keine Vorlage an.",
        ],
      },
      {
        title: "Eine Vorlage anlegen",
        description:
          "Eine Vorlage ergänzt das Basisformular. Sie ersetzt es nicht.",
        steps: [
          "Wählen Sie oben rechts `+ Neue Vorlage`.",
          "Tragen Sie bei `Name der Vorlage` ein, wofür sie gilt, zum Beispiel `Ferienbetreuung Sommer 2026`.",
          "Gehen Sie `Pflichtstatus der Basisfelder` durch. Was auf `Optional` steht, können Sie zur Pflicht machen.",
          "Ergänzen Sie unter `Was Eltern zusätzlich beantworten sollen` Ihre Fragen.",
          "Sehen Sie rechts in der Vorschau nach, wie das Formular wirkt.",
          "Wählen Sie unten `Formularvorlage erstellen`.",
        ],
      },
      {
        title: "Die richtige Sorte Frage wählen",
        description:
          "Zwei Sorten sehen im Formular gleich aus, landen aber woanders. Das ist die wichtigste Entscheidung in diesem Fenster.",
        steps: [
          "Soll die Antwort später beim Kind stehen? Nehmen Sie einen `Stammdaten-Vorschlag` und wählen Sie `Hinzufügen`.",
          "Zur Auswahl stehen unter anderem `Abholzeiten`, `Ankunftszeiten`, `Erlaubte Heimwege` und `Gesundheitsinformationen`.",
          "Geht es nur um diese eine Anmeldung? Wählen Sie `+ Freie Zusatzfrage`.",
          "Wollen Sie nur etwas erklären, ohne zu fragen? Wählen Sie `Infotext`.",
        ],
      },
      {
        title: "Die Vorlage in der Anmeldephase auswählen",
        description:
          "Eine Vorlage wirkt erst, wenn eine Phase sie benutzt. Ohne diesen Schritt sehen die Eltern weiter das Basisformular.",
        steps: [
          "Öffnen Sie `Anmeldephasen`.",
          "Öffnen Sie die Phase zum Bearbeiten.",
          "Wählen Sie im Abschnitt `Formular` die Option `Eigene Vorlage wählen`.",
          "Wählen Sie Ihre Vorlage aus der Liste.",
          "Speichern Sie die Phase.",
        ],
      },
      {
        title: "Eigene Texte für Eltern übersetzen",
        description:
          "Eltern wählen im Formular oben ihre Sprache. moto übersetzt nur die festen Texte. Ihre eigenen Fragen und Zustimmungen übersetzen Sie selbst.",
        steps: [
          "Öffnen Sie Ihre Vorlage und gehen Sie nach unten zu `Übersetzungen für Eltern`.",
          "Wählen Sie die Sprache, zum Beispiel `Русский`.",
          "Links steht Ihr deutscher Text. Tragen Sie rechts bei `Übersetzung` den Text in der Sprache ein.",
          "Wählen Sie unten `Änderungen speichern`.",
          "Denselben Abschnitt finden Sie beim Bearbeiten einer Anmeldephase und eines Betreuungsangebots.",
        ],
      },
    ],
    result:
      "Eltern füllen in dieser Phase das Basisformular und Ihre zusätzlichen Fragen aus.",
    notes: [
      "Eine Vorlage gilt für alle Phasen, die sie auswählen. Sie brauchen nicht für jede Phase eine eigene.",
      "Mit `Vorschau öffnen` sehen Sie den Entwurf in einem neuen Tab, bevor Sie speichern.",
      "Die Zustimmungen `Datenschutzinformation`, `Fotoeinwilligung` und `E-Mail-Kontakt` tragen `Aus Einstellungen übernommen`. Sie kommen aus Ihren Einstellungen und lassen sich je Vorlage ausblenden.",
      "Über `+ Eigene Zustimmung hinzufügen` ergänzen Sie einen Text, der nur in dieser Vorlage steht.",
    ],
    differences: [
      "Bei einem `Stammdaten-Vorschlag` können Sie Beschriftung und Typ nicht ändern. Beides ist fest, damit die Antwort beim Kind ankommt.",
      "Manche Basisfelder stehen auf `Immer Pflicht` und lassen sich nicht abschalten, zum Beispiel `E-Mail (Elternteil)`.",
      "Steht bei einer Zustimmung `Ist für diese Vorlage ausgeblendet`, fragt dieses Formular sie nicht ab.",
    ],
    troubleshootingDetails: [
      "Die Eltern sehen Ihre Fragen nicht? Dann steht die Phase noch auf `Basisformular`. Wählen Sie dort `Eigene Vorlage wählen`.",
      "In der Phase steht `Noch keine eigenen Formulare vorhanden`? Dann ist die Vorlage noch nicht gespeichert.",
      "Eine Antwort taucht beim Kind nicht auf? Dann war es eine `Freie Zusatzfrage`. Nur `Stammdaten-Vorschläge` schreiben in die Daten des Kindes.",
      "Eltern lesen einen Text auf Deutsch, obwohl Sie ihn übersetzt haben? Dann haben Sie den deutschen Text danach geändert. Bei der Übersetzung steht `Bitte prüfen`. Passen Sie sie an oder wählen Sie `Passt noch`, und speichern Sie.",
      "Fehlt `Anmeldungen` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadEnrollmentSetup,
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}

/**
 * Pruefung einer eingegangenen Anmeldung, `/admin/enrollments/[id]`. Die vier
 * Entscheidungen stehen in DECISION_ACTIONS (admin-enrollment-detail.tsx:100).
 */
function enrollmentReviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.enrollments,
    title: "Eingegangene Anmeldungen prüfen",
    question: "Wie prüfe ich eine neue Anmeldung?",
    summary: "Sie prüfen die Angaben und entscheiden je Kind.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "ListChecks",
    steps: [
      "Klappen Sie in der Seitenleiste `Eltern` auf.",
      "Öffnen Sie `Anmeldungen` und danach `Überblick`.",
      "Wählen Sie bei der Phase `Anmeldungen ansehen`.",
      "Öffnen Sie eine Anmeldung aus der Liste.",
      "Prüfen Sie die `Angaben der Kinder` und die `Zusatzfragen (Eltern)`.",
      "Wählen Sie eine Entscheidung: `Bestätigen`, `Warteliste`, `Ablehnen` oder `Zur Prüfung`.",
    ],
    result:
      "Die Eltern sehen die Entscheidung bei ihrer Anmeldung. Bei `Bestätigen` wird das Kind übernommen.",
    notes: [
      "Sie entscheiden je Kind. Eine Anmeldung kann mehrere Kinder enthalten.",
      "Mit `Statusseite öffnen` sehen Sie, was die Eltern sehen.",
      "Offene Rückfragen der Eltern stehen im Bereich `Anfragen`.",
    ],
    differences: [
      "Der Überblick zählt je Phase, wie viele Eingänge offen, bestätigt und abgelehnt sind.",
      "Auf dem Handy erscheint nur ein Hinweis. Die Prüfung läuft am Computer.",
    ],
    troubleshootingDetails: [
      "Eine Anmeldung war versehentlich abgelehnt? Wählen Sie `Anmeldung wiederherstellen`.",
    ],
    related: [
      HELP_TOPICS.leadEnrollmentSetup,
      HELP_TOPICS.leadEnrollmentCleanup,
      HELP_TOPICS.parentRequests,
    ],
  };
}

/** Export einer Phase, `ExportMenuButton` in der Kopfzeile der Phasenseite. */
function enrollmentExportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentExport,
    title: "Anmeldungen exportieren",
    question: "Wie bekomme ich die Anmeldungen als Datei?",
    summary: "Jede Phase lässt sich als Datei herunterladen.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "Download",
    steps: [
      "Öffnen Sie `Anmeldungen` und danach `Überblick`.",
      "Wählen Sie bei der Phase `Anmeldungen ansehen`.",
      "Wählen Sie oben `Anmeldungen exportieren`.",
      "Wählen Sie `Als PDF exportieren`, `Als Word-Dokument exportieren` oder `Als Excel-Datei exportieren`.",
    ],
    result: "Die Datei wird erstellt und heruntergeladen.",
    notes: [
      "Oben steht, wie viele Eingänge die Phase hat und wie viele davon offen sind.",
      "Die Seite hat drei Exporte: `Anmeldungen exportieren`, `Klassenliste exportieren` und `Auswertung exportieren`.",
      "`Klassenliste exportieren` gilt für die Klasse, die Sie unter `Klasse für Klassenliste` wählen.",
    ],
    differences: [
      "Auf dem Handy erscheint nur ein Hinweis. Der Export läuft am Computer.",
    ],
    troubleshootingDetails: [
      "Brauchen Sie Listen aus dem laufenden Betrieb? Die stehen unter `Exporte` in der `Datenverwaltung`.",
    ],
    related: [
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadExports,
      HELP_TOPICS.leadEnrollmentSetup,
    ],
  };
}

/** Loeschen einer Anmeldung aus der Detailansicht (Knopf im Statusbereich). */
function enrollmentCleanupTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadEnrollmentCleanup,
    title: "Eine fehlerhafte Anmeldung löschen",
    question: "Wie entferne ich eine doppelte oder falsche Anmeldung?",
    summary: "Löschen entfernt die ganze Anmeldung samt aller Kinder darin.",
    group: "anmeldeverwaltung",
    audience: "lead",
    icon: "Trash2",
    steps: [
      "Öffnen Sie `Anmeldungen` und danach `Überblick`.",
      "Wählen Sie bei der Phase `Anmeldungen ansehen`.",
      "Öffnen Sie die betroffene Anmeldung.",
      "Wählen Sie rechts unter `Status der Anmeldung` den Knopf `Gesamte Anmeldung löschen`.",
      "Bestätigen Sie mit `Ja, löschen`.",
    ],
    result: "Die Anmeldung ist entfernt. Die Eltern können sich neu anmelden.",
    notes: [
      "Soll die Anmeldung nur nicht angenommen werden? Wählen Sie stattdessen `Ablehnen`.",
    ],
    differences: [
      "Eine abgelehnte Anmeldung lässt sich mit `Anmeldung wiederherstellen` zurückholen. Eine gelöschte nicht.",
    ],
    troubleshootingDetails: [
      "Ist das Kind schon übernommen? Dann ändert das Löschen der Anmeldung daran nichts. Beenden Sie stattdessen die Betreuung des Kindes.",
    ],
    related: [
      HELP_TOPICS.enrollments,
      HELP_TOPICS.leadEndCare,
      HELP_TOPICS.leadEnrollmentSetup,
    ],
  };
}

/**
 * Personal anlegen: einzeln ueber `/database/personal` mit Einladung per
 * E-Mail (`InvitationForm`), viele auf einmal unter
 * `/database/personal/import`. An keine der drei Arbeitsweisen gebunden;
 * sichtbar mit `staff:manage` oder `staff:stammdaten` (sidebar.tsx:704).
 */
function inviteStaffTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadInviteStaff,
    title: "Mitarbeitende anlegen und einladen",
    question: "Wie bekommt das Team Zugang zu moto?",
    summary: "Einzeln über eine Einladung. Viele auf einmal über eine Vorlage.",
    group: "einrichten",
    audience: "lead",
    icon: "UserRoundPlus",
    steps: [],
    instructionGroups: [
      {
        title: "Eine Person einzeln einladen",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
          "Wählen Sie oben rechts `+ Personal`.",
          "Tragen Sie die `E-Mail-Adresse` ein.",
          "Wählen Sie eine `Rolle`.",
          "Tragen Sie bei Bedarf `Vorname (optional)` und `Nachname (optional)` ein.",
          "Tragen Sie bei Bedarf eine `Position (optional)` ein, zum Beispiel `Pädagogische Fachkraft`.",
          "Wählen Sie `Einladung senden`.",
        ],
      },
      {
        title: "Viele Personen auf einmal übernehmen",
        description:
          "Für ein ganzes Team ist die Vorlage schneller als einzelne Einladungen.",
        steps: [
          "Öffnen Sie `Personal` und wählen Sie oben `Importieren`.",
          "Wählen Sie bei `Format wählen` `Excel (.xlsx)` oder `CSV (Komma-getrennt)`.",
          "Wählen Sie `Vorlage herunterladen`.",
          "Füllen Sie `Vorname`, `Nachname` und `Rolle` aus. Diese drei sind Pflicht.",
          "Tragen Sie bei Bedarf E-Mail, Personalnummer, Adresse und Vertrag ein.",
          "Wählen Sie unter `Was soll der Import tun?` `Nur neue anlegen`, `Nur bestehende aktualisieren` oder `Beides`.",
          "Laden Sie die Datei hoch und prüfen Sie die Vorschau.",
          "Bestätigen Sie den Import.",
        ],
      },
    ],
    result:
      "Wer eine E-Mail-Adresse hat, bekommt einen Link und legt sein Passwort selbst fest.",
    notes: [
      "Die `Rolle` bestimmt, was die Person später sehen und tun darf.",
      "Nach der Annahme steht die Person unter `Mitarbeiter` und lässt sich einer Gruppe zuweisen.",
      "Das Blatt `Hinweise` in der Vorlage erklärt jede Spalte.",
    ],
    differences: [
      "Beim Import muss die Spalte `Rolle` genau einer vorhandenen Rolle entsprechen.",
      "Zur Wahl stehen `Administrator`, `Betreuer`, `Gast`, `Erziehungsberechtigter` und `Lehrkraft`.",
      "Fehlt in einer Zeile die E-Mail-Adresse, gibt es keinen Zugang. Die Person steht dann nur in der Liste.",
    ],
    troubleshootingDetails: [
      "Die Einladung kam nicht an? Prüfen Sie die E-Mail-Adresse und den Spam-Ordner. Danach laden Sie erneut ein.",
      "Eine Zeile wird als Fehler gemeldet? Dann gibt es die Person schon.",
      "moto erkennt eine bekannte Zeile an der Personalnummer, sonst an der E-Mail, sonst am Namen.",
      "Fehlt `Personal` in der Seitenleiste? Dafür braucht Ihre Rolle ein eigenes Recht.",
    ],
    related: [
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadStaffPermissions,
      HELP_TOPICS.leadTeacherAccess,
    ],
  };
}

/**
 * `/staff/[id]`, die sieben Reiter der Personalakte. Live geprueft an einer
 * Person mit Daten. Schwerpunkt ist das `Arbeitszeitmodell`: daran rechnet
 * jedes Stundenkonto, und es war in der Hilfe bisher nur eine Fehlerzeile.
 */
function staffRecordTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStaffRecord,
    title: "Die Personalakte einer Person führen",
    question: "Wo pflege ich die Angaben zu einer Person?",
    summary:
      "Die Personalakte bündelt Arbeitszeit, Abwesenheiten und Unterlagen.",
    group: "personal",
    audience: "lead",
    icon: "FolderOpen",
    steps: [],
    instructionGroups: [
      {
        title: "Die Akte öffnen",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf.",
          "Öffnen Sie `Mitarbeiter`.",
          "Geben Sie den Namen in das Feld `Name suchen` ein.",
          "Wählen Sie die Karte der Person.",
        ],
      },
      {
        title: "Das Arbeitszeitmodell eintragen",
        description:
          "Ohne Modell hat die Person kein Wochensoll. Stundenkonto und Saldo haben dann nichts, womit sie vergleichen könnten.",
        steps: [
          "Wählen Sie oben `Arbeitszeitmodell`.",
          "Wählen Sie rechts `Bearbeiten`.",
          "Wählen Sie `Vorlage zuweisen` für ein Modell, das mehrere Personen teilen.",
          "Oder wählen Sie `Eigenes Modell` nur für diese Person.",
          "Wählen Sie bei `Rotation` zum Beispiel `Standard (1 Woche)` oder `A/B-Wochen (2 Wochen)`.",
          "Tragen Sie je Wochentag die Stunden ein, zum Beispiel `4,45`.",
          "Wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Die weiteren Bereiche der Akte",
        ordered: false,
        steps: [
          "`Übersicht`: Stundenkonto, Urlaubstage und Krankheitstage des Jahres.",
          "`Zeiterfassung`: die einzelnen Buchungen dieser Person.",
          "`Abwesenheiten`: Resturlaub, `Krank melden` und `Freizeitausgleich eintragen`.",
          "`Stammdaten`: Person, Kontakt, Arbeitsvertrag, Qualifikationen und Personalnummer.",
          "`Dokumente`: Dateien zur Akte, je mit Kategorie und Frist.",
          "`Klassen`: nur bei Lehrkräften.",
        ],
      },
    ],
    result:
      "Unter `Arbeitszeitmodell` zeigt die `Vorschau` das Wochensoll der nächsten vier Wochen.",
    notes: [
      "Qualifikationen sind Nachweise wie Erste-Hilfe-Kurs oder Schwimmschein, jeweils mit Ablaufdatum.",
      "Die Personalnummer braucht die spätere Abrechnung. Ohne sie fehlt die Person dort.",
      "In der `Übersicht` buchen Sie unter `Stundenkonto-Verwaltung` Auszahlung, Freizeitausgleich oder einen Eröffnungssaldo.",
    ],
    differences: [
      "Ändern Sie eine Vorlage, wirkt das künftig auf alle Personen mit dieser Zuordnung.",
      "Den Bereich `Klassen` gibt es nur bei Lehrkräften.",
      "Bankverbindung und Steuernummer sind verborgen. Jeder Abruf wird festgehalten.",
    ],
    troubleshootingDetails: [
      "Ein Saldo stimmt nicht? Prüfen Sie zuerst das `Arbeitszeitmodell`.",
      "Ein Bereich fehlt Ihnen? Dann fehlt Ihrer Rolle das Recht dafür.",
    ],
    related: [
      HELP_TOPICS.leadTargetOverride,
      HELP_TOPICS.leadWorkTimeReview,
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadPayroll,
    ],
  };
}

/**
 * `/staff/[id]`, Reiter `Arbeitszeitmodell`, Abschnitt `Sonderarbeitszeiten`
 * (#3259). Der Fall aus der Praxis: die Herbstferien sind ein Schließtag,
 * einige arbeiten trotzdem in der Ferienbetreuung. Ohne Sonderarbeitszeit
 * bekämen sie Plusstunden.
 */
function targetOverrideTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadTargetOverride,
    title: "Eine Sonderarbeitszeit eintragen",
    question: "Wie trage ich andere Stunden für die Ferienbetreuung ein?",
    summary:
      "Für Personen, die in den Ferien andere Stunden arbeiten, zum Beispiel in den Herbstferien.",
    group: "personal",
    audience: "lead",
    icon: "CalendarPlus",
    requirements: [
      "Sie dürfen die Zeiterfassung aller Mitarbeitenden verwalten.",
    ],
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf.",
      "Öffnen Sie `Mitarbeiter`.",
      "Wählen Sie die Person.",
      "Wählen Sie oben `Arbeitszeitmodell`.",
      "Wählen Sie `Sonderarbeitszeit anlegen`.",
      "Tragen Sie `Erster Tag`, `Letzter Tag` und `Stunden pro Tag` ein, zum Beispiel `8,5`.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "In diesem Zeitraum gelten die neuen Stunden. Danach gilt wieder das Arbeitszeitmodell.",
    notes: [
      "Die Stunden gelten Montag bis Freitag, auch an Schließtagen. Gesetzliche Feiertage bleiben frei.",
      "Mit `0` Stunden muss die Person an diesen Tagen nicht arbeiten.",
      "Ändern geht nicht. Löschen Sie den Eintrag über die drei Punkte und legen Sie ihn neu an.",
    ],
    troubleshootingDetails: [
      "Es gibt in dem Zeitraum schon eine Sonderarbeitszeit? Löschen Sie diese zuerst.",
      "Der Monat ist abgeschlossen? Öffnen Sie ihn zuerst wieder im Reiter `Zeiterfassung`.",
    ],
    related: [
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadCalendarPeriods,
      HELP_TOPICS.leadWorkTimeReview,
    ],
  };
}

/**
 * `/database/personal`, Knopf `Löschen`. Der Knopf heisst `Endgültig
 * löschen`, der Vorgang ist aber eine Abschaltung: der Text des Dialogs sagt
 * ausdruecklich, dass Anwesenheiten und Zeiterfassung bleiben und die Person
 * erneut eingeladen werden kann. Genau das muss der Artikel sagen.
 */
function removeStaffTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadRemoveStaff,
    title: "Eine Person aus dem Team entfernen",
    question: "Wie entferne ich jemanden aus unserem Team?",
    summary: "Der Zugang geht aus. Die bisherigen Einträge bleiben erhalten.",
    group: "personal",
    audience: "lead",
    icon: "Trash2",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
      "Wählen Sie die Person aus der Liste.",
      "Wählen Sie oben rechts `Löschen`.",
      "Lesen Sie, was mit den Angaben der Person geschieht.",
      "Tippen Sie Vor- und Nachnamen der Person ein.",
      "Wählen Sie `Endgültig löschen`.",
    ],
    result:
      "Der Zugang ist aus. Die Person steht in keiner Liste der OGS mehr.",
    notes: [
      "Anwesenheiten und Zeiterfassung bleiben erhalten. Ihre Auswertungen bleiben vollständig.",
      "Sie können dieselbe Person später wieder einladen.",
    ],
    differences: [
      "Der Knopf heißt `Endgültig löschen`. Gelöscht werden die bisherigen Einträge trotzdem nicht.",
    ],
    troubleshootingDetails: [
      "`Endgültig löschen` lässt sich nicht wählen? Dann stimmt der eingetippte Name noch nicht.",
    ],
    related: [
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadStaffPermissions,
    ],
  };
}

/**
 * Rollen unter `/database/roles`, Rechteliste unter `/database/permissions`.
 * Systemrollen sind gesperrt: `role.isSystem` blendet die Kopfaktionen aus
 * (roles-master-detail.tsx:146).
 */
function staffPermissionsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStaffPermissions,
    title: "Mitarbeitende und Rechte verwalten",
    question: "Wie lege ich fest, was eine Person sehen darf?",
    summary: "Jede Person hat eine Rolle. Die Rolle trägt die Rechte.",
    group: "personal",
    audience: "lead",
    icon: "ShieldCheck",
    steps: [],
    instructionGroups: [
      {
        title: "Eine eigene Rolle anlegen",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Rollen`.",
          "Wählen Sie oben rechts `+ Rolle`.",
          "Tragen Sie `Name` und `Beschreibung` ein.",
          "Wählen Sie `Erstellen`.",
        ],
      },
      {
        title: "Die Rechte einer Rolle ändern",
        steps: [
          "Öffnen Sie `Rollen` und wählen Sie die Rolle aus der Liste.",
          "Wählen Sie oben rechts `Berechtigungen`.",
          "Haken Sie an, was die Rolle dürfen soll.",
          "Speichern Sie die Auswahl.",
        ],
      },
      {
        title: "Die Rolle einer Person ändern",
        steps: [
          "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
          "Wählen Sie die Person aus der Liste.",
          "Ändern Sie die Rolle und wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "Die neue Rolle gilt, sobald sich die Person das nächste Mal anmeldet.",
    notes: [
      "Unter `Berechtigungen` sehen Sie alle Rechte, die es in moto gibt. Ändern lässt sich dort nichts.",
      "Die Rolle entscheidet auch darüber, welche Menüpunkte jemand sieht.",
    ],
    differences: [
      "Frisch eingerichtet bringt moto alle Rollen selbst mit: `Administrator`, `Betreuer`, `Lehrkraft`, `Erziehungsberechtigter` und `Gast`. moto nennt sie Systemrollen.",
      "Bei einer Systemrolle fehlen `Berechtigungen`, `Bearbeiten` und `Löschen`. Dort steht: `System-Rollen können nicht bearbeitet oder gelöscht werden.`",
      "Zum Ändern von Rechten legen Sie deshalb zuerst eine eigene Rolle an.",
    ],
    troubleshootingDetails: [
      "Eine Person sieht einen Bereich nicht? Prüfen Sie die Rechte ihrer Rolle.",
    ],
    related: [
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadMissingMenu,
      HELP_TOPICS.leadTeacherAccess,
    ],
  };
}

/**
 * Lehrkraft-Zugang: Systemrolle `Lehrkraft` plus Klassenzuweisung im Reiter
 * `Klassen` der Personalakte (#1772, staff/[id]/page.tsx:216). Lehrkraefte
 * arbeiten in moto schule, einem eigenen Portal mit eigener Adresse.
 */
function leadTeacherAccessTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadTeacherAccess,
    title: "Einen Lehrkraft-Zugang vorbereiten",
    question: "Wie bekommt eine Lehrkraft Zugang zu moto schule?",
    summary: "Lehrkräfte sehen ihre Klasse in einem eigenen Bereich.",
    group: "personal",
    audience: "lead",
    icon: "GraduationCap",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
      "Wählen Sie oben rechts `+ Personal`.",
      "Tragen Sie die `E-Mail-Adresse` ein.",
      "Wählen Sie bei `Rolle` die Rolle `Lehrkraft`.",
      "Wählen Sie `Einladung senden`.",
      "Öffnen Sie danach `Mitarbeiter` und die Person.",
      "Wählen Sie den Reiter `Klassen`.",
      "Tragen Sie die Klassen ein, zum Beispiel `1a`.",
    ],
    result:
      "Die Lehrkraft sieht nach der Anmeldung ihre Klassen und den heutigen Tag.",
    notes: [
      "Schreiben Sie die Klasse genau wie bei den Kindern. Groß- und Kleinschreibung spielt keine Rolle.",
      "Beim Jahrgangswechsel wandern die Klassen automatisch mit. Prüfen Sie danach den Reiter `Klassen`.",
      "Eine Lehrkraft sieht nur ihre Klassen, keine Stammdaten und keine Kontaktdaten.",
    ],
    differences: [
      "moto schule hat eine eigene Adresse. Der Link in der Einladung führt dorthin.",
      "Über die Adresse Ihrer OGS kommt eine Lehrkraft nicht hinein.",
    ],
    troubleshootingDetails: [
      "Die Lehrkraft sieht keine Kinder? Dann fehlt die Klasse im Reiter `Klassen` oder sie ist anders geschrieben.",
      "Soll die Lehrkraft die Ankunftszeit selbst ändern dürfen? Geben Sie das in den `Einstellungen` frei.",
    ],
    related: [
      HELP_TOPICS.leadInviteStaff,
      HELP_TOPICS.leadStaffPermissions,
      HELP_TOPICS.settings,
    ],
  };
}

/**
 * Tagesinformationen unter `/tagesinformationen`: lesen alle, anlegen nur
 * Admins (page.tsx:183). Die Zielgruppen stehen in AUDIENCE_LABELS
 * (lib/staff-notices-api.ts:239).
 */
function staffNoticesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStaffNotices,
    title: "Tagesinformationen an das Team schreiben",
    question: "Wie erreiche ich das ganze Team mit einem Hinweis?",
    summary: "Eine Tagesinformation sehen alle, die heute arbeiten.",
    group: "planung",
    audience: "lead",
    icon: "Megaphone",
    steps: [
      "Klappen Sie in der Seitenleiste `Team` auf.",
      "Öffnen Sie `Tagesinformationen`.",
      "Wählen Sie `Neue Tagesinformation`.",
      "Tragen Sie `Titel` und `Hinweis` ein.",
      "Wählen Sie bei `Für wen` `Alle`, `Nur Betreuung` oder `Nur Lehrkräfte`.",
      "Wählen Sie bei `Wichtigkeit`, wie dringend der Hinweis ist.",
      "Wählen Sie bei `Woche`, ob der Hinweis sich wiederholt.",
      "Tragen Sie `Gilt ab` und bei Bedarf `Gilt bis (optional)` ein.",
      "Haken Sie unter `Wochentage` die Tage an, an denen der Hinweis erscheint.",
      "Wählen Sie `Speichern`.",
    ],
    result: "Der Hinweis steht sofort unter `Heute` für die gewählte Gruppe.",
    notes: [
      "Mit `Kenntnisnahme verlangen` sehen Sie später, wer den Hinweis bestätigt hat.",
      "`Nur Lehrkräfte` erreicht die Lehrkräfte in moto schule. Unter `Für wen` steht das auch im Fenster.",
    ],
    differences: [
      "Ein wichtiger Hinweis trägt `Wichtig`, ein abgeschalteter `Abgeschaltet`.",
      "Lesen können alle im Team. Anlegen und ändern darf nur die Leitung.",
    ],
    troubleshootingDetails: [
      "Beim Löschen gehen die bereits erfassten Kenntnisnahmen mit verloren.",
      "Soll die Nachricht an Eltern gehen? Nutzen Sie stattdessen eine Elternmitteilung.",
    ],
    related: [
      HELP_TOPICS.leadTeacherAccess,
      HELP_TOPICS.leadParentAnnouncement,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}

/**
 * `/calendar-periods` heisst in der Seitenleiste `Schuljahr und Ferien`, auf
 * der Seite selbst aber `Zeiträume` (planning-navigation.ts:70 gegen
 * calendar-periods/page.tsx:46). Der Artikel nennt beides, sonst sucht man.
 */
function calendarPeriodsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCalendarPeriods,
    title: "Schuljahr und Ferien eintragen",
    question: "Wo trage ich Schuljahr, Ferien und Schließtage ein?",
    summary: "moto plant nur an Tagen, an denen Ihre OGS geöffnet hat.",
    group: "planung",
    audience: "lead",
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Zeitraum oder Schließtag?",
        steps: [
          "Ein Zeitraum der Art `Ferien`, etwa die Herbstferien, markiert nur die Ferienzeit. Termine und Sollstunden bleiben.",
          "Hat die OGS zu, legen Sie einen Schließtag an. Dann fallen Termine aus, und niemand hat Sollstunden.",
          "Arbeitet jemand trotzdem, etwa in der Ferienbetreuung? Tragen Sie für die Person eine Sonderarbeitszeit ein.",
        ],
        ordered: false,
      },
      {
        title: "Einen Zeitraum anlegen",
        steps: [
          "Klappen Sie in der Seitenleiste `Planung` auf.",
          "Öffnen Sie `Schuljahr und Ferien`.",
          "Wählen Sie `Halbjahr anlegen` oder `Zeitraum anlegen`.",
          "Tragen Sie `Bezeichnung`, `Art`, `Startdatum` und `Enddatum` ein. Für Ferien wählen Sie die Art `Ferien`.",
          "Wählen Sie `Anlegen`.",
        ],
      },
      {
        title: "Einen Schließtag eintragen",
        steps: [
          "Öffnen Sie `Schuljahr und Ferien`.",
          "Gehen Sie zum Abschnitt `Schließtage`.",
          "Wählen Sie `Schließtag anlegen`.",
          "Tragen Sie `Grund`, `Von` und `Bis` ein, zum Beispiel `Herbstferien`.",
          "Wählen Sie `Speichern`.",
          "Stehen an diesen Tagen schon Termine, fragt moto nach. Sagen Sie sie ab wie im nächsten Abschnitt.",
        ],
      },
      {
        title: "Termine an Schließtagen absagen",
        description: "Für Termine, die schon vor dem Schließtag geplant waren.",
        steps: [
          "Wählen Sie beim Schließtag die drei Punkte.",
          "Wählen Sie `Termine absagen`.",
          "Prüfen Sie den `Zeitraum`. Sie können ihn kürzen, etwa auf die erste Ferienwoche.",
          "Serien für die Ferienbetreuung bleiben im Plan. Sollen sie auch ausfallen, setzen Sie das Häkchen bei `Auch diese Serien absagen`.",
          "Wählen Sie `Termine absagen` und dann `Endgültig absagen`.",
        ],
      },
    ],
    result:
      "An Schließtagen und gesetzlichen Feiertagen plant moto keine Termine. Abgesagte Termine verschwinden aus dem Plan. Eltern bekommen keine Nachricht.",
    notes: [
      "Die Seite heißt oben `Zeiträume`. In der Seitenleiste steht `Schuljahr und Ferien`.",
      "Ein Zeitraum, den noch nichts benutzt, trägt `Nicht verwendet`.",
      "Ferienbetreuung an Schließtagen: Wählen Sie beim Speichern der Serie `Auch an Schließtagen planen`. Tragen Sie bei `Letzter Tag` den letzten Ferientag ein. Dann endet die Serie mit den Ferien.",
      "Termine ohne Schließtag absagen: Wählen Sie im Betreuungsplan im Menü `Termine im Zeitraum absagen`.",
    ],
    differences: [
      "Legen Sie eine Schicht auf einen Schließtag, fragt moto vorher nach.",
      "Sie sehen `Termine absagen` nicht? Dann fehlt Ihnen das Recht, den Betreuungsplan zu bearbeiten.",
    ],
    troubleshootingDetails: [
      "Der Betreuungsplan sagt `Noch kein Planungszeitraum`? Dann fehlt für diese Woche ein Zeitraum.",
    ],
    related: [
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadDutyRoster,
      HELP_TOPICS.leadEnrollmentSetup,
    ],
  };
}

/**
 * `/betreuungsplan`. Vier Ansichten (`Tag`, `Woche`, `Monat`, `Serien`), das
 * Aendern bleibt Admins vorbehalten -- ohne das Recht traegt die Kopfzeile
 * `Nur ansehen` (betreuungsplan-view.tsx:1630).
 */
function leadCarePlanTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadCarePlan,
    title: "Einen Betreuungsplan erstellen",
    question: "Wie plane ich die Woche der Betreuung?",
    summary: "Der Betreuungsplan legt fest, wer wann welche Kinder betreut.",
    group: "planung",
    audience: "lead",
    icon: "Table",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Betreuungsplan`.",
      "Wählen Sie oben die Ansicht `Tag`, `Woche`, `Monat` oder `Serien`.",
      "Blättern Sie mit den Pfeilen zur gewünschten Woche.",
      "Legen Sie die Termine der Woche an.",
      "Prüfen Sie zum Schluss die gemeldeten Konflikte.",
    ],
    result: "Ihr Team sieht den Plan im `Tagesplan` und unter `Mein Kalender`.",
    notes: [
      "Ein Regeltermin wiederholt sich. Die Ansicht `Serien` zeigt alle Regeltermine.",
      "Über das Menü mit den drei Punkten geht `Drucken oder exportieren`.",
    ],
    differences: [
      "Steht oben `Nur ansehen`? Dann dürfen Sie den Plan lesen, aber nicht ändern.",
      "`Monat` und `Serien` stehen nicht in jeder Ansicht zur Wahl.",
    ],
    troubleshootingDetails: [
      "Die Woche sagt `Noch kein Planungszeitraum`? Wählen Sie `Planungszeitraum anlegen` oder tragen Sie zuerst `Schuljahr und Ferien` ein.",
      "Ein Regeltermin lässt sich nur ab heute beenden. Vergangene Termine löschen Sie einzeln.",
    ],
    related: [
      HELP_TOPICS.leadCalendarPeriods,
      HELP_TOPICS.leadSubstitutionPlan,
      HELP_TOPICS.leadDayLists,
    ],
  };
}

/** `/dienstplan`. Ansichten `Woche` und `Halbjahr`, Schichten je Person. */
function dutyRosterTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDutyRoster,
    title: "Einen Dienstplan erstellen",
    question: "Wie plane ich die Schichten des Teams?",
    summary: "Der Dienstplan zeigt, wer wann arbeitet.",
    group: "planung",
    audience: "lead",
    icon: "Clock3",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Dienstplan`.",
      "Wählen Sie oben `Woche` oder `Halbjahr`.",
      "Blättern Sie zur gewünschten Woche.",
      "Wählen Sie in der Zeile einer Person den passenden Tag.",
      "Tragen Sie die Schicht ein und speichern Sie sie.",
    ],
    result: "Die Person sieht ihre Schichten unter `Mein Kalender`.",
    notes: [
      "Jede Schicht kann eine Schichtart tragen. Die Farbe kommt von der Schichtart.",
      "Über `Schichtarten verwalten` pflegen Sie die Arten.",
      "Über das Menü mit den drei Punkten geht `Drucken oder exportieren`.",
    ],
    differences: [
      "Eine neue Schicht auf einem Schließtag löst eine Rückfrage aus.",
      "Ist ein Tag nicht vollständig durch Schichten abgedeckt, weist moto darauf hin.",
    ],
    troubleshootingDetails: [
      "Fehlt eine Person im Plan? Dann ist sie noch nicht als Personal angelegt.",
    ],
    related: [
      HELP_TOPICS.leadWorkTimeReview,
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}

/** `/vertretung`. Heisst in der Seitenleiste `Vertretungsplan`. */
function substitutionPlanTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadSubstitutionPlan,
    title: "Eine Vertretung planen",
    question: "Wie plane ich eine Vertretung?",
    summary: "Sie tragen ein, wer einen Einsatz übernimmt.",
    group: "planung",
    audience: "lead",
    icon: "Repeat",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Vertretungsplan`.",
      "Wählen Sie oben `Tag` oder `Woche`.",
      "Wählen Sie den Einsatz, der vertreten werden soll.",
      "Wählen Sie die Person, die übernimmt.",
      "Speichern Sie die Vertretung.",
    ],
    result: "Die vertretende Person sieht den Einsatz unter `Mein Kalender`.",
    notes: [
      "Mit `Sammel-Vertretung` tragen Sie mehrere Einsätze auf einmal ein.",
    ],
    differences: [
      "Die Seite heißt oben `Vertretung`. In der Seitenleiste steht `Vertretungsplan`.",
      "`Vertretungen` im `Tagesbetrieb` ist etwas anderes: dort steht, was heute vertreten wird.",
    ],
    troubleshootingDetails: [
      "Fehlt der Einsatz? Dann ist er noch nicht im Betreuungsplan angelegt.",
    ],
    related: [
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadDutyRoster,
      HELP_TOPICS.leadWorkTimeReview,
    ],
  };
}

/** `/lists`: druckbare Tageslisten aus den Betreuungsplan-Slots (#1565). */
function dayListsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDayLists,
    title: "Tageslisten erstellen",
    question: "Wie drucke ich eine Liste für einen Tag?",
    summary: "Tageslisten zeigen für einen Tag, wer wo sein soll.",
    group: "planung",
    audience: "lead",
    icon: "ClipboardList",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Tageslisten`.",
      "Wählen Sie oben das Datum.",
      "Wählen Sie unter `Quelle` die Art der Liste.",
      "Prüfen Sie die `Vorschau`.",
      "Öffnen Sie das Menü mit den drei Punkten.",
      "Wählen Sie `Drucken`, `PDF herunterladen` oder `Excel herunterladen`.",
    ],
    result: "Die Liste ist gedruckt oder als Datei gespeichert.",
    notes: [
      "Unter `Quelle` stehen `Randstunden`, `Lernzeit`, `AG-Angebote`, `Mensa`, `Ganztag (frühe Abholung)`, `Ganztag (späte Abholung)` und `Freie Angebotsauswahl`.",
      "Als Datenbasis wählen Sie `Plan`, `Ist` oder `Abgleich`.",
      "Unter der Datumszeile steht, woher die Daten kommen.",
    ],
    differences: [
      "Eine Art steht nicht zur Wahl? Dann gibt es an diesem Tag nichts dafür.",
    ],
    troubleshootingDetails: [
      "Die Vorschau bleibt leer? Prüfen Sie Datum und Art. An einem Schließtag gibt es keine Liste.",
    ],
    related: [
      HELP_TOPICS.leadCarePlan,
      HELP_TOPICS.leadClassListEntries,
      HELP_TOPICS.leadExports,
    ],
  };
}

/**
 * Arbeitszeiten des Teams: Ansehen und Korrigieren unter `/time-tracking`
 * (mit `time_tracking:manage`), Antraege im Reiter `Mitarbeitende` des
 * Anfragen-Moduls (#2433).
 */
function workTimeReviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadWorkTimeReview,
    title: "Arbeitszeiten des Teams prüfen",
    question: "Wie prüfe ich Arbeitszeiten, Urlaub und Korrekturen?",
    summary: "Zeiten stehen bei `Mitarbeiter`, Anträge unter `Anfragen`.",
    group: "personal",
    audience: "lead",
    icon: "Clock3",
    steps: [],
    instructionGroups: [
      {
        title: "Zeiten des Teams ansehen",
        steps: [
          "Klappen Sie in der Seitenleiste `Team` auf.",
          "Öffnen Sie `Mitarbeiter`.",
          "Wählen Sie oben den Reiter `Zeitkonten`.",
          "Lesen Sie in der `Einrichtungs-Übersicht` die Zahlen für `Woche` oder `Monat`.",
          "Filtern Sie die Liste bei Bedarf nach `Minusstunden`, `Plusstunden` oder `über +20 Std.`.",
        ],
      },
      {
        title: "Eine einzelne Zeit korrigieren",
        steps: [
          "Öffnen Sie `Mitarbeiter` und wählen Sie die Person.",
          "Wählen Sie den Reiter `Zeiterfassung`.",
          "Öffnen Sie den Eintrag, den Sie ändern möchten.",
          "Ändern Sie die Zeiten und wählen Sie `Speichern`.",
        ],
      },
      {
        title: "Einen Antrag entscheiden",
        steps: [
          "Öffnen Sie im `Tagesbetrieb` den Bereich `Anfragen`.",
          "Wählen Sie den Reiter `Mitarbeitende`.",
          "Öffnen Sie den Antrag.",
          "Wählen Sie `Genehmigen`, `Ablehnen` oder stellen Sie eine Rückfrage.",
        ],
      },
    ],
    result:
      "Die Person sieht die Entscheidung sofort in ihrer eigenen Zeiterfassung.",
    notes: [
      "Nachträge sind der Leitung vorbehalten. Ihr Team kann Zeiten nicht selbst nachtragen.",
      "Wochensaldo und Stundenkonto rechnen gegen das hinterlegte Arbeitszeitmodell.",
      "`Team` → `Zeiterfassung` zeigt nur Ihre eigene Stempeluhr, nicht die des Teams.",
      "Mit `Monat abschließen` frieren Sie den Stand ein. Das geht ab dem 1. des Folgemonats.",
    ],
    differences: [
      "Fehlt der Reiter `Mitarbeitende`? Dann dürfen Sie keine Anträge entscheiden.",
    ],
    troubleshootingDetails: [
      "Der Saldo einer Person stimmt nicht? Prüfen Sie ihr `Arbeitszeitmodell` in der Personalakte.",
    ],
    related: [
      HELP_TOPICS.leadDutyRoster,
      HELP_TOPICS.leadPayroll,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}

/** `/absences`: Auswertung ueber einen frei gewaehlten Zeitraum. */
function absenceReportTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadAbsenceReport,
    title: "Abwesenheiten auswerten",
    question: "Wie sehe ich, wer wann gefehlt hat?",
    summary: "Die Auswertung zeigt Krankmeldungen und Entschuldigungen.",
    group: "auswertung",
    audience: "lead",
    icon: "BellRing",
    steps: [
      "Öffnen Sie im `Tagesbetrieb` den Bereich `Alle Kinder`.",
      "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
      "Wählen Sie `Abwesenheiten`.",
      "Wählen Sie oben den Zeitraum.",
      "Filtern Sie bei Bedarf nach `Status` und `Gruppe`.",
    ],
    result:
      "Unter `Eingetragene Abwesenheitstage` steht jeder Tag mit Kind und Grund.",
    notes: [
      "Als `Status` stehen `Krank`, `Entschuldigt` und `Klassenfahrt` zur Wahl.",
      "Der Zeitraum lässt sich frei wählen. Es gibt auch fertige Zeiträume.",
      "Über die Suche finden Sie ein einzelnes Kind.",
    ],
    differences: ["Die Auswertung zeigt auch geplante Tage in der Zukunft."],
    troubleshootingDetails: [
      "Ein Tag fehlt? Prüfen Sie den gewählten Zeitraum und die Filter.",
    ],
    related: [
      HELP_TOPICS.leadStatistics,
      HELP_TOPICS.dayLog,
      HELP_TOPICS.leadManageStudent,
    ],
  };
}

/** `/statistics`: Quoten je Bereich ueber einen frei gewaehlten Zeitraum. */
function statisticsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadStatistics,
    title: "Eine Statistik öffnen",
    question: "Wo sehe ich Quoten und Auslastung?",
    summary: "Die Statistik zeigt die Anwesenheit über einen Zeitraum.",
    group: "auswertung",
    audience: "lead",
    icon: "BarChart3",
    steps: [
      "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
      "Öffnen Sie `Statistik`.",
      "Wählen Sie oben den Bereich: `Gruppen`, `Kinder`, `Räume` oder `Kurse`.",
      "Wählen Sie den Zeitraum.",
      "Lesen Sie die Zahlen der Tabelle.",
    ],
    result: "Die Zahlen gelten für den gewählten Bereich und Zeitraum.",
    notes: [
      "Bei `Kurse` wählen Sie zusätzlich zwischen `Je Kurs` und `Je Kind`.",
      "Über das Menü mit den drei Punkten geht `Exportieren` als `PDF`, `Excel` oder `Word`.",
      "Die Pfeile schieben den Zeitraum um seine eigene Länge weiter.",
    ],
    differences: [
      "`Räume` und `Kurse` zeigen nur dann Zahlen, wenn Ihre OGS damit arbeitet.",
    ],
    troubleshootingDetails: [
      "Fehlt `Statistik` in der Seitenleiste? Der Bereich ist der Leitung vorbehalten.",
    ],
    related: [
      HELP_TOPICS.leadAbsenceReport,
      HELP_TOPICS.leadExports,
      HELP_TOPICS.dayLog,
    ],
  };
}

/** `/database/exports`: fertige Listen als Datei (Kinder, Personal, Notfall). */
function exportsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadExports,
    title: "Eine Liste exportieren",
    question: "Wie bekomme ich Daten aus moto als Datei?",
    summary: "Fertige Listen lassen sich als Datei herunterladen.",
    group: "auswertung",
    audience: "lead",
    icon: "Download",
    steps: [
      "Öffnen Sie `Datenverwaltung` und danach `Exporte`.",
      "Suchen Sie die passende Liste und wählen Sie `Liste erstellen`.",
      "Wählen Sie im Fenster unter `Format` `PDF`, `DOCX` oder `XLSX`.",
      "Haken Sie unter `Spalten` an, was in der Liste stehen soll.",
      "Wählen Sie `Exportieren`.",
    ],
    result: "Die Datei liegt im Download-Ordner Ihres Geräts.",
    notes: [
      "Die Listen sind in `Kinderlisten`, `Personallisten` und `Momentaufnahmen` geordnet.",
      "Eine `Momentaufnahme` wie die `Notfallliste` zeigt den Stand von jetzt.",
      "Die `Gesundheitsliste` zeigt Allergien, Medikamente und andere Gesundheitsinformationen.",
      "Sie enthält zuerst nur Kinder mit einem Eintrag.",
      "`Auch Kinder ohne Eintrag` nimmt alle Kinder auf.",
      "Jeder Export der `Gesundheitsliste` wird protokolliert.",
      "Jede Datei enthält personenbezogene Daten. Behandeln Sie sie wie jede andere Unterlage dieser Art.",
    ],
    differences: [
      "Sie sehen weniger Listen als eine Kollegin? Jede Liste hängt an einem eigenen Recht.",
    ],
    troubleshootingDetails: [
      "Fehlt ein Kind auf der `Gesundheitsliste`? Dann ist bei ihm nichts eingetragen. Haken Sie `Auch Kinder ohne Eintrag` an.",
      "Brauchen Sie Zahlen statt Namen? Nutzen Sie die `Statistik`.",
      "Brauchen Sie eine Liste für einen bestimmten Tag? Nutzen Sie die `Tageslisten`.",
    ],
    related: [
      HELP_TOPICS.leadStatistics,
      HELP_TOPICS.leadDayLists,
      HELP_TOPICS.dataManagement,
    ],
  };
}

/**
 * `/payroll`: keine Abrechnung, sondern deren Konfiguration. Drei Karten:
 * `Vollständigkeit`, `Lohnarten`, `DATEV-Mandant` (payroll/page.tsx:109).
 */
function payrollTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadPayroll,
    title: "Eine Abrechnung für DATEV vorbereiten",
    question: "Wie bereite ich die Lohnabrechnung vor?",
    summary: "Sie hinterlegen einmal die Lohnarten Ihres Trägers.",
    group: "personal",
    audience: "lead",
    icon: "Landmark",
    steps: [
      "Klappen Sie in der Seitenleiste `Planung` auf.",
      "Öffnen Sie `Abrechnung`.",
      "Prüfen Sie oben unter `Vollständigkeit`, was noch fehlt.",
      "Tragen Sie unter `Lohnarten` die Nummern Ihres Lohnsystems ein.",
      "Tragen Sie unter `DATEV-Mandant` `Beraternummer` und `Mandantennummer` ein.",
      "Tragen Sie die `Personalnummern` bei den Personen nach. Das Feld steht beim Personal im Reiter `Stammdaten`.",
    ],
    result: "Ist alles vollständig, kann die Datei für DATEV erzeugt werden.",
    notes: [
      "Änderungen werden sofort gespeichert. Es gibt keinen Speichern-Knopf.",
      "Die Lohnartnummern kommen vom Träger, nicht von moto.",
      "`Lohn und Gehalt` braucht die Mandanten-Kennzahlen nicht.",
    ],
    differences: [
      "Eine Kategorie ohne Lohnartnummer wird nicht exportiert.",
      "Dieselbe Nummer bei mehreren Kategorien ist erlaubt, führt aber meist zu doppelten Stunden. moto warnt davor.",
    ],
    troubleshootingDetails: [
      "Der Export erzeugt keine Datei? Dann fehlt oben unter `Vollständigkeit` noch etwas.",
    ],
    related: [
      HELP_TOPICS.leadWorkTimeReview,
      HELP_TOPICS.leadStaffRecord,
      HELP_TOPICS.leadExports,
    ],
  };
}

/**
 * `/settings`. Die Reiter kommen aus dem Schema des Servers und sind je Schule
 * verschieden (settings/page.tsx:32) -- deshalb nennt der Artikel keine feste
 * Liste, sondern den Weg und die Regeln.
 */
function settingsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.settings,
    title: "Den Betrieb der OGS einstellen",
    question: "Wo stelle ich ein, wie unsere OGS arbeitet?",
    summary: "Die Einstellungen steuern Anwesenheit, Funktionen und Abläufe.",
    group: "konfiguration",
    audience: "lead",
    icon: "Settings",
    steps: [
      "Wählen Sie unten in der Seitenleiste `Einstellungen`.",
      "Wählen Sie oben den Bereich.",
      "Ändern Sie die gewünschte Einstellung.",
    ],
    result: "Die Änderung gilt sofort für alle in Ihrer OGS.",
    notes: [
      "Oben steht, wie viele Einstellungen von der Vorgabe abweichen.",
      "Jede Einstellung hat einen Satz darunter, der sagt, was sie bewirkt.",
      "Wer Blöcke starten oder beenden darf, legen Sie unter `Betrieb` fest: bei `Wer darf Blöcke starten?` und `Wer darf Blöcke beenden?`.",
      "`Das ganze Team` geht dort nur, wenn das Team alle Gruppen und Blöcke sieht.",
    ],
    differences: [
      "Welche Bereiche Sie sehen, hängt von Ihren Rechten und den Funktionen Ihrer OGS ab.",
    ],
    troubleshootingDetails: [
      "Eine Einstellung fehlt? Manche darf nur das moto-Team ändern. Sie erscheinen gar nicht erst.",
      "Der Wechsel des Anwesenheitsmodus wird abgelehnt? Das geht nicht, solange Kinder eingecheckt sind.",
    ],
    related: [
      HELP_TOPICS.leadParentVisibility,
      HELP_TOPICS.leadDevices,
      HELP_TOPICS.leadMissingMenu,
    ],
  };
}

/** Elternportal-Schalter, alle im Einstellungsbereich `Betrieb`. */
function parentVisibilityTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadParentVisibility,
    title: "Festlegen, was Eltern in moto sehen",
    question: "Was sehen Eltern von unserer OGS?",
    summary: "Sie schalten einzeln ein, was Eltern sehen und tun dürfen.",
    group: "konfiguration",
    audience: "lead",
    icon: "Eye",
    steps: [
      "Wählen Sie unten in der Seitenleiste `Einstellungen`.",
      "Bleiben Sie oben im Reiter `Betrieb`.",
      "Tragen Sie in `Einstellung suchen` `Elternportal` ein.",
      "Gehen Sie zur Gruppe `Elternportal`.",
      "Schalten Sie ein oder aus, was Eltern dürfen sollen.",
    ],
    result: "Eltern sehen die Änderung, wenn sie moto das nächste Mal öffnen.",
    notes: [
      "Einzeln schaltbar sind zum Beispiel `Krankmeldung über Elternportal`, `Abholzeit über Elternportal ändern` und `Stammdaten über Elternportal bearbeiten`.",
      "`Krankmeldung muss bestätigt werden` legt fest, ob eine Meldung erst durch Ihr Team freigegeben wird.",
      "Der Essensplan ist ein eigener Schalter.",
    ],
    differences: [
      "Ist ein Schalter aus, sehen Eltern den Bereich gar nicht. Er wird nicht ausgegraut.",
      "`Weitere Bezugspersonen einladen (Eltern)` ist kein Schalter, sondern eine Auswahl: `Deaktiviert`, `Direkt` oder `Mit Freigabe durch das Team`.",
      "Bei `Mit Freigabe durch das Team` landen die Einladungen unter `Elternzugänge`.",
    ],
    troubleshootingDetails: [
      "Eltern kommen nicht hinein? Dann fehlt ihr Zugang. Das steht unter `Elternzugänge`.",
    ],
    related: [
      HELP_TOPICS.settings,
      HELP_TOPICS.leadInviteGuardians,
      HELP_TOPICS.leadMealPlan,
    ],
  };
}

/**
 * Das Aufstellen des Tablets, vom Karton bis zum Startbildschirm.
 *
 * Eigener Artikel, weil die NFC-Themen der Betreuung ein laufendes Geraet
 * voraussetzen: sie fangen beim Anmelden an. Das gedruckte Blatt im
 * Versandkarton (`/help/nfc/erste-schritte`) deckt dieselben drei Schritte
 * kuerzer ab.
 */
function tabletSetupTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadTabletSetup,
    title: "Ein neues NFC-Tablet aufstellen",
    question: "Wie stelle ich ein neues NFC-Tablet auf?",
    summary: "Standort wählen, Kabel einstecken, den Startbildschirm abwarten.",
    group: "einrichten",
    audience: "lead",
    icon: "PlugZap",
    requirements: ["Eine Steckdose in der Nähe des Standorts"],
    steps: [
      "Wählen Sie einen Ort, an dem die Kinder täglich vorbeikommen.",
      "Stellen Sie das Tablet auf einen Tisch oder hängen Sie es an die Wand.",
      "Achten Sie darauf, dass der NFC-Sensor am Fuß frei bleibt.",
      "Legen Sie das Kabel so, dass niemand darüber stolpert.",
      "Stecken Sie das Netzkabel in die Steckdose.",
      "Warten Sie ein bis zwei Minuten.",
      "Auf dem Bildschirm erscheint `Willkommen bei moto!`.",
    ],
    result:
      "Das Tablet ist bereit. Ihr Team kann sich mit der Geräte-PIN anmelden.",
    notes: [
      "Einen Einschaltknopf gibt es nicht. Das Tablet startet mit dem Strom.",
      "Auf der Rückseite sind Löcher für eine Wandhalterung.",
      "Das Tablet ist schon mit dem WLAN Ihrer Schule verbunden.",
      "Ein Netzwerkkabel können Sie hinten anschließen.",
    ],
    differences: [
      "Auf dem Tablet läuft nur moto. Andere Apps finden Sie dort nicht.",
      "Ein rotes Zeichen unten rechts heißt: die Internetverbindung fehlt.",
    ],
    troubleshootingDetails: [
      "Der Bildschirm bleibt dunkel? Prüfen Sie die Steckdose und warten Sie zwei Minuten.",
      "Das rote Zeichen bleibt? Prüfen Sie Ihr WLAN oder fragen Sie das moto-Team.",
    ],
    related: [
      HELP_TOPICS.tabletLogin,
      HELP_TOPICS.tagAssignment,
      HELP_TOPICS.leadNfcSettings,
      HELP_TOPICS.leadDevices,
    ],
  };
}

/**
 * Der Bereich `Geräte` der Einstellungen plus die `Tägliche Abmeldezeit`
 * unter `Betrieb`. Alle diese Schalter haengen an NFC (`DependsOn` in
 * backend/services/config/defaults/devices.go und operations.go), stehen bei
 * einfacher Anwesenheit aber trotzdem da: `scanBinary` schaltet nur die
 * Anwesenheit um und fragt kein Ziel ab. Deshalb sagt der Artikel das zuerst.
 */
function nfcSettingsTopic(presenceMode: HelpPresenceMode): HelpTopic {
  const tracksRooms = presenceMode !== "binary";
  return {
    id: HELP_TOPICS.leadNfcSettings,
    title: "Einstellen, was das Tablet anzeigt",
    question: "Wo stelle ich die NFC-Tablets ein?",
    summary: tracksRooms
      ? "Sie setzen die Geräte-PIN und legen fest, wohin Kinder auschecken dürfen."
      : "Sie setzen die PIN, mit der sich Ihr Team am Tablet anmeldet.",
    group: "nfc",
    audience: "lead",
    icon: "SlidersHorizontal",
    steps: [],
    instructionGroups: [
      {
        title: "Die Geräte-PIN setzen",
        description:
          "Bei der Auslieferung ist eine einfache PIN gesetzt. Ändern Sie sie vor dem ersten Betreuungstag.",
        steps: [
          "Wählen Sie unten in der Seitenleiste `Einstellungen`.",
          "Wählen Sie oben den Bereich `Geräte`.",
          "Tragen Sie unter `OGS Geräte-PIN` eine vierstellige Zahl ein.",
          "Geben Sie die neue PIN an Ihr Team weiter.",
        ],
      },
      ...(tracksRooms
        ? [
            {
              title: "Die Ziele beim Auschecken festlegen",
              description:
                "Diese Schalter bestimmen, welche Knöpfe ein Kind am Tablet sieht, wenn es sein Armband ein zweites Mal auflegt.",
              steps: [
                "Bleiben Sie im Bereich `Geräte`.",
                "Schalten Sie `Raumwechsel-Button anzeigen` ein, wenn Kinder den Raum wechseln dürfen.",
                "Schalten Sie `Schulhof-Button anzeigen` und `Toilette-Button anzeigen` nach Bedarf ein.",
                "`„Nach Hause“ in jedem Raum anzeigen` erlaubt den Heimweg aus jedem Raum.",
              ],
              ordered: false,
            },
            {
              title: "Festlegen, ab wann Kinder nach Hause dürfen",
              steps: [
                "Wählen Sie oben den Bereich `Betrieb`.",
                "Tragen Sie bei `Tägliche Abmeldezeit` eine Uhrzeit ein.",
                "Lassen Sie das Feld leer, wenn `nach Hause` immer gelten soll.",
              ],
            },
          ]
        : []),
    ],
    result:
      "moto speichert jede Änderung sofort. Das Tablet übernimmt sie beim nächsten Start.",
    notes: [
      "Finden Sie eine Einstellung nicht? Nutzen Sie oben `Einstellung suchen`.",
      ...(tracksRooms
        ? [
            "`Schulhof` und `Toilette` legen jeweils einen passenden Raum mit an.",
            "`Details bei vollem Raum anzeigen` nennt am Tablet Name und Belegung.",
          ]
        : []),
    ],
    differences: [
      // Der Grund, warum eine OGS mit einfacher Anwesenheit hier fast nichts
      // einstellen kann: das Tablet fragt kein Ziel ab.
      ...(presenceMode === "binary"
        ? [
            "Ihre OGS hält nur fest, ob ein Kind da ist. Dann wirkt hier nur die `OGS Geräte-PIN`.",
            "Die Knöpfe zum Auschecken und die `Tägliche Abmeldezeit` ändern bei Ihnen nichts.",
          ]
        : []),
      "Arbeitet Ihre OGS ohne NFC, bleibt `Geräte` in den `Einstellungen` leer.",
      "Die `OGS Geräte-PIN` darf nur ändern, wer die OGS verwaltet.",
    ],
    troubleshootingDetails: [
      "Die neue PIN klappt am Tablet nicht? Starten Sie das Tablet neu.",
      "Der Bereich bleibt leer? Dann arbeitet Ihre OGS ohne NFC.",
    ],
    related: [
      HELP_TOPICS.nfcCheckIn,
      HELP_TOPICS.leadDevices,
      HELP_TOPICS.settings,
      HELP_TOPICS.leadTabletSetup,
    ],
  };
}

/** `/database/devices`. Nur mit NFC (NFC_ONLY_HREFS, sidebar.tsx:369). */
function devicesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadDevices,
    title: "NFC-Geräte verwalten",
    question: "Wie verwalte ich unsere NFC-Tablets?",
    summary: "Sie sehen alle Geräte Ihrer OGS und ihren Zustand.",
    group: "nfc",
    audience: "lead",
    icon: "Devices",
    steps: [],
    instructionGroups: [
      {
        title: "Die Geräte nachsehen",
        description:
          "Ein geliefertes Tablet steht schon in der Liste. Sie müssen es nicht anlegen.",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Datenverwaltung` und danach `Geräte`.",
          "Wählen Sie ein Gerät aus der Liste.",
          "Lesen Sie bei `Verbindung`, ob das Gerät `Online` oder `Offline` ist.",
          "Lesen Sie bei `Letzter Standort`, wo es zuletzt benutzt wurde.",
          "Lesen Sie bei `Status`, ob es `Aktiv` ist.",
        ],
      },
      {
        title: "Ein Gerät von Hand anlegen",
        description:
          "Nur nötig, wenn ein Gerät nicht in der Liste steht. Fragen Sie vorher beim moto-Team nach.",
        steps: [
          "Wählen Sie oben rechts `+ Gerät`.",
          "Tragen Sie bei `Geräte-ID` die Kennung des Geräts ein.",
          "Wählen Sie bei `Gerätetyp` `Terminal` oder `Info-Point`.",
          "Tragen Sie bei `Gerätename` einen Namen ein, zum Beispiel `Eingang`.",
          "Wählen Sie `Erstellen`.",
        ],
      },
    ],
    result:
      "Sie sehen für jedes Gerät, ob es erreichbar ist und wo es zuletzt stand.",
    notes: [
      "Steht bei `Letzter Standort` `Noch nicht verwendet`, wurde am Gerät noch nichts gescannt.",
      "Nach dem Anlegen zeigt moto einmalig einen Zugangscode. Nur das Gerät braucht ihn.",
      "Geben Sie diesen Code nicht weiter. Er ist danach nicht mehr zu sehen.",
    ],
    differences: [
      "Arbeitet Ihre OGS ohne NFC, fehlt `Geräte` in der `Datenverwaltung`.",
    ],
    troubleshootingDetails: [
      "Ein Gerät meldet sich nicht? Prüfen Sie `Verbindung` und `Status` in der Liste.",
      "`Offline` und der Strom stimmt? Prüfen Sie das WLAN am Standort des Geräts.",
    ],
    related: [
      HELP_TOPICS.leadNfcSettings,
      HELP_TOPICS.leadTabletSetup,
      HELP_TOPICS.nfcProblem,
      HELP_TOPICS.leadInfoDisplays,
    ],
  };
}

/** `/info-displays`. Nur wenn die Schule die Funktion einschaltet. */
function infoDisplaysTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadInfoDisplays,
    title: "Ein Info-Display verwalten",
    question: "Wie richte ich einen Bildschirm im Flur ein?",
    summary:
      "Ein Bildschirm im Flur zeigt Raumbelegung, Angebote und Abholzeiten.",
    group: "konfiguration",
    audience: "lead",
    icon: "Monitor",
    steps: [],
    instructionGroups: [
      {
        title: "Ein Display anlegen",
        description:
          "Ein Display lohnt sich dort, wo viele vorbeikommen. Eltern sehen beim Abholen selbst, in welchem Raum die Gruppen gerade sind. Ihr Team muss dann nicht jedem einzeln Auskunft geben.",
        steps: [
          "Klappen Sie in der Seitenleiste `Verwaltung` auf.",
          "Öffnen Sie `Info-Displays`.",
          "Wählen Sie `Neues Display`.",
          "Tragen Sie einen Namen ein, zum Beispiel `Eingang`.",
          "Speichern Sie das Display.",
          "Öffnen Sie den Link des Displays auf dem Bildschirm im Flur.",
        ],
      },
    ],
    result: "Der Bildschirm zeigt die Angaben ohne Anmeldung an.",
    notes: [
      "Auf dem Bildschirm stehen nur Anzahlen, Raumnamen und Uhrzeiten. Namen von Kindern erscheinen nicht.",
      "Oben läuft eine Uhr. Bei fehlender Verbindung steht dort, wie alt der Stand ist.",
      "Legen Sie für jeden Bildschirm ein eigenes Display an.",
    ],
    differences: [
      "Fehlt `Info-Displays` in der Seitenleiste? Dann ist die Funktion ausgeschaltet. Schalten Sie sie in den `Einstellungen` unter `Betrieb` bei `Info-Displays aktivieren` ein.",
    ],
    troubleshootingDetails: [
      "Der Bildschirm zeigt nichts? Prüfen Sie, ob der Link vollständig geöffnet wurde.",
    ],
    related: [
      HELP_TOPICS.leadDevices,
      HELP_TOPICS.settings,
      HELP_TOPICS.leadParentVisibility,
    ],
  };
}

/**
 * Der Problemartikel der Leitung. Bewusst getrennt vom Betreuer-Thema
 * `missingMenu`: dort fehlt einem selbst etwas, hier einer anderen Person.
 */
function leadMissingMenuTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.leadMissingMenu,
    title: "Eine Person sieht einen Menüpunkt nicht",
    question: "Warum fehlt jemandem ein Bereich in der Seitenleiste?",
    summary: "Meist fehlt der Rolle das Recht für diesen Bereich.",
    group: "probleme",
    audience: "lead",
    icon: "EyeSlash",
    steps: [
      "Fragen Sie, welcher Bereich genau fehlt.",
      "Öffnen Sie `Datenverwaltung` und danach `Personal`.",
      "Prüfen Sie, welche Rolle die Person hat.",
      "Öffnen Sie `Rollen` und wählen Sie diese Rolle.",
      "Wählen Sie `Berechtigungen` und prüfen Sie die Haken.",
      "Ergänzen Sie das fehlende Recht und speichern Sie.",
    ],
    result:
      "Der Bereich erscheint, sobald sich die Person das nächste Mal anmeldet.",
    notes: [
      "moto blendet einen Bereich aus, statt ihn zu sperren. Ein fehlender Menüpunkt ist kein Fehler.",
    ],
    differences: [
      "Manche Bereiche hängen nicht an Rechten, sondern an Funktionen Ihrer OGS.",
      "Ohne NFC fehlen `Aktivitäten` und `Geräte`. Bei einfacher Anwesenheit fehlen `Räume` und `Tagesplan`.",
      "Ohne feste Gruppen fehlt `Meine Gruppen`.",
      "`Info-Displays` hängt an einem eigenen Schalter in den `Einstellungen`, nicht an einem Recht.",
    ],
    troubleshootingDetails: [
      "Die Rolle bringt moto selbst mit? Dann lassen sich ihre Rechte nicht ändern. Legen Sie eine eigene Rolle an.",
      "Fehlt Ihnen selbst ein Bereich? Dann fehlt Ihrem Konto das Recht. Fragen Sie das moto-Team.",
    ],
    related: [
      HELP_TOPICS.leadStaffPermissions,
      HELP_TOPICS.settings,
      HELP_TOPICS.leadInviteStaff,
    ],
  };
}

function leadTopics(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): readonly HelpTopic[] {
  return [
    // --- moto fuer die OGS einrichten ---
    goLiveTopic(presenceMode, groupMode, nfcEnabled),
    dataManagementTopic(),
    roomsCatalogTopic(presenceMode),
    inviteStaffTopic(),
    groupsCatalogTopic(groupMode),
    activitiesCatalogTopic(),
    createStudentTopic(),
    careTimesTopic(),
    tabletSetupTopic(),

    // --- Kinder verwalten ---
    manageStudentTopic(),
    endCareTopic(),
    deleteStudentTopic(),
    gradeTransitionTopic(),
    classListEntriesTopic(),

    // --- Eltern und Anfragen ---
    // Zuerst der Zugang: ohne Konto sieht eine Familie weder Mitteilung
    // noch Elternbrief noch Essensplan.
    inviteGuardiansTopic(),
    parentAnnouncementTopic(),
    parentLetterTopic(),
    parentSurveyTopic(),
    mealPlanTopic(),
    bankDetailsTopic(),

    // --- Anmeldungen ---
    enrollmentSetupTopic(),
    enrollmentFormTopic(),
    enrollmentReviewTopic(),
    enrollmentExportTopic(),
    enrollmentCleanupTopic(),

    // --- Personalverwaltung ---
    staffRecordTopic(),
    targetOverrideTopic(),
    staffPermissionsTopic(),
    workTimeReviewTopic(),
    payrollTopic(),
    removeStaffTopic(),
    leadTeacherAccessTopic(),

    // --- Betreuung und Team planen ---
    calendarPeriodsTopic(),
    leadCarePlanTopic(),
    dutyRosterTopic(),
    substitutionPlanTopic(),
    dayListsTopic(),
    staffNoticesTopic(),

    // --- Auswerten und exportieren ---
    absenceReportTopic(),
    statisticsTopic(),
    exportsTopic(),

    // --- Einstellungen und Geräte ---
    settingsTopic(),
    parentVisibilityTopic(),
    infoDisplaysTopic(),

    // --- Wenn etwas nicht klappt ---
    leadMissingMenuTopic(),
  ];
}

/**
 * Einladung annehmen unter `/parents/accept-guardian-invite/[token]`. Die
 * Passwortregeln stehen im Formular selbst (i18n `guardianInvite.passwordRules`).
 */
function parentAccountTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentAccount,
    title: "Mein Eltern-Konto einrichten",
    question: "Wie richte ich mein Eltern-Konto ein?",
    summary: "Sie öffnen den Link aus der E-Mail und legen Ihr Passwort fest.",
    group: "einstieg",
    audience: "parent",
    icon: "KeyRound",
    requirements: ["Sie haben eine Einladungs-E-Mail Ihrer OGS bekommen."],
    steps: [
      "Öffnen Sie den Link aus der E-Mail.",
      "Prüfen Sie oben, für welche Schule die Einladung gilt.",
      "Tragen Sie bei `Passwort` Ihr neues Passwort ein.",
      "Tragen Sie es bei `Passwort bestätigen` noch einmal ein.",
      "Wählen Sie `Einladung akzeptieren`.",
      "Melden Sie sich danach mit Ihrer E-Mail-Adresse und dem neuen Passwort an.",
    ],
    result:
      "Ihr Konto ist angelegt. Sie sehen danach Ihr Kind im Elternportal.",
    notes: [
      "Das Passwort braucht mindestens 8 Zeichen, einen Großbuchstaben, einen Kleinbuchstaben, eine Zahl und ein Sonderzeichen.",
      "Unter `Passwortanforderungen` sehen Sie, was noch fehlt.",
    ],
    differences: [
      "Sie sehen `Konto erstellt`? Dann hat es geklappt. Melden Sie sich jetzt an.",
    ],
    troubleshootingDetails: [
      "Die Seite sagt, die Einladung sei abgelaufen oder schon benutzt? Bitten Sie Ihre OGS um eine neue Einladung.",
      "Die Seite sagt, für diese E-Mail gebe es schon ein Konto? Dann melden Sie sich direkt an.",
    ],
    related: [
      HELP_TOPICS.parentLogin,
      HELP_TOPICS.parentAccountProblem,
      HELP_TOPICS.parentInstallApp,
    ],
  };
}

/**
 * `/parents/login`. Eigenes Portal mit eigener Adresse: ein Personalkonto
 * wird hier abgewiesen (i18n `parentLogin.errors.notAGuardian`).
 */
function parentLoginTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentLogin,
    title: "Bei moto anmelden",
    question: "Wie melde ich mich im Elternportal an?",
    summary:
      "Das Elternportal hat eine eigene Adresse und eigene Zugangsdaten.",
    group: "einstieg",
    audience: "parent",
    icon: "LogIn",
    steps: [
      "Öffnen Sie die Adresse des Elternportals.",
      "Tragen Sie Ihre `E-Mail-Adresse` ein.",
      "Tragen Sie Ihr `Passwort` ein.",
      "Wählen Sie `Anmelden`.",
    ],
    result: "moto öffnet die Startseite mit dem heutigen Tag Ihres Kindes.",
    notes: [
      "Mit dem Augensymbol sehen Sie Ihr Passwort im Klartext.",
      "Legen Sie sich die Adresse als Lesezeichen an. Oder fügen Sie moto zum Startbildschirm hinzu.",
    ],
    differences: [
      "Passwort vergessen? Wählen Sie `Passwort vergessen?`. Es öffnet sich das Fenster `Passwort zurücksetzen`. Tragen Sie Ihre E-Mail-Adresse ein und wählen Sie `Link senden`. Den Link bekommen Sie per E-Mail.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Konto gehöre zum Personal einer Schule? Dann melden Sie sich über die Schul-Anmeldung an.",
      "moto sagt, Ihr Konto sei deaktiviert? Wenden Sie sich an Ihre OGS.",
      "Nach vielen Versuchen sperrt moto die Anmeldung kurz. Warten Sie einen Moment.",
    ],
    related: [
      HELP_TOPICS.parentAccount,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentAccountProblem,
    ],
  };
}

/**
 * Eltern-Fassung des App-Themas. Der Samsung-Hinweis ist echt und steht in
 * den Einstellungen wie im Installationshinweis (i18n `pwaInstallHint`).
 */
function parentInstallAppTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentInstallApp,
    title: "moto als App hinzufügen",
    question: "Wie bekomme ich moto auf meinen Startbildschirm?",
    summary: "Sie fügen moto zum Startbildschirm hinzu. Ohne App Store.",
    group: "einstieg",
    audience: "parent",
    icon: "Smartphone",
    steps: [],
    instructionGroups: [
      {
        title: "Auf dem iPhone oder iPad",
        steps: [
          "Öffnen Sie moto in Safari.",
          "Tippen Sie unten auf `Teilen`.",
          "Tippen Sie auf `Zum Home-Bildschirm`.",
        ],
      },
      {
        title: "Auf einem Android-Gerät",
        steps: [
          "Öffnen Sie moto in Chrome.",
          "Öffnen Sie das Browser-Menü.",
          "Tippen Sie auf `App installieren` oder `Zum Startbildschirm hinzufügen`.",
        ],
      },
    ],
    result: "moto öffnet sich danach im Vollbild, ohne Browserleiste.",
    notes: [
      "Manchmal bietet moto die Installation direkt an. Dann genügt ein Tippen auf `App installieren`.",
    ],
    differences: [
      "Nutzen Sie Samsung Internet? Installieren Sie moto darüber nicht. Öffnen Sie moto in Chrome und melden Sie sich dort an.",
    ],
    troubleshootingDetails: [
      "Sie finden den Punkt im Menü nicht? Prüfen Sie, ob Sie wirklich in Safari oder Chrome sind.",
      "Benachrichtigungen kommen erst an, wenn moto auf dem Startbildschirm liegt.",
    ],
    related: [
      HELP_TOPICS.parentNotifications,
      HELP_TOPICS.parentLogin,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}

/**
 * `/parents/children`. Live geprueft: die Kindseite zeigt oben `Heute` mit
 * `Aktueller Status`, `Abholung` und drei Knoepfen, darunter die drei
 * Reiter `Betreuung`, `Angaben` und `Kontakte`. Die langen Bereichsnamen
 * aus `parentChild.areas.*.title` stehen nur im aria-label.
 */
function parentChildOverviewTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentChildOverview,
    title: "Mein Kind und den heutigen Tag ansehen",
    question: "Wo sehe ich, wie der Tag meines Kindes läuft?",
    summary: "Die Seite `Kinder` zeigt den heutigen Tag und drei Bereiche.",
    group: "mein-kind",
    audience: "parent",
    icon: "UserRoundSearch",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Haben Sie mehrere Kinder, wählen Sie oben das Kind.",
      "Lesen Sie unter `Heute` den `Aktueller Status` und die `Abholung`.",
      "Wählen Sie darunter einen Reiter: `Betreuung`, `Angaben` oder `Kontakte`.",
    ],
    result: "Sie sehen den gewählten Reiter mit allen Angaben dazu.",
    notes: [
      "`Betreuung`: Abwesenheiten, Wochenplan, gebuchte Betreuung und der Weg nach Hause.",
      "`Angaben`: persönliche Angaben, Klasse und Gesundheit.",
      "`Kontakte`: Abholberechtigte, Kontakte und verbundene Konten.",
      "Unter `Heute` stehen die Knöpfe zum Abmelden, zur Abholzeit und für eine Nachricht.",
    ],
    differences: [
      "Haben Sie mehrere Kinder, heißt der Punkt unten `Meine Kinder`.",
      "Ist Ihr Kind heute abgemeldet, steht dort `Heute abgemeldet, keine Abholung`.",
      "Wurde die Abholzeit für heute geändert, steht die Zeit mit dem Hinweis `geändert für heute`.",
      "Ist für heute nichts eingetragen, steht dort `Für heute ist keine Abholung eingetragen`.",
    ],
    troubleshootingDetails: [
      "Sie sehen kein Kind? Dann hat die OGS Ihren Zugang noch nicht freigegeben.",
    ],
    related: [
      HELP_TOPICS.parentChildData,
      HELP_TOPICS.parentReportAbsence,
      HELP_TOPICS.parentCalendar,
    ],
  };
}

/**
 * Bereich `Angaben zum Kind` (i18n `parentMasterData`). Drei Arten von
 * Feldern: direkt aenderbar, nur auf Anfrage, und Gesundheitshinweise, die
 * sofort gespeichert werden.
 */
function parentChildDataTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentChildData,
    title: "Angaben zu meinem Kind ansehen und ändern",
    question: "Wie ändere ich die Angaben zu meinem Kind?",
    summary: "Manches ändern Sie selbst. Anderes prüft die OGS zuerst.",
    group: "mein-kind",
    audience: "parent",
    icon: "FileText",
    steps: [],
    instructionGroups: [
      {
        title: "Die Angaben öffnen",
        steps: [
          "Tippen Sie unten auf `Mein Kind`.",
          "Wählen Sie den Reiter `Angaben`.",
        ],
      },
      {
        title: "Name, Geburtsdatum oder Klasse ändern",
        steps: [
          "Gehen Sie zum Abschnitt `Persönliche Angaben`.",
          "Tippen Sie auf `Änderungen anfragen`.",
          "Ändern Sie nur die Angaben, die nicht mehr stimmen.",
          "Tippen Sie auf `Anfrage an OGS senden`.",
        ],
      },
      {
        title: "Gesundheitshinweise eintragen",
        steps: [
          "Gehen Sie zum Abschnitt `Gesundheit`.",
          "Tragen Sie bei `Gesundheitshinweise und Allergien` ein, was wichtig ist.",
          "Die Eingabe wird automatisch gespeichert.",
        ],
      },
    ],
    result:
      "Angefragte Änderungen tragen `In Prüfung`. Bis zur Bestätigung gelten die bisherigen Angaben.",
    notes: [
      "Gesundheitshinweise gehen sofort an das Team, ohne Prüfung.",
      "Eine offene Anfrage lässt sich mit `Anfrage bearbeiten` noch ändern.",
    ],
    differences: [
      "Ein Feld trägt `In Prüfung`? Dann wartet dort schon eine Änderung auf die OGS.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Bearbeiten sei deaktiviert? Dann nimmt Ihre OGS keine Änderungen über die App an. Schreiben Sie ihr eine Nachricht.",
    ],
    related: [
      HELP_TOPICS.parentGuardians,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentFeatureMissing,
    ],
  };
}

/**
 * Reiter `Kontakte`. Live geprueft: eine Karte `Abholberechtigte und
 * Kontakte` mit dem Knopf `Kontakt oder Abholperson hinzufügen`, darunter
 * die Karte `Verbundene Konten` mit `Einladen`. Ein Zwischenschritt
 * `Person hinzufügen` erscheint nicht. Die Schalter `Darf abholen` und
 * `Notfallkontakt` stehen schon im Anlegen-Fenster; spaeter aendern darf sie
 * nur, wer `can_manage_pickup` hat -- sonst heisst das Fenster nur
 * `Hinweis zur Abholung` (guardians-panel.tsx).
 */
function parentGuardiansTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentGuardians,
    title: "Kontakte und Abholung verwalten",
    question: "Wer darf mein Kind abholen und wer wird erreicht?",
    summary: "Sie hinterlegen Kontakte und legen Abholrechte fest.",
    group: "mein-kind",
    audience: "parent",
    icon: "Users",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Kontakt hinzufügen",
        steps: [
          "Tippen Sie unten auf `Mein Kind`.",
          "Wählen Sie den Reiter `Kontakte`.",
          "Tippen Sie auf `Kontakt oder Abholperson hinzufügen`.",
          "Wählen Sie im Fenster `Kontakt hinzufügen` die `Beziehung zum Kind`.",
          "Schalten Sie bei Bedarf `Darf abholen` und `Notfallkontakt` ein.",
          "Tragen Sie `Vorname` und `Nachname` ein.",
          "Tippen Sie unten auf `Kontakt hinzufügen`.",
        ],
      },
      {
        title: "Abholrecht später ändern",
        description:
          "Bei jeder Person stehen rechts zwei Symbole: der Stift für die Kontaktdaten, das zweite für die Abholung.",
        steps: [
          "Tippen Sie bei der Person auf das zweite Symbol `Abholrecht verwalten`.",
          "Schalten Sie im Fenster `Abholung verwalten` `Darf abholen` ein oder aus.",
          "Schalten Sie bei Bedarf `Notfallkontakt` ein.",
          "Tragen Sie bei Bedarf einen `Hinweis zur Abholung` ein.",
          "Tippen Sie auf `Speichern`.",
        ],
      },
      {
        title: "Einer Person App-Zugang geben",
        steps: [
          "Gehen Sie zur Karte `Verbundene Konten`.",
          "Tippen Sie auf `Einladen`.",
          "Tragen Sie die E-Mail-Adresse der Person ein.",
          "Tippen Sie auf `Einladung senden`.",
        ],
      },
    ],
    result:
      "Die OGS sieht den Kontakt sofort. Die Kennzeichen stehen an der Person.",
    notes: [
      "An einer Person stehen `Primär`, `Darf abholen` oder `Notfallkontakt`.",
      "Ein Kontakt braucht kein eigenes Konto. Zum Beispiel Oma oder Opa.",
      "Eine E-Mail beim Kontakt lädt niemanden ein. Der App-Zugang ist ein eigener Schritt.",
    ],
    differences: [
      "Unter `Verbundene Konten` sehen Sie, wer die Eltern-App für Ihr Kind nutzt.",
    ],
    troubleshootingDetails: [
      "moto sagt, die Person verwalte ihre Daten über ein eigenes Konto? Dann ändert sie ihre Angaben selbst.",
      "moto sagt, die Kontaktdaten würden von der Schule verwaltet? Dann wenden Sie sich an die OGS.",
      "Die Verwaltung ist deaktiviert? Dann nimmt Ihre OGS das nicht über die App an.",
    ],
    related: [
      HELP_TOPICS.parentChildData,
      HELP_TOPICS.parentPickupChange,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}

/**
 * Abwesenheit melden (i18n `parentChildCare.sick`). Krank geht meist sofort
 * durch, andere Gruende sind eine Anfrage -- je nach Einstellung der OGS.
 */
function parentReportAbsenceTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentReportAbsence,
    title: "Eine Abwesenheit melden",
    question: "Wie melde ich mein Kind krank oder ab?",
    summary: "Sie melden der OGS, an welchen Tagen Ihr Kind nicht kommt.",
    group: "mein-kind",
    audience: "parent",
    icon: "BellRing",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Tippen Sie unter `Heute` auf den Knopf mit dem Namen Ihres Kindes und `abmelden`.",
      "Wählen Sie im Fenster `Abwesenheit melden` bei `Grund der Abwesenheit` `Krank melden` oder `Entschuldigen`.",
      "Wählen Sie bei `Tage der Abwesenheit` den `Erster Tag` und den `Letzter Tag`.",
      "Tragen Sie bei `Grund / Hinweis an die OGS` kurz ein, worum es geht. Das ist ein Pflichtfeld.",
      "Tippen Sie auf `Krankmeldung an die OGS senden` oder `Entschuldigung an die OGS senden`.",
    ],
    result:
      "Die Abmeldung steht am Kind. Die Anmeldung bei der OGS bleibt unverändert.",
    notes: [
      "`Krank melden` ist für Krankheitstage. Die OGS sieht die Meldung sofort.",
      "`Entschuldigen` ist für Termine und geplante Abwesenheiten.",
      "Für einen einzelnen Tag wählen Sie denselben Tag zweimal.",
    ],
    differences: [
      "Manche OGS bestätigen jede Meldung erst. Dann steht `Freigabe ausstehend`, und Ihr Kind gilt bis dahin als erwartet.",
      "Am Kind steht danach `Krank` oder `Entschuldigt` mit den Tagen.",
      "Wurde eine Anfrage abgelehnt, steht dort `Abgelehnt`.",
    ],
    troubleshootingDetails: [
      "moto meldet, für einen Tag liege schon eine Abmeldung vor? Prüfen Sie die bestehende Meldung mit `Anfrage bearbeiten`.",
      "Ihr Kind kommt nur später? Ändern Sie stattdessen die Abholzeit oder schreiben Sie der OGS.",
    ],
    related: [
      HELP_TOPICS.parentPickupChange,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentMessages,
    ],
  };
}

/** Abholzeit fuer einen Tag (i18n `parentChildCare.pickup`). Immer Anfrage. */
function parentPickupChangeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentPickupChange,
    title: "Die Abholzeit für einen Tag ändern",
    question: "Wie ändere ich die Abholzeit für einen einzelnen Tag?",
    summary: "Sie fragen eine andere Abholzeit an. Der Wochenplan bleibt.",
    group: "mein-kind",
    audience: "parent",
    icon: "Clock3",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Tippen Sie unter `Heute` auf `Abholzeit für ... ändern`.",
      "Wählen Sie bei `Tag` den betroffenen Tag.",
      "Tragen Sie bei `Abholzeit` die neue Zeit ein, zum Beispiel 15:30.",
      "Tragen Sie bei `Grund für die Änderung` kurz ein, warum.",
      "Tippen Sie auf `Anfrage senden`.",
    ],
    result:
      "Die Änderung gilt erst, nachdem die OGS sie bestätigt hat. Der reguläre Wochenplan bleibt unverändert.",
    notes: [
      "Der Grund ist Pflicht und darf höchstens 255 Zeichen lang sein.",
      "Soll sich die Zeit dauerhaft ändern? Dann fragen Sie eine Änderung der Betreuung an.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Ändern der Abholzeit sei für diese OGS nicht aktiviert? Dann schreiben Sie der OGS eine Nachricht.",
    ],
    related: [
      HELP_TOPICS.parentCareChange,
      HELP_TOPICS.parentReportAbsence,
      HELP_TOPICS.parentGuardians,
    ],
  };
}

/**
 * Abschnitt `So geht ... nach Hause` im Reiter `Betreuung` (Komponente
 * `DepartureSection` in components/parent/child-master-data.tsx). Je
 * Wochentag ein Kaestchen pro Weg; `accompanied` steht nur lesend da, weil
 * ein begleitetes Kind ein zweites Kind betrifft. Geaendert wird immer per
 * Anfrage (`requestButton`), nie direkt.
 */
function parentDepartureTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentDeparture,
    title: "Den Weg nach Hause ändern",
    question: "Wie ändere ich, wie mein Kind nach Hause geht?",
    summary: "Sie wählen je Wochentag den Weg und fragen die Änderung an.",
    group: "mein-kind",
    audience: "parent",
    icon: "Repeat",
    steps: [
      "Tippen Sie unten auf `Mein Kind`.",
      "Wählen Sie den Reiter `Betreuung`.",
      "Gehen Sie ganz nach unten zum Abschnitt `So geht ... nach Hause`. Dort steht der Name Ihres Kindes.",
      "Setzen Sie je Wochentag einen Haken bei `Geht allein`, `Bus` oder `Wird abgeholt`.",
      "Tippen Sie auf `Änderung anfragen`.",
    ],
    result:
      "Die OGS prüft Ihre Anfrage. Bis zur Bestätigung gilt der bisherige Weg.",
    notes: [
      "Pro Tag sind mehrere Wege möglich. Dann darf Ihr Kind jeden davon nehmen.",
      "Die Tage sind `Mo` bis `Fr`. Jeder Tag hat eine eigene Auswahl.",
      "Gibt es weitere Sorgeberechtigte mit App-Zugang, wählen Sie unter `Anfrage teilen (optional)`, wer mitlesen darf.",
    ],
    differences: [
      "Steht dort `In Prüfung`, wartet schon eine Änderung auf die OGS.",
      "Geht Ihr Kind mit einem anderen Kind, steht `Geht mit anderem Kind` nur zum Lesen da.",
      "Steht dort `Es gibt keine weiteren Sorgeberechtigten mit Zugang zur Eltern-App`? Dann gibt es niemanden zum Mitlesen.",
    ],
    troubleshootingDetails: [
      "Die Haken lassen sich nicht setzen? Geht Ihr Kind an einem Tag mit einem anderen Kind, ändern Sie den Weg nicht selbst. Schreiben Sie der OGS.",
      "moto sagt, Änderungsanfragen seien deaktiviert? Dann nimmt Ihre OGS keine Änderungen über die App an.",
    ],
    related: [
      HELP_TOPICS.parentCareChange,
      HELP_TOPICS.parentGuardians,
      HELP_TOPICS.parentPickupChange,
    ],
  };
}

/**
 * Zwei getrennte Anfragen: der Wochenplan (i18n `careSchedule`) und die
 * gebuchten Angebote (i18n `careOfferings`). Welche moeglich ist, entscheidet
 * die OGS -- bei buchungsgefuehrten Schulen laeuft alles ueber die Angebote.
 */
function parentCareChangeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentCareChange,
    title: "Eine Änderung der Betreuung anfragen",
    question: "Wie ändere ich die Betreuungstage meines Kindes?",
    summary: "Sie stellen eine Anfrage. Die OGS entscheidet darüber.",
    group: "mein-kind",
    audience: "parent",
    icon: "CalendarDays",
    steps: [],
    instructionGroups: [
      {
        title: "Den Wochenplan ändern",
        steps: [
          "Tippen Sie unten auf `Mein Kind`.",
          "Wählen Sie den Reiter `Betreuung`.",
          "Tippen Sie bei `Dein OGS Wochenplan` auf `Änderungen anfragen`.",
          "Schalten Sie je Tag `Betreuung an diesem Tag` ein oder aus.",
          "Tippen Sie auf `Anfrage an OGS senden`.",
        ],
      },
      {
        title: "Die gebuchte Betreuung ändern",
        steps: [
          "Gehen Sie zum Abschnitt `Gebuchte Betreuung`.",
          "Tippen Sie auf `Betreuung ändern`.",
          "Wählen Sie die gewünschten Angebote.",
          "Senden Sie die Anfrage ab.",
        ],
      },
    ],
    result:
      "Bis zur Bestätigung bleibt alles wie bisher. Die Anfrage trägt `In Prüfung`.",
    notes: [
      "Eine Änderung am Wochenplan ist dauerhaft und gilt jede Woche.",
      "Geht es nur um einen Tag? Ändern Sie stattdessen die Abholzeit.",
      "Eine offene Anfrage lässt sich mit `Anfrage bearbeiten` noch ändern.",
    ],
    differences: [
      "Die Entscheidung der OGS steht unter `Beantragte Änderung`: `Änderung übernommen` oder `Anfrage abgelehnt` mit Begründung.",
      "Sie sehen nur Änderungen, die Ihre OGS erlaubt.",
    ],
    troubleshootingDetails: [
      "moto sagt, Ihre OGS nehme keine Anfragen zum Wochenplan an? Dann laufen Änderungen über die gebuchten Angebote.",
      "moto sagt, für dieses Kind werde bereits eine Änderung geprüft? Warten Sie die Entscheidung ab.",
      "moto sagt, der Betreuungszeitraum sei abgelaufen? Dann melden Sie Ihr Kind für den neuen Zeitraum neu an.",
    ],
    related: [
      HELP_TOPICS.parentPickupChange,
      HELP_TOPICS.parentEnroll,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}

/** `/parents/messages`: je Kind eine Unterhaltung mit dem OGS-Team. */
function parentMessagesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentMessages,
    title: "Nachrichten lesen und schreiben",
    question: "Wie erreiche ich die OGS?",
    summary: "Sie schreiben dem OGS-Team und lesen seine Antworten.",
    group: "nachrichten",
    audience: "parent",
    icon: "MessageSquareText",
    steps: [
      "Tippen Sie unten auf `Nachrichten`.",
      "Tippen Sie auf die Unterhaltung des Kindes.",
      "Tippen Sie unten in das Feld `Nachricht an die OGS schreiben`.",
      "Tippen Sie rechts auf den Pfeil `Senden`.",
    ],
    result: "Die OGS bekommt Ihre Nachricht sofort.",
    notes: [
      "Jedes Kind hat eine eigene Unterhaltung. Oben steht, für welches Kind sie gilt.",
      "Unter Ihrer Nachricht steht `Gesendet` und später `Von der OGS gelesen`.",
    ],
    differences: [
      "Bei einem Kind steht `Für dieses Kind können Sie nicht schreiben`? Dann dürfen Sie den Verlauf nur lesen.",
    ],
    troubleshootingDetails: [
      "moto sagt, Ihre Sitzung sei abgelaufen? Melden Sie sich neu an und schicken Sie die Nachricht erneut.",
      "Es geht um eine Abwesenheit oder eine andere Abholzeit? Nutzen Sie dafür die passende Meldung statt einer Nachricht.",
    ],
    related: [
      HELP_TOPICS.parentNews,
      HELP_TOPICS.parentNotifications,
      HELP_TOPICS.parentChildOverview,
    ],
  };
}

/**
 * `/parents/news`: Elternbriefe, Mitteilungen und Umfragen. Manche verlangen
 * eine Lesebestaetigung, Umfragen eine Antwort.
 */
function parentNewsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentNews,
    title: "Elternbriefe lesen und beantworten",
    question: "Wo finde ich Post von der OGS?",
    summary: "Unter `Elternbriefe` stehen Mitteilungen und Umfragen.",
    group: "nachrichten",
    audience: "parent",
    icon: "Megaphone",
    steps: [],
    instructionGroups: [
      {
        title: "Einen Elternbrief bestätigen",
        steps: [
          "Tippen Sie unten auf `Mehr`.",
          "Wählen Sie `Elternbriefe`.",
          "Tippen Sie auf den Eintrag unter `Offen`.",
          "Lesen Sie den Text.",
          "Tippen Sie auf `Gelesen bestätigen`.",
        ],
      },
      {
        title: "Eine Umfrage beantworten",
        steps: [
          "Öffnen Sie die Umfrage unter `Offen`.",
          "Wählen Sie Ihre Antwort.",
          "Tippen Sie auf `Antwort speichern`.",
        ],
      },
    ],
    result: "Erledigte Einträge wandern von `Offen` zu `Erledigt`.",
    notes: [
      "Die Bestätigung heißt `Lesebestätigung`. Damit bestätigen Sie nur, dass Sie den Brief gelesen haben.",
      "Bei mehreren Kindern zeigt moto, wie viele Antworten noch fehlen.",
      "Angehängte Dateien öffnen Sie, indem Sie auf den Namen tippen.",
    ],
    differences: [
      "Ein Eintrag trägt `Elternbrief`, `Umfrage`, `Wichtig` oder `Betreuung fällt aus`.",
      "Bei einer Umfrage steht `Antwort bis` mit dem letzten Tag.",
      "Ist die Frist vorbei, steht dort `Umfrage geschlossen`.",
    ],
    troubleshootingDetails: [
      "moto sagt, der Elternbrief sei nicht mehr aktuell? Laden Sie die Seite neu.",
      "Fehlt `Elternbriefe` ganz? Dann nutzt Ihre OGS diese Funktion nicht.",
    ],
    related: [
      HELP_TOPICS.parentMessages,
      HELP_TOPICS.parentNotifications,
      HELP_TOPICS.parentFeatureMissing,
    ],
  };
}

/** `/parents/calendar`: Termine der naechsten drei Monate, mit Zu- und Absage. */
function parentCalendarTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentCalendar,
    title: "Den Kalender ansehen",
    question: "Wo sehe ich Termine und Einladungen?",
    summary: "Der Kalender zeigt die Termine Ihrer Kinder.",
    group: "nachrichten",
    audience: "parent",
    icon: "CalendarDays",
    steps: [
      "Tippen Sie unten auf `Kalender`.",
      "Lesen Sie die Termine unter `Diese Woche`, `Nächste Woche` und `Später`.",
      "Tippen Sie einen Termin an, um Einzelheiten zu sehen.",
    ],
    result: "Sie sehen Tag, Uhrzeit und für welches Kind der Termin gilt.",
    notes: [
      "Ein Termin ist eine `Betreuungszeit`, eine `Einladung` oder ein `Termin`.",
      "Bei einer Einladung antworten Sie mit `Zusagen` oder `Absagen`.",
      "Danach steht am Termin `Zugesagt` oder `Abgesagt`.",
      "Rechts steht ein Monatskalender. Mit den Pfeilen wechseln Sie den Monat.",
      "Unter `Kalender abonnieren` holen Sie sich mit `Abo-Link anzeigen` die Termine in Ihren eigenen Kalender.",
    ],
    differences: [
      "Ein Termin trägt `Antwort erforderlich`? Dann wartet die OGS auf Ihre Antwort.",
      "Hat die OGS abgesagt, steht dort `Von der OGS abgesagt`.",
      "Der Kalender zeigt die nächsten 3 Monate.",
    ],
    troubleshootingDetails: [
      "Der Kalender ist leer? Dann stehen in den nächsten 3 Monaten keine Termine an.",
    ],
    related: [
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentReportAbsence,
      HELP_TOPICS.parentMealPlan,
    ],
  };
}

/** `/parents/meal-plan`: Wochenplan des Mittagessens, nur mit Freischaltung. */
function parentMealPlanTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentMealPlan,
    title: "Das Mittagessen ansehen",
    question: "Wo sehe ich, was es zu essen gibt?",
    summary: "Unter `Mittagessen` steht der Plan der Woche.",
    group: "nachrichten",
    audience: "parent",
    icon: "UtensilsCrossed",
    steps: [
      "Tippen Sie unten auf `Mehr`.",
      "Wählen Sie `Mittagessen`.",
      "Wechseln Sie bei Bedarf mit den Pfeilen die Woche.",
    ],
    result: "Sie sehen je Tag das Gericht und ob Ihr Kind mitisst.",
    notes: [
      "Über dem Plan steht die Kalenderwoche, zum Beispiel `KW 38 · Diese Woche`.",
    ],
    differences: [
      "Steht an einem Tag `Kein Essen eingetragen`, hat die OGS dafür nichts hinterlegt.",
    ],
    troubleshootingDetails: [
      "moto sagt, der Essensplan sei nicht freigeschaltet? Dann nutzt Ihre OGS diese Funktion nicht.",
      "Die Woche ist leer? Dann hat die OGS den Plan noch nicht eingetragen.",
    ],
    related: [
      HELP_TOPICS.parentCalendar,
      HELP_TOPICS.parentReportAbsence,
      HELP_TOPICS.parentFeatureMissing,
    ],
  };
}

/**
 * `/parents/settings`, Abschnitt `Benachrichtigungen`. Zwei Stellen
 * entscheiden: oben die Themen je Bereich, darunter der Knopf `Aktivieren`
 * unter `Benachrichtigungen auf diesem Gerät`. Die vier Schritte folgen der
 * Eltern-Anleitung `moto-eltern-push-benachrichtigungen.pdf`.
 */
function parentNotificationsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentNotifications,
    title: "Benachrichtigungen einstellen",
    question: "Wie erfahre ich, wenn es Neues zu meinem Kind gibt?",
    // „erlauben moto Ihr Gerät" war kein Deutsch: man erlaubt jemandem etwas,
    // und ein Gerät erlaubt man ohnehin nicht. Erlaubt werden die
    // Benachrichtigungen darauf. Zwei Sätze, weil es zwei Stellen sind --
    // genau darauf weist der Artikel weiter unten noch einmal hin.
    summary:
      "Sie wählen die Themen aus. Danach erlauben Sie die Benachrichtigungen auf Ihrem Gerät.",
    group: "einstieg",
    audience: "parent",
    icon: "BellRing",
    steps: [
      "Tippen Sie unten auf `Mehr` und dann auf `Einstellungen`.",
      "Schalten Sie unter `Benachrichtigungen` die Themen ein. Oder wählen Sie `Alle aktivieren`.",
      "Wählen Sie darunter bei `Benachrichtigungen auf diesem Gerät` den Knopf `Aktivieren`. Auf breiten Bildschirmen heißt er `Benachrichtigungen einschalten`.",
      "Ihr Gerät fragt einmal nach Erlaubnis. Wählen Sie `Erlauben`.",
    ],
    result:
      "moto meldet sich ab jetzt bei den gewählten Themen. Auch wenn die App geschlossen ist.",
    notes: [
      "Die Themen stehen unter `Kinder`, `Mitteilungen` und `Termine`.",
      "Sie können Themen später jederzeit ein- oder ausschalten.",
      "Sie brauchen beide Stellen: die Themen und den Knopf `Aktivieren`.",
      "Mit `Testbenachrichtigung senden` prüfen Sie danach, ob etwas ankommt.",
    ],
    differences: [
      "Am Computer steht `Einstellungen` in der Navigation statt hinter `Mehr`.",
      "Ein Thema trägt `Von Ihrer Schule derzeit deaktiviert`? Dann nutzt Ihre OGS es nicht.",
      "Hat Ihre Schule alle Benachrichtigungen abgeschaltet, bleibt Ihre Auswahl gespeichert und gilt später wieder.",
      "Auf Android sind die Schritte gleich. Die Frage Ihres Geräts kann anders aussehen.",
    ],
    troubleshootingDetails: [
      "Es kommt nichts an? Öffnen Sie moto über das Symbol auf Ihrem Startbildschirm. Nur so kommen die Hinweise an.",
      "Sie finden `Benachrichtigungen auf diesem Gerät` nicht? Dann liegt moto noch nicht auf Ihrem Startbildschirm.",
      "Sie haben die Frage Ihres Geräts abgelehnt? Erlauben Sie moto die Hinweise in den Einstellungen Ihres Geräts.",
    ],
    related: [
      HELP_TOPICS.parentInstallApp,
      HELP_TOPICS.parentMessages,
      HELP_TOPICS.parentNews,
    ],
  };
}

/** `/parents/anmeldung`: Phase waehlen, Formular ausfuellen, absenden. */
function parentEnrollTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentEnroll,
    title: "Mein Kind anmelden",
    question: "Wie melde ich mein Kind für die OGS an?",
    summary: "Sie wählen die Anmeldung und füllen ein Formular aus.",
    group: "anmeldung",
    audience: "parent",
    icon: "ListChecks",
    steps: [
      "Tippen Sie unten auf `Mehr`.",
      "Wählen Sie `Neue Anmeldung`.",
      "Wählen Sie unter `Ihre Schulen` oder `Weitere Schulen` die Schule.",
      "Tippen Sie auf die passende Anmeldung. Das Formular öffnet sich sofort.",
      "Tragen Sie Ihre Kontaktdaten und die Angaben zum Kind ein.",
      "Wählen Sie unter `Zusätzlich wählen` mindestens ein Angebot. `Pflichtangebote` sind schon gesetzt.",
      "Tippen Sie auf `Anmeldung absenden`.",
    ],
    result:
      "Sie bekommen eine Bestätigungs-E-Mail mit einem Link zum Stand Ihrer Anmeldung.",
    notes: [
      "Mit `Weiteres Kind` melden Sie Geschwister in derselben Anmeldung an.",
      "Bewahren Sie die Bestätigungs-E-Mail auf. Der Link darin führt zum Stand.",
      "Eine Anmeldung gilt für ein `Schuljahr`, eine `Ferienbetreuung` oder `Sonstiges`.",
    ],
    differences: [
      "Manche Anmeldungen sind `Nur bereits angemeldete Kinder`.",
      "An jeder Anmeldung steht `Anmeldung bis` mit der Frist.",
      "Über dem Formular steht `Andere Anmeldung wählen`. Damit gehen Sie zurück.",
    ],
    troubleshootingDetails: [
      "moto sagt, aktuell sei keine Anmeldephase geöffnet? Dann wenden Sie sich an die OGS.",
      "moto sagt, die Online-Anmeldung sei nicht freigeschaltet? Dann nimmt diese OGS keine Anmeldungen über moto an.",
    ],
    related: [
      HELP_TOPICS.parentEnrollStatus,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentCareChange,
    ],
  };
}

/** Statusseite ueber den Link aus der Bestaetigungs-E-Mail. */
function parentEnrollStatusTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentEnrollStatus,
    title: "Eine Anmeldung weiter bearbeiten",
    question: "Wo sehe ich, was aus meiner Anmeldung geworden ist?",
    summary: "Der Link aus der Bestätigungs-E-Mail führt zum Stand.",
    group: "anmeldung",
    audience: "parent",
    icon: "Eye",
    requirements: ["Sie haben die Bestätigungs-E-Mail zu Ihrer Anmeldung."],
    steps: [],
    instructionGroups: [
      {
        title: "Den Stand ansehen",
        steps: [
          "Öffnen Sie den Link aus der Bestätigungs-E-Mail.",
          "Lesen Sie unter `Aktueller Stand`, wie weit die Prüfung ist.",
          "Unter `Was jetzt passiert` steht, was als Nächstes kommt.",
        ],
      },
      {
        title: "Angaben ändern",
        steps: [
          "Öffnen Sie die Statusseite.",
          "Tippen Sie auf `Anmeldung bearbeiten`.",
          "Ändern Sie die Angaben.",
          "Tippen Sie auf `Änderungen speichern`.",
        ],
      },
      {
        title: "Eine Anmeldung zurückziehen",
        steps: [
          "Öffnen Sie die Statusseite.",
          "Tippen Sie bei dem Kind auf `Dieses Kind zurückziehen`.",
          "Bestätigen Sie mit `Endgültig zurückziehen`.",
        ],
      },
    ],
    result:
      "Sobald die OGS entschieden hat, sehen Sie das unter `Kinder` und bekommen eine E-Mail.",
    notes: [
      "Zurückziehen können Sie nicht selbst rückgängig machen. Wenden Sie sich dann an die OGS.",
      "Bei Rückfragen meldet sich die OGS über die angegebene E-Mail-Adresse.",
    ],
    differences: [
      "Steht dort `Bestätigung erforderlich`? Dann müssen Sie mit `Anmeldung bestätigen` zusagen, sonst läuft die Anmeldung zur Frist ab.",
      "Steht dort `Anmeldung wurde verlängert`? Dann müssen Sie nichts tun.",
      "Bei einem Kind steht `Diese Anmeldung kann nicht mehr online geändert werden`? Dann wenden Sie sich an die OGS.",
    ],
    troubleshootingDetails: [
      "moto sagt, der Status-Link sei ungültig? Prüfen Sie die Adresse aus der E-Mail oder fragen Sie bei der OGS nach.",
    ],
    related: [
      HELP_TOPICS.parentEnroll,
      HELP_TOPICS.parentMessages,
      HELP_TOPICS.parentChildMissing,
    ],
  };
}

/** Problemartikel: Einladung laesst sich nicht annehmen. */
function parentAccountProblemTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentAccountProblem,
    title: "Ich kann mein Konto nicht einrichten",
    question: "Was tun, wenn die Einladung nicht funktioniert?",
    summary: "Meist ist der Link abgelaufen oder schon benutzt.",
    group: "probleme",
    audience: "parent",
    icon: "ShieldAlert",
    steps: [
      "Lesen Sie, was auf der Seite steht.",
      "Prüfen Sie, ob Sie den Link schon einmal geöffnet haben.",
      "Versuchen Sie, sich direkt anzumelden.",
      "Klappt das nicht, bitten Sie Ihre OGS um eine neue Einladung.",
    ],
    result: "Mit einer neuen Einladung können Sie Ihr Passwort festlegen.",
    notes: ["Ein Einladungslink lässt sich nur einmal benutzen."],
    differences: [
      "`Diese Einladung ist abgelaufen oder wurde bereits verwendet.` Fragen Sie nach einer neuen.",
      "`Für diese E-Mail existiert bereits ein Konto.` Melden Sie sich direkt an.",
      "`Das Passwort erfüllt noch nicht alle Sicherheitsanforderungen.` Prüfen Sie die Liste unter dem Feld.",
    ],
    troubleshootingDetails: [
      "Sie haben ein Konto, aber kein Passwort mehr? Nutzen Sie auf der Anmeldeseite `Passwort vergessen?`.",
    ],
    related: [
      HELP_TOPICS.parentAccount,
      HELP_TOPICS.parentLogin,
      HELP_TOPICS.parentChildMissing,
    ],
  };
}

/** Problemartikel: kein Kind sichtbar. Der Zugang haengt an der Freigabe. */
function parentChildMissingTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentChildMissing,
    title: "Mein Kind wird nicht angezeigt",
    question: "Warum sehe ich mein Kind nicht?",
    summary: "Die OGS muss Ihren Zugang zum Kind erst freigeben.",
    group: "probleme",
    audience: "parent",
    icon: "Search",
    steps: [
      "Prüfen Sie, ob Sie mit der richtigen E-Mail-Adresse angemeldet sind.",
      "Haben Sie mehrere Kinder, prüfen Sie oben die Auswahl.",
      "Schreiben Sie sonst der OGS und nennen Sie Name und Klasse des Kindes.",
    ],
    result: "Sobald die OGS Ihren Zugang freigibt, erscheint das Kind.",
    notes: [
      "Ein Kind kommt nicht automatisch zu Ihrem Konto. Die OGS verbindet beides.",
      "Eine abgegebene Anmeldung allein genügt nicht. Das Kind erscheint erst nach der Übernahme.",
    ],
    differences: [
      "moto sagt, mit Ihrem Konto sei noch kein Kind verknüpft? Dann fehlt die Freigabe.",
    ],
    troubleshootingDetails: [
      "Sie haben zwei E-Mail-Adressen? Melden Sie sich mit der an, an die die Einladung ging.",
    ],
    related: [
      HELP_TOPICS.parentAccount,
      HELP_TOPICS.parentChildOverview,
      HELP_TOPICS.parentMessages,
    ],
  };
}

/** Problemartikel: fehlende Bereiche haengen an den Schaltern der OGS. */
function parentFeatureMissingTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentFeatureMissing,
    title: "Eine Funktion ist für mich nicht verfügbar",
    question: "Warum fehlt mir ein Bereich im Elternportal?",
    summary: "Jede OGS schaltet selbst frei, was Eltern sehen und tun dürfen.",
    group: "probleme",
    audience: "parent",
    icon: "EyeSlash",
    steps: [
      "Prüfen Sie, ob der Bereich hinter `Mehr` liegt.",
      "Fehlt er auch dort, nutzt Ihre OGS ihn nicht.",
      "Brauchen Sie ihn, schreiben Sie der OGS eine Nachricht.",
    ],
    result: "Die OGS kann die Funktion für Ihre Schule einschalten.",
    notes: [
      "moto blendet einen Bereich aus, statt ihn zu sperren. Ein fehlender Eintrag ist kein Fehler.",
      "`Elternbriefe` und `Mittagessen` gibt es nur, wenn Ihre Schule sie führt.",
    ],
    differences: [
      "Manche Dinge dürfen Sie nur ansehen, nicht ändern. Dann fehlt der Knopf, nicht der Bereich.",
    ],
    troubleshootingDetails: [
      "Das Ändern von Angaben ist deaktiviert? Schreiben Sie der OGS, was geändert werden soll.",
    ],
    related: [
      HELP_TOPICS.parentMessages,
      HELP_TOPICS.parentChildData,
      HELP_TOPICS.parentChildMissing,
    ],
  };
}

// Eltern-Portal. Geruest nach der echten Navigation der Eltern-App
// (`PARENT_PRIMARY_NAV` und `PARENT_MORE_NAV`): Start, Kinder, Nachrichten,
// Kalender, dahinter Elternbriefe, Mittagessen, Einstellungen und Anmeldung.
//
// Die Eltern-App ist ein eigenes Portal mit eigener Adresse. Themen der
// Betreuung werden hier NICHT geteilt: Eltern melden sich woanders an, sehen
// andere Seiten und duerfen vieles nur anfragen statt aendern.
//
// Die Ablaeufe sind noch nicht Schritt fuer Schritt gegen die App geprueft,
// deshalb bleiben es Entwuerfe.
const PARENT_DRAFT_TOPICS: readonly HelpTopic[] = [
  // --- Einstieg und Konto ---
  parentAccountTopic(),
  parentLoginTopic(),
  parentInstallAppTopic(),
  parentNotificationsTopic(),

  // --- Mein Kind und die Betreuung ---
  parentChildOverviewTopic(),
  parentChildDataTopic(),
  parentGuardiansTopic(),
  parentReportAbsenceTopic(),
  parentPickupChangeTopic(),
  parentCareChangeTopic(),
  parentDepartureTopic(),

  // --- Nachrichten und Infos ---
  parentMessagesTopic(),
  parentNewsTopic(),
  parentCalendarTopic(),
  parentMealPlanTopic(),

  // --- Anmeldung ---
  parentEnrollTopic(),
  parentEnrollStatusTopic(),

  // --- Wenn etwas nicht klappt ---
  parentAccountProblemTopic(),
  parentChildMissingTopic(),
  parentFeatureMissingTopic(),
] as const;

/** Einladung annehmen: geteilte Strecke, danach geht es nach moto schule. */
function teacherAccessTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherAccess,
    title: "Zugang zu moto schule einrichten",
    question: "Wie bekomme ich Zugang zu moto schule?",
    summary: "Sie öffnen den Link aus der E-Mail und legen Ihr Passwort fest.",
    group: "einstieg",
    audience: "teacher",
    icon: "KeyRound",
    requirements: ["Sie haben eine Einladungs-E-Mail der OGS bekommen."],
    steps: [
      "Öffnen Sie den Link aus der E-Mail.",
      "Legen Sie Ihr Passwort fest.",
      "Nehmen Sie die Einladung an.",
      "Öffnen Sie danach moto schule und melden Sie sich an.",
    ],
    result: "Sie landen in der `Klassenansicht` mit Ihren Klassen.",
    notes: [
      "Haben Sie schon ein Konto? Dann melden Sie sich an und kehren zur Einladung zurück. Ihr Passwort bleibt unverändert.",
      "Der Link aus der E-Mail führt schon zur richtigen Adresse.",
    ],
    differences: [
      "moto schule hat eine eigene Adresse. Über die Adresse der OGS kommen Sie nicht hinein.",
    ],
    troubleshootingDetails: [
      "Der Link funktioniert nicht mehr? Bitten Sie die OGS um eine neue Einladung.",
    ],
    related: [
      HELP_TOPICS.teacherLogin,
      HELP_TOPICS.teacherLoginProblem,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}

/** `/school/login`. Eigenes Portal; ein Konto ohne Rolle wird abgewiesen. */
function teacherLoginTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherLogin,
    title: "Bei moto schule anmelden",
    question: "Wie melde ich mich bei moto schule an?",
    summary: "moto schule hat eine eigene Adresse und eine eigene Anmeldung.",
    group: "einstieg",
    audience: "teacher",
    icon: "LogIn",
    steps: [
      "Öffnen Sie die Adresse von moto schule.",
      "Tragen Sie Ihre `E-Mail-Adresse` ein.",
      "Tragen Sie Ihr `Passwort` ein.",
      "Wählen Sie `Anmelden`.",
    ],
    result: "moto öffnet die `Klassenansicht` mit dem heutigen Tag.",
    notes: [
      "Legen Sie sich die Adresse als Lesezeichen an.",
      "Passwort vergessen? Wählen Sie `Passwort vergessen?` auf der Anmeldeseite.",
    ],
    differences: [
      "Oben steht `Willkommen im Schulportal`. So erkennen Sie, dass Sie richtig sind.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Konto habe keinen Zugang zum Schul-Portal? Dann fehlt Ihnen die Rolle. Wenden Sie sich an die OGS-Verwaltung.",
      "moto sagt `Ungültige E-Mail oder Passwort`? Prüfen Sie beides und versuchen Sie es erneut.",
    ],
    related: [
      HELP_TOPICS.teacherAccess,
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherLoginProblem,
    ],
  };
}

/** `/school/einstellungen`: Auswahl der Arten, danach das Geraet einrichten. */
function teacherSettingsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherSettings,
    title: "Benachrichtigungen einstellen",
    question: "Wie werde ich über Neues informiert?",
    summary:
      "Erst wählen, worüber Sie informiert werden, dann das Gerät einrichten.",
    group: "einstieg",
    audience: "teacher",
    icon: "Settings",
    steps: [
      "Öffnen Sie `Einstellungen`.",
      "Wählen Sie unter `Benachrichtigungen` die Arten aus.",
      "Erlauben Sie danach die Benachrichtigungen auf Ihrem Gerät.",
    ],
    result: "moto meldet sich, sobald etwas Neues für Sie ansteht.",
    notes: [
      "Die Hinweise kommen aus dem Team-Chat der OGS.",
      "Benachrichtigungen kommen erst an, wenn moto schule auf dem Startbildschirm liegt.",
    ],
    differences: [
      "Nutzen Sie Samsung Internet? Öffnen Sie moto schule stattdessen in Chrome.",
    ],
    troubleshootingDetails: [
      "Es kommt nichts an? Prüfen Sie, ob Sie moto auf dem Gerät erlaubt haben, Benachrichtigungen zu senden.",
    ],
    related: [
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherNotices,
      HELP_TOPICS.teacherLogin,
    ],
  };
}

/** `/school` (`class-day-overview.tsx`): Tagesuebersicht ueber alle Klassen. */
function teacherClassDayTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherClassDay,
    title: "Meinen Klassentag ansehen",
    question: "Wer aus meinen Klassen bleibt heute in der Betreuung?",
    summary:
      "Die `Klassenansicht` zeigt Ihre Klassen und die Zahlen des Tages.",
    group: "klasse",
    audience: "teacher",
    icon: "Users",
    steps: [
      "Melden Sie sich bei moto schule an.",
      "Sie landen in der `Klassenansicht`.",
      "Lesen Sie bei jeder Klasse die Zahlen des Tages.",
      "Wählen Sie eine Klasse, um ihre Liste zu öffnen.",
    ],
    result: "Sie sehen, wer heute bleibt und wer nach Hause geht.",
    notes: [
      "Steht bei einer Klasse `4 Kinder anders als sonst`, weicht dort heute etwas vom üblichen Plan ab.",
      "Mit den Pfeilen sehen Sie den vorherigen oder nächsten Tag.",
    ],
    differences: ["An einem Tag ohne Unterricht steht `Kein Schultag`."],
    troubleshootingDetails: [
      "Es steht `Keine Klassen zugewiesen`? Dann hat die OGS Ihnen noch keine Klasse zugeordnet.",
    ],
    related: [
      HELP_TOPICS.teacherClassList,
      HELP_TOPICS.teacherChildDetails,
      HELP_TOPICS.teacherArrivalChange,
    ],
  };
}

/**
 * Klassenseite (`class-day-class.tsx`). Fuenf benannte Abschnitte plus die
 * Statuskennzeichen aus `status-labels.ts`.
 */
function teacherClassListTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherClassList,
    title: "Die Liste einer Klasse lesen",
    question: "Was bedeuten die Abschnitte in der Klassenliste?",
    summary: "Die Liste trennt, wer bleibt, wer nach Hause geht und wer fehlt.",
    group: "klasse",
    audience: "teacher",
    icon: "ListChecks",
    steps: [],
    instructionGroups: [
      {
        title: "Die Liste öffnen",
        steps: [
          "Wählen Sie in der `Klassenansicht` eine Klasse.",
          "Lesen Sie die Abschnitte von oben nach unten.",
        ],
      },
      {
        title: "Was die Abschnitte bedeuten",
        steps: [
          "`Bleiben in der Betreuung`: diese Kinder übergeben Sie an die OGS.",
          "`Gehen nach Hause`: diese Kinder gehen nach dem Unterricht.",
          "`Abgemeldet`: für heute wurde die Betreuung abgesagt.",
          "`Keine Betreuung`: diese Kinder sind gar nicht in der OGS.",
          "`Klassenverband`: alle Kinder der Klasse zusammen.",
        ],
        ordered: false,
      },
      {
        title: "Was die Kennzeichen bedeuten",
        steps: [
          "`Krank`, `Entschuldigt` oder `Klassenfahrt`: das Kind fehlt heute.",
          "`Heute abgemeldet`: die Betreuung für heute wurde abgesagt.",
          "`Andere Abholzeit`: das Kind geht heute zu einer anderen Zeit.",
        ],
        ordered: false,
      },
    ],
    result:
      "Sie sehen ohne Rückfrage, welches Kind nach dem Unterricht wohin geht.",
    notes: [
      "Bei einer anderen Abholzeit steht darunter die übliche Zeit mit `sonst`.",
      "Kam die Meldung heute herein, steht die Uhrzeit dabei.",
    ],
    differences: ["Geändert wird das im OGS-Team, nicht in moto schule."],
    troubleshootingDetails: [
      "Ein Kind fehlt in der Liste? Dann ist es nicht in dieser Klasse eingetragen. Schreiben Sie der OGS.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherChildDetails,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}

/**
 * Kindangaben. Die harte Grenze: Abhol- und Notfallkontakte gibt es nur zu
 * Kindern einer laufenden Aufsicht (`student-sheet-modal.tsx`).
 */
function teacherChildDetailsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherChildDetails,
    title: "Angaben zu einem Kind ansehen",
    question: "Was sehe ich zu einem einzelnen Kind?",
    summary: "Sie sehen den heutigen Tag. Kontaktdaten nur in Ihrer Aufsicht.",
    group: "klasse",
    audience: "teacher",
    icon: "UserRoundSearch",
    steps: [
      "Öffnen Sie die Liste Ihrer Klasse.",
      "Wählen Sie das Kind aus der Liste.",
      "Lesen Sie unter `Heute`, wann es kommt und geht.",
    ],
    result:
      "Sie sehen `Kommt um`, `Geht um` und `Geht so nach Hause` für den heutigen Tag.",
    notes: [
      "Mehr als den heutigen Tag zeigt moto schule nicht. Keine Stammdaten, keine Adressen.",
      "Jeder Zugriff auf die Ansicht wird protokolliert.",
    ],
    differences: [
      "In einer laufenden Aufsicht sehen Sie zusätzlich `Darf abholen` und `Im Notfall anrufen`.",
      "Außerhalb einer Aufsicht fehlen diese beiden Abschnitte.",
    ],
    troubleshootingDetails: [
      "Sie brauchen einen Kontakt, haben aber keine Aufsicht? Fragen Sie im OGS-Team nach.",
    ],
    related: [
      HELP_TOPICS.teacherClassList,
      HELP_TOPICS.teacherSupervisionRoster,
      HELP_TOPICS.teacherMessages,
    ],
  };
}

/**
 * `Ankunft heute ändern` (`class-arrival-exception-dialog.tsx`). Nur mit
 * Freigabe der OGS; gilt fuer einen Tag und die ganze Klasse.
 */
function teacherArrivalChangeTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherArrivalChange,
    title: "Die Ankunftszeit meiner Klasse ändern",
    question: "Der Unterricht fällt aus. Wie sage ich das der OGS?",
    summary: "Sie tragen die neue Ankunftszeit für einen einzelnen Tag ein.",
    group: "klasse",
    audience: "teacher",
    icon: "Clock3",
    steps: [
      "Öffnen Sie die Liste Ihrer Klasse.",
      "Wählen Sie `Ankunft heute ändern`.",
      "Wählen Sie den Tag.",
      "Tragen Sie bei `Kommt um` die neue Zeit ein.",
      "Tragen Sie bei Bedarf einen `Grund (optional)` ein, zum Beispiel `Wandertag`.",
      "Wählen Sie `Tag speichern`.",
    ],
    result:
      "Die OGS sieht die neue Zeit sofort. Sie gilt nur für Kinder mit Betreuung an diesem Tag.",
    notes: [
      "Die Zeit gilt für alle Kinder der Klasse und nur an diesem Tag.",
      "Mit `Entfernen` nehmen Sie einen Eintrag wieder zurück.",
    ],
    differences: [
      "Fehlt `Ankunft heute ändern`? Dann hat Ihre OGS das Eintragen nicht freigegeben. Sie sehen dann nur die Zeit, die die OGS eingetragen hat.",
      "Bei einem Eintrag der OGS steht `Eingetragen von der OGS`.",
    ],
    troubleshootingDetails: [
      "moto sagt, das Eintragen sei nicht mehr freigegeben? Dann hat die OGS es zurückgenommen. Schreiben Sie ihr eine Nachricht.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}

/** `/school/aufsichten`: die eigenen Dienste des Tages. */
function teacherSupervisionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherSupervision,
    title: "Meine Aufsichten ansehen",
    question: "Wann habe ich heute Aufsicht?",
    summary: "Unter `Meine Aufsichten` stehen Ihre eigenen Dienste des Tages.",
    group: "aufsicht",
    audience: "teacher",
    icon: "Eye",
    steps: [
      "Öffnen Sie `Meine Aufsichten`.",
      "Lesen Sie, welche Aufsichten heute für Sie eingeteilt sind.",
      "Wählen Sie eine Aufsicht, um Einzelheiten zu sehen.",
    ],
    result: "Sie sehen Zeit, Raum und die Kinder Ihrer Aufsicht.",
    notes: ["Mit `Alle Aufsichten heute` kommen Sie zurück zur Übersicht."],
    differences: [
      "Eine Aufsicht trägt `Fällt aus`? Dann findet sie heute nicht statt.",
      "Vertreten Sie jemanden, steht das an der Aufsicht.",
    ],
    troubleshootingDetails: [
      "Die Liste ist leer? Dann sind Sie heute nicht für eine Aufsicht eingeteilt. Eingeteilt wird im OGS-Team.",
    ],
    related: [
      HELP_TOPICS.teacherStartSupervision,
      HELP_TOPICS.teacherSupervisionRoster,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}

/** Aufsicht starten: erst danach zeigt moto die Kinder mit Kontakten. */
function teacherStartSupervisionTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherStartSupervision,
    title: "Eine Aufsicht starten",
    question: "Wie starte ich meine Aufsicht?",
    summary: "Mit dem Start sehen Sie die Kinder, die zu Ihnen gehören.",
    group: "aufsicht",
    audience: "teacher",
    icon: "Clock3",
    steps: [
      "Öffnen Sie `Meine Aufsichten`.",
      "Wählen Sie die Aufsicht, die jetzt beginnt.",
      "Wählen Sie `Aufsicht starten`.",
    ],
    result: "moto zeigt danach die Kinder dieser Aufsicht mit ihren Angaben.",
    notes: [
      "Starten können Sie erst kurz vor dem Beginn, nicht schon am Morgen.",
    ],
    differences: [
      "Ist `Aufsicht starten` grau, ist es noch zu früh oder die Aufsicht läuft schon.",
    ],
    troubleshootingDetails: [
      "Der Knopf fehlt ganz? Dann ist diese Aufsicht nicht Ihre. Prüfen Sie die Übersicht.",
    ],
    related: [
      HELP_TOPICS.teacherSupervision,
      HELP_TOPICS.teacherSupervisionRoster,
      HELP_TOPICS.teacherFeatureMissing,
    ],
  };
}

/** Kindblatt in der laufenden Aufsicht: Abholung und Notfallkontakt. */
function teacherSupervisionRosterTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherSupervisionRoster,
    title: "Die Kinder meiner Aufsicht sehen",
    question: "Wer darf ein Kind abholen und wen rufe ich im Notfall an?",
    summary:
      "In einer laufenden Aufsicht sehen Sie Abholung und Notfallkontakt.",
    group: "aufsicht",
    audience: "teacher",
    icon: "Users",
    steps: [
      "Starten Sie Ihre Aufsicht.",
      "Wählen Sie ein Kind aus der Liste.",
      "Lesen Sie unter `Heute` die Zeiten des Kindes.",
      "Unter `Darf abholen` steht, wer es abholen darf.",
      "Unter `Im Notfall anrufen` stehen die Notfallkontakte.",
    ],
    result:
      "Sie können ein Kind sicher übergeben, ohne im OGS-Team nachzufragen.",
    notes: [
      "Diese Angaben sehen Sie nur zu Kindern Ihrer laufenden Aufsicht.",
      "Jeder Zugriff wird protokolliert.",
    ],
    differences: [
      "In der `Klassenansicht` fehlen `Darf abholen` und `Im Notfall anrufen`. Das ist Absicht.",
    ],
    troubleshootingDetails: [
      "Ein Kontakt fehlt oder stimmt nicht? Melden Sie das dem OGS-Team. Ändern können Sie es hier nicht.",
    ],
    related: [
      HELP_TOPICS.teacherStartSupervision,
      HELP_TOPICS.teacherChildDetails,
      HELP_TOPICS.teacherSupervision,
    ],
  };
}

/** `/school/nachrichten`: derselbe Team-Chat wie in der OGS, optional. */
function teacherMessagesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherMessages,
    title: "Nachrichten an die OGS schreiben",
    question: "Wie erreiche ich die OGS?",
    summary: "Sie schreiben dem OGS-Team und lesen seine Antworten.",
    group: "nachrichten",
    audience: "teacher",
    icon: "MessageSquareText",
    steps: [
      "Öffnen Sie `Nachrichten`.",
      "Wählen Sie eine Unterhaltung. Oder wählen Sie `Neue Nachricht`.",
      "Schreiben Sie Ihre Nachricht.",
      "Senden Sie sie ab.",
    ],
    result: "Das OGS-Team bekommt Ihre Nachricht sofort.",
    notes: [
      "Vor Ihrer eigenen Nachricht steht `Sie:`.",
      "Hier melden Sie, was die OGS wissen muss, zum Beispiel eine falsche Abholzeit.",
    ],
    differences: [
      "Fehlt `Nachrichten` ganz? Dann nutzt Ihre OGS diese Funktion nicht.",
    ],
    troubleshootingDetails: [
      "Geht es um die Ankunftszeit Ihrer Klasse? Tragen Sie die lieber direkt ein, wenn Ihre OGS das freigegeben hat.",
    ],
    related: [
      HELP_TOPICS.teacherNotices,
      HELP_TOPICS.teacherSettings,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}

/** `/school/tagesinformationen`: Hinweise der OGS-Leitung, teils zu bestaetigen. */
function teacherNoticesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherNotices,
    title: "Tagesinformationen lesen",
    question: "Wo stehen die Hinweise der OGS-Leitung?",
    summary: "Unter `Tagesinformationen` steht, was für heute gilt.",
    group: "nachrichten",
    audience: "teacher",
    icon: "Megaphone",
    steps: [
      "Öffnen Sie `Tagesinformationen`.",
      "Lesen Sie die Hinweise für heute.",
      "Wählen Sie bei Bedarf `Zur Kenntnis nehmen`.",
    ],
    result: "Bestätigte Hinweise tragen danach `Zur Kenntnis genommen`.",
    notes: [
      "Sie sehen nur Hinweise, die für Lehrkräfte oder für alle gelten.",
      "Ein Zähler neben dem Eintrag zeigt, wie viele Hinweise offen sind.",
    ],
    differences: [
      "Nicht jeder Hinweis verlangt eine Bestätigung. Dann fehlt der Knopf.",
    ],
    troubleshootingDetails: [
      "moto meldet, die Kenntnisnahme sei nicht gespeichert worden? Versuchen Sie es noch einmal.",
    ],
    related: [
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherSettings,
      HELP_TOPICS.teacherClassDay,
    ],
  };
}

/** Problemartikel: falsches Portal ist die haeufigste Ursache. */
function teacherLoginProblemTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherLoginProblem,
    title: "Die Anmeldung klappt nicht",
    question: "Was tun, wenn die Anmeldung nicht klappt?",
    summary: "Meist wurde die Anmeldung der OGS statt moto schule benutzt.",
    group: "probleme",
    audience: "teacher",
    icon: "ShieldAlert",
    steps: [
      "Prüfen Sie, ob oben `Willkommen im Schulportal` steht.",
      "Steht dort etwas anderes, nutzen Sie die Adresse von moto schule.",
      "Prüfen Sie E-Mail-Adresse und Passwort.",
      "Nutzen Sie sonst `Passwort vergessen?`.",
    ],
    result: "Nach der Anmeldung landen Sie in der `Klassenansicht`.",
    notes: [
      "Der Link aus der Einladungs-E-Mail führt immer zur richtigen Adresse.",
    ],
    differences: [
      "`Dieses Konto hat keinen Zugang zum Schul-Portal.` Dann fehlt Ihrem Konto die Rolle. Wenden Sie sich an die OGS-Verwaltung.",
      "`Ungültige E-Mail oder Passwort.` Prüfen Sie beides.",
    ],
    troubleshootingDetails: [
      "Sie werden immer wieder abgemeldet? Melden Sie sich einmal neu an.",
    ],
    related: [
      HELP_TOPICS.teacherLogin,
      HELP_TOPICS.teacherAccess,
      HELP_TOPICS.teacherNoClasses,
    ],
  };
}

/** Problemartikel: Klassen weist die OGS in der Personalakte zu. */
function teacherNoClassesTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherNoClasses,
    title: "Mir wird keine Klasse angezeigt",
    question: "Warum sehe ich meine Klasse nicht?",
    summary: "Die OGS muss Ihnen die Klassen erst zuweisen.",
    group: "probleme",
    audience: "teacher",
    icon: "Search",
    steps: [
      "Lesen Sie, ob dort `Keine Klassen zugewiesen` steht.",
      "Schreiben Sie der OGS und nennen Sie Ihre Klassen.",
      "Nennen Sie die Klassen genau so, wie sie an der Schule heißen.",
    ],
    result:
      "Nach der Zuweisung erscheinen Ihre Klassen in der `Klassenansicht`.",
    notes: [
      "Die Zuweisung macht die OGS in Ihrer Personalakte.",
      "Die Schreibweise muss zur Klasse der Kinder passen. Groß- und Kleinschreibung spielt keine Rolle.",
    ],
    differences: [
      "Sie sehen eine Klasse, aber keine Kinder? Dann ist die Klasse anders geschrieben als bei den Kindern.",
      "Nach einem Jahrgangswechsel wandern die Klassen automatisch mit.",
    ],
    troubleshootingDetails: [
      "Es steht `Kein Schultag`? Dann ist heute kein Unterricht. Prüfen Sie einen anderen Tag.",
    ],
    related: [
      HELP_TOPICS.teacherClassDay,
      HELP_TOPICS.teacherAccess,
      HELP_TOPICS.teacherMessages,
    ],
  };
}

/** Problemartikel: fehlende Bereiche haengen an den Schaltern der OGS. */
function teacherFeatureMissingTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.teacherFeatureMissing,
    title: "Ein Bereich fehlt mir",
    question: "Warum fehlt mir ein Bereich in moto schule?",
    summary:
      "Jede OGS schaltet selbst frei, was Lehrkräfte sehen und tun dürfen.",
    group: "probleme",
    audience: "teacher",
    icon: "EyeSlash",
    steps: [
      "Prüfen Sie, welcher Bereich genau fehlt.",
      "Schreiben Sie der OGS, wenn Sie ihn brauchen.",
    ],
    result: "Die OGS kann den Bereich für Ihre Schule einschalten.",
    notes: [
      "moto blendet einen Bereich aus, statt ihn zu sperren. Ein fehlender Eintrag ist kein Fehler.",
      "moto schule zeigt bewusst wenig: Ihre Klassen und Ihre eigenen Aufsichten.",
    ],
    differences: [
      "Fehlt `Nachrichten`? Dann nutzt Ihre OGS den Team-Chat nicht.",
      "Fehlt `Ankunft heute ändern`? Dann hat die OGS das Eintragen nicht freigegeben.",
      "Abhol- und Notfallkontakte gibt es nur in einer laufenden Aufsicht.",
    ],
    troubleshootingDetails: [
      "Sie brauchen Stammdaten oder Kontaktdaten? Die zeigt moto schule nicht. Fragen Sie im OGS-Team nach.",
    ],
    related: [
      HELP_TOPICS.teacherMessages,
      HELP_TOPICS.teacherArrivalChange,
      HELP_TOPICS.teacherNoClasses,
    ],
  };
}

// Portal "moto schule" (#2207). Geruest nach der echten Navigation des
// Schul-Portals (`SCHOOL_PRIMARY_NAV`): Klassenansicht, Meine Aufsichten,
// Nachrichten, Tagesinformationen.
//
// Eine Lehrkraft sieht bewusst wenig: ihre Klassen und ihre eigenen
// Aufsichten. Abhol- und Notfallkontakte gibt es nur zu den Kindern einer
// laufenden Aufsicht. Deshalb wird hier nichts mit der Betreuung geteilt.
//
// Die Ablaeufe sind noch nicht Schritt fuer Schritt gegen die App geprueft,
// deshalb bleiben es Entwuerfe.
const TEACHER_DRAFT_TOPICS: readonly HelpTopic[] = [
  // --- Einstieg ---
  teacherAccessTopic(),
  teacherLoginTopic(),
  teacherSettingsTopic(),

  // --- Meine Klasse ---
  teacherClassDayTopic(),
  teacherClassListTopic(),
  teacherChildDetailsTopic(),
  teacherArrivalChangeTopic(),

  // --- Aufsicht ---
  teacherSupervisionTopic(),
  teacherStartSupervisionTopic(),
  teacherSupervisionRosterTopic(),

  // --- Nachrichten und Infos ---
  teacherMessagesTopic(),
  teacherNoticesTopic(),

  // --- Wenn etwas nicht klappt ---
  teacherLoginProblemTopic(),
  teacherNoClassesTopic(),
  teacherFeatureMissingTopic(),
] as const;

// Themen, die es ohne NFC in der OGS schlicht nicht gibt: das Tablet, die
// Armbaender und der Bereich `Aktivitaeten`, den moto nur mit NFC anzeigt.
// Sagt eine OGS ausdruecklich "kein NFC", verschwinden sie aus Seitenleiste
// und Suche, statt eine Nichtverfuegbarkeit zu erklaeren.
//
// Nur bei einem ausdruecklichen `false`. Ist die Arbeitsweise unbekannt
// (`null`), bleiben die Themen stehen und erklaeren, wovon sie abhaengen --
// sonst verschwiegen wir jemandem ein Thema, den wir nie gefragt haben.
const NFC_ONLY_TOPIC_IDS: ReadonlySet<HelpTopicId> = new Set([
  HELP_TOPICS.tabletLogin,
  HELP_TOPICS.tagAssignment,
  HELP_TOPICS.nfcWorkTime,
  HELP_TOPICS.nfcSupervision,
  HELP_TOPICS.nfcCheckIn,
  HELP_TOPICS.nfcProblem,
  HELP_TOPICS.manageActivity,
  // Ohne NFC hat die OGS keine Geraete, die sie verwalten koennte -- und der
  // Aktivitaetenkatalog fehlt ihr ebenfalls: beide Seiten stehen in
  // `NFC_ONLY_HREFS` (dashboard/sidebar.tsx).
  HELP_TOPICS.leadDevices,
  HELP_TOPICS.leadActivities,
  HELP_TOPICS.leadTabletSetup,
  HELP_TOPICS.leadNfcSettings,
]);

// Dasselbe fuer die einfache Anwesenheit: haelt eine OGS nur fest, ob ein
// Kind da ist, gibt es weder Raeume noch Aufsichten noch Aktivitaeten -- und
// damit auch keinen Aufenthaltsort, den man aendern koennte. Das An- und
// Abmelden steht als eigenes Thema in der Liste.
//
// Wieder nur bei einem ausdruecklichen "binary". Bei unbekannter
// Arbeitsweise bleiben die Themen stehen und erklaeren, wovon sie abhaengen.
const DETAILED_ONLY_TOPIC_IDS: ReadonlySet<HelpTopicId> = new Set([
  HELP_TOPICS.rooms,
  HELP_TOPICS.activeSupervision,
  HELP_TOPICS.manageActivity,
  HELP_TOPICS.changeLocation,
  // Der Tagesplan liegt hinter dem BinaryModeGuard
  // (`app/[tenant]/(protected)/tagesplan/page.tsx`).
  HELP_TOPICS.dayPlan,
]);

// Und dasselbe fuer die offene Betreuung: ohne feste eigene Gruppen gibt es
// weder `Meine Gruppen` noch eine Gruppe, die man uebergeben koennte.
const FIXED_GROUPS_ONLY_TOPIC_IDS: ReadonlySet<HelpTopicId> = new Set([
  HELP_TOPICS.ownGroups,
  HELP_TOPICS.transferGroup,
]);

/** So viele Themen zeigt die Startseite einer Rolle als Kacheln. */
export const HOME_TOPIC_COUNT = 6;

export function getHelpTopics(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode = "fixed_groups",
  nfcEnabled: boolean | null = true,
): readonly HelpTopic[] {
  const topics = [
    invitationTopic(),
    loginTopic(presenceMode, groupMode),
    installAppTopic(),
    appOverviewTopic(),
    ...caregiverTopics(presenceMode, groupMode, nfcEnabled),
    ...leadTopics(presenceMode, groupMode, nfcEnabled),
    ...PARENT_DRAFT_TOPICS,
    ...TEACHER_DRAFT_TOPICS,
  ];

  const hidden = new Set<HelpTopicId>();
  if (nfcEnabled === false) {
    for (const id of NFC_ONLY_TOPIC_IDS) hidden.add(id);
  }
  if (presenceMode === "binary") {
    for (const id of DETAILED_ONLY_TOPIC_IDS) hidden.add(id);
  }
  if (groupMode === "open_care") {
    for (const id of FIXED_GROUPS_ONLY_TOPIC_IDS) hidden.add(id);
  }
  if (hidden.size === 0) return topics;

  return topics.filter((topic) => !hidden.has(topic.id));
}

/**
 * Ueberschrift einer Themengruppe fuer eine konkrete Arbeitsweise.
 *
 * Die Ueberschrift darf nur nennen, was in der Gruppe auch steht. Bei
 * einfacher Anwesenheit sind Raeume und Aufsichten ausgeblendet, bei offener
 * Betreuung die Gruppen -- sonst sucht jemand in der Gruppe nach etwas, das
 * dort nie stehen wird.
 */
export function helpGroupLabel(
  group: HelpTopicGroup,
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): string {
  if (group === "gruppen") {
    const hasGroups = groupMode !== "open_care";
    const hasRooms = presenceMode !== "binary";
    if (hasGroups && !hasRooms) return "Gruppen";
    if (!hasGroups && hasRooms) return "Räume und Aufsicht";
    // Fehlt beides, ist die Gruppe leer und die Ueberschrift entfaellt
    // ohnehin -- der Rueckfall bleibt trotzdem sinnvoll.
  }
  return HELP_GROUP_LABELS[group];
}

export const HELP_GROUPS = [
  "einstieg",
  "tagesplanung",
  "kinder",
  "gruppen",
  "team",
  "arbeitszeit",
  "nfc",
  "probleme",
  "einrichten",
  "kinderdaten",
  "elternarbeit",
  "anmeldeverwaltung",
  "personal",
  "planung",
  "auswertung",
  "konfiguration",
  "mein-kind",
  "nachrichten",
  "anmeldung",
  "klasse",
  "aufsicht",
] as const satisfies readonly HelpTopicGroup[];

export const HELP_GROUP_LABELS: Readonly<Record<HelpTopicGroup, string>> = {
  einstieg: "Einstieg",
  tagesplanung: "Tagesplanung",
  kinder: "Kinder und Anwesenheit",
  gruppen: "Gruppen, Räume und Aufsicht",
  team: "Mit Eltern und dem Team arbeiten",
  arbeitszeit: "Meine Arbeitszeit",
  nfc: "NFC-Tablet und Armbänder",
  probleme: "Wenn etwas nicht klappt",
  einrichten: "moto für die OGS einrichten",
  kinderdaten: "Kinder verwalten",
  elternarbeit: "Eltern und Anfragen",
  anmeldeverwaltung: "Anmeldungen",
  personal: "Personalverwaltung",
  planung: "Betreuung und Team planen",
  auswertung: "Auswerten und exportieren",
  konfiguration: "Einstellungen",
  "mein-kind": "Mein Kind und die Betreuung",
  // Traegt bei den Eltern auch Elternbriefe, Kalender und Mittagessen.
  nachrichten: "Nachrichten und Infos",
  anmeldung: "Anmeldung",
  klasse: "Meine Klasse",
  aufsicht: "Aufsicht",
};
