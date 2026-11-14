import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

// Portal "moto schule" (#2207). Geruest nach der echten Navigation des
// Schul-Portals (`SCHOOL_PRIMARY_NAV`): Klassenansicht, Meine Aufsichten,
// Nachrichten, Tagesinformationen, Kalender.
//
// Eine Lehrkraft sieht bewusst wenig: ihre Klassen und ihre eigenen
// Aufsichten. Abhol- und Notfallkontakte gibt es nur zu den Kindern einer
// laufenden Aufsicht. Deshalb wird hier nichts mit der Betreuung geteilt.
//
// Die Ablaeufe sind noch nicht Schritt fuer Schritt gegen die App geprueft,
// deshalb bleiben es Entwuerfe.
/**
 * `/traeger/login` (#4649): eigene Adresse, danach die Liste der Schulen.
 * Der Code per E-Mail ist Pflicht, beim ersten Mal mit Einrichtung (#4651);
 * ein gemerktes Gerät braucht ihn beim selben Träger eine Zeit lang nicht (#6955).
 * „Passwort vergessen?“ schickt den Link auf den Träger-Host (#5149).
 */
export function traegerLoginTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerLogin,
    title: "Im Träger-Portal anmelden",
    question: "Wie melde ich mich im Träger-Portal an?",
    summary:
      "Das Träger-Portal hat eine eigene Adresse. Dort sehen Sie alle Schulen Ihres Trägers.",
    group: "einstieg",
    audience: "traeger",
    icon: "Building2",
    requirements: [
      "Sie gehören zum Träger-Büro. Sie haben eine Einladung angenommen, oder das moto-Team hat Ihr Konto zugeordnet.",
      "Zugriff auf Ihr E-Mail-Postfach. Bei jeder Anmeldung kommt ein Code per E-Mail.",
    ],
    steps: [
      "Öffnen Sie die Adresse des Träger-Portals.",
      "Tragen Sie Ihre `E-Mail-Adresse` ein.",
      "Tragen Sie Ihr `Passwort` ein.",
      "Wählen Sie `Anmelden`.",
      "Gehören Sie zu mehreren Trägern? Wählen Sie den Träger aus.",
      "Wählen Sie dann `Träger öffnen`.",
      "moto zeigt die `Code-Eingabe`. Öffnen Sie die E-Mail von moto.",
      "Geben Sie den sechsstelligen Code ein.",
      "Ist das Ihr eigenes Gerät? Wählen Sie `Bestätigen und dieses Gerät 90 Tage merken`. Dann fragt moto hier 90 Tage lang nicht nach einem Code.",
      "Ist es ein geteiltes Gerät? Wählen Sie `Nur dieses Mal bestätigen`.",
    ],
    result: "moto öffnet die Seite `Schulen` mit allen Schulen Ihres Trägers.",
    notes: [
      "Legen Sie sich die Adresse als Lesezeichen an.",
      "Ein neues Passwort gilt auch für Ihr Konto in der OGS.",
      "Gemerkte Geräte sehen Sie unter `Konto` bei `Meine vertrauten Geräte`. Dort können Sie ein Gerät wieder entfernen.",
    ],
    differences: [
      "Oben steht `Willkommen im Träger-Portal`. So erkennen Sie, dass Sie richtig sind.",
      "Haben Sie auch ein Konto in einer OGS? Dann nutzen Sie dasselbe Konto. Ein zweites Passwort brauchen Sie nicht.",
      "Melden Sie sich zum ersten Mal an? Dann richten Sie den Code zuerst ein. Wählen Sie `Code an meine E-Mail senden`. Geben Sie danach den Code ein. Wählen Sie `Weiter zum Dashboard`.",
      "Der Code ist im Träger-Portal Pflicht. Nur ein gemerktes Gerät braucht ihn eine Zeit lang nicht.",
      "Ein gemerktes Gerät gilt nur für diesen Träger. Gehören Sie zu mehreren Trägern, fragt moto beim anderen Träger wieder nach einem Code.",
      "Am Computer stehen die Bereiche links in der Navigation. Auf dem Handy stehen sie unten. Was dort keinen Platz hat, finden Sie unter `Mehr`.",
      "`Einstellungen` steht unten in der Navigation. Ein Klick darauf zeigt dort die Einstellungen. `Zurück zur App` führt zurück.",
    ],
    troubleshootingDetails: [
      "Keine E-Mail mit Code? Schauen Sie im Spam-Ordner nach. Wählen Sie sonst `Neuen Code anfordern`.",
      "Zu oft einen falschen Code eingegeben? Warten Sie 15 Minuten. Oder bitten Sie das moto-Team, den Code zurückzusetzen. Dann richten Sie ihn bei der nächsten Anmeldung neu ein.",
      "Kein Zugriff mehr auf Ihre E-Mails? Wenden Sie sich an das moto-Team.",
      "moto sagt, Ihr Konto habe keinen Zugang zum Träger-Portal? Wenden Sie sich an das moto-Team.",
      "moto meldet Sie ab und lässt Sie nicht wieder hinein? Dann ist der Zugang Ihres Trägers ausgeschaltet. Wenden Sie sich an das moto-Team.",
      "Eine Schule fehlt in der Liste? Das moto-Team ordnet die Schulen Ihrem Träger zu.",
      "moto sagt `Die E-Mail-Adresse oder das Passwort stimmt nicht.`? Prüfen Sie beides und versuchen Sie es erneut.",
      "Passwort vergessen? Wählen Sie `Passwort vergessen?`. Tragen Sie Ihre E-Mail-Adresse ein und wählen Sie `Link senden`. Öffnen Sie den Link aus der E-Mail und legen Sie ein neues Passwort fest.",
      "Keine E-Mail zum Zurücksetzen? Schauen Sie im Spam-Ordner nach. Kommt keine, gehört Ihr Konto nicht zum Träger-Büro. Wenden Sie sich an das moto-Team.",
    ],
    related: [HELP_TOPICS.errorMessages],
  };
}
