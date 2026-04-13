# Iteration 1 — Timetable System RFC Review Notes

## `is_spontaneous` ist redundant

`schedule.activity_instances.is_spontaneous` kann aus `activity_group_id IS NULL` abgeleitet werden. Wenn eine Instance kein Template hat, ist sie per Definition spontan.

Zwei Optionen:

**A) Feld entfernen, NULL-Check nutzen**
- Weniger Felder, kein Sync-Risiko (jemand setzt `is_spontaneous = false` aber `activity_group_id = NULL`)
- Query: `WHERE activity_group_id IS NULL` statt `WHERE is_spontaneous = true`
- Nachteil: weniger lesbar, Intent nicht sofort klar

**B) Feld behalten als explizites Flag**
- Lesbarere Queries, klarer Intent
- Ermöglicht Sonderfall: spontane Aktivität die nachträglich einem Template zugeordnet wird (z.B. "das war so gut, machen wir jetzt jede Woche")
- Nachteil: redundante Daten, muss konsistent gehalten werden

Tendenz: **B behalten**. Der Sonderfall "spontan wird zu Template" ist ein realistisches Szenario. Betreuer macht spontan Yoga, läuft gut, Admin macht daraus eine wiederkehrende AG. Dann hat die ursprüngliche Instance `is_spontaneous = true` UND `activity_group_id` gesetzt (nachträglich verlinkt). Ohne das Flag verlieren wir die Info, dass der Ursprung spontan war.

---

## Das Timetable-System in einfacher Sprache

### Was schon da ist

**Aktivitäten** (`activities.*`)
Stell dir einen Ordner vor mit Karteikarten. Jede Karte ist eine Aktivität: "Fußball AG", "Lernzeit Jg.3". Auf der Karte steht: welcher Raum, welche Kinder sind angemeldet, welcher Betreuer ist zuständig, und an welchen Wochentagen findet das statt. Das ist die **Planung auf Papier** — was jede Woche passieren soll.

**Live-System** (`active.*`)
Das ist die **Realität vor Ort**. Jemand macht einen Raum auf ("Session starten"), Kinder kommen rein und werden abgehakt. Das System weiß: Kind X ist gerade in Raum Y, seit 14:03. Wenn das Kind geht, wird die Zeit gestempelt. Das ist das, was heute schon über NFC läuft.

**Problem:** Die beiden kennen sich nicht. Die Planung weiß nicht, was live passiert. Das Live-System weiß nicht, was geplant war. Es gibt keine Brücke.

---

### Was sich ändert

**Aktivitäten** bekommen drei neue Infos auf der Karteikarte:
- **Typ**: Ist das eine AG, eine Betreuung (Mensa/Lernzeit), oder was Externes (DAZ/Musik)?
- **Klasse**: Gehört diese Aktivität zu einer bestimmten Schulklasse?
- **Vorlage ja/nein**: Ist das eine wiederkehrende Sache oder einmalig?

Sonst bleibt alles gleich. Keine bestehende Logik wird angefasst.

---

### Was neu dazukommt

**1. Tageseinträge** (`schedule.activity_instances`)

Das ist der **Tagesplan an der Wand**. Jeden Freitag nimmt das System automatisch alle Karteikarten (Aktivitäten) und schreibt für die nächste Woche konkrete Einträge:

> "Montag, 15. September: Lernzeit Jg.3A, 13:45-14:30, Raum 4, Franziska"

Das ist eine **Kopie** der Karteikarte für einen bestimmten Tag. Die Kopie kann man ändern — z.B. Raum tauschen oder Franziska durch Lisa ersetzen weil Franziska krank ist — ohne die Original-Karteikarte anzufassen.

Spontane Aktivitäten sind einfach Tageseinträge ohne Karteikarte dahinter. Ein Betreuer sagt mittags: "Ich mach jetzt Yoga." → neuer Tageseintrag, fertig.

**2. Betreuer pro Tageseintrag** (`schedule.instance_staff`)

Wer ist am konkreten Tag für diesen Eintrag zuständig? Normalerweise kopiert vom Template, aber änderbar (Vertretung).

**3. Kinder pro Tageseintrag** (`schedule.instance_students`)

Welche Kinder werden erwartet? Hier passiert der **Plan-vs-Realität-Abgleich**: 15 Kinder erwartet, 13 eingecheckt, 1 entschuldigt, 1 fehlt unentschuldigt.

**4. Schulstundenplan** (`schedule.class_timetable`)

Einfache Tabelle: Klasse 3b hat montags 5 Stunden, Schule endet 12:45, OGS erwartet die Kinder ab 12:50. Damit weiß das System, wann welches Kind ankommen sollte.

**5. Ausnahmen** (`schedule.class_timetable_exceptions` + `schedule.activity_exceptions`)

Wandertag, Hitzefrei, AG fällt aus. Einträge die sagen: "An diesem Datum weicht es vom Normalplan ab."

---

### Wie die drei Schichten zusammenspielen

```
PLANUNG (existiert)          TAGESPLAN (neu)              VOR ORT (existiert)

"Lernzeit Jg.3A"            "Mo 15.09., 13:45"           Raum 4 ist offen
 jeden Mo+Mi                  Raum 4, Franziska            Kind A ✓ 13:47
 Raum 4, Franziska            15 Kinder erwartet           Kind B ✓ 13:48
 15 Kinder                    Status: aktiv ──────────►    Kind C ✓ 13:51
                                                           Kind D ✗ fehlt
        │                            │                          │
        │  Freitag automatisch       │  Betreuer klickt        │
        │  kopiert für nächste       │  "Starten"              │
        │  Woche                     │                          │
        ▼                            ▼                          ▼
    TEMPLATE                     INSTANCE                    LIVE
    (Wochenplan)              (konkreter Tag)          (Echtzeit Check-in)
```

**Der Ablauf im Alltag:**

1. Admin legt einmal die Karteikarten an (Template)
2. System kopiert jeden Freitag die Woche (Instanzen)
3. Büro passt an wenn nötig (Vertretung, Raumtausch)
4. Betreuer öffnet morgens die App, sieht "Mein Tag"
5. Betreuer startet Aktivität → Live-System springt an
6. Kinder werden abgehakt → Check-in wie gehabt
7. Am Ende: Plan vs. Realität vergleichbar

**Das Schöne:** Schritt 5-6 ist exakt das, was heute schon funktioniert. Wir packen nur die Planungsschicht davor.
