# Elternnachrichten: Zahl pro Person, Lesebestätigung nur beim Öffnen

Status: accepted. Gilt für den Posteingang `Nachrichten` im OGS-Portal aus
[#3673](https://github.com/moto-nrw/project-phoenix/issues/3673). Baut auf
[#3654](https://github.com/moto-nrw/project-phoenix/issues/3654) (Team-Markierung
„ungelesen“) und
[#3663](https://github.com/moto-nrw/project-phoenix/issues/3663) („Alle als
gelesen markieren“) auf. Recherche:
[team-inbox-lesestatus-2026-09-30.md](../research/team-inbox-lesestatus-2026-09-30.md).

## Kontext

In einer OGS beantwortet oft eine Koordinatorin die Elternnachrichten. Die
Kolleginnen und Kollegen sehen trotzdem die Zahl bei `Nachrichten`. Seit #3663
klicken sie regelmäßig „Alle als gelesen markieren“, damit die Zahl
verschwindet. Dieses Sammel-Lesen bewegte den Lesecursor der Person. Aus den
Lesecursorn des Teams entsteht die Lesebestätigung der Eltern („Von der OGS
gelesen“). Eltern sahen also „gelesen“, obwohl die zuständige Person die
Nachricht nie gesehen hatte.

Die Recherche zeigt: „Gelesen“ ist in geteilten Posteingängen fast immer
persönlich. Wer die Zahl sieht, hängt an Zuständigkeit oder an einer
persönlichen Einstellung. In moto richten sich die Push-Benachrichtigungen zu
Elternnachrichten schon nach Zuständigkeit (Betreuende der Gruppe des Kindes
und Leitung) und nach persönlicher Einwilligung. Die Zahl in der Seitenleiste
tat das nicht.

## Entscheidung

1. **Die Zahl ist eine persönliche Einstellung.** Jede Person wählt im Profil
   unter „Zahl bei Nachrichten“, was ihre Zahl zählt: alle Nachrichten (Standard),
   nur Kinder aus den eigenen Gruppen (Gruppen des eigenen Lehrerprofils und
   heutige Gruppenvertretungen) oder keine. Gespeichert pro Schule und Konto in
   `users.parent_message_count_preferences` (Owner Communication). Keine
   Schul-Einstellung, keine Zuweisung pro Unterhaltung.
2. **Die Einstellung gilt nur für die Zahl.** Sie wirkt auf die Zahl in der
   Seitenleiste und auf der Startseite, und damit auch auf den Wert, den „Alle
   als gelesen markieren“ zurückgibt. Posteingang, Filter „Nur ungelesen“, die
   Markierung an der Unterhaltung und die Karte im Kinderprofil bleiben
   unverändert. Nichts verschwindet aus dem Posteingang.
3. **Die Einstellung gilt auch für Team-Markierungen.** Bei „keine“ zählt eine
   vom Team als ungelesen markierte Unterhaltung nicht. Bei „eigene Gruppen“
   zählt sie nur für Kinder dieser Gruppen.
4. **Nur echtes Öffnen setzt die Lesebestätigung.** „Alle als gelesen
   markieren“ bewegt eine eigene persönliche Grenze
   (`cleared_up_to_at`/`cleared_up_to_message_id` an `users.parent_message_reads`),
   nicht den Lesecursor. Die Grenze zählt für alle eigenen Ungelesen-Zahlen,
   nie für die Lesebestätigung der Eltern.
5. **Ein Menüeintrag ohne Wirkung steht nicht im Menü.** „Alle als gelesen
   markieren“ erscheint nur, wenn die eigene Zahl oder der Posteingang etwas
   Ungelesenes zeigt. Das Menü führt außerdem zur Einstellung („Zahl bei
   Nachrichten einstellen“).

Unverändert bleiben: Eine Antwort erledigt die Unterhaltung für das ganze Team.
Die Team-Markierung aus #3654 wirkt weiter für alle, bis jemand öffnet oder
antwortet. Die Zahl für Anfragen ist nicht betroffen.

## Folgen

- Kolleginnen und Kollegen ohne Elternnachrichten stellen die Zahl einmal ab und
  müssen nicht mehr täglich klicken.
- Eltern sehen „gelesen“ nur, wenn wirklich jemand die Unterhaltung geöffnet hat.
  Das Aufräumen der eigenen Zahl erzeugt keine falsche Bestätigung mehr.
- Wer „keine“ wählt, bemerkt neue Nachrichten nur im Posteingang. Das ist
  gewollt und steht im Hilfeartikel. Ein Risiko bleibt, wenn alle Personen
  einer Schule „keine“ wählen. Eine Schul-Einstellung dagegen ist nicht gebaut.
  Sie wäre der nächste Schritt, falls das in der Praxis vorkommt.
- „Eigene Gruppen“ zählt bei einer Person ohne Gruppe nichts. Die Einstellung
  sagt das an der Stelle, an der man sie wählt.
