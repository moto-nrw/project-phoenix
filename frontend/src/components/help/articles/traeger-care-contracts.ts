import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/traeger/vertraege` (#4678, #4679, #4680, #4681): festlegen, wann der
 * Vertrag gilt, Vertragsbedingungen je Angebot festlegen, Entwürfe aus
 * angenommenen Anmeldungen erstellen, prüfen und senden; die Unterschrift
 * der Eltern und den Papiervertrag erfassen; Versionen nach einer Änderung;
 * Kündigung und Widerruf (#4682); Änderungswünsche der Eltern freigeben (#5979).
 */
export function traegerCareContractsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareContracts,
    title: "Betreuungsverträge erstellen und senden",
    question: "Wie schicke ich den Eltern den Betreuungsvertrag?",
    summary:
      "Aus jeder angenommenen Anmeldung entsteht ein Vertrag mit den gebuchten Modulen je Wochentag. Entweder gilt er gleich bei der Annahme, oder Sie prüfen den Entwurf und senden ihn per Mail und in die Eltern-App. Die Eltern unterschreiben ihn dann online oder auf Papier.",
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
          "Unter `Widerrufsbelehrung für das Anmeldeformular` und `Häkchen „Betreuung vor Ende der Widerrufsfrist“` stehen zwei weitere Texte. Die Schule kopiert beide als eigene Rechtstexte ins Anmeldeformular. Die Belehrung muss bestätigt werden, das Häkchen nicht.",
          "Die Widerrufsbelehrung ist das gesetzliche Muster. Sie gilt in jedem Bundesland gleich. Dieselbe Belehrung steht im Vertrag, auf der Seite zum Unterschreiben und in der Bestätigung per Mail. Die Anschrift kommt aus Ihrem Briefkopf, Telefon und E-Mail aus `Allgemein` › `Kontakt` › `Kontakt für Widerrufe`. Der Text für das Anmeldeformular und das PDF enthalten auch das Muster-Widerrufsformular.",
          "Für `Bei der Annahme`: Tragen Sie den `Vertragspartner` immer ein. Speichern Sie die Bedingungen. Kopieren Sie den Text aus `Gespeicherter Text für das Anmeldeformular` unverändert. Die Schule wählt im Anmeldeformular `Eigene Zustimmung hinzufügen`. Sie trägt ihn unter `Rechtstext / Erklärung` ein. Sie setzt `Muss bestätigt werden` und behält bestehende Rechtstexte bei. Bei mehreren Modulen müssen deren Bedingungen übereinstimmen.",
        ],
      },
      {
        title: "Bei der Annahme: nichts zu tun",
        steps: [
          "Im Anmeldeformular stimmen die Eltern dem vollständigen gespeicherten Bedingungstext zu. Die eigene Zustimmung muss bestätigt werden. Ein allgemeiner Verweis auf Bedingungen reicht nicht. Die Schule nimmt die Anmeldung an.",
          "Nach wenigen Minuten steht der Vertrag in der Liste. Sein Status ist `Aktiv`.",
          "Die Eltern bekommen das PDF per Mail und in der Eltern-App. Sie müssen nichts unterschreiben.",
          "Auf der Vertragsseite steht unter `Abgeschlossen`, wann der Vertrag gilt. Die Karte `Unterschrift` zeigt ebenfalls `Abgeschlossen`. Eine weitere Unterschrift ist nicht nötig.",
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
          "Wählen Sie `Vertrag senden`. moto fragt noch einmal und nennt die Mail-Adresse. Bestätigen Sie mit `Vertrag senden`.",
          "Mehrere Entwürfe auf einmal: Setzen Sie in der Liste die Haken und wählen Sie oben `Entwürfe senden`.",
        ],
      },
      {
        title: "Unterschrift der Eltern",
        description:
          "Die Eltern unterschreiben in der Eltern-App oder über den Link in der Mail. Sie zeichnen mit dem Finger oder tippen ihren Namen.",
        steps: [
          "Öffnen Sie den Vertrag. Die Karte `Unterschrift` zeigt `Wartet auf Eltern` oder `Unterschrieben`.",
          "Nach der Unterschrift ist der Vertrag `Aktiv`. Die Karte zeigt, wer wann auf welchem Weg unterschrieben hat, die Widerrufsfrist und die gespeicherte Unterschrift.",
          "Die Eltern bekommen eine Bestätigung mit dem Vertrag und der Widerrufsbelehrung per Mail.",
        ],
      },
      {
        title: "Papiervertrag erfassen",
        description:
          "Für Familien ohne Internet oder einen Vertrag, den es nur auf Papier gibt.",
        steps: [
          "Wählen Sie oben `PDF herunterladen` und drucken Sie den Vertrag.",
          "Die Eltern unterschreiben ihn. Scannen Sie den unterschriebenen Vertrag als PDF.",
          "Wählen Sie auf der Vertragsseite in der Karte `Unterschrift` den Knopf `Papiervertrag erfassen`.",
          "Prüfen Sie `Unterschrieben von` und `Unterschrieben am`. Wählen Sie mit `PDF wählen` die Datei.",
          "Wählen Sie `Vertrag abschließen`. Der Vertrag ist dann `Aktiv`. Mit `Papiervertrag laden` holen Sie das PDF später wieder.",
        ],
      },
      {
        title: "Änderungswunsch der Eltern freigeben",
        steps: [
          "Die Eltern beantragen in der Eltern-App andere Tage oder Module. Sie oder die Schule entscheiden.",
          "Wählen Sie in der Liste das Kind. Rechts steht die Karte `Änderungswunsch der Eltern`.",
          "Je Modul sehen Sie die bisherige und die gewünschte Buchung. Bei festen Tagen steht dort `Gebucht (feste Tage)`.",
          "Wählen Sie `Freigeben`. Prüfen Sie `Gilt ab` und die angezeigten Änderungen. Wählen Sie noch einmal `Freigeben`.",
          "Oder wählen Sie `Ablehnen`. Schreiben Sie eine Begründung für die Eltern und wählen Sie `Ablehnen`.",
          "Die Eltern bekommen eine Nachricht in der Eltern-App.",
        ],
      },
      {
        title: "Wenn sich die Betreuung ändert: neue Version",
        steps: [
          "Nach einer Freigabe durch Sie oder die Schule bekommt der aktive Vertrag nach wenigen Minuten eine neue Version. Sie gilt ab dem freigegebenen Tag.",
          "Wählen Sie in der Liste das Kind. Oben steht, ab wann die neue Version gilt.",
          "Unter `Versionen` sehen Sie jede Version mit ihren Tagen. `Geändert` markiert die Wochentage, die anders sind als vorher.",
          "Mit `PDF` laden Sie eine Version als PDF.",
        ],
      },
      {
        title: "Kündigung oder Widerruf eintragen",
        steps: [
          "Eltern kündigen selbst in der Eltern-App. Dann steht die Kündigung von allein beim Vertrag. Sie müssen nichts tun.",
          "Kam eine Kündigung per Mail oder Brief? Wählen Sie in der Liste das Kind.",
          "Rechts steht die Karte `Vertrag beenden`. Sie zeigt den Kündigungstermin und die Absendefrist für einen Widerruf.",
          "Wählen Sie `Kündigung eintragen`.",
          "Wählen Sie bei `Eingegangen am` den Tag, an dem die Mail oder der Brief ankam.",
          "Die Auswahl gilt für diesen Eingangstag, auch bei älteren Erklärungen. Wählen Sie `Kündigung` oder `Widerruf`. Bei einer Kündigung wählen Sie den Termin oder `Außerordentlich, aus wichtigem Grund` und tragen den Grund der Eltern ein.",
          "Bei einem Widerruf: Tragen Sie bei `Abgesendet am` den Absende-Tag ein. Ein rechtzeitig abgesendeter Brief darf später ankommen.",
          "Wählen Sie `Kündigung eintragen` oder `Widerruf eintragen`.",
        ],
      },
    ],
    steps: [],
    result:
      "Die Eltern bekommen den Vertrag als PDF per Mail. Haben sie ein Konto in der Eltern-App, liegt er dort auch unter `Dokumente`. Wer dort `Neues Dokument vom Träger` eingeschaltet hat, bekommt dazu eine Nachricht aufs Handy. Gilt der Vertrag bei der Annahme, ist sein Status `Aktiv`. Sonst wird er `Versendet`: Der Versand ist beauftragt. Ob die Mail gesendet wurde, sehen Sie unter `Versand`.",
    notes: [
      "Die Suche und die Filter gelten für alle Verträge. Mit `Weitere Verträge laden` zeigen Sie ältere Treffer an.",
      "Beim ersten Senden hält moto das PDF fest. Ändern Sie danach Bedingungen oder Brieftext, bleibt der versendete Vertrag gleich.",
      "Unter `Versand` sehen Sie, ob die Mail gesendet wurde. Mit `Versand aktualisieren` prüfen Sie den neuen Stand. Mit `Erneut senden` geht dasselbe PDF noch einmal an die Eltern; moto fragt vorher. Bei einem aktiven Vertrag steht `Erneut senden` im Menü mit den drei Punkten oben rechts. Eine zweite Nachricht aufs Handy gibt es dabei nicht. Nur eine korrigierte Mail-Adresse bekommt eine eigene Nachricht.",
      "Neben jedem Versand steht, was aus der Mail geworden ist: wie bei der Abrechnung. Der Hinweis oben nennt den Grund und den nächsten Schritt. Bei `Verzögert` warten Sie. Hat die Schule die Mails gestoppt, klären Sie das mit der Schule. Bei `Gesperrt` klärt das der moto-Support. Solange das gilt, ist `Erneut senden` aus.",
      "Den Brieftext des Vertrags ändern Sie unter `Einstellungen` → `Briefkopf und Vorlagen` bei der Vorlage `Vertrag`.",
      "Mit `PDF herunterladen` bekommen Sie den Vertrag zum Drucken, etwa für Familien ohne Mail.",
      "Wartet ein versendeter Vertrag auf die Unterschrift, bekommen die Eltern Erinnerungen per Mail. Es gibt höchstens drei. Bereits abgeschlossene Verträge bekommen keine Erinnerung. Nach wie vielen Tagen, legen Sie unter `Einstellungen` → `Allgemein` im Bereich `Betreuungsverträge` fest. Bei 0 gibt es keine Erinnerung.",
      "Der Link in der Mail gilt einmal und 30 Tage. Jede Erinnerung bringt einen neuen Link.",
      "Frühere Versionen bleiben unter `Versionen` lesbar. `Gilt jetzt` zeigt die Version von heute. Die Abrechnung nimmt für jeden Tag die Version, die an diesem Tag gilt.",
      "Bei einer außerordentlichen Kündigung steht in `Vertrag beenden` der Grund der Eltern und `Ende noch offen`. Prüfen Sie den Grund. Bis zu Ihrer Entscheidung gilt der Vertrag weiter.",
      "Erkennen Sie den Grund an? Wählen Sie `Vertragsende festlegen` und den letzten Tag.",
      "Erkennen Sie den Grund nicht an? Wählen Sie `Grund nicht anerkennen`. Dann gilt die Kündigung als ordentliche Kündigung. Der Vertrag endet zum nächsten möglichen Termin, gezählt ab dem Eingang. Das Datum steht unter dem Knopf und im Fenster, bevor Sie bestätigen.",
      "Beide Entscheidungen lassen sich danach nicht mehr ändern. Die Eltern bekommen eine Mitteilung mit dem letzten Betreuungstag per Mail und in der Eltern-App. Die Texte legen Sie unter `Einstellungen` → `Allgemein` im Reiter `Betreuungsverträge` fest: `Mitteilung: Vertragsende festgelegt` und `Mitteilung: Grund nicht anerkannt`. Die Mitteilung steht danach in der Karte `Vertrag beenden`, so wie sie verschickt wurde.",
      "Nach einer Kündigung hat der Vertrag den Status `Gekündigt`. Er gilt bis zum Vertragsende und wird bis dahin abgerechnet. Nach einem Widerruf ist er `Widerrufen` und gilt an keinem Tag.",
      "Die Eltern bekommen sofort eine Bestätigung per Mail und in der Eltern-App. Den Text legen Sie unter `Einstellungen` → `Allgemein` im Reiter `Betreuungsverträge` fest. Die Bestätigung steht danach in der Karte `Vertrag beenden`, so wie sie verschickt wurde.",
      "Den Kündigungstermin rechnet moto aus den Bedingungen im Vertrag, nicht aus den heutigen Vertragsbedingungen. Die Widerrufsfrist beginnt frühestens mit dem Erhalt der Belehrung. Ohne Nachweis gilt die längere Frist.",
      "Eine neue Version gilt nie rückwirkend. Kommt eine Freigabe zu spät, gilt die Version ab dem Tag, an dem moto sie anlegt.",
      "Beginnt an einem Tag schon eine Version, und Sie oder die Schule geben für denselben Tag eine weitere Änderung frei, ersetzt moto diese Version. Die neue Version enthält beide Änderungen. Die alte steht unter `Versionen` als `Ersetzt` und gilt an keinem Tag.",
      "Ist schon eine spätere Version geplant, gilt eine frühere Änderung nur bis dahin. Die geplante Version übernimmt sie nicht von selbst.",
    ],
    differences: [
      "Sie sehen `Verträge` nicht in der Navigation? Ihnen fehlt das Recht `Verträge verwalten`. Wenden Sie sich an eine Person mit der Rolle `Leitung`.",
    ],
    troubleshootingDetails: [
      "In `Vertrag beenden` steht, dass die Mitteilung zum Vertragsende nicht rausging? Keine Adresse des Vertrags gehört noch einer Bezugsperson, die für das Kind handeln darf, zum Beispiel weil die Schule das Recht entzogen hat. Die Entscheidung gilt trotzdem. Teilen Sie den Eltern das Vertragsende selbst mit, zum Beispiel per Brief.",
      "`Vertragsende festlegen` oder `Grund nicht anerkennen` meldet, dass kein Ende mehr offen ist? Eine andere Person hat schon entschieden. Laden Sie die Seite neu. Die Entscheidung steht in `Vertrag beenden`.",
      "Bei einem Änderungswunsch fehlt `Freigeben`? Die Eltern melden damit die ganze Betreuung ab. Das ist eine Kündigung. Tragen Sie sie unter `Vertrag beenden` ein, oder lehnen Sie den Wunsch ab.",
      "Nach der Freigabe bleiben Regeltermine oder Abholzeiten unverändert? Geben Sie die angezeigten Hinweise an die Schule weiter. Die Schule muss sie prüfen.",
      "Die Karte `Änderungswunsch der Eltern` fehlt? Sie steht nur bei aktiven Verträgen.",
      "`Kündigung eintragen` fehlt? Der Vertrag ist noch nicht abgeschlossen oder schon beendet. Einen Entwurf oder versendeten Vertrag verwerfen Sie oder lassen ihn liegen.",
      "Ein Brief kam nach der Widerrufsfrist an? Entscheidend ist der Absende-Tag. Tragen Sie ihn bei `Abgesendet am` ein.",
      "Die Schule hat angenommen, aber nach einigen Minuten steht kein aktiver Vertrag da? Dann fehlt etwas. Das Kind steht in `Verträge erstellen` mit dem Grund, zum Beispiel `Vertragsbedingungen fehlen`. Fehlt die E-Mail-Adresse der Eltern, ergänzt die Schule sie in der Anmeldung. moto versucht es danach von selbst noch einmal.",
      "Ein Kind zeigt `Mail-Adresse gesperrt`? Mails an die Adresse der Eltern kamen nicht an. Die Schule prüft die Adresse beim Kind unter `Erziehungsberechtigte` und wählt `Sperre aufheben`. Danach entsteht der Vertrag von selbst.",
      "Ein Kind zeigt `Mails gestoppt`? Die Schule hat die Mails an diese Eltern gestoppt. Der Vertrag kommt erst, wenn die Schule beim Kind unter `Erziehungsberechtigte` `Mails wieder senden` wählt.",
      "Ein Kind zeigt `Zustimmung zum Vertrag fehlt` oder `Bedingungen nach der Anmeldung geändert`? Die Eltern haben in der Anmeldung den Vertragsbedingungen nicht zugestimmt, oder Sie haben die Bedingungen danach geändert. Dann entsteht kein aktiver Vertrag. Stellen Sie die Schule auf `Später verschicken` und schicken Sie den Vertrag. Eine frühere Anmeldung wird nicht nachträglich verbindlich.",
      "Nach Änderungen an den Bedingungen: Speichern Sie diese zuerst. Ersetzen Sie danach den Text im Anmeldeformular durch den neuen gespeicherten Text. Unverändertes Speichern lässt eine passende Zustimmung weiter gelten.",
      "Schulen mit `Bei der Annahme` erlauben keine manuellen Entwürfe. Bereits vorhandene Entwürfe werden nicht automatisch abgeschlossen. Prüfen und verwerfen Sie diese bei Bedarf.",
      "Ein Kind fehlt in `Verträge erstellen`? Die Schule hat die Anmeldung noch nicht angenommen, oder das Kind hat schon einen Vertrag.",
      "Ein Kind zeigt `Bedingungen der Module weichen ab`? Das Kind hat mehrere Module gebucht, und deren Vertragsbedingungen sind verschieden. Ein Vertrag hat eine Laufzeit und eine Frist. Gleichen Sie die Bedingungen der Module an.",
      "Eine Endzeit fehlt? Das Modul hat für diesen Tag keine Gehzeit. Die Schule trägt sie beim Angebot ein.",
      "Ein Entwurf ist falsch? Wählen Sie auf seiner Seite oben `Entwurf verwerfen`. Danach erstellen Sie ihn neu. Einen versendeten Vertrag können Sie nicht verwerfen.",
      "Der Versand zeigt `Nicht angekommen`? Prüfen Sie die Adresse mit der Schule. Tragen Sie auf der Vertragsseite unter `Versand` die `Neue Versandadresse` ein. Wählen Sie `An diese Adresse senden`. Das geht auch bei aktiven Verträgen. Das PDF, der Abschluss, die Vertragspartei und bisherige Versanddaten bleiben gleich. Spätere Wiederholungen nutzen die letzte Versandadresse.",
      "Der Name des Vertragspartners fehlt? Ergänzen Sie den Briefkopf. Oder verwerfen Sie den Entwurf und erstellen ihn mit eingetragenem Vertragspartner neu.",
      "Die Eltern sagen, der Link gilt nicht mehr? Er war schon benutzt oder ist abgelaufen. Wählen Sie `Erneut senden`. Die Eltern bekommen einen neuen Link. Oder sie unterschreiben in der Eltern-App unter `Dokumente`.",
      "Die Karte `Unterschrift` meldet, die Prüfsumme stimmt nicht? Der gespeicherte Nachweis wurde verändert. Melden Sie das dem moto-Team.",
      "Eine Änderung ist freigegeben, aber es gibt keine neue Version? Neue Versionen bekommen nur aktive Verträge. Bucht das Kind ab dem Tag gar keine Betreuung mehr, ist das eine Kündigung und keine neue Version.",
    ],
    related: [
      HELP_TOPICS.traegerDocumentTemplates,
      HELP_TOPICS.traegerBriefkopf,
      HELP_TOPICS.traegerAccounts,
    ],
  };
}
