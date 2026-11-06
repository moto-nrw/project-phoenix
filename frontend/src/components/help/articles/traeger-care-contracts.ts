import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/vertraege` (#4678, #4680, #4681): festlegen, wann der Vertrag
 * gilt, Vertragsbedingungen je Angebot festlegen, Entwürfe aus angenommenen
 * Anmeldungen erstellen, prüfen und senden; Versionen nach einer Änderung.
 */
export function traegerCareContractsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareContracts,
    title: "Betreuungsverträge erstellen und senden",
    question: "Wie schicke ich den Eltern den Betreuungsvertrag?",
    summary:
      "Aus jeder angenommenen Anmeldung entsteht ein Vertrag mit den gebuchten Modulen je Wochentag. Entweder gilt er gleich bei der Annahme, oder Sie prüfen den Entwurf und senden ihn per Mail und in die Eltern-App.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "Signature",
    requirements: [
      "Sie haben die Rolle `Verwaltung`. Sie gibt das Recht `Verträge verwalten`.",
      "Die Schule hat die Anmeldung angenommen, und das Kind hat Betreuung gebucht.",
    ],
    instructionGroups: [
      {
        title: "Einmal: Festlegen, wann der Vertrag gilt",
        steps: [
          "Wählen Sie in der Navigation `Einstellungen` → `Allgemein`.",
          "Wählen Sie den Reiter `Anmeldung`.",
          "Wählen Sie bei `Wann gilt der Betreuungsvertrag?` eine Möglichkeit. `Bei der Annahme`: Nimmt die Schule eine Anmeldung an, gilt der Vertrag sofort. `Später verschicken`: Sie erstellen den Vertrag und schicken ihn den Eltern.",
          "Eine Schule kann in ihren eigenen Einstellungen etwas anderes wählen. Was für welche Schule gilt, sehen Sie unter `Verträge` in der Karte `Wann gilt der Vertrag?`.",
        ],
      },
      {
        title: "Einmal: Vertragsbedingungen festlegen",
        steps: [
          "Wählen Sie in der Navigation `Verträge`.",
          "Wählen Sie oben den Reiter `Vertragsbedingungen`.",
          "Wählen Sie links ein Angebot. `Fehlt` heißt: Für dieses Angebot gibt es noch keine Bedingungen.",
          "Wählen Sie die `Laufzeit`, die `Frist in Monaten` und wann die `Kündigung wirkt`.",
          "Erlauben Sie bei Bedarf eine `Sonderkündigung`. Schreiben Sie dazu, wann sie möglich ist.",
          "Müssen die Eltern eine Satzung anerkennen? Setzen Sie den Haken bei `Satzung`. Tragen Sie den Namen und, wenn vorhanden, den Link ein.",
          "Tragen Sie den Namen des `Vertragspartners` ein, wenn er nicht im Briefkopf steht.",
          "Wählen Sie `Bedingungen speichern`.",
        ],
      },
      {
        title: "Bei der Annahme: nichts zu tun",
        steps: [
          "Im Anmeldeformular stimmen die Eltern den Vertragsbedingungen zu. Dafür ist bei `AGB / Teilnahmebedingungen im Anmeldeformular anzeigen` der Schalter an. Die Schule nimmt die Anmeldung an.",
          "Nach wenigen Minuten steht der Vertrag in der Liste. Sein Status ist `Aktiv`.",
          "Die Eltern bekommen das PDF per Mail und in der Eltern-App. Sie müssen nichts unterschreiben.",
          "Auf der Vertragsseite steht unter `Abgeschlossen`, wann der Vertrag gilt.",
        ],
      },
      {
        title: "Später verschicken: Entwürfe erstellen",
        steps: [
          "Wählen Sie den Reiter `Verträge`.",
          "Wählen Sie `Verträge erstellen`. Die Zahl dahinter nennt die Kinder ohne Vertrag.",
          "Prüfen Sie die Liste. moto übernimmt Kind, Eltern, Beginn und je Wochentag die gebuchten Module mit ihrer Endzeit, zum Beispiel `Mo bis 14:00 · Di bis 16:30`.",
          "Wählen Sie die Kinder aus. Ein Vertrag umfasst alle Module eines Kindes. Fehlen bei einem Modul die Bedingungen, oder weichen sie zwischen den Modulen ab, legen Sie sie zuerst fest.",
          "Wählen Sie `Entwürfe erstellen`.",
        ],
      },
      {
        title: "Prüfen und senden",
        steps: [
          "Wählen Sie in der Liste ein Kind. Die Seite zeigt die `Vertragsangaben` und die `Vorschau` des PDF.",
          "Prüfen Sie unter `Gebuchte Betreuung` jeden Wochentag: das Modul und bis wann das Kind betreut wird. Tage ohne Betreuung stehen als `Keine Betreuung` da.",
          "Prüfen Sie Beginn und Beitrag.",
          "Wählen Sie `Vertrag senden`.",
          "Mehrere Entwürfe auf einmal: Setzen Sie in der Liste die Haken und wählen Sie oben `Entwürfe senden`.",
        ],
      },
      {
        title: "Wenn sich die Betreuung ändert: neue Version",
        steps: [
          "Die Eltern beantragen in der Eltern-App andere Tage oder Zeiten. Die Schule gibt die Änderung frei.",
          "Nach wenigen Minuten bekommt der aktive Vertrag eine neue Version. Sie gilt ab dem Tag, den die Schule freigegeben hat.",
          "Wählen Sie in der Liste das Kind. Oben steht, ab wann die neue Version gilt.",
          "Unter `Versionen` sehen Sie jede Version mit ihren Tagen. `Geändert` markiert die Wochentage, die anders sind als vorher.",
          "Mit `PDF` laden Sie eine Version als PDF.",
        ],
      },
    ],
    steps: [],
    result:
      "Die Eltern bekommen den Vertrag als PDF per Mail. Haben sie ein Konto in der Eltern-App, liegt er dort auch unter `Dokumente`. Gilt der Vertrag bei der Annahme, ist sein Status `Aktiv`. Sonst wird er `Versendet`: Der Versand ist beauftragt. Ob die Mail gesendet wurde, sehen Sie unter `Versand`.",
    notes: [
      "Die Suche und die Filter gelten für alle Verträge. Mit `Weitere Verträge laden` zeigen Sie ältere Treffer an.",
      "Beim ersten Senden hält moto das PDF fest. Ändern Sie danach Bedingungen oder Brieftext, bleibt der versendete Vertrag gleich.",
      "Unter `Versand` sehen Sie, ob die Mail gesendet wurde. Mit `Versand aktualisieren` prüfen Sie den neuen Stand. Mit `Erneut senden` geht dasselbe PDF noch einmal an die Eltern.",
      "Den Brieftext des Vertrags ändern Sie unter `Einstellungen` → `Briefkopf und Vorlagen` bei der Vorlage `Vertrag`.",
      "Mit `PDF herunterladen` bekommen Sie den Vertrag zum Drucken, etwa für Familien ohne Mail.",
      "Frühere Versionen bleiben unter `Versionen` lesbar. `Gilt jetzt` zeigt die Version von heute. Die Abrechnung nimmt für jeden Tag die Version, die an diesem Tag gilt.",
      "Eine neue Version gilt nie rückwirkend. Kommt eine Freigabe zu spät, gilt die Version ab dem Tag, an dem moto sie anlegt.",
    ],
    differences: [
      "Sie sehen `Verträge` nicht in der Navigation? Ihnen fehlt das Recht `Verträge verwalten`. Wenden Sie sich an eine Person mit der Rolle `Leitung`.",
    ],
    troubleshootingDetails: [
      "Die Schule hat angenommen, aber nach einigen Minuten steht kein aktiver Vertrag da? Dann fehlt etwas. Das Kind steht in `Verträge erstellen` mit dem Grund, zum Beispiel `Vertragsbedingungen fehlen`. Fehlt die E-Mail-Adresse der Eltern, ergänzt die Schule sie in der Anmeldung. moto versucht es danach von selbst noch einmal.",
      "Ein Kind zeigt `Zustimmung zum Vertrag fehlt`? Die Eltern haben in der Anmeldung den Vertragsbedingungen nicht zugestimmt, oder Sie haben die Bedingungen danach geändert. Dann entsteht kein aktiver Vertrag. Stellen Sie die Schule auf `Später verschicken` und schicken Sie den Vertrag. Eine frühere Anmeldung wird nicht nachträglich verbindlich.",
      "Schulen mit `Bei der Annahme` erlauben keine manuellen Entwürfe. Bereits vorhandene Entwürfe werden nicht automatisch abgeschlossen. Prüfen und verwerfen Sie diese bei Bedarf.",
      "Ein Kind fehlt in `Verträge erstellen`? Die Schule hat die Anmeldung noch nicht angenommen, oder das Kind hat schon einen Vertrag.",
      "Ein Kind zeigt `Bedingungen der Module weichen ab`? Das Kind hat mehrere Module gebucht, und deren Vertragsbedingungen sind verschieden. Ein Vertrag hat eine Laufzeit und eine Frist. Gleichen Sie die Bedingungen der Module an.",
      "Eine Endzeit fehlt? Das Modul hat für diesen Tag keine Gehzeit. Die Schule trägt sie beim Angebot ein.",
      "Ein Entwurf ist falsch? Wählen Sie auf seiner Seite oben `Entwurf verwerfen`. Danach erstellen Sie ihn neu. Einen versendeten Vertrag können Sie nicht verwerfen.",
      "Der Versand zeigt `Nicht angekommen`? Prüfen Sie die Adresse mit der Schule. Tragen Sie auf der Vertragsseite unter `Versand` die `Neue Versandadresse` ein. Wählen Sie `An diese Adresse senden`. Das geht auch bei aktiven Verträgen. Das PDF, der Abschluss, die Vertragspartei und bisherige Versanddaten bleiben gleich. Spätere Wiederholungen nutzen die letzte Versandadresse.",
      "Der Name des Vertragspartners fehlt? Ergänzen Sie den Briefkopf. Oder verwerfen Sie den Entwurf und erstellen ihn mit eingetragenem Vertragspartner neu.",
      "Die Schule hat eine Änderung freigegeben, aber es gibt keine neue Version? Neue Versionen bekommen nur aktive Verträge. Bucht das Kind ab dem Tag gar keine Betreuung mehr, ist das eine Kündigung und keine neue Version.",
    ],
    related: [
      HELP_TOPICS.traegerDocumentTemplates,
      HELP_TOPICS.traegerBriefkopf,
      HELP_TOPICS.traegerAccounts,
    ],
  };
}
