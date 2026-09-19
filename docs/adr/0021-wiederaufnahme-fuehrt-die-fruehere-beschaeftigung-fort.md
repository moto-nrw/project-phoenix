---
status: accepted
---

# Eine Wiederaufnahme führt die frühere Beschäftigung fort

Der Supportfall in [#3376](https://github.com/moto-nrw/project-phoenix/issues/3376)
hat einen zweiten Fehler freigelegt, der in
[#3394](https://github.com/moto-nrw/project-phoenix/issues/3394) steht: Der
Personalaustritt trennt den Personensatz vom Konto, und wer später erneut
aufgenommen wird, bekommt einen zweiten Personensatz und einen zweiten
Staff-Satz. Die Historie bleibt an den alten hängen. Für Kinder ist dieselbe
Frage längst entschieden (ADR 0007, `CONTEXT.md`: „Eine Wiederaufnahme behält
seine Stammdaten und seine Historie"), für Personal gab es weder einen
Glossareintrag noch eine Regel.

Wir entscheiden: Ein Mensch hat an einer Schule **einen** Personensatz, der
seine Identität trägt und das Konto behält. Die **Beschäftigung** ist der
Staff-Satz, und davon kann es mehrere nacheinander geben. Beim Einladen wählt
die Schule zwischen **Wiederaufnahme** (die frühere Beschäftigung wird
fortgeführt) und **Neuaufnahme** (eine neue beginnt daneben). Ein zweiter
Personensatz für denselben Menschen an derselben Schule entsteht in keinem
der beiden Fälle.

## Warum die Ebene Person und die Ebene Beschäftigung getrennt bleiben

Auf dem Personensatz liegen Name, Transponder und Kontolink. Adresse, Telefon,
Notfallkontakt, Eintrittsdatum, Wochenstunden, Personalnummer, Qualifikationen,
Zeitkonto und Urlaubskonto hängen alle am Staff-Satz. Ein frischer Personensatz
bringt deshalb nichts Frisches; er erzeugt nur eine zweite Zeile mit demselben
Namen in jeder Personenliste. Alles, was eine Schule mit „neu anfangen" meint,
liegt eine Ebene tiefer.

## Entscheidungen

1. **Die Wahl trifft die Schule beim Einladen, nicht die eingeladene Person.**
   Erkennt das System zu der Adresse eine frühere Beschäftigung, zeigt es
   deren Zeitraum und lässt zwischen Wiederaufnahme und Neuaufnahme wählen.
   Die Wahl wird auf der Einladung festgehalten und beim Einlösen ausgeführt.
   Über Personalhistorie, Zeitkonto und Urlaub entscheidet die Schule; die
   eingeladene Person sieht davon beim Einlösen nichts. Berechtigung ist
   `users:manage`, dieselbe wie fürs Einladen heute.

2. **Der Austritt hört auf wegzuwerfen, was die Rückkehr braucht.** Er lässt
   den Kontolink stehen (bisher `People.UnlinkAccount`) und das
   Arbeitszeitmodell gesetzt (bisher `ClearWorkTimeModel`). Beides war
   unbegründet: kein ADR, kein Kommentar, kein Test trägt es, und kein
   Unique-Index erzwingt es. Ohne den Kontolink findet der Identitätspfad die
   Person nicht wieder, und für das Arbeitszeitmodell gibt es kein Audit, der
   alte Wert wäre endgültig verloren. Beide Änderungen öffnen keinen Zugang:
   die Kette Konto → Person → Staff filtert stillgelegte Staff-Sätze
   (`FindStaffByPerson` mit `deleted_at IS NULL`).

3. **Der Transponder bleibt gelöst.** Anders als Kontolink und
   Arbeitszeitmodell ist er ein Gegenstand. Wer geht, gibt die Karte ab, und
   sie wandert an die nächste Person. Bliebe sie verknüpft, blockierte
   `idx_persons_tenant_tag` die Neuvergabe an jemanden, dem sie physisch
   längst gehört.

4. **Zurücksetzen, nicht löschen.** Vor einer Wiederaufnahme kann die Schule
   drei Bereiche zurücksetzen: Stammdaten samt Personalnummer, Qualifikationen,
   und die Arbeitszeit aus Zeitkonto, Urlaubskonto und Arbeitszeitmodell. Die
   Arbeitszeit nur als Ganzes, weil ein volles Zeitkonto ohne Sollstunden
   gegen nichts rechnet. Zurücksetzen bewahrt den bisherigen Stand als
   Historie: `ResetBalance` schreibt eine `reset`-Buchung, die Jahreszeile
   bekommt `carryover_days = 0`, Stammdaten und Personalnummer stehen in ihren
   Audit-Tabellen. Qualifikationen bekommen dafür einen Soft-Delete, und zwar
   durchgängig für jedes Entfernen, nicht nur für die Wiederaufnahme: eine
   Spalte mit zwei Bedeutungen je nach Aufrufer erklärt später niemand mehr.

5. **Was nie zurückkommt, wird benannt statt angeboten.** Personaldokumente
   löscht der Austritt endgültig (ADR 0013). Künftige Schichten und
   Abwesenheiten löscht er hart; vergangene lässt er stehen, die kommen mit
   dem Staff-Satz ohnehin zurück. Gruppen, Klassen und Vertretungen werden nie
   automatisch wieder wirksam, die Schule setzt sie neu. Die Vorschau sagt das,
   statt einen Schalter dafür anzubieten.

6. **Ändert sich zwischen Einladen und Einlösen etwas, wird neu bewertet statt
   abgebrochen.** Ein Abbruch träfe die eingeladene Person, die für eine
   schulseitige Änderung nichts kann. Ist die Personalnummer inzwischen
   vergeben, wird sie geleert statt übernommen; die Wiederaufnahme läuft durch.
   Einzige Ausnahme: ist die Person bereits wieder aktiv beschäftigt, stellt
   die Einladung nur noch den Schulzugang her und legt keine zweite
   Beschäftigung an.

7. **Frühere Beschäftigungen bleiben auffindbar, wo Personal gesucht wird.**
   Ein Filter „Ehemalige" in der bestehenden Personalliste, die Detailseite im
   Lesemodus. Keine zweite Liste, kein neuer Menüpunkt. Gibt es mehrere
   frühere Beschäftigungen, bietet der Einladen-Dialog nur die jüngste an;
   eine ältere fortzusetzen ergibt keinen sinnvollen Zustand.

8. **Geltungsbereich.** Nur die Einladung im Schulportal bekommt die Wahl. Der
   Staff-Import hat die Zuordnung bereits über den `PersonID`-Pfad aus
   [#2600](https://github.com/moto-nrw/project-phoenix/issues/2600). Der
   Operator-Weg über Schulzugänge ist der Notfallweg und erbt nur die
   Wiedererkennung, ohne eigene Wahloberfläche.

## Verworfen

- **Wiederaufnahme still und ohne Wahl.** Wäre die kleinste Änderung, nimmt der
  Schule aber die Entscheidung über Zeitkonto und Urlaub ab, die ihr gehört.
  Bleibt als Zwischenstand der ersten Auslieferungsstufe bestehen, nicht als
  Endzustand.
- **Neuaufnahme legt einen zweiten Personensatz an.** Genau die Dublette, wegen
  der es das Issue gibt, nur absichtlich. Sie brächte nichts, weil auf dem
  Personensatz nichts liegt, was eine Schule zurücksetzen wollen würde.
- **Abgewählte Bereiche löschen statt zurücksetzen.** Nimmt der Schule, was sie
  für einen Nachweis über eine frühere Beschäftigungszeit später doch braucht.
  Der wirkliche Wunsch ist „soll nicht mehr gelten", nicht „soll weg sein".
- **Urlaubsanspruch anteilig auf den Rest des Jahres kürzen.** Wartezeit,
  Teiljahresansprüche und Vertragsart sind arbeitsrechtlich nicht eindeutig,
  und das System kennt die dafür nötigen Angaben nicht. Die Jahreszeile ist
  bearbeitbar, der volle Anspruch ist deshalb ein Startwert, keine Festlegung.
- **Beim Einlösen abbrechen, wenn sich etwas geändert hat** (das Muster des
  Austritts mit `ErrConflict`). Bestraft die falsche Person für eine Änderung,
  die sie nicht verursacht hat.
- **Bestandsdubletten in derselben Änderung auflösen.** Eine Zusammenführung
  zweier Personensätze ist ein eigener Eingriff mit eigenen Risiken für
  Zeiterfassung, Urlaubskonto und Auditverweise. Sie wird nach dieser
  Entscheidung entworfen, nicht neben ihr.

## Folgen

- Der stillgelegte Staff-Satz ist über den Identitätspfad heute **nicht
  erreichbar**: `ensureIdentityStaff` prüft zwar `!staff.Deleted`, die Abfrage
  dahinter filtert stillgelegte Sätze aber weg. Der Zweig ist toter Code. Die
  Wiederaufnahme braucht einen Lesezugriff, der sie bewusst mitnimmt;
  `StaffFilter.IncludeDeleted` kann das bereits, dieser Port nutzt es nicht.
- Der Einladen-Dialog braucht eine schulseitige Vorschau zu einer Adresse.
  Die bestehende Einladungsvorschau gehört zur Annahmeseite und beantwortet
  eine andere Frage.
- Das Merkmal „hat Konto" in Personenlisten stimmt nach Entscheidung 2 nicht
  mehr, wenn es aus dem Kontolink abgeleitet wird. Es muss aus dem Zustand der
  Beschäftigung kommen.
- Der Soft-Delete für Qualifikationen betrifft eine Tabelle, die in genau einer
  Datei gelesen wird (`workforce/internal/adapters/postgres/staffrecord_store.go`).
- Ausgeliefert wird nach [#3391](https://github.com/moto-nrw/project-phoenix/pull/3391)
  und [#3392](https://github.com/moto-nrw/project-phoenix/pull/3392), auf deren
  Einlösepfad beides aufsetzt, und in zwei Stufen: erst die beiden Stellen aus
  Entscheidung 2, dann die Wahl mit Oberfläche. Die zweite Stufe zieht
  Hilfe-Guide und Screenshots nach `.claude/rules/help-guide-sync.md` mit.

## Offen

Ob der Urlaubsanspruch bei einer Wiederaufnahme arbeitsrechtlich anteilig sein
müsste, ist hier bewusst nicht entschieden. Der volle Jahresanspruch ohne
Übertrag ist der Startwert, den die Schule korrigieren kann.
