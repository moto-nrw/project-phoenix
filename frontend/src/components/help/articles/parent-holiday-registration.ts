import type { HelpTopic } from "../help-types";
import { HELP_TOPICS } from "~/lib/help-topics";

/**
 * `/ferienanmeldung/<link>` (#4661): der Link eines Trägers, ohne Konto, oder
 * die Karte `Ferienbetreuung` auf der Startseite der Eltern-App (#6671).
 * Die Frage nach dem Elternkonto, die zweite E-Mail und der Stand unter
 * `Ferienanmeldungen` (#6673).
 */
export function parentHolidayRegistrationTopic(): HelpTopic {
  return {
    id: HELP_TOPICS.parentHolidayRegistration,
    title: "Für die Ferienbetreuung anmelden",
    question: "Wie melde ich mein Kind für die Ferien an?",
    summary:
      "Sie starten in der Eltern-App oder mit dem Link vom Träger. Sie melden bis zu drei Kinder an.",
    image: "/help/screens/ferienanmeldung-zweite-person.webp",
    imageAlt: "Die freiwillige Karte für die zweite sorgeberechtigte Person.",
    group: "anmeldung",
    audience: "parent",
    icon: "SunHorizon",
    requirements: [
      "Sie haben die Eltern-App oder den Link zur Ferienanmeldung vom Träger.",
    ],
    steps: [
      "Öffnen Sie die Eltern-App. Auf der Startseite steht `Ferienbetreuung`. Wählen Sie `Jetzt anmelden`.",
      "Ohne Eltern-App öffnen Sie den Link vom Träger.",
      "Tragen Sie unter `Kinder` Vorname, Nachname und Geburtsdatum ein.",
      "Wählen Sie die `Schule` Ihres Kindes. Steht sie nicht in der Liste? Wählen Sie `Eine andere Schule`.",
      "Wählen Sie unter `Wochen` für jede Woche, wie lange Ihr Kind bleibt. Bei jeder Zeit steht der Preis pro Woche. Gibt es eine Woche an mehreren Schulen, wählen Sie auch die Schule.",
      "Steht bei der Zeit „je Tag“? Dann wählen Sie darunter die Tage, an denen Ihr Kind kommt. Der Preis gilt je Tag.",
      "Weitere Kinder fügen Sie mit `Geschwisterkind hinzufügen` hinzu.",
      "Tragen Sie `Ihre Angaben` ein.",
      "Fragt der Träger die `Anschrift` ab, tragen Sie Straße, PLZ und Ort ein. Die Anschrift steht auf der Rechnung.",
      "Sie können eine `Zweite sorgeberechtigte Person` angeben. Dann sind Vorname und Nachname nötig. E-Mail und Telefon sind freiwillig.",
      "Tragen Sie unter `Kontakte` jede weitere Person einmal ein. Wählen Sie `Notfallkontakt`, `Darf die Kinder abholen` oder beides. Mindestens ein Notfallkontakt mit Telefon ist nötig.",
      "Setzen Sie die Haken unter `Einverständnis`. Dazu gehört das Sorgerecht: Sie sind allein sorgeberechtigt, oder die andere sorgeberechtigte Person ist einverstanden.",
      "Tragen Sie unter `Ihre Erklärung` Ihren Vor- und Nachnamen ein. Damit geben Sie die Erklärungen ab.",
      "Wählen Sie `Weiter zur Zusammenfassung`.",
      "Prüfen Sie alles. Unter `Beitrag` steht, was die Ferienbetreuung kostet.",
      "Bekommen Sie Leistungen wie Bürgergeld, Wohngeld oder Kinderzuschlag? Dann setzen Sie den Haken unter `Beitrag`, wenn er angeboten wird.",
      "Sie können den Nachweis gleich darunter hochladen. Wählen Sie bei `Nachweis über die Leistungen` den Knopf `Datei auswählen`. Erlaubt sind PDF, JPG und PNG.",
      "Ohne Eltern-App steht über dem Knopf die Frage `Möchten Sie ein Elternkonto anlegen?`. Wählen Sie `Ja` oder `Nein`.",
      "Wählen Sie dann `Anmeldung absenden`.",
    ],
    result:
      "Sie bekommen eine E-Mail: Die Anmeldung ist eingegangen. Das ist noch keine Zusage. Der Träger schreibt Ihnen, ob Ihr Kind einen Platz bekommt. In der E-Mail steht auch `Ihre Erklärung` mit dem genauen Text. Heben Sie die E-Mail auf. In der Eltern-App sehen Sie den Stand unter `Ferienanmeldungen`.",
    notes: [
      "Achtet der Träger auf den Ferienwunsch aus der OGS-Anmeldung, steht das über den Kindern. Mit Elternkonto sehen Sie beim Kind, was in der OGS-Anmeldung steht.",
      "Ein Elternkonto brauchen Sie nicht. Mit Konto sehen Sie in der Eltern-App den Stand der Anmeldung, die Bestätigung und die Rechnung.",
      "Haben Sie bei `Möchten Sie ein Elternkonto anlegen?` `Ja` gewählt? Dann bekommen Sie eine zweite E-Mail. Öffnen Sie den Link darin. Legen Sie zweimal Ihr Passwort fest und wählen Sie `Konto anlegen`. Danach wählen Sie `Anmelden`.",
      "Haben Sie schon ein Elternkonto? Dann steht nach dem Link `Sie haben schon ein Elternkonto`. Wählen Sie `Anmelden`. Ihr Passwort bleibt gleich.",
      "Unter `Ferienanmeldungen` steht jede Anmeldung mit ihrem Stand: `Eingegangen`, `Bestätigt` oder `Storniert`. Bei jedem Kind sehen Sie die Wochen mit `Platz reserviert`, `Warteliste` oder `Platz offen`. Auf der Startseite steht dazu die Karte `Ihre Ferienanmeldungen`.",
      "Ist die Anmeldung bestätigt, finden Sie die Bestätigung und die Rechnung unter `Dokumente`.",
      "Vergibt der Träger die Plätze nach festen Regeln? Dann steht im Formular `Fragen für die Platzvergabe`. Beantworten Sie jede Frage mit Ja oder Nein. Das Büro kann nach einem Nachweis fragen. Nach dem Absenden steht bei jeder Woche `Platz offen`. Das Büro vergibt die Plätze. Sie bekommen eine E-Mail.",
      "Nach dem Absenden sehen Sie bei jeder Woche `Platz` oder `Warteliste`. Ist eine Woche voll, steht Ihr Kind dort auf der Warteliste. Wird ein Platz frei, bekommt Ihr Kind ihn automatisch. Sie bekommen dann eine E-Mail. Bei einer verbindlichen Anmeldung ist auch dieser Platz verbindlich.",
      "Lesen Sie die Anmeldung in einer anderen Sprache? Dann steht unter jeder Erklärung eine Übersetzung. Es gilt der deutsche Text.",
      "Gesundheitsangaben sehen der Träger und die OGS, die Ihr Kind in der Ferienwoche betreut. Nur wenn Sie zustimmen.",
      "Die OGS, die Ihr Kind in einer Ferienwoche betreut, sieht Name und Telefon der zweiten sorgeberechtigten Person. Sie sieht auch die Antworten, die der Träger für sie freigibt. Zum Beispiel, ob Ihr Kind allein nach Hause gehen darf.",
      "Die OGS sieht diese Angaben schon vor der Ferienwoche. So kann sie sich vorbereiten. Die Liste der Kinder sieht sie bis zum letzten Tag der Woche. Name, Telefon und Antworten kann sie danach höchstens 7 Tage lesen. Dann werden sie gelöscht. Gesundheitsangaben sieht sie nur bis zum letzten Tag der Woche.",
      "Ein Kind von einer anderen Schule kommt als Feriengast.",
      "Manche Ferien haben eine Grenze, zum Beispiel höchstens 5 Tage je Kind. Dann steht sie über den Wochen. Eine ganze Woche zählt mit allen Tagen von Montag bis Freitag. Melden Sie Ihr Kind ein zweites Mal an, zählen die Tage beider Anmeldungen zusammen.",
      "Ein Feiertag oder Schließtag ist bei den Tagen durchgestrichen. An diesem Tag ist keine Betreuung, Sie können ihn nicht wählen.",
      "Steht unter den Tagen `Schon voll`? An diesen Tagen sind alle Plätze vergeben. Wählen Sie einen davon, kommt Ihr Kind für diese Woche auf die Warteliste.",
      "Der Preis bei den Wochen gilt ohne Ermäßigung oder Befreiung. Geschwister und Leistungsbezug rechnet moto erst unter `Beitrag` ein.",
      "Der Beitrag gilt erst, wenn der Träger die Anmeldung bestätigt. Bei Leistungsbezug braucht der Träger einen Nachweis.",
      "Der Nachweis ist freiwillig. Sie können ihn auch später beim Träger abgeben.",
    ],
    differences: [
      "Fragt das Formular `Hat Ihr Kind Allergien oder Unverträglichkeiten?`? Dann braucht der Träger die Angaben zu Allergien und Medikamenten. Wählen Sie bei jeder Frage `Nein, keine` oder `Ja`. Bei `Ja` schreiben Sie auf, was Ihr Kind hat. Weitere Hinweise bleiben freiwillig.",
      "Steht in der Eltern-App `Anmeldung ab` mit einem Datum? Dann beginnt die Anmeldung an diesem Tag. Ab dann sehen Sie den Knopf `Jetzt anmelden`.",
      "Sie haben den Link geöffnet und sind nicht angemeldet? Oben steht `Sie haben schon die Eltern-App?`. Wählen Sie `Anmelden`. Danach kommen Sie zurück zum Formular.",
      "Ist die Anmeldung verbindlich, heißt der Knopf `Verbindlich anmelden`.",
      "Sind Sie mit Ihrem Elternkonto in moto angemeldet? Dann steht unter `Kinder` der Abschnitt `Ihre Kinder aus dem Elternkonto`. Setzen Sie den Haken bei Ihrem Kind. Name, Geburtsdatum und Schule stehen dann schon im Formular, Ihre Angaben auch.",
      "Sie können diese Angaben vor dem Absenden ändern. Das gilt nur für die Ferienanmeldung. Bei der Schule ändert sich nichts.",
      "Mit Elternkonto fragt moto am Ende nicht nach einem Konto. Die Anmeldung steht sofort unter `Ferienanmeldungen`. Nach dem Absenden führt `Zu Ihren Ferienanmeldungen` dorthin.",
    ],
    troubleshootingDetails: [
      "Sie sehen in der Eltern-App keine `Ferienbetreuung`? Dann läuft gerade keine Anmeldung für die Schule Ihres Kindes. Oder die Frist ist vorbei. Haben Sie einen Link vom Träger, nutzen Sie den Link.",
      "moto meldet zu viele Tage je Kind? Die Grenze gilt für alle Anmeldungen des Kindes zusammen. Bitte wählen Sie weniger Tage oder fragen Sie das Träger-Büro. Der Hinweis zeigt keine Tage aus anderen Anmeldungen.",
      "moto sagt, die Anmeldung sei nicht offen? Oben auf der Seite stehen die Tage, an denen Sie anmelden können.",
      "moto sagt, den Anmeldelink gebe es nicht? Fragen Sie den Träger nach dem richtigen Link.",
      "Der Link zum Elternkonto gilt nicht mehr? Er gilt 30 Tage. Haben Sie schon ein Konto, melden Sie sich an. Sonst bitten Sie den Träger um einen neuen Link.",
      "Sie sehen eine Anmeldung nicht unter `Ferienanmeldungen`? Dort stehen Anmeldungen mit der E-Mail-Adresse Ihres Kontos. Haben Sie eine andere Adresse eingetragen, fragen Sie den Träger.",
    ],
    related: [HELP_TOPICS.parentEnroll],
  };
}
