# Inhalte der Schule folgen der Frist ihrer Datenart, Inhalte ohne Ende löscht die Schule selbst

Status: accepted. Gilt für [#4045](https://github.com/moto-nrw/project-phoenix/issues/4045),
Teil der Spec [#3949](https://github.com/moto-nrw/project-phoenix/issues/3949).
Baut auf [ADR 0048](0048-loeschfristen-traegt-das-moto-team-je-schule-ein.md) auf.

## Kontext

Inhalte, die die Schule selbst anlegt, blieben, bis jemand sie von Hand
löschte: Elternbriefe, Tagesinformationen, Termine im Kalender,
Jahrgangswechsel, die Dateiablage und Klassenlisten-Einträge. Der Wächtertest
aus #3952 führte sie als offene Lücke; der AVV konnte für sie keine Frist
zusagen.

Verworfene Alternativen:

- **Eine eigene Einstellung je Datenart.** Das moto-Team müsste je Schule sechs
  weitere Fristen eintragen, die sich inhaltlich nicht von bestehenden
  unterscheiden.
- **Eine Frist ab der letzten Änderung auch für die Dateiablage und die
  Klassenlisten.** Eine Datei, die die Schule nutzt, oder ein Kind, das die
  Schule noch besucht, hat kein Ende. Jede Frist würde gültige Daten löschen.

## Entscheidung

1. **Inhalte mit Ende folgen der Frist ihrer Datenart.** Elternbriefe der
   Frist der Elternnachrichten, Tagesinformationen der des Team-Chats, Termine
   im Kalender der des Betreuungsplans, Jahrgangswechsel, deren Verlauf Kinder
   nennt, der Frist für Kinder nach Austritt. Es gibt keine neue Einstellung.
   Die Bezeichnungen und Beschreibungen der vier Einstellungen nennen die
   zusätzlichen Inhalte.
2. **Startzeitpunkt ist das Ende des Inhalts:** Ablauf oder Veröffentlichung
   eines Elternbriefs, letzter gültiger Tag oder Ausschalten einer
   Tagesinformation, letzter Tag eines Termins oder einer Serie, Anwenden oder
   Zurücknehmen eines Jahrgangswechsels. Ein Entwurf zählt ab seiner letzten
   Änderung. Was kein Ende hat (eine Serie ohne Ende, eine eingeschaltete
   Tagesinformation ohne letzten Tag), bleibt.
3. **Nachweise gehen vor.** Ein Einverständnis mit Antworten bleibt, bis die
   Antworten mit ihrem Kind gehen.
4. **Inhalte ohne Ende löscht die Schule selbst.** Für die Dateiablage und die
   Klassenlisten-Einträge sagt der AVV zu, dass die Schule als Verantwortliche
   löscht. Der Wächtertest führt sie in einer eigenen Klasse
   (`school deletes`) mit dem Löschweg der Schule. Was dabei ohne Zweck
   zurückbleibt, etwa die Zeile einer gelöschten Datei, löscht moto nach einer
   festen Frist.

## Folgen

- Verkürzt das moto-Team eine dieser Fristen, gilt das auch für die Inhalte,
  die ihr folgen. Die Bestätigung im Operator-Dashboard bleibt dieselbe.
- Ein Jahrgangswechsel lässt sich nach seiner Frist nicht mehr zurücknehmen.
- Abgesagte und gelöschte Termine löscht der nächtliche Lauf mit Eintrag im
  Löschprotokoll, nicht mehr der Aufräumlauf des Betreuungsplans. Dessen
  Methode `CleanupExpiredFeedTombstones` löscht danach nur noch die Absagen im
  Dienstplan-Kalender und zählt als einfache Weiterleitung; das Budget der
  Weiterleitungen von `modules/schoolcalendar` steigt dafür um eins (ADR 0057
  Punkt 5). Ebenso steigen das Budget der Weiterleitungen von
  `modules/communication` um eins und das Zeilenbudget seiner Komposition um
  vier: Die Komposition reicht die Löschung der Elternbriefe an ihren Service
  weiter, denselben Weg wie das Löschen von Hand.
- Die Klasse `school deletes` ist keine Hintertür für neue Tabellen: Sie
  verlangt einen Löschweg der Schule und eine Begründung, warum keine Frist
  passt.
