import type { HelpIconName } from "./help-icons";
import type {
  HelpTopic,
  HelpTopicGroup,
  HelpPresenceMode,
  HelpGroupMode,
  HelpTopicId,
} from "./help-types";

export function loginResult(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
): string {
  if (presenceMode === "binary" || groupMode === "open_care") {
    return "Nach der Anmeldung öffnet moto den Bereich `Alle Kinder`.";
  }
  if (presenceMode === "detailed" && groupMode === "fixed_groups") {
    return "Nach der Anmeldung öffnet moto den Bereich `Gruppen`.";
  }
  return "Nach der Anmeldung öffnet moto die für Sie vorgesehene Startseite.";
}

export function unavailableNfcTopic(
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

export function unknownNfcTopic(
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

// Auf "Ein Menuepunkt fehlt" ist die Arbeitsweise der OGS meistens schon
// bekannt. Dann gehoert die zutreffende Zeile als Tatsache hin, nicht als
// eine von vier Moeglichkeiten -- sonst raetselt die Person auf genau der
// Seite weiter, die ihr die Antwort geben sollte.
export function missingMenuDifferences(
  presenceMode: HelpPresenceMode,
  groupMode: HelpGroupMode,
  nfcEnabled: boolean | null,
): readonly string[] {
  const lines: string[] = [];

  if (presenceMode === "binary") {
    // Vollstaendige Liste aus sidebar.tsx BINARY_HIDDEN_HREFS (/rooms,
    // /activities, /tagesplan) plus dem separat gegateten Aufsicht-Akkordeon.
    lines.push(
      "In Ihrer OGS gibt es `Räume`, `Aktivitäten`, `Aktuelle Aufsicht` und `Heute` nicht. Ihre OGS hält nur fest, ob ein Kind da ist.",
    );
  } else if (presenceMode === "unknown") {
    lines.push(
      "Bei einfacher Anwesenheit fehlen `Räume`, `Aktivitäten`, `Aktuelle Aufsicht` und `Heute`.",
    );
  }

  if (groupMode === "open_care") {
    lines.push(
      "In Ihrer OGS gibt es den Bereich `Gruppen` nicht. Ihre OGS arbeitet mit offener Betreuung.",
    );
  } else if (groupMode === "unknown") {
    lines.push("Bei offener Betreuung fehlt `Gruppen`.");
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

/**
 * Mitteilung, Elternbrief und Umfrage liegen auf derselben Seite
 * `/parent-announcements`; `KIND_ITEMS` schaltet zwischen ihnen um
 * (page.tsx:136). Deshalb teilen sich die drei Artikel denselben Weg dorthin
 * und unterscheiden sich nur in dem, was die Art ausmacht.
 */
export function parentAnnouncementSteps(
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
