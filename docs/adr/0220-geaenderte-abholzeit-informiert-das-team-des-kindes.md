---
status: proposed
---

# Geänderte Abholzeit informiert das Team des Kindes

Entscheidung zu
[#6855](https://github.com/moto-ogs/project-phoenix/issues/6855). Die
Vorgaben hat Yannick Wenger am 12.11.2026 festgelegt; die Freigabe erteilt er
mit dem Merge des Pull Requests, der diese ADR einführt. Sie baut auf
[ADR 0219](0219-kindernotiz-informiert-das-team.md) auf und führt keinen neuen
Owner ein.

## Kontext

Eltern ändern die Abholzeit für einen einzelnen Tag per Anfrage; erst die
Freigabe der OGS macht sie gültig. Das Team, das das Kind übergibt, erfuhr
davon bisher nur, wenn jemand die Kinderakte öffnete. Ein kurzer Hinweis in
der App (sechs Sekunden) reicht nicht: wer gerade nicht hinsieht, verpasst
ihn.

## Entscheidung

- **Anlass.** Der Hinweis entsteht bei der Freigabe einer Tagesanfrage mit
  Uhrzeit und bei einer Korrektur, die sie zurücknimmt („wieder normale
  Abholzeit“), nicht beim Absenden. moto zeigt nie eine noch nicht gültige
  Zeit als aktuell. Vergangene Tage sind keine Neuigkeit.
- **Empfänger.** Das Team des Kindes: die Betreuung seiner Gruppe und die
  Leitung (Admins) der Schule, beschränkt auf Konten mit `users:read`, dem
  Recht, Abholzeiten zu lesen. Wer entschieden hat, bekommt keinen Hinweis.
  Die Regel „wer ist zuständig“ bleibt bei Delivery
  (`contract.ResponsibleStaff`); sie wendet weder eine persönliche Wahl noch
  die Dienst-Regel an. Die Schule ist die Mandantengrenze.
- **Ungelesen-Markierung in moto.** Je Anfrage und Empfänger eine Zeile in
  `schedule.pickup_change_team_notices` (Care Plan). Sie trägt IDs, den Tag
  und zwei Zeitpunkte; Uhrzeit und Stand bleiben an der Anfrage, eine
  Korrektur erreicht die Markierung also über die Anfrage. Eine Korrektur
  setzt die Zeile wieder auf ungelesen. Die Zeile geht mit ihrer Anfrage und
  mit der Mitgliedschaft der Person an der Schule (`auth.account_tenants`),
  also auch mit der Demo-Löschung; die Schul-Löschung kennt die Tabelle.
- **Ein Zähler.** Die Kopfzeile zeigt geteilte Notizen (ADR 0219) und
  geänderte Abholzeiten zusammen unter „Neu für Sie“ mit einem roten Zähler,
  auf dem Handy am Knopf „Mehr“. Eine Abholzeit-Zeile nennt den Vornamen
  des Kindes und öffnet das Kind. In der Kinderakte trägt der ungelesene Tag
  „Neu“; das Öffnen des Kindes bestätigt nur die Hinweise, die dabei geladen
  waren, wie bei den Notizen.
- **Gerät.** Über `owners.StaffPushNotifier` (ADR 0219): Delivery-Typ
  `student_pickup_changed` (Gruppe „Abholungen“, für `users:read`,
  voreingestellt an, je Typ abschaltbar). Abschalten wirkt nur auf die
  Mitteilung am Gerät; die Markierung in moto bleibt. Der Text nennt kein
  Kind (Sperrbildschirm, geteilte Tablets), nur Tag und Uhrzeit.
- **Ein Weg für voreingestellte Typen.** Der parallele Empfängerfilter, der
  voreingestellte Typen in der Delivery-Empfängerauflösung zustellte,
  entfällt. Wer abgeschaltet hat, filtert allein `FilterNotOptedOut` im
  gemeinsamen Staff-Push.
- **Atomar mit der Entscheidung.** Markierungen und Push-Eintrag entstehen
  in der Transaktion der Entscheidung. Scheitert eins, scheitert die
  Entscheidung; moto meldet nie eine Freigabe, von der das Team nichts
  erfahren hat. Schule ohne Mitteilungen oder außerhalb des Fensters: kein
  Push, die Markierung reicht.

## Verworfene Alternative: eine gemeinsame Hinweistabelle

Bevorzugt wäre eine allgemeine Tabelle für Hinweise an das Personal mit Typ,
in die auch die Notiz-Markierungen aus ADR 0219 umziehen. Sie ist hier
unverhältnismäßig:

- Beide Markierungen leben von ihrem Gegenstand. Die Notiz-Markierung
  verschwindet, wenn die Notiz gelöscht oder enger gestellt wird; die
  Abholzeit-Markierung zeigt den aktuellen Stand der Anfrage und
  verschwindet mit ihr. Eine gemeinsame Tabelle bräuchte einen eigenen
  Owner und müsste Notizen (People Directory) und Anfragen (Care Plan)
  fremd lesen oder bei jeder Änderung von beiden benachrichtigt werden: neue
  Cross-Owner-Wege und eine zweite Wahrheit über Sichtbarkeit und Stand.
- Der Umzug der gerade eingeführten Notiz-Markierungen bräuchte eine
  Datenmigration, neue Schul-Löschfunktionen und neue Routen.

Gewählt ist das kleinste Design mit **einer** Liste und **einem** Zähler:
jeder Owner hält die Markierung zu seinem Gegenstand, die Oberfläche führt
beide in „Neu für Sie“ zusammen, und das Lesen folgt demselben Muster
(bestätigt werden nur die geladenen Revisionen). Eine dritte Hinweisart
sollte die gemeinsame Tabelle neu prüfen.

## Grenzen

Care Plan besitzt Anfrage und Markierung und entscheidet, wann ein Hinweis
entsteht. Wer zuständig ist und wie die Person am Gerät davon erfährt, bindet
die Root (`owners.pickupChangeTeamReach`) über Delivery; beide entstehen im
Owner-Graphen nach Care Plan (ADR 0058). Der Dienst öffnet keine
Transaktion, sondern verlangt die der Entscheidung oder der Schülerroute.

Der Konstruktor `NewPickupChangeTeam` in `modules/careplan/compose` braucht
genau 13 LOC; diese ADR hebt das Compose-Budget von Care Plan um diesen
Betrag (ADR 0057 Punkt 5).

## Command Effects

Die neuen Einträge sind Plattformteilnahme (dauerhafter Push-Eintrag,
ADR 0098) oder reine Lesezugriffe (Zuständigkeit und Recht über Delivery,
Abmeldungen vom Benachrichtigungstyp).
