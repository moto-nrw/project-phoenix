import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

export function traegerCareReductionsTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.traegerCareReductions,
    title: "Ermäßigungen im Monatsbeitrag",
    question:
      "Wie lege ich Geschwister, Einkommensstufen und Höchstbeträge fest?",
    summary:
      "Sie legen fest, wann eine Familie weniger zahlt. Der Monatslauf rechnet jede Ermäßigung selbst aus. Sie steht als eigene Zeile auf der Rechnung.",
    group: "anmeldeverwaltung",
    audience: "traeger",
    icon: "SlidersHorizontal",
    requirements: [
      "Ihre Rolle im Träger-Büro ist Abrechnung.",
      "Die Preise der Module stehen in der `Preisliste`.",
    ],
    steps: [],
    instructionGroups: [
      {
        title: "Regel anlegen",
        description:
          "Es gibt vier Arten: Einkommensstufen, Geschwister, Alleinerziehende und Höchstbetrag. Der Monatslauf rechnet sie in dieser Reihenfolge.",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Ermäßigungen`.",
          "Wählen Sie bei der Art den Knopf `Regel anlegen`, `Stufe anlegen` oder `Höchstbetrag anlegen`.",
          "Geben Sie einen Namen ein. Er steht so auf der Rechnung.",
          "Wählen Sie die Monate und die Schulen, für die die Regel gilt.",
          "Soll die Regel nur für einzelne Module gelten, wählen Sie die Module. Sonst lassen Sie das Feld leer.",
          "Wählen Sie die Wirkung: `Anteil weniger`, `Betrag weniger` oder `Fester Preis`. Tragen Sie die Zahl ein.",
          "Wählen Sie `Regel speichern`.",
        ],
      },
      {
        title: "Geschwister",
        description:
          "Geschwister sind Kinder mit derselben Rechnungsadresse. Das Kind mit dem höchsten Beitrag ist das 1. Kind. Bei gleichem Beitrag zählt das ältere Kind zuerst.",
        steps: [
          "Wählen Sie bei `Ab dem Kind`, ab welchem Kind die Ermäßigung gilt, zum Beispiel ab dem 2. Kind.",
          "Wählen Sie bei `Geschwister zählen an` die Schulen, deren Kinder mitzählen. Lassen Sie das Feld leer, zählen nur die Schulen der Regel.",
          "`Fester Preis` heißt: Das Kind zahlt genau diesen Betrag. `Betrag weniger` zieht den Betrag vom Preis ab.",
          "Sollen auch Geschwister ohne Betreuungsvertrag zählen? Schalten Sie `Geschwister ohne Vertrag mitzählen` ein.",
        ],
      },
      {
        title: "Kinderzahl der Familie prüfen",
        description:
          "Eltern geben im Vertrag an, wie viele Kinder ihre Familie hat. Sie bestätigen die Zahl. Erst dann zählt sie.",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Kinderzahl`.",
          "Unter `Zu prüfen` stehen neue Angaben der Eltern und Zahlen, die bald ablaufen.",
          "Wählen Sie bei der Familie `Prüfen`. Im Menü einer Familie heißt es `Kinderzahl bestätigen` oder `Kinderzahl ändern`.",
          "Prüfen Sie die Zahl. Ändern Sie sie, wenn nötig.",
          "Wählen Sie, ab wann die Zahl gilt. Soll sie nur eine Zeit lang gelten, wählen Sie auch `Gilt bis`.",
          "Wählen Sie `Kinderzahl bestätigen`.",
        ],
      },
      {
        title: "Nachweis eintragen",
        description:
          "Einkommensstufe und Ermäßigung für Alleinerziehende brauchen einen Nachweis je Kind.",
        steps: [
          "Wählen Sie `Abrechnung` und dann den Reiter `Nachweise`.",
          "Wählen Sie beim Kind das Menü und dann `Einkommensstufe eintragen` oder `Alleinerziehend eintragen`.",
          "Wählen Sie die Stufe. Nennen Sie den geprüften Nachweis, zum Beispiel Einkommensteuerbescheid 2025.",
          "Tragen Sie ein, wann Sie ihn geprüft haben und ab wann er gilt.",
          "Wählen Sie `Nachweis speichern`.",
        ],
      },
      {
        title: "Nachweise ändern und vorab eintragen",
        description:
          "Die Liste zeigt alle Zeiträume, auch abgelaufene und zukünftige.",
        steps: [
          "Wählen Sie beim Kind das Menü. Jeder Nachweis zeigt seinen Zeitraum.",
          "Wählen Sie beim passenden Zeitraum `Einkommensstufe ändern` oder `Alleinerziehend ändern`.",
          "Für einen weiteren Zeitraum wählen Sie `Einkommensstufe eintragen` oder `Alleinerziehend eintragen`.",
          "Beenden Sie den bisherigen Zeitraum, bevor der neue beginnt. Zeiträume derselben Art dürfen sich nicht überschneiden.",
          "Wählen Sie `Nachweis speichern`.",
        ],
      },
    ],
    result:
      "Der nächste Monatslauf rechnet die Ermäßigungen aus. Jede Rechnung zeigt jede Ermäßigung mit Namen und Betrag. Bei Geschwistern steht dabei, das wievielte Kind es ist, zum Beispiel `2. Kind von 3`.",
    differences: [
      "Geschwister erkennt moto an derselben E-Mail-Adresse der Rechnung. Haben zwei Familien dieselbe Adresse, zählen ihre Kinder zusammen. Hat eine Familie zwei Adressen, zählen ihre Kinder nicht als Geschwister. Gleichen Sie die Adresse dann im Vertrag an.",
      "Gilt ein Nachweis erst ab Mitte des Monats, zählt er für den ganzen Monat.",
      "Zählt eine Regel die Kinderzahl, gilt die bestätigte Zahl. Es zählen aber immer mindestens die Kinder mit Vertrag. Geschwister ohne Vertrag stehen in der Reihe vorn: Bei 3 Kindern und einem Kind mit Vertrag ist es das 3. Kind.",
      "Geschwister zählen je Schule getrennt. Ein Kind mit Vertrag an einer anderen Schule zählt bei der Kinderzahl nur mit, wenn die Regel unter `Geschwister zählen an` auch diese Schule nennt. Geschwister ohne Vertrag zählen an jeder Schule mit.",
      "Eine Angabe der Eltern, die Sie noch nicht bestätigt haben, zählt nicht. Ist die Zahl abgelaufen, zählen nur die Kinder mit Vertrag. Der Monatslauf zeigt dann einen Hinweis.",
      "Eine Rechnung, die schon ausgestellt ist, bleibt, wie sie ist. Eine geänderte Regel gilt ab dem nächsten Monatslauf.",
    ],
    notes: [
      "moto speichert nur die Art des Nachweises und das Prüfdatum. Das Dokument und das Einkommen bleiben bei Ihnen.",
      "Jedes Öffnen der Seite `Nachweise` steht im Zugriffsprotokoll.",
      "Bei der Kinderzahl speichert moto nur die Zahl. Namen und Geburtstage der Geschwister ohne Vertrag erfasst moto nicht. Jedes Öffnen und Ändern steht im Zugriffsprotokoll.",
    ],
    related: [HELP_TOPICS.traegerCareMonths, HELP_TOPICS.traegerAccessLog],
  };
}
