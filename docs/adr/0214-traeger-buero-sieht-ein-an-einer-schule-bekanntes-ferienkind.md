---
status: accepted
---

# Träger-Büro sieht ein an einer Schule bekanntes Ferienkind

Vorschlag vom 11.11.2026 zu [#6683](https://github.com/moto-nrw/project-phoenix/issues/6683),
Option A des Issues. Er ergänzt
[ADR 0110](0110-oeffentliche-ferienanmeldung-schreibt-in-der-traeger-transaktion.md)
Punkt 4 und [ADR 0114](0114-ferienanmeldungen-pruefen-mit-eigenem-protokoll-in-der-traeger-transaktion.md)
und liest wie [ADR 0151](0151-schulansicht-des-traegers-liest-ihre-zahlen-ueber-die-fach-owner.md)
und [ADR 0154](0154-belegung-des-traegers-liest-gruppen-und-buchungen-ueber-die-fach-owner.md)
über die Fach-Owner. Er führt keinen neuen Owner und keine neue Transaktionsstelle ein.

## Kontext

Beim Prüfen einer Ferienanmeldung will das Träger-Büro wissen, ob es das Kind
schon kennt: Eltern melden ihr Kind oft als Feriengast an, obwohl es eine Schule
des Trägers besucht, oder nennen eine andere Schule. Davon hängen Vorrang,
Ferienwunsch (#4703) und Rückfragen ab. Bisher zeigte moto nur Doppelanmeldungen
zwischen Anmeldungen. Die Anmeldung selbst sucht nach ADR 0110 Punkt 4 keine
vorhandenen Kinder, damit Name und Geburtsdatum über den öffentlichen Link nichts
freischalten.

Die Träger-Transaktion sieht keine Schultabelle. Die Kinder der Schulen liegen
bei People Directory (Person, Schülerprofil) und School Membership
(Zugehörigkeit und Status).

## Entscheidung

1. **Nur ein Hinweis.** Die Prüfung zeigt bei einem Kind „bekannt an <Schule>“.
   moto verknüpft nichts, übernimmt keine Angaben und öffnet keine Daten des
   Kindes an der Schule. Die Antwort nennt je Kind nur die ID der Schule.
2. **Bekannt heißt eindeutig.** Ein Kind ist an einer Schule bekannt, wenn von
   allen Kindern, die die Schulen des Trägers heute nach der
   Stichtagszahl-Regel (#2791) aktiv führen, genau eines Vorname, Nachname und
   Geburtsdatum des angemeldeten Kindes hat. Namen vergleicht moto wie die
   Erkennung angemeldeter Kinder der Online-Anmeldung (#6053). Zwei oder mehr
   Treffer, an einer Schule oder an mehreren, geben keinen Hinweis. Die
   Definition steht an `traegerview.KnownChild`.
3. **Lesen über die Owner in der Admin-Einheit der Projektion.** Die Route
   `GET /traeger/holiday-periods/{id}/registrations/known-children`
   (optional `?registration_id=`) fragt vor der Träger-Transaktion die
   Träger-Projektion, wie der Export die Schulnamen (ADR 0114). Die Projektion
   liest in ihrer Admin-Einheit die Schulen des Trägers, bei Enrollment die
   Kinder der nicht stornierten Anmeldungen des Zeitraums
   (`HolidayRegisteredChildren`, Filter auf den Träger), bei People Directory
   ihre Namensgleichen unter den Schülerprofilen dieser Schulen
   (`StudentNamesakes`, `tenant_id = ANY(?)`) und bei School Membership die
   heute geführten Kinder der Schulen mit einem Treffer (`SchoolRosters`).
   Jede Abfrage liegt im Postgres-Paket ihres Owners und verweigert ohne die
   Admin-Einheit. Es gibt keinen Join über Owner.
4. **Protokoll vor der Antwort.** Danach schreibt die Route in der
   Träger-Transaktion einen Eintrag `known_children` in die Spur des
   Zeitraums, mit der angefragten Anmeldung und den IDs der gefundenen Kinder,
   nie mit Namen. Scheitert der Eintrag, antwortet die Route mit einem Fehler
   und zeigt keinen Hinweis. Auch ein Abgleich ohne Treffer steht im
   Protokoll. Das Zugriffsprotokoll des Trägers zählt den Eintrag als Lesen.
5. **Recht.** Nur das Recht „Anmeldung verwalten“ öffnet die Route, wie die
   übrige Prüfung. Ein Zeitraum eines anderen Trägers antwortet mit 404.
6. **Wurzel und Register.** Die Wurzel übersetzt die Kinder von Enrollment,
   die Namensgleichen von People Directory und die bekannten Kinder der
   Projektion (`api/traeger_known_children_ports.go`); die zwei Übersetzungen
   ohne eigene Regel stehen im Pass-through-Register. Die Konstruktoren heben
   das Compose-Budget von `modules/enrollment/compose` um 10 Zeilen und von
   `modules/peopledirectory/compose` um 13 Zeilen, zusammen um 23 Zeilen.

## Verworfene Optionen

- **Echte Verknüpfung mit dem Schülerdatensatz (Option B):** übernähme Angaben
  zwischen Anmeldung und Schule und bräuchte eine Rechtsgrundlage für den
  Austausch. Für die Prüfung reicht der Hinweis.
- **Nichts bauen (Option C):** Das Büro müsste jedes Kind von Hand bei den
  Schulen erfragen.
- **Abgleich beim Absenden der Anmeldung:** widerspricht ADR 0110 Punkt 4; die
  Antwort an die Eltern hinge von den Kindern einer Schule ab.
- **Hinweis in der Liste und im Detail der Prüfung:** Jede Änderung antwortet
  mit dem Detail und läse dann jedes Mal die Schulen. Die eigene Route liest
  nur, wenn die Seite den Hinweis zeigt, und protokolliert genau das.
- **Ein Statement über Personen, Profile und Zugehörigkeiten:** wäre ein Join
  über zwei Owner wie in ADR 0154 verworfen.

## Folgen

- Das Budget `api.traeger.holiday_known_children` liegt bei 9 Statements, flach
  in der Zahl der Kinder und Schulen.
- Name und Geburtsdatum aus dem öffentlichen Formular schalten weiterhin
  nichts frei: Erst das Büro mit dem Recht „Anmeldung verwalten“ sieht den
  Hinweis, und das Protokoll hält jeden Abgleich fest.
- Den Filter auf die Schulen des Trägers sichert
  `api/traeger_holiday_known_children_api_test.go` mit einem Namensgleichen an
  der Schule eines anderen Trägers.
- Wer „Anmeldung verwalten“ hat, kann Name und Geburtsdatum eines Kindes
  ändern und damit erneut abgleichen. Das Protokoll zeigt dann die Änderung
  (`edit` mit den geänderten Feldern) und jeden Abgleich (`known_children`)
  mit Person und Zeit. Das Recht hat nur das Träger-Büro, das die Kinder seiner
  Schulen ohnehin betreut.
- Ein Kind, das die Schule erst nach dem Stichtag aufnimmt (Status „pending“),
  gilt noch nicht als bekannt.
