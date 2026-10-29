import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/** `/ferienanmeldung/<link>` (#4661): der Link eines Trägers, ohne Konto. */
export function parentHolidayRegistrationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentHolidayRegistration,
    title: "Für die Ferienbetreuung anmelden",
    question: "Wie melde ich mein Kind für die Ferien an?",
    summary:
      "Der Träger gibt Ihnen einen Link. Dort melden Sie bis zu drei Kinder an.",
    image: "/help/screens/ferienanmeldung-zweite-person.webp",
    imageAlt: "Die freiwillige Karte für die zweite sorgeberechtigte Person.",
    group: "anmeldung",
    audience: "parent",
    icon: "SunHorizon",
    requirements: ["Sie haben den Link zur Ferienanmeldung vom Träger."],
    steps: [
      "Öffnen Sie den Link.",
      "Tragen Sie unter `Kinder` Vorname, Nachname und Geburtsdatum ein.",
      "Wählen Sie die `Schule` Ihres Kindes. Steht sie nicht in der Liste? Wählen Sie `Eine andere Schule`.",
      "Wählen Sie unter `Wochen` für jede Woche, wie lange Ihr Kind bleibt. Bei jeder Zeit steht der Preis pro Woche.",
      "Weitere Kinder fügen Sie mit `Geschwisterkind hinzufügen` hinzu.",
      "Tragen Sie `Ihre Angaben` und die `Notfallkontakte` ein.",
      "Sie können eine `Zweite sorgeberechtigte Person` angeben. Dann sind Vorname und Nachname nötig. E-Mail und Telefon sind freiwillig.",
      "Tragen Sie unter `Wer darf die Kinder abholen?` alle Personen außer Ihnen ein.",
      "Setzen Sie die Haken unter `Einverständnis`.",
      "Wählen Sie `Weiter zur Zusammenfassung`.",
      "Prüfen Sie alles. Unter `Beitrag` steht, was die Ferienbetreuung kostet.",
      "Bekommen Sie Leistungen wie Bürgergeld, Wohngeld oder Kinderzuschlag? Dann setzen Sie den Haken unter `Beitrag`, wenn er angeboten wird.",
      "Sie können den Nachweis gleich darunter hochladen. Wählen Sie bei `Nachweis über die Leistungen` den Knopf `Datei auswählen`. Erlaubt sind PDF, JPG und PNG.",
      "Wählen Sie dann `Anmeldung absenden`.",
    ],
    result:
      "Sie bekommen eine E-Mail: Die Anmeldung ist eingegangen. Das ist noch keine Zusage. Der Träger schreibt Ihnen, ob Ihr Kind einen Platz bekommt.",
    notes: [
      "Ein Elternkonto brauchen Sie nicht.",
      "Gesundheitsangaben sind freiwillig. Nur die Betreuung sieht sie.",
      "Die OGS, die Ihr Kind in einer Ferienwoche betreut, sieht Name und Telefon der zweiten sorgeberechtigten Person. Sie sieht auch die Antworten, die der Träger für sie freigibt. Zum Beispiel, ob Ihr Kind allein nach Hause gehen darf. Gesundheitsangaben sieht sie nicht.",
      "Die OGS sieht diese Angaben schon vor der Ferienwoche. So kann sie sich vorbereiten. Die Liste der Kinder sieht sie bis zum letzten Tag der Woche. Name, Telefon und Antworten kann sie danach höchstens 7 Tage lesen. Dann werden sie gelöscht.",
      "Ein Kind von einer anderen Schule kommt als Feriengast.",
      "Der Preis bei den Wochen gilt ohne Ermäßigung oder Befreiung. Geschwister und Leistungsbezug rechnet moto erst unter `Beitrag` ein.",
      "Der Beitrag gilt erst, wenn der Träger die Anmeldung bestätigt. Bei Leistungsbezug braucht der Träger einen Nachweis.",
      "Der Nachweis ist freiwillig. Sie können ihn auch später beim Träger abgeben.",
    ],
    differences: [
      "Ist die Anmeldung verbindlich, heißt der Knopf `Verbindlich anmelden`.",
      "Sind Sie mit Ihrem Elternkonto in moto angemeldet? Dann steht unter `Kinder` der Abschnitt `Ihre Kinder aus dem Elternkonto`. Setzen Sie den Haken bei Ihrem Kind. Name, Geburtsdatum und Schule stehen dann schon im Formular, Ihre Angaben auch.",
      "Sie können diese Angaben vor dem Absenden ändern. Das gilt nur für die Ferienanmeldung. Bei der Schule ändert sich nichts.",
    ],
    troubleshootingDetails: [
      "moto sagt, die Anmeldung sei nicht offen? Oben auf der Seite stehen die Tage, an denen Sie anmelden können.",
      "moto sagt, den Anmeldelink gebe es nicht? Fragen Sie den Träger nach dem richtigen Link.",
    ],
    related: [HELP_TOPICS.parentEnroll],
  };
}
