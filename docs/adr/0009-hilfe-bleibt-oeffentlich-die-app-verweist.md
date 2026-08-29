# Der Hilfebereich bleibt öffentlich, die App verweist in das richtige Thema

Status: accepted. Gilt für den Hilfebereich unter `/help` in allen vier
Portalen. Ergänzt ADR 0008, der Format und Plattform festlegt; hier geht es um
Zielgruppen und Personalisierung. Entstanden aus der Recherche zu #2229.

## Kontext

Der Wunsch, jeder Rolle nur ihre eigene Anleitung zu zeigen, ist berechtigt:
`/help/features` führt rund 60 Themen, von denen für eine einzelne Person nur
ein Bruchteil gilt. Die naheliegende Umsetzung — Hilfe nach angemeldeter Rolle
und nach Mandanteneinstellungen filtern — hat jedoch Folgen, die im Issue nicht
benannt sind.

Stand am 29.08.2026:

- `/help` ist **öffentlich und ohne Login erreichbar**; `proxy.ts` lässt den
  Pfad auf allen Hosts durch. Daran hängen drei Dinge: die PDF-Erzeugung
  rendert per Playwright genau diese öffentlichen Seiten, Links sind teilbar
  (eine Leitung schickt die Anleitung an eine Person ohne Konto), und alle
  Portale zeigen auf dieselbe Quelle. Eine rollen- oder mandantenabhängige
  Hilfe ist authentifiziert — PDF, teilbare Links und statisches Rendern fallen
  damit.
- Verlinkt wird die Hilfe heute aus dem Tenant-Portal
  (`mobile-bottom-nav.tsx`, `section-navigation.ts`) und aus dem Schul-Portal
  (`school-nav-items.ts:86`). **Das Eltern-Portal hat keinen Hilfebereich.**
- Eine Lehrkraft kann im Schul-Portal genau zwei Dinge tun (Klassenansicht,
  Nachrichten), landet über `school-nav-items.ts` aber auf der vollständigen
  OGS-Anleitung. Ihre drei Themen liegen in `appChapters` zwischen 60 anderen.
- Die zwei von drei dokumentierten Missverständnisfällen in
  `.claude/rules/verstaendlichkeit.md` betreffen das Eltern-Portal (#2296
  „AGs und Gruppen" wird als Anmeldung gelesen, #2297 Push ohne installierte
  App) — also die Zielgruppe ohne jede Hilfe.
- Im Tenant-Portal stehen sich **Leitung (`admin`) und Betreuungskraft
  (`isCaregiver`, Rollen `user`/`teacher`)** gegenüber. Der Unterschied ist
  kapitel-, nicht satzförmig: die tägliche Arbeit ist identisch (Kinder,
  Aktivitäten, Räume, Gruppen, Aufsicht, Zeiterfassung), der Unterschied steckt
  in ganzen Zusatzbereichen (Datenverwaltung mit 10 Unterpunkten, Einstellungen,
  Anmeldungen, Statistik). Von rund 17 Einträgen der Hauptnavigation sind nur
  drei `requiresAdmin`.
- **Der Doppelfall ist real, nicht theoretisch.** `sidebar.tsx:568`
  (`if (item.hideForAdmin && userIsAdmin && !userIsCaregiver) return false;`)
  existiert allein, um Personen zu behandeln, die Leitung *und* Betreuungskraft
  zugleich sind. Die Leitung ist damit auch keine saubere Obermenge — es gibt
  Einträge, die eine reine Leitung nicht sieht.
- Es gibt rund 86 registrierte Einstellungen. Eine Filterung nach allen erzeugt
  eine Variantenmenge, die niemand testet, und ein PDF je Schule.
- Die App blendet Navigationsbereiche bereits nach Fähigkeiten aus:
  `sidebar.tsx:569-570` prüft `nfcEnabled` und `isBinaryMode`. Entsprechende
  Schalter müssen für die Hilfe also nicht erfunden, sondern nachgenutzt werden.
- `operations.presence_mode` ist der Sonderfall: im Modus `binary` gibt es laut
  Cross-Repo-Vertrag am Kiosk keine Raumauswahl, keinen Raumwechsel und keine
  WC-Taste. Die NFC-Anleitung nennt „Schulhof" 15-mal, „Raumwechsel" 5-mal und
  „WC" 3-mal, den Modus selbst genau einmal in einem Einstellungs-Thema. Für
  eine Schule im Binär-Modus ist die Anleitung damit nicht überflüssig, sondern
  **falsch**.

## Entscheidung

- **Die Hilfe bleibt öffentlich und ohne Login.** Personalisiert wird nicht der
  Hilfebereich, sondern der Weg dorthin.
- **Die Grenze der Inhaltsbestände folgt dem Portal, nicht der Rolle.** Es gibt
  drei Bestände: OGS-Team (Tenant-Portal), Lehrkraft (Schul-Portal), Eltern
  (Eltern-Portal). Das Portal ist bereits eine harte Grenze — eigene Session,
  eigener Host, eigenes Publikum, eigene Sprache. Die Rolle ist keine solche
  Grenze: sie trennt Einstiege, nicht Bestände (siehe nächster Punkt). Die Zahl
  der Einstiege ist damit höher als die der Bestände.
- **Leitung und Betreuungskraft bekommen getrennte Einstiege auf einem
  gemeinsamen Inhaltsbestand.** Getrennt ist, was die Person sieht; gemeinsam
  ist, was wir pflegen. Die Trennung darf dabei **bis zu eigenen
  Einstiegskarten** gehen: „Für Betreuungskräfte" und „Für die Leitung" auf
  `/help`, je mit eigener Startseite und eigener Themenreihenfolge, sodass es
  sich wie zwei Anleitungen liest. Das ist ausdrücklich erwünscht und nicht die
  schwächere Variante — eine Betreuungskraft soll DATEV-Abrechnung,
  Datenverwaltung und Anmeldephasen nicht zwischen ihren Themen finden, denn
  das sind Funktionen, die sie nicht erreichen kann
  (`verstaendlichkeit.md`: „State without action is explained").
- **Zwei Inhaltsbestände wären der Fehler, nicht zwei Einstiege.** Grob ein
  Drittel der 108 Themen ist leitungsspezifisch (Verwaltung, Einstellungen,
  Anmeldungen, Info-Displays), zwei Drittel sind gemeinsam — Kinder,
  Aktivitäten, Räume, Gruppen, Aufsicht, Zeiterfassung, App-Installation
  (Schätzung aus der Kapitelliste, keine Messung). Bei getrennten Beständen
  würde diese Mehrheit entweder dupliziert und driftet auseinander — unter
  einem Regime, in dem 88 % der Guide-Änderungen nebenbei im Feature-PR
  passieren (ADR 0008) — oder quer verlinkt, womit es wieder ein Bestand mit
  zwei Einstiegen ist, nur mit zusätzlicher Mechanik. Hinzu kommt: wegen
  `hideForAdmin` ist die Leitung keine Obermenge, eine reine Leitungs-Anleitung
  wäre also ausgerechnet dort unvollständig, wo die Leitung eine neue
  Betreuungskraft einarbeitet.
- **Die Grenze der Trennung: erreichbar bleibt alles.** Themen der anderen
  Rolle stehen nicht zwischen den eigenen, aber sie sind von jedem Einstieg aus
  sichtbar erreichbar — etwa als „Weitere Themen für die Leitung" am Ende, und
  über die Suche, die immer den ganzen Bestand durchsucht. Gründe: der belegte
  Doppelfall Leitung + Betreuungskraft; dass wer über eine fremde Rolle liest,
  meist gerade nicht in ihr ist (Einarbeitung, Fehlersuche für Kollegen); und
  dass stilles Ausblenden die Sackgasse erzeugt, die `verstaendlichkeit.md`
  verbietet — wem gesagt wird „steht in der Hilfe", der darf nicht davorstehen,
  ohne zu erkennen, dass es nur woanders einsortiert ist.
- **Der primäre Personalisierungsmechanismus ist der kontextuelle Deep-Link aus
  der App.** Die App kennt Rolle und Einstellungen bereits; der Hilfebereich
  muss beides nicht wissen. Ein Hilfe-Verweis auf dem Betreuungsplan springt in
  das Thema Betreuungsplan. Voraussetzung ist die Route pro Thema aus ADR 0008.
- **Die Rollenvorbelegung reist im Link, nicht in der Session** (etwa
  `?rolle=betreuungskraft`). Damit bleibt die Hilfe statisch, öffentlich und
  cachebar, obwohl sie vorbelegt ankommt.
- **Keine Filterung nach den 86 Einstellungen.** Erlaubt ist eine kleine,
  kuratierte Liste von Schaltern, die entscheiden, ob ein Bereich für eine
  Schule überhaupt existiert — Kandidaten: `attendance.nfc_enabled`,
  `enrollment.enabled`, `display.enabled`, `operations.presence_mode`; die
  ersten und der letzte steuern in `sidebar.tsx` bereits die Navigation. Bei
  „nicht gebucht" wird **gekennzeichnet, nicht versteckt**. Nur
  `presence_mode` wechselt tatsächlich Inhalt, weil dort falsch schlimmer ist
  als abwesend.

## Folgen

- Eltern-Hilfe und Lehrkraft-Hilfe sind **neue Zielgruppen, keine Umstruktu-
  rierung**. Sie gehören in eigene Issues neben #2229, sonst wächst das Epic
  weiter. Die Eltern-Hilfe hat dabei das beste Verhältnis von Aufwand zu
  belegtem Nutzen, weil sie auf gezählte Supportfälle zielt.
- `school-nav-items.ts:86` wird auf die Lehrkraft-Anleitung umgehängt. Die drei
  Lehrkraft-Themen wandern aus `appChapters` dorthin.
- Das PDF bleibt möglich, aber es gibt es künftig **je Zielgruppe, nicht je
  Schule**. Ein mandantenspezifisches PDF ist mit dieser Entscheidung
  ausgeschlossen.
- Die NFC-Anleitung braucht eine sichtbare Verzweigung nach Anwesenheits-Modus.
  Bis dahin ist sie für Schulen im Binär-Modus fehlerhaft; das ist ein Defekt,
  kein Komfortthema.
- Die Vorbelegung braucht eine Zuordnung Kapitel → Rolle. Sie ist billig, aber
  sie veraltet: ein neues Kapitel ohne Zuordnung landet sonst stumm bei beiden
  Rollen ganz unten. Die Zuordnung gehört deshalb neben die Kapiteldefinition,
  nicht in eine separate Liste.
- Kontextuelle Deep-Links binden Themen-Routen an Bildschirme. Wird ein Thema
  umbenannt oder zusammengelegt, bricht der Verweis. Die Themen-Ids werden damit
  zu einem Vertrag zwischen App und Hilfe und brauchen dieselbe Sorgfalt wie
  eine Route — analog zum bestehenden Wächter `help-search-sync.test.ts`.
