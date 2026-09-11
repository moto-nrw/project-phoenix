import {
  BellRing,
  CalendarDays,
  Clock3,
  Database,
  Eye,
  FileText,
  FolderOpen,
  KeyRound,
  ListChecks,
  LogIn,
  MessageSquareText,
  Search,
  Settings,
  ShieldAlert,
  Smartphone,
  TabletSmartphone,
  UserRoundSearch,
  Users,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";
import { HELP_TOPICS, type HelpTopicId } from "~/lib/help-topics";

export type PrototypeRole = "caregiver" | "lead";
export type PrototypePresenceMode = "detailed" | "binary" | "unknown";
export type PrototypeGroupMode = "fixed_groups" | "open_care" | "unknown";
export type PrototypeTopicGroup =
  | "einstieg"
  | "tagesplanung"
  | "kinder"
  | "gruppen"
  | "team"
  | "arbeitszeit"
  | "nfc"
  | "probleme"
  | "leitung";

export interface PrototypeTopic {
  readonly id: HelpTopicId;
  readonly title: string;
  readonly question: string;
  readonly summary: string;
  readonly group: PrototypeTopicGroup;
  readonly audience: "all" | PrototypeRole;
  readonly icon: LucideIcon;
  readonly status?: "draft";
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
  readonly note?: string;
  readonly troubleshooting?: HelpTopicId;
  readonly troubleshootingDetails?: readonly string[];
  readonly image?: string;
  readonly imageAlt?: string;
  readonly related: readonly HelpTopicId[];
}

interface DraftTopic {
  readonly id: HelpTopicId;
  readonly title: string;
  readonly question: string;
  readonly summary: string;
  readonly group: PrototypeTopicGroup;
  readonly audience?: PrototypeRole;
  readonly icon: LucideIcon;
  readonly related?: readonly HelpTopicId[];
}

function draftTopic(topic: DraftTopic): PrototypeTopic {
  return {
    ...topic,
    audience: topic.audience ?? "caregiver",
    status: "draft",
    steps: [],
    related: topic.related ?? [],
  };
}

function loginResult(
  presenceMode: PrototypePresenceMode,
  groupMode: PrototypeGroupMode,
): string {
  if (presenceMode === "binary" || groupMode === "open_care") {
    return "Nach der Anmeldung öffnet moto den Bereich `Alle Kinder`.";
  }
  if (presenceMode === "detailed" && groupMode === "fixed_groups") {
    return "Nach der Anmeldung öffnet moto den Bereich `Meine Gruppen`.";
  }
  return "Nach der Anmeldung öffnet moto die für Sie vorgesehene Startseite.";
}

function invitationTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.acceptInvitation,
    title: "Einladung annehmen und Konto einrichten",
    question: "Wie nehme ich meine Einladung an?",
    summary:
      "Öffnen Sie Ihre Einladungs-Mail und legen Sie Ihr persönliches Passwort fest.",
    group: "einstieg",
    audience: "caregiver",
    icon: LogIn,
    requirements: [
      "Ihre Einladungs-Mail von moto",
      "Zugriff auf das E-Mail-Postfach, an das die Einladung gesendet wurde",
    ],
    steps: [
      "Öffnen Sie die Einladungs-Mail von moto.",
      "Wählen Sie in der E-Mail `Einladung annehmen`.",
      "Prüfen Sie, ob die angezeigte Rolle stimmt.",
      "Ergänzen Sie `Vorname` und `Nachname`, falls die Felder leer sind.",
      "Geben Sie unter `Passwort` ein persönliches Passwort ein.",
      "Geben Sie dasselbe Passwort unter `Passwort bestätigen` erneut ein.",
      "Prüfen Sie, ob alle Passwortanforderungen grün markiert sind.",
      "Wählen Sie `Einladung akzeptieren`.",
    ],
    result:
      "Ihr moto-Konto ist eingerichtet und Sie können sich jetzt anmelden. Bewahren Sie Ihre E-Mail-Adresse und Ihr Passwort sicher auf und geben Sie beides nicht weiter.",
    troubleshootingDetails: [
      "Sie finden keine Einladungs-Mail? Prüfen Sie auch Ihren Spam-Ordner. Bitten Sie sonst Ihre Leitung um eine neue Einladung.",
      "Die Einladung ist abgelaufen oder wurde schon verwendet? Bitten Sie Ihre Leitung um eine neue Einladung.",
      "Die Einladung ist nicht für Sie oder die Rolle ist falsch? Nehmen Sie sie nicht an und wenden Sie sich an Ihre Leitung.",
    ],
    related: [HELP_TOPICS.login, HELP_TOPICS.installApp],
  };
}

function loginTopic(
  presenceMode: PrototypePresenceMode,
  groupMode: PrototypeGroupMode,
): PrototypeTopic {
  return {
    id: HELP_TOPICS.login,
    title: "Bei moto anmelden",
    question: "Wie melde ich mich bei moto an?",
    summary:
      "Melden Sie sich über die moto-Seite Ihrer OGS mit Ihrem persönlichen Konto an.",
    group: "einstieg",
    audience: "caregiver",
    icon: LogIn,
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
    note: "Nutzen mehrere Personen das Gerät? Melden Sie sich bei moto ab, wenn Sie fertig sind.",
    troubleshooting: HELP_TOPICS.loginProblem,
    related: [
      HELP_TOPICS.acceptInvitation,
      HELP_TOPICS.installApp,
      HELP_TOPICS.loginProblem,
    ],
  };
}

function installAppTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.installApp,
    title: "moto als App auf dem Handy oder Tablet hinzufügen",
    question: "Wie füge ich moto auf meinem Handy oder Tablet hinzu?",
    summary:
      "Fügen Sie moto zum Startbildschirm hinzu. Sie brauchen dafür keinen App Store.",
    group: "einstieg",
    audience: "caregiver",
    icon: Smartphone,
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
    related: [HELP_TOPICS.login, HELP_TOPICS.mySchedule],
  };
}

function appOverviewTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.appOverview,
    title: "Sich in moto zurechtfinden",
    question: "Wie ist moto aufgebaut?",
    summary:
      "Über die Navigation öffnen Sie die Bereiche, die Sie für Ihren Arbeitstag brauchen.",
    group: "einstieg",
    audience: "caregiver",
    icon: ListChecks,
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
          "Sehen Sie neben einer Überschrift ein Fragezeichen? Wählen Sie es, um die passende Anleitung zu öffnen.",
          "Oder öffnen Sie unter `Mehr` den Bereich `Hilfe`.",
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
  presenceMode: PrototypePresenceMode,
): PrototypeTopic {
  const currentStatusStep =
    presenceMode === "detailed"
      ? "Prüfen Sie oben den Aufenthaltsort sowie die heutige Ankunft und Abholung."
      : presenceMode === "binary"
        ? "Prüfen Sie oben die Anwesenheit sowie die heutige Ankunft und Abholung."
        : "Prüfen Sie oben den heutigen Status sowie die Ankunft und Abholung.";

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
    audience: "caregiver",
    icon: Search,
    steps: [],
    instructionGroups: [
      {
        title: "Ein Kind finden",
        steps: [
          "Öffnen Sie `Alle Kinder`.",
          "Geben Sie in `Name suchen...` den Namen ein.",
          "Sie können auch nur einen Namensteil eingeben.",
          "Zu viele Treffer? Wählen Sie `Filter`.",
          "Wählen Sie zum Beispiel eine Klasse oder Gruppe.",
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
          "Wählen Sie `Anmeldungen` für Angaben aus der Halbjahresanmeldung.",
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
      "Einige Bereiche brauchen zusätzliche Rechte: `Betreuungsplan`, `Anmeldungen`, `Dokumente` und `Änderungsprotokoll`. Fehlt ein Bereich? Fragen Sie Ihre Leitung.",
      ...(presenceDifference ? [presenceDifference] : []),
    ],
    troubleshooting: HELP_TOPICS.missingChildOrGroup,
    related: [
      HELP_TOPICS.editStudent,
      HELP_TOPICS.webAttendance,
      HELP_TOPICS.absences,
    ],
  };
}

function editStudentTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.editStudent,
    title: "Angaben oder Betreuungszeiten eines Kindes ändern",
    question: "Wie ändere ich Angaben oder Betreuungszeiten eines Kindes?",
    summary:
      "Ändern Sie persönliche Angaben, den Wochenplan oder einen einzelnen Tag.",
    group: "kinder",
    audience: "caregiver",
    icon: FileText,
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
        steps: [
          "Öffnen Sie beim Kind den Bereich `Betreuungszeiten`.",
          "Wählen Sie `Wochenplan`.",
          "Wählen Sie die regelmäßigen Betreuungstage.",
          "Tragen Sie Ankunfts- und Abholzeiten ein.",
          "Ergänzen Sie bei Bedarf eine Notiz.",
          "Wählen Sie `Wochenplan speichern`.",
        ],
      },
      {
        title: "Nur einen Tag ändern",
        steps: [
          "Öffnen Sie in `Betreuungszeiten` den gewünschten Tag.",
          "Wählen Sie beim Tag `Ausnahme`.",
          "Ändern Sie `Ankunft` oder `Abholung`.",
          "Tragen Sie bei Bedarf einen Grund ein.",
          "Wählen Sie `Speichern`.",
        ],
      },
    ],
    result:
      "Die gespeicherten Angaben gelten sofort. Eine Ausnahme ändert den regelmäßigen Wochenplan nicht.",
    differences: [
      "Die Kennzeichnung `Für Eltern sichtbar` zeigt Angaben, die Eltern sehen.",
      "Kommen die Betreuungstage aus Buchungen? Dann lassen sich die Tage nicht auswählen. Ändern Sie nur Zeiten an gebuchten Tagen.",
      "Fehlt `Bearbeiten` oder `Wochenplan`? Fragen Sie Ihre Leitung nach den nötigen Rechten.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.carePlan,
      HELP_TOPICS.dayLog,
    ],
  };
}

function webAttendanceTopic(
  presenceMode: PrototypePresenceMode,
): PrototypeTopic {
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
    icon: ListChecks,
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
    differences: [
      "moto zeigt nur die Aktion, die zum aktuellen Status passt.",
      "Möchten Sie mehrere Kinder ändern? Wählen Sie im Sammelmodus `Mehrere`.",
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

function changeLocationTopic(
  presenceMode: PrototypePresenceMode,
): PrototypeTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.changeLocation,
      title: "Den Aufenthaltsort eines Kindes ändern",
      question: "Wie ändere ich den Aufenthaltsort eines Kindes?",
      summary:
        "Bei einfacher Anwesenheit erfasst moto keine Räume oder Aufenthaltsorte.",
      group: "kinder",
      audience: "caregiver",
      icon: Eye,
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
      icon: Eye,
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
    icon: Eye,
    requirements: [
      "Ihre OGS erfasst die Räume der Kinder.",
      "Das Kind ist anwesend.",
      "Sie beaufsichtigen den aktuellen Raum oder den Zielraum.",
      "Im Zielraum läuft eine Aufsicht.",
    ],
    steps: [
      "Öffnen Sie `Räume`.",
      "Öffnen Sie den aktuellen Raum des Kindes.",
      "Wählen Sie das Kind unter `Kinder im Raum` aus.",
      "Wählen Sie unter `Zielraum` den neuen Raum.",
      "Wählen Sie `In Raum setzen`.",
    ],
    result: "moto zeigt sofort den neuen Raum. Der bisherige Raumbesuch endet.",
    differences: [
      "Steht das Kind unter `Kinder unterwegs`? Öffnen Sie `Kinder unterwegs`. Wählen Sie dann das Kind und den Zielraum.",
      "Sie können mehrere Kinder auswählen und gemeinsam verschieben.",
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

function absencesTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.absences,
    title: "Ein Kind krank oder entschuldigt melden",
    question: "Wie melde ich ein Kind krank oder entschuldigt?",
    summary:
      "Tragen Sie eine Krankmeldung oder Entschuldigung für die passenden Tage ein.",
    group: "kinder",
    audience: "caregiver",
    icon: BellRing,
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
        title: "Eine Meldung entfernen",
        steps: [
          "Suchen Sie das Kind unter `Alle Kinder`.",
          "Öffnen Sie die Karte des Kindes.",
          "Wählen Sie `Krank melden` oder `Entschuldigen`.",
          "Wählen Sie beim eingetragenen Tag `Entfernen`.",
        ],
      },
    ],
    result:
      "Die Krankmeldung oder Entschuldigung ist gespeichert. Das Kind erscheint am gewählten Tag mit dem passenden Status.",
    differences: [
      "Fehlen `Krank melden` und `Entschuldigen`? Fragen Sie Ihre Leitung, ob Ihre Rolle Abwesenheiten bearbeiten darf.",
    ],
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.dayLog,
      HELP_TOPICS.webAttendance,
    ],
  };
}

function dayLogTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.dayLog,
    title: "Den heutigen Betreuungstag prüfen",
    question: "Wie prüfe ich den heutigen Betreuungstag?",
    summary: "Prüfen Sie für heute, welche Kinder anwesend oder abwesend sind.",
    group: "kinder",
    audience: "caregiver",
    icon: CalendarDays,
    requirements: ["Ihre OGS hat das `Anwesenheitsprotokoll` eingeschaltet."],
    steps: [],
    instructionGroups: [
      {
        title: "Den Betreuungstag prüfen",
        steps: [
          "Öffnen Sie `Tagesauswertung` in der Seitenleiste.",
          "Prüfen Sie oben die Zahlen für alle Gruppen.",
          "Wählen Sie bei einer Gruppe `Details`.",
          "Prüfen Sie die Kinder in den einzelnen Bereichen.",
          "Wählen Sie ein Kind, um seine Angaben zu öffnen.",
        ],
      },
      {
        title: "Die Auswertung ausgeben",
        steps: [
          "Bleiben Sie für alle Gruppen in der Übersicht.",
          "Für eine einzelne Gruppe öffnen Sie deren `Details`.",
          "Wählen Sie `Drucken`, `PDF` oder `Excel`.",
        ],
      },
    ],
    result:
      "Sie sehen die Anwesenheit für heute, geordnet nach dem Status der Kinder. Kinder ohne Meldung stehen unter `Unentschuldigt abwesend`. Vergangene Tage können Sie hier nicht auswählen.",
    differences: [
      "Fehlt `Tagesauswertung`? Bitten Sie Ihre Leitung, das `Anwesenheitsprotokoll` unter `Einstellungen` und `Datenschutz` einzuschalten.",
    ],
    related: [
      HELP_TOPICS.absences,
      HELP_TOPICS.emergency,
      HELP_TOPICS.ownGroups,
    ],
  };
}

function emergencyTopic(presenceMode: PrototypePresenceMode): PrototypeTopic {
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
    audience: "caregiver",
    icon: ShieldAlert,
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
      "Je nach Einstellung enthält die Liste auch `Gesundheit / Allergien`.",
      "`Nicht hinterlegt` bedeutet: Es fehlen Gesundheitsinfos, eine Allergie ist trotzdem möglich.",
    ].join(" "),
    note: "Erstellen Sie im Notfall eine neue Liste. Ein älterer Ausdruck kann veraltet sein. Lassen Sie den Ausdruck nicht offen liegen.",
    related: [
      HELP_TOPICS.studentSearch,
      HELP_TOPICS.dayLog,
      HELP_TOPICS.ownGroups,
    ],
  };
}

function ownGroupsTopic(
  presenceMode: PrototypePresenceMode,
  groupMode: PrototypeGroupMode,
): PrototypeTopic {
  if (groupMode === "open_care") {
    return {
      id: HELP_TOPICS.ownGroups,
      title: "Meine Gruppen ansehen",
      question: "Wie sehe ich meine Gruppen an?",
      summary:
        "Bei offener Betreuung gibt es keine festen eigenen Gruppen in moto.",
      group: "gruppen",
      audience: "caregiver",
      icon: Users,
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
      audience: "caregiver",
      icon: Users,
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
    audience: "caregiver",
    icon: Users,
    steps: [
      "Öffnen Sie `Meine Gruppen` in der Seitenleiste.",
      "Wählen Sie die gewünschte Gruppe.",
      "Prüfen Sie die Zahlen für `krank` und `entschuldigt`.",
      "Suchen Sie bei Bedarf über `Name suchen...` nach einem Kind.",
      "Wählen Sie `Filter`, um die Liste zu sortieren oder einzugrenzen.",
      "Wählen Sie die Karte eines Kindes, um seine Angaben zu öffnen.",
    ],
    result: statusResult,
    differences: [
      "Sehen Sie nur eine Gruppe? Dann heißt der Bereich `Meine Gruppe`.",
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

function transferGroupTopic(groupMode: PrototypeGroupMode): PrototypeTopic {
  if (groupMode === "open_care") {
    return {
      id: HELP_TOPICS.transferGroup,
      title: "Meine Gruppe vorübergehend übergeben",
      question: "Wie übergebe ich meine Gruppe vorübergehend?",
      summary:
        "Bei offener Betreuung gibt es keine feste eigene Gruppe zum Übergeben.",
      group: "gruppen",
      audience: "caregiver",
      icon: Users,
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
      audience: "caregiver",
      icon: Users,
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
    audience: "caregiver",
    icon: Users,
    requirements: ["Sie sind dieser Gruppe fest zugeordnet."],
    steps: [],
    instructionGroups: [
      {
        title: "Gruppe übergeben",
        steps: [
          "Öffnen Sie Ihre Gruppe unter `Meine Gruppen`.",
          "Öffnen Sie oben rechts das Menü mit den drei Punkten.",
          "Wählen Sie `Gruppe übergeben`.",
          "Wählen Sie unter `Übergeben an` die andere Betreuungskraft aus.",
          "Wählen Sie `Übergeben`.",
        ],
      },
      {
        title: "Eine Übergabe früher beenden",
        description:
          "Eine Übergabe endet von selbst um 23:59 Uhr. Sie können sie auch vorher beenden.",
        steps: [
          "Öffnen Sie das Menü mit den drei Punkten.",
          "Wählen Sie `Gruppe übergeben`.",
          "Wählen Sie unter `Aktuell übergeben an` neben der Person `Entfernen`.",
        ],
      },
    ],
    result:
      "Die andere Betreuungskraft sieht die Gruppe bis zum Ende des Tages unter `Meine Gruppen`. Sie behalten selbst den Zugriff.",
    note: "Sie können die Gruppe an mehrere Betreuungskräfte übergeben. Wiederholen Sie dafür die Schritte. Alle Übergaben stehen im Fenster unter `Aktuell übergeben an`.",
    troubleshootingDetails: [
      "Fehlt der Eintrag `Gruppe übergeben` im Menü? Dann betreuen Sie die Gruppe nur als Vertretung. Eine Vertretung kann die Gruppe nicht weitergeben.",
      "Fehlt die gesuchte Person in der Liste? Bitten Sie Ihre Leitung, die Person anzulegen.",
    ],
    related: [HELP_TOPICS.ownGroups, HELP_TOPICS.missingChildOrGroup],
  };
}

function myScheduleTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.mySchedule,
    title: "Meine Termine und Einsätze ansehen",
    question: "Wo sehe ich meine Termine und Einsätze?",
    summary:
      "Prüfen Sie persönliche Termine, geplante Schichten und Ihre Einsätze im Betreuungsplan.",
    group: "tagesplanung",
    audience: "caregiver",
    icon: CalendarDays,
    steps: [],
    instructionGroups: [
      {
        title: "Termine und Einsätze ansehen",
        steps: [
          "Öffnen Sie `Mein Kalender` in der Seitenleiste.",
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
    differences: [
      "Mit `Sa/So` können Sie das Wochenende einblenden. Eine Zahl zeigt, dass dort Einträge liegen.",
      "Bei einer Einladung können Sie im geöffneten Termin `Zusagen` oder `Absagen` wählen.",
      "Das Kalender-Abo ist nur zum Lesen. Änderungen in Ihrem persönlichen Kalender werden nicht an moto übertragen.",
    ],
    related: [HELP_TOPICS.carePlan, HELP_TOPICS.trackWorkTime],
  };
}

function carePlanTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.carePlan,
    title: "Den Betreuungsplan ansehen",
    question: "Wie sehe ich den Betreuungsplan an?",
    summary:
      "Prüfen Sie, welche Betreuung für die Schule geplant ist und wo Sie selbst eingesetzt sind.",
    group: "tagesplanung",
    audience: "caregiver",
    icon: CalendarDays,
    steps: [
      "Öffnen Sie `Mein Kalender`.",
      "Wählen Sie den Tab `Betreuungsplan`.",
      "Wählen Sie oben `Tag` oder `Woche`.",
      "Nutzen Sie die Pfeile oder `Heute`, um zum passenden Tag zu wechseln.",
      "Wählen Sie einen Block, um Zeit, Raum, Betreuungsteam und Kinder zu sehen.",
    ],
    result:
      "Sie sehen, was die OGS geplant hat und wo Sie eingesetzt sind. Als Betreuungskraft können Sie den Plan nicht verändern.",
    troubleshootingDetails: [
      "Der Tab `Betreuungsplan` fehlt? Bitten Sie Ihre Leitung, Ihren Zugang zu prüfen.",
    ],
    related: [HELP_TOPICS.mySchedule, HELP_TOPICS.activeSupervision],
  };
}

function roomsTopic(presenceMode: PrototypePresenceMode): PrototypeTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.rooms,
      title: "Räume und ihre aktuelle Belegung ansehen",
      question: "Wo sehe ich Räume und ihre aktuelle Belegung?",
      summary: "Bei einfacher Anwesenheit ordnet moto Kinder keinen Räumen zu.",
      group: "gruppen",
      audience: "caregiver",
      icon: Eye,
      steps: [
        "Öffnen Sie `Alle Kinder`.",
        "Prüfen Sie dort, welche Kinder anwesend oder abwesend sind.",
      ],
      result:
        "Der Bereich `Räume` wird nicht angezeigt. moto speichert in diesem Modus keinen Aufenthaltsort.",
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
      audience: "caregiver",
      icon: Eye,
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
    audience: "caregiver",
    icon: Eye,
    steps: [],
    instructionGroups: [
      {
        title: "Einen Raum und seine Belegung ansehen",
        steps: [
          "Öffnen Sie `Räume` in der Seitenleiste.",
          "Suchen Sie den Raum über `Raum suchen...`.",
          "Oder wählen Sie `Filter` und unter `Status` die Angabe `Belegt` oder `Frei`.",
          "Wählen Sie einen Raum aus.",
          "Prüfen Sie die aktuelle Aktivität, die Aufsicht und die Kinder im Raum.",
          "Wählen Sie bei Bedarf ein Kind, um seine Angaben zu öffnen.",
        ],
      },
      {
        title: "Kinder ohne Raum in einen Raum setzen",
        description:
          "Sie können Kinder nur in Räume setzen, in denen Sie selbst die Aufsicht führen. Die Leitung kann alle Räume auswählen.",
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

function activeSupervisionTopic(
  presenceMode: PrototypePresenceMode,
): PrototypeTopic {
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.activeSupervision,
      title: "Eine Aufsicht starten, führen und beenden",
      question: "Wie arbeite ich mit einer laufenden Aufsicht?",
      summary:
        "Bei einfacher Anwesenheit führt moto keine Aufsichten für einzelne Räume oder Aktivitäten.",
      group: "gruppen",
      audience: "caregiver",
      icon: Eye,
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
      audience: "caregiver",
      icon: Eye,
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
    audience: "caregiver",
    icon: Eye,
    steps: [],
    instructionGroups: [
      {
        title: "Eine Aufsicht starten",
        steps: [
          "Öffnen Sie `Aktuelle Aufsicht`.",
          "Wählen Sie unter `Als Nächstes` einen geplanten Termin.",
          "Starten Sie ihn, sobald die Schaltfläche freigegeben ist.",
          "Oder wählen Sie bei einem freien Raum `Beaufsichtigen`.",
          "Prüfen Sie Raum, Aktivität und Betreuungsteam.",
        ],
      },
      {
        title: "Die Kinderliste führen",
        steps: [
          "Suchen Sie ein Kind in der laufenden Aufsicht.",
          "Wählen Sie `Hinzufügen`, wenn das Kind anwesend ist und zur Aufsicht kommen soll.",
          "Prüfen Sie geplante Abholzeiten und Hinweise in der Liste.",
          "Nutzen Sie den passenden Status, wenn ein Kind den Raum wechselt oder nach Hause geht.",
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
      "Welche freien Räume Sie übernehmen können, legt Ihre OGS fest.",
    ],
    troubleshooting: HELP_TOPICS.attendanceProblem,
    related: [HELP_TOPICS.rooms, HELP_TOPICS.ownGroups],
  };
}

function manageActivityTopic(
  presenceMode: PrototypePresenceMode,
  nfcEnabled: boolean | null,
): PrototypeTopic {
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
      icon: ListChecks,
      steps: [
        "Nutzen Sie den `Betreuungsplan`, um geplante Angebote anzusehen.",
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
      icon: ListChecks,
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
    icon: ListChecks,
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

function parentMessageTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.parentMessage,
    title: "Eltern eine Nachricht schreiben",
    question: "Wie schreibe ich Eltern eine Nachricht?",
    summary:
      "Starten Sie eine Unterhaltung mit einer Bezugsperson oder antworten Sie auf eine Nachricht.",
    group: "team",
    audience: "caregiver",
    icon: MessageSquareText,
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
          "Schreiben Sie Ihre Antwort in `Nachricht an die Eltern schreiben...`.",
          "Wählen Sie `Senden`.",
        ],
      },
    ],
    result:
      "Die Bezugsperson sieht die Nachricht in der Eltern-App. Sie wird dort im Namen der OGS angezeigt.",
    differences: [
      "Mit `Nur ungelesen` sehen Sie nur neue Unterhaltungen.",
      "Über `Zum Kinderprofil` wechseln Sie direkt zu den Angaben des Kindes.",
      "Sehen Sie `Nachrichten` nicht? Vielleicht ist die Funktion ausgeschaltet.",
      "Möglicherweise fehlt Ihnen auch der Zugriff auf die betroffenen Kinder.",
    ],
    note: "Schreiben Sie persönliche Angaben nur in die Unterhaltung der richtigen Bezugsperson.",
    related: [HELP_TOPICS.parentRequests, HELP_TOPICS.studentSearch],
  };
}

function parentRequestsTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.parentRequests,
    title: "Anfragen von Eltern prüfen und bearbeiten",
    question: "Wie prüfe und bearbeite ich Anfragen von Eltern?",
    summary:
      "Vergleichen Sie die bisherigen Angaben mit dem Elternwunsch. Tragen Sie danach Ihre Entscheidung ein.",
    group: "team",
    audience: "caregiver",
    icon: ListChecks,
    requirements: [
      "Ihr Konto darf Kinderdaten bearbeiten.",
      "Sie haben Zugriff auf die Gruppe des Kindes.",
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
      "Über `Historie` sehen Sie bereits entschiedene oder zurückgezogene Anfragen.",
      "Fehlt eine Anfrage? Sie sehen nur Kinder, auf die Sie Zugriff haben.",
    ],
    related: [HELP_TOPICS.parentMessage, HELP_TOPICS.editStudent],
  };
}

function teamChatTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.teamChat,
    title: "Den Team-Chat nutzen",
    question: "Wie nutze ich den Team-Chat?",
    summary:
      "Schreiben Sie einer Betreuungskraft oder Lehrkraft Ihrer Schule eine Nachricht.",
    group: "team",
    audience: "caregiver",
    icon: MessageSquareText,
    steps: [
      "Öffnen Sie `Team-Chat` in der Seitenleiste.",
      "Wählen Sie `Neue Nachricht`.",
      "Suchen Sie die Person und wählen Sie sie aus.",
      "Schreiben Sie Ihre Nachricht.",
      "Wählen Sie `Senden`.",
    ],
    result:
      "Die Nachricht erscheint in Ihrer Unterhaltung mit dieser Person. Lehrkräfte lesen sie in moto schule.",
    differences: [
      "Mit `Nur ungelesen` sehen Sie nur Unterhaltungen mit neuen Nachrichten.",
      "Nachrichten im Team-Chat können nicht nachträglich geändert oder gelöscht werden.",
      "Der Bereich fehlt? Dann hat Ihre OGS den Team-Chat nicht eingeschaltet.",
    ],
    note: "Eltern können den Team-Chat nicht sehen. Für Eltern nutzen Sie `Nachrichten` im Bereich `Eltern`.",
    related: [HELP_TOPICS.findStaff, HELP_TOPICS.parentMessage],
  };
}

function findStaffTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.findStaff,
    title: "Eine Person aus dem Team finden",
    question: "Wie finde ich eine Person aus dem Team?",
    summary:
      "Suchen Sie eine Person und prüfen Sie ihren aktuellen Arbeitsstatus.",
    group: "team",
    audience: "caregiver",
    icon: UserRoundSearch,
    steps: [
      "Öffnen Sie `Mitarbeiter` in der Seitenleiste.",
      "Geben Sie den Namen in das Suchfeld ein.",
      "Wählen Sie bei Bedarf `Filter`.",
      "Filtern Sie zum Beispiel nach `Anwesend`, `Homeoffice` oder `Krank/Urlaub`.",
      "Prüfen Sie den Status auf der Karte der Person.",
    ],
    result:
      "Sie sehen den Arbeitsstatus der Person. Manchmal steht dort auch ihre Aufsicht.",
    differences: [
      "Persönliche Personalunterlagen und Arbeitszeiten sind besonders geschützt. Ohne zusätzliches Recht sehen Sie diese Angaben nicht.",
      "Möchten Sie der Person schreiben? Öffnen Sie den `Team-Chat`.",
    ],
    related: [HELP_TOPICS.teamChat, HELP_TOPICS.activeSupervision],
  };
}

function sharedFilesTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.sharedFiles,
    title: "Eine gemeinsame Datei öffnen oder hochladen",
    question: "Wie öffne oder teile ich eine gemeinsame Datei?",
    summary: "Nutzen Sie die gemeinsame Dateiablage Ihrer OGS.",
    group: "team",
    audience: "caregiver",
    icon: FolderOpen,
    steps: [],
    instructionGroups: [
      {
        title: "Eine Datei öffnen",
        steps: [
          "Öffnen Sie `Dateien` in der Seitenleiste.",
          "Wählen Sie den passenden Ordner.",
          "Suchen Sie die Datei.",
          "Wählen Sie `Im Browser öffnen` oder `Herunterladen`.",
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
      "Legen Sie keine zweite Datei mit fast gleichem Namen an. Prüfen Sie zuerst, ob bereits eine aktuelle Fassung vorhanden ist.",
    ],
    note: "Speichern Sie persönliche Angaben nur in einem dafür vorgesehenen und geschützten Ordner.",
    related: [HELP_TOPICS.missingMenu],
  };
}

function trackWorkTimeTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.trackWorkTime,
    title: "Arbeitszeit und Pausen erfassen",
    question: "Wie erfasse ich meine Arbeitszeit und Pausen?",
    summary:
      "Stempeln Sie sich zu Arbeitsbeginn ein. Erfassen Sie Pausen und das Arbeitsende.",
    group: "arbeitszeit",
    audience: "caregiver",
    icon: Clock3,
    steps: [],
    instructionGroups: [
      {
        title: "Arbeitszeit starten",
        steps: [
          "Öffnen Sie `Zeiterfassung`.",
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
    differences: [
      "Eine geplante Schicht steht oberhalb der Stempeluhr. Sie startet die Zeiterfassung nicht automatisch.",
      "Bei einer deutlichen Abweichung von Ihrer geplanten Schicht kann moto nach einem Grund fragen.",
      "Nach der gewählten Pausenlänge läuft die Arbeitszeit automatisch weiter.",
    ],
    related: [HELP_TOPICS.correctWorkTime, HELP_TOPICS.nfcWorkTime],
  };
}

function correctWorkTimeTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.correctWorkTime,
    title: "Eigene Arbeitszeit prüfen und korrigieren",
    question: "Wie prüfe oder korrigiere ich meine Arbeitszeit?",
    summary:
      "Prüfen Sie Ihre erfassten Zeiten und berichtigen Sie einen vorhandenen Eintrag.",
    group: "arbeitszeit",
    audience: "caregiver",
    icon: Clock3,
    steps: [
      "Öffnen Sie `Zeiterfassung`.",
      "Wechseln Sie in der Tabelle zwischen `Woche` und `Monat`.",
      "Suchen Sie den betroffenen Tag.",
      "Wählen Sie beim Tag das Stift-Symbol.",
      "Ändern Sie Beginn, Ende oder Pause.",
      "Tragen Sie einen Grund für die Änderung ein.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die korrigierte Zeit und der neue Saldo werden angezeigt. Die Änderung bleibt in der Historie nachvollziehbar.",
    differences: [
      "Einen vollständig fehlenden Arbeitstag kann nur Ihre Leitung nachtragen.",
      "Ein abgeschlossener Monat kann nachträgliche Änderungen anzeigen, sein übertragener Saldo bleibt jedoch festgeschrieben.",
      "Klappen Sie einen geänderten Tag auf, um die Änderungshistorie zu sehen.",
    ],
    related: [HELP_TOPICS.trackWorkTime, HELP_TOPICS.vacation],
  };
}

function vacationTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.vacation,
    title: "Urlaub beantragen und den Stand prüfen",
    question: "Wie beantrage ich Urlaub und prüfe den Stand?",
    summary:
      "Senden Sie einen Urlaubsantrag und verfolgen Sie die Entscheidung Ihrer Leitung.",
    group: "arbeitszeit",
    audience: "caregiver",
    icon: CalendarDays,
    steps: [
      "Öffnen Sie `Zeiterfassung`.",
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
      "Einen offenen oder zukünftigen genehmigten Antrag können Sie stornieren.",
      "Bei einer `Rückfrage` schreiben Sie eine Antwort und wählen `Antwort senden & erneut einreichen`.",
      "Krankheit, Fortbildung und andere Abwesenheiten melden Sie über `Abwesend` in der Stempeluhr. Dafür stellen Sie keinen Urlaubsantrag.",
    ],
    related: [HELP_TOPICS.ownAbsence, HELP_TOPICS.correctWorkTime],
  };
}

function ownAbsenceTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.ownAbsence,
    title: "Eine eigene Abwesenheit eintragen",
    question: "Wie trage ich eine eigene Abwesenheit ein?",
    summary:
      "Melden Sie Krankheit, Fortbildung oder eine andere Abwesenheit für den passenden Zeitraum.",
    group: "arbeitszeit",
    audience: "caregiver",
    icon: BellRing,
    steps: [
      "Öffnen Sie `Zeiterfassung`.",
      "Wählen Sie in der `Stempeluhr` den Status `Abwesend`.",
      "Wählen Sie `Abwesenheit melden`.",
      "Wählen Sie unter `Art der Abwesenheit` den passenden Eintrag.",
      "Tragen Sie `Von` und `Bis` ein.",
      "Wählen Sie bei Bedarf `Halber Tag` und ergänzen Sie eine Notiz.",
      "Wählen Sie `Speichern`.",
    ],
    result:
      "Die Abwesenheit ist eingetragen und erscheint in Ihrer Zeiterfassung.",
    differences: [
      "Ihre OGS kann zusätzliche Abwesenheitsarten anbieten.",
      "Urlaub beantragen Sie in der Karte `Urlaub` und nicht über diese Aktion.",
      "Öffnen Sie eine eigene Abwesenheit über die Tageszeile. Dort können Sie mögliche Änderungen prüfen.",
      "Freizeitausgleich trägt ausschließlich die Leitung ein.",
    ],
    related: [HELP_TOPICS.vacation, HELP_TOPICS.trackWorkTime],
  };
}

function unavailableNfcTopic(
  id: HelpTopicId,
  title: string,
  question: string,
  related: readonly HelpTopicId[],
  group: PrototypeTopicGroup = "nfc",
): PrototypeTopic {
  return {
    id,
    title,
    question,
    summary:
      "Dieser Ablauf ist nicht verfügbar, weil Ihre OGS NFC nicht nutzt.",
    group,
    audience: "caregiver",
    icon: TabletSmartphone,
    steps: [
      "Nutzen Sie die entsprechenden Funktionen in der moto-App.",
      "Fragen Sie Ihre Leitung, wenn Sie ein NFC-Tablet erwartet haben.",
    ],
    result: "In Ihrer OGS werden Anwesenheit und Arbeitszeit ohne NFC erfasst.",
    related,
  };
}

function unknownNfcTopic(
  id: HelpTopicId,
  title: string,
  question: string,
  related: readonly HelpTopicId[],
  group: PrototypeTopicGroup = "nfc",
): PrototypeTopic {
  return {
    id,
    title,
    question,
    summary: "Ob dieser Ablauf verfügbar ist, hängt von Ihrer OGS ab.",
    group,
    audience: "caregiver",
    icon: TabletSmartphone,
    steps: [
      "Prüfen Sie, ob Ihre OGS ein NFC-Tablet und Armbänder nutzt.",
      "Ist kein NFC-Tablet vorhanden? Nutzen Sie die entsprechenden Funktionen in der moto-App.",
      "Fragen Sie Ihre Leitung, wenn Sie nicht sicher sind.",
    ],
    result:
      "Die vollständigen Schritte gelten nur für eine OGS mit eingeschalteter NFC-Nutzung.",
    related,
  };
}

function tabletLoginTopic(nfcEnabled: boolean | null): PrototypeTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.tabletLogin,
      "Mit der PIN am Tablet anmelden",
      "Wie melde ich mich mit meiner PIN am Tablet an?",
      [HELP_TOPICS.login, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.tabletLogin,
      "Mit der PIN am Tablet anmelden",
      "Wie melde ich mich mit meiner PIN am Tablet an?",
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
    audience: "caregiver",
    icon: KeyRound,
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

function tagAssignmentTopic(nfcEnabled: boolean | null): PrototypeTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.tagAssignment,
      "Ein Armband zuweisen oder ändern",
      "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
      [HELP_TOPICS.studentSearch, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.tagAssignment,
      "Ein Armband zuweisen oder ändern",
      "Wie weise ich ein Armband zu oder ändere die Zuweisung?",
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
    audience: "caregiver",
    icon: TabletSmartphone,
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

function nfcWorkTimeTopic(nfcEnabled: boolean | null): PrototypeTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.nfcWorkTime,
      "Die eigene Arbeitszeit mit dem Armband erfassen",
      "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
      [HELP_TOPICS.trackWorkTime, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcWorkTime,
      "Die eigene Arbeitszeit mit dem Armband erfassen",
      "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
      [HELP_TOPICS.trackWorkTime, HELP_TOPICS.nfcProblem],
    );
  }

  return {
    id: HELP_TOPICS.nfcWorkTime,
    title: "Die eigene Arbeitszeit mit dem Armband erfassen",
    question: "Wie erfasse ich meine Arbeitszeit mit dem Armband?",
    summary: "Stempeln Sie sich am NFC-Tablet ein, aus oder in eine Pause.",
    group: "nfc",
    audience: "caregiver",
    icon: Clock3,
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
  presenceMode: PrototypePresenceMode,
  nfcEnabled: boolean | null,
): PrototypeTopic {
  if (nfcEnabled === null || presenceMode === "unknown") {
    return unknownNfcTopic(
      HELP_TOPICS.nfcSupervision,
      "Eine Aufsicht am Tablet starten und beenden",
      "Wie starte oder beende ich eine Aufsicht am Tablet?",
      [HELP_TOPICS.activeSupervision, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcSupervision,
      "Eine Aufsicht am Tablet starten und beenden",
      "Wie starte oder beende ich eine Aufsicht am Tablet?",
      [HELP_TOPICS.activeSupervision, HELP_TOPICS.nfcProblem],
    );
  }
  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.nfcSupervision,
      title: "Eine Aufsicht am Tablet starten und beenden",
      question: "Wie starte oder beende ich eine Aufsicht am Tablet?",
      summary:
        "Das NFC-Tablet unterstützt einfache Anwesenheit noch nicht zuverlässig.",
      group: "nfc",
      audience: "caregiver",
      icon: TabletSmartphone,
      steps: [
        "Starten Sie am Tablet keine Aufsicht mit Raum oder Aktivität.",
        "Öffnen Sie in der moto-App den Bereich `Alle Kinder`.",
        "Erfassen Sie die Anwesenheit dort über `An- & Abmelden`.",
        "Informieren Sie Ihre Leitung, wenn das Tablet trotzdem eine Raumauswahl zeigt.",
      ],
      result:
        "Die Anwesenheit ist in der moto-App erfasst. Eine Raumaufsicht wird nicht angelegt.",
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
    audience: "caregiver",
    icon: TabletSmartphone,
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
    differences: [
      "Mit `Letzte Aufsicht wiederholen` übernehmen Sie Aktivität, Team und Raum der vorherigen Aufsicht.",
      "Unter `Team anpassen` können Sie während der Aufsicht Betreuungskräfte hinzufügen oder entfernen.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.nfcCheckIn, HELP_TOPICS.activeSupervision],
  };
}

function nfcCheckInTopic(
  presenceMode: PrototypePresenceMode,
  nfcEnabled: boolean | null,
): PrototypeTopic {
  if (nfcEnabled === null || presenceMode === "unknown") {
    return unknownNfcTopic(
      HELP_TOPICS.nfcCheckIn,
      "Kinder mit dem Armband ein- und auschecken",
      "Wie checken sich Kinder mit dem Armband ein und aus?",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.nfcProblem],
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcCheckIn,
      "Kinder mit dem Armband ein- und auschecken",
      "Wie checken sich Kinder mit dem Armband ein und aus?",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.nfcProblem],
    );
  }

  if (presenceMode === "binary") {
    return {
      id: HELP_TOPICS.nfcCheckIn,
      title: "Kinder mit dem Armband ein- und auschecken",
      question: "Wie checken sich Kinder mit dem Armband ein und aus?",
      summary:
        "Das NFC-Tablet unterstützt einfache Anwesenheit noch nicht zuverlässig.",
      group: "nfc",
      audience: "caregiver",
      icon: TabletSmartphone,
      steps: [
        "Öffnen Sie in der moto-App den Bereich `Alle Kinder`.",
        "Wählen Sie `An- & Abmelden`.",
        "Wählen Sie die Karte des Kindes.",
        "Prüfen Sie den neuen Status und wählen Sie danach `Fertig`.",
        "Informieren Sie Ihre Leitung, wenn das Tablet eine Raum- oder Aktivitätsauswahl zeigt.",
      ],
      result:
        "moto zeigt `Anwesend` oder `Abwesend`. Räume werden nicht erfasst.",
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
    audience: "caregiver",
    icon: TabletSmartphone,
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
          "Wählen Sie `Raumwechsel` für einen anderen betreuten Raum.",
          "Wählen Sie `Nach Hause`, wenn das Kind die OGS verlässt.",
          "Je nach Einstellung können auch `Schulhof` oder `Toilette` erscheinen.",
        ],
      },
    ],
    result:
      "moto aktualisiert Anwesenheit und Aufenthaltsort. Nach `Nach Hause` ist das Kind abgemeldet.",
    differences: [
      "Welche Ziele angezeigt werden, legt Ihre OGS fest.",
      "Eine tägliche Abmeldezeit kann verhindern, dass `Nach Hause` zu früh gewählt wird.",
      "Nach dem Abmelden kann ein freiwilliges Tages-Feedback erscheinen.",
    ],
    troubleshooting: HELP_TOPICS.nfcProblem,
    related: [HELP_TOPICS.tagAssignment, HELP_TOPICS.nfcSupervision],
  };
}

function missingMenuTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.missingMenu,
    title: "Ein Menüpunkt fehlt",
    question: "Was kann ich tun, wenn ein Menüpunkt fehlt?",
    summary:
      "Ein Bereich kann wegen Ihrer Rolle, einer Einstellung oder der gewählten Arbeitsweise der OGS fehlen.",
    group: "probleme",
    audience: "caregiver",
    icon: ShieldAlert,
    steps: [
      "Öffnen Sie auf einem kleinen Bildschirm zuerst das Menü mit den drei Balken.",
      "Prüfen Sie, ob der gesuchte Bereich in einer aufgeklappten Gruppe der Seitenleiste liegt.",
      "Laden Sie die Seite neu.",
      "Melden Sie sich ab und wieder an, wenn Ihre Rolle oder Rechte gerade geändert wurden.",
      "Fragen Sie Ihre Leitung, ob die Funktion für Ihre OGS eingeschaltet ist.",
      "Bitten Sie Ihre Leitung, Ihre Rolle und Rechte zu prüfen.",
    ],
    result:
      "Ihre Leitung prüft die Einstellungen und Ihren Zugang. Danach kann sie die Ursache klären.",
    differences: [
      "Bei einfacher Anwesenheit fehlen `Räume`, `Aktivitäten` und `Aktuelle Aufsicht`.",
      "Bei offener Betreuung fehlt `Meine Gruppen`.",
      "Ohne NFC fehlt der Bereich `Aktivitäten`.",
      "Team-Chat, Eltern-Nachrichten und einige Auswertungen können von der OGS ein- oder ausgeschaltet werden.",
    ],
    related: [HELP_TOPICS.missingChildOrGroup, HELP_TOPICS.loginProblem],
  };
}

function missingChildOrGroupTopic(
  groupMode: PrototypeGroupMode,
): PrototypeTopic {
  return {
    id: HELP_TOPICS.missingChildOrGroup,
    title: "Ein Kind oder eine Gruppe fehlt",
    question: "Was kann ich tun, wenn ein Kind oder eine Gruppe fehlt?",
    summary:
      "Prüfen Sie Suche, Filter, gewählten Tag und Ihre Gruppenzuordnung.",
    group: "probleme",
    audience: "caregiver",
    icon: Search,
    steps: [
      "Löschen Sie den Text im Suchfeld.",
      "Öffnen Sie `Filter` und setzen Sie aktive Filter zurück.",
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

function attendanceProblemTopic(
  presenceMode: PrototypePresenceMode,
): PrototypeTopic {
  return {
    id: HELP_TOPICS.attendanceProblem,
    title: "Ich kann ein Kind nicht an- oder abmelden",
    question: "Warum kann ich ein Kind nicht an- oder abmelden?",
    summary: "Prüfen Sie den aktuellen Status, Ihre Auswahl und Ihre Rechte.",
    group: "probleme",
    audience: "caregiver",
    icon: ShieldAlert,
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

function nfcProblemTopic(nfcEnabled: boolean | null): PrototypeTopic {
  if (nfcEnabled === null) {
    return unknownNfcTopic(
      HELP_TOPICS.nfcProblem,
      "Das NFC-Tablet funktioniert nicht",
      "Was kann ich tun, wenn das NFC-Tablet nicht funktioniert?",
      [HELP_TOPICS.webAttendance, HELP_TOPICS.trackWorkTime],
      "probleme",
    );
  }
  if (!nfcEnabled) {
    return unavailableNfcTopic(
      HELP_TOPICS.nfcProblem,
      "Das NFC-Tablet funktioniert nicht",
      "Was kann ich tun, wenn das NFC-Tablet nicht funktioniert?",
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
    audience: "caregiver",
    icon: TabletSmartphone,
    steps: [
      "Prüfen Sie, ob Tablet und NFC-Sensor Strom haben.",
      "Prüfen Sie, ob das Tablet mit dem Internet verbunden ist.",
      "Legen Sie das Armband flach und für einen kurzen Moment auf den NFC-Sensor.",
      "Testen Sie ein zweites, sicher funktionierendes Armband.",
      "Melden Sie sich mit der Geräte-PIN an und öffnen Sie `Armband identifizieren`.",
      "Prüfen Sie dort, ob das Armband erkannt und der richtigen Person zugewiesen ist.",
      "Starten Sie das Tablet neu, wenn weiterhin kein Armband erkannt wird.",
      "Informieren Sie Ihre Leitung und nennen Sie die angezeigte Fehlermeldung.",
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

function loginProblemTopic(): PrototypeTopic {
  return {
    id: HELP_TOPICS.loginProblem,
    title: "Ich kann mich nicht anmelden",
    question: "Was kann ich tun, wenn die Anmeldung nicht klappt?",
    summary:
      "Prüfen Sie die Seite Ihrer OGS, Ihre E-Mail-Adresse und Ihr Passwort.",
    group: "probleme",
    audience: "caregiver",
    icon: KeyRound,
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
      "Die vierstellige Geräte-PIN des NFC-Tablets ist nicht Ihr persönliches moto-Passwort.",
      "Ein Sicherheitscode kann nur für kurze Zeit verwendet werden. Fordern Sie bei Bedarf einen neuen Code an.",
    ],
    related: [HELP_TOPICS.login, HELP_TOPICS.acceptInvitation],
  };
}

function caregiverTopics(
  presenceMode: PrototypePresenceMode,
  groupMode: PrototypeGroupMode,
  nfcEnabled: boolean | null,
): readonly PrototypeTopic[] {
  return [
    myScheduleTopic(),
    carePlanTopic(),
    studentSearchTopic(presenceMode),
    editStudentTopic(),
    webAttendanceTopic(presenceMode),
    changeLocationTopic(presenceMode),
    absencesTopic(),
    dayLogTopic(),
    emergencyTopic(presenceMode),
    ownGroupsTopic(presenceMode, groupMode),
    transferGroupTopic(groupMode),
    roomsTopic(presenceMode),
    activeSupervisionTopic(presenceMode),
    manageActivityTopic(presenceMode, nfcEnabled),
    parentMessageTopic(),
    parentRequestsTopic(),
    teamChatTopic(),
    findStaffTopic(),
    sharedFilesTopic(),
    trackWorkTimeTopic(),
    correctWorkTimeTopic(),
    vacationTopic(),
    ownAbsenceTopic(),
    tabletLoginTopic(nfcEnabled),
    tagAssignmentTopic(nfcEnabled),
    nfcWorkTimeTopic(nfcEnabled),
    nfcSupervisionTopic(presenceMode, nfcEnabled),
    nfcCheckInTopic(presenceMode, nfcEnabled),
    missingMenuTopic(),
    missingChildOrGroupTopic(groupMode),
    attendanceProblemTopic(presenceMode),
    nfcProblemTopic(nfcEnabled),
    loginProblemTopic(),
  ];
}

const LEAD_DRAFT_TOPICS: readonly PrototypeTopic[] = [
  draftTopic({
    id: HELP_TOPICS.dataManagement,
    title: "Schuldaten verwalten",
    question: "Wo verwalte ich die Daten der Einrichtung?",
    summary: "Leitungen finden hier Importe, Exporte und Schuljahreswechsel.",
    group: "leitung",
    audience: "lead",
    icon: Database,
    related: [HELP_TOPICS.enrollments, HELP_TOPICS.settings],
  }),
  draftTopic({
    id: HELP_TOPICS.enrollments,
    title: "Anmeldungen prüfen",
    question: "Wie prüfe ich eine neue Anmeldung?",
    summary: "Prüfen Sie neue Angaben vor der Übernahme.",
    group: "leitung",
    audience: "lead",
    icon: ListChecks,
    related: [HELP_TOPICS.dataManagement, HELP_TOPICS.settings],
  }),
  draftTopic({
    id: HELP_TOPICS.settings,
    title: "Einstellungen der Einrichtung ändern",
    question: "Wo ändere ich die Einstellungen der Einrichtung?",
    summary: "Leitungen steuern hier Funktionen und Abläufe der Einrichtung.",
    group: "leitung",
    audience: "lead",
    icon: Settings,
    related: [HELP_TOPICS.dataManagement, HELP_TOPICS.enrollments],
  }),
] as const;

export function getPrototypeTopics(
  presenceMode: PrototypePresenceMode,
  groupMode: PrototypeGroupMode = "fixed_groups",
  nfcEnabled: boolean | null = true,
): readonly PrototypeTopic[] {
  const topics = [
    invitationTopic(),
    loginTopic(presenceMode, groupMode),
    installAppTopic(),
    appOverviewTopic(),
    ...caregiverTopics(presenceMode, groupMode, nfcEnabled),
    ...LEAD_DRAFT_TOPICS,
  ];

  return topics;
}

export const PROTOTYPE_GROUP_LABELS: Readonly<
  Record<PrototypeTopicGroup, string>
> = {
  einstieg: "Einstieg",
  tagesplanung: "Tagesplanung",
  kinder: "Kinder und Anwesenheit",
  gruppen: "Gruppen, Räume und Aufsicht",
  team: "Mit Eltern und dem Team arbeiten",
  arbeitszeit: "Meine Arbeitszeit",
  nfc: "NFC-Tablet benutzen",
  probleme: "Wenn etwas nicht klappt",
  leitung: "Für die Leitung",
};
