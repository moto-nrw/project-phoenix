# Datenverwaltung und Alle Kinder sind getrennte Bereiche

Status: accepted (#2826). Gilt für die Mitarbeiter-Navigation des OGS-Portals
(Desktop-Seitenleiste, eingeklappter Streifen, mobiles Mehr-Menü). Die
Gliederung der Navigation selbst steht in ADR 0009.

## Kontext

Das OGS-Portal hat zwei Listen aller Kinder, die beide „Kinder" hießen:

- `Alle Kinder` (`/students/search`): der laufende Tag. Wer ist da, wo ist ein
  Kind gerade, An- und Abmelden, Sammelaktionen, Abwesenheiten, Export,
  fünfzehn Filter, Einstieg in die Kinderkartei. Sichtbar für alle
  Mitarbeitenden. Keine Stammdaten-Bearbeitung.
- `Datenverwaltung -> Kinder` (`/database/students`): der Datensatz. Kind
  anlegen, importieren, löschen, Betreuung beenden und fortsetzen,
  Klassenlisteneinträge, Jahrgangswechsel. Nur für Admins. Ein einziger
  Filter (Gruppe).

Im Testdurchlauf zu #2826 fiel auf: „In der Datenverwaltung kann man nicht
alles machen, was unter Alle Kinder geht." Das ist keine Lücke, sondern die
Absicht, nur sagte es die Oberfläche nicht. Beide Einträge standen in einer
flachen Liste, der eine hieß „Kinder", der andere „Alle Kinder", und nichts
erklärte den Unterschied.

Zur Wahl standen: die beiden Bereiche zusammenlegen (Anlegen, Import und
Löschen nach `Alle Kinder` holen), sie sichtbar trennen und die Grenze
benennen, oder den ganzen Bereich in „Stammdaten" umbenennen.

## Entscheidung

- Die beiden Bereiche bleiben getrennt. `Alle Kinder` ist der Tagesbetrieb,
  die Datenverwaltung ist der Datensatz. Eine Zusammenlegung hätte
  Admin-Funktionen (Anlegen, Import, Jahrgangswechsel, Löschen) in eine
  Fläche gebracht, die jede Betreuungskraft mehrmals täglich benutzt, und
  die Rechteprüfung vom Bereich auf einzelne Schaltflächen verlagert.
- Der Name „Datenverwaltung" bleibt. Die Schulen kennen ihn, die Hilfe ist
  voll davon, und das Issue verlangt keine Umbenennung.
- Der Unterpunkt und die Kachel heißen **Kinderdaten**, nicht „Kinder". So
  gibt es im Portal nur noch einen Eintrag, der „Kinder" heißt, und der
  steht im Tagesbetrieb.
- Die Grenze steht auf der Seite selbst, nicht nur in der Hilfe: die
  Datenverwaltungs-Übersicht sagt in einem Satz, dass hier angelegt und
  gepflegt wird und der laufende Tag unter `Alle Kinder` steht, mit Link
  dorthin.
- In der Seitenleiste steht `Alle Kinder` in der Gruppe Tagesbetrieb, die
  Datenverwaltung in der Gruppe Verwaltung (ADR 0009). Die beiden Listen
  stehen damit nie direkt untereinander.

## Konsequenzen

- Wer eine Kinderfunktion baut, ordnet sie einer der beiden Flächen zu:
  Tagesgeschäft (Anwesenheit, Status, Filter, Sammelaktionen) nach
  `Alle Kinder`, Datensatz (anlegen, ändern, löschen, Import) nach
  `Kinderdaten`.
- Ein späteres Zusammenlegen ist nicht ausgeschlossen, braucht aber eine
  eigene Entscheidung, weil es die Rechteprüfung verschiebt.
- Die Hilfe spricht von `Datenverwaltung -> Kinderdaten`; der Screenshot
  der Datenverwaltung zeigt die neue Bezeichnung.
