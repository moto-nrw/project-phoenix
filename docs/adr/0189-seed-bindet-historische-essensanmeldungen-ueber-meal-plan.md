---
status: proposed
---

# Der Entwicklungsseed bindet historische Essensanmeldungen über Meal Plan

Für #6343 braucht der frische Seed einen abgeschlossenen Mensa-Monat mit
Anmeldungen, Anwesenheit und ausgestellten Rechnungspositionen. Die reguläre
Schul-API verbietet Änderungen vergangener Essenstage zu Recht. Diese Grenze
bleibt bestehen.

Der Dev-Tools-Prozess darf deshalb genau `modules/mealplan/compose` binden,
analog zu seinem bestehenden History-Writer von Student Presence. Meal Plan
validiert vergangene Kalendertage und schreibt innerhalb der Schultransaktion
über seinen privaten Postgres-Adapter. Der Seed registriert nur synthetische
Kinder mit einer bereits vorhandenen Essensanmeldung. Bestehende Abmeldungen
und aktuelle Pläne bleiben erhalten. Freigabe und Rechnungsstellung nutzen
weiter die regulären APIs.

Policy-Epoche 99 ergänzt nur diesen endlichen Kompositionszugriff von
`dev-tools/compose`, mit exaktem Zielpaket und ausschließlich im
Produktions-Importscope. Der Akzeptor verlangt die erhöhte Epoche und lehnt
andere Owner, Rollen, Pakete, Klassen und Testscopes ab. Er gewährt weder
fremdes SQL noch eine allgemeine Tool-Kompositionsberechtigung.
Die Coverage-Ausnahmen für Monatskinder und Mensa-Positionen entfallen.

Alternativen: Eine rückdatierbare Schul-API würde eine Produktinvariante für
Seed-Zwecke öffnen. SQL im Seed würde den Daten-Owner umgehen. Beide werden
verworfen. Die Entscheidung folgt ADR 0067/0068 und der CLI-Komposition aus
ADR 0060; sie schafft keinen neuen Owner und keine neue Tabelle.

Beide Meal-Plan-Konstruktoren verwenden denselben Transaktionsadapter. Der
bisherige Callback in `compose.New` wird als genau eine Grenze nach
`transaction.Run` verschoben; der Transaktionsregister-Eintrag behält seine
Begründung. Ein begrenzter Relokationsnachweis verlangt den alten Eintrag,
dessen Entfernung und unveränderte Begründung. Er erlaubt weder eine Kopie
noch einen weiteren Scope. Das Register wächst nicht (1 → 1).

Die gebundenen Settings-Resolver und ihre Validierung gehören der Anwendung;
die Komposition behält Alias und Konstruktor. Damit bleibt die Komposition
mit dem History-Konstruktor unterhalb ihres bisherigen LOC-Budgets (105 → 102; Gesamtsumme 13173 → 13170).
