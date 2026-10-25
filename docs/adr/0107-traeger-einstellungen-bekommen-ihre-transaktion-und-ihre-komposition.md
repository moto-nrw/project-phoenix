---
status: accepted
---

# Träger-Einstellungen bekommen ihre Transaktion und ihre Komposition

Entscheidung vom 25.10.2026 zu [#4652](https://github.com/moto-nrw/project-phoenix/issues/4652),
erster Bau-Schnitt von
[ADR 0086](0086-traeger-werte-liegen-zwischen-standard-und-schule.md).
ADR 0086 hat die Träger-Ebene im Settings-System beschlossen. Diese ADR hält
fest, was ihr erster Code in den Registern ändert, damit kein Register
stillschweigend wächst (ADR 0057 Punkt 5, ADR 0070), und welche Schlüssel
v1 auf die Träger-Ebene legt (ADR 0086 Abschnitt 4: „Die Liste je Schlüssel
legt das Bau-Ticket fest“).

## Entscheidung

1. **Träger-Transaktion.** Die Träger-Einstellungen nutzen die
   Träger-Transaktion, die
   [ADR 0105](0105-ferienzeitraum-liegt-beim-traeger-in-der-traeger-transaktion.md)
   eingeführt hat: `tenant.WithinTraeger`, Rolle `phoenix_tenant`,
   `app.current_org_id` aus der Träger-Sitzung, keine Schule. Sie bringen
   keine eigene Transaktionsart, keinen eigenen Adapter und keine zweite
   Bindung in der Transaction Runtime mit. `tenant.RequireTransaction`
   erkennt die Träger-Transaktion als Einheit, damit die Advisory-Sperre
   einer Änderung in ihr gilt.
2. **Öffnen und Register.** Anders als der Ferienzeitraum öffnen die
   Träger-Einstellungen die Träger-Transaktion selbst, nicht über
   `TraegerTxMiddleware`: Die Schulen erfahren von einer Änderung erst nach
   deren Commit (Punkt 5), und dafür braucht es sonst einen neuen Rückruf
   nach dem Commit, den `durable_side_effect_notices.json` nicht zulässt.
   Die Routen unter `/traeger/settings` nehmen die Middleware deshalb nicht.
   Das ist die eigene Entscheidung, die ADR 0105 Punkt 4 für jede weitere
   Stelle verlangt: `backend/test/traeger_transaction.go` und
   `transaction_boundaries.json` bekommen genau drei Stellen, die Bindung in
   `TenantSettingsRuntime.WithinTraeger` sowie das Lesen (`Schema`) und das
   Ändern (`change`) in `TraegerSettings`. Admin-Register und
   `durable_side_effect_notices.json` bleiben unverändert.
3. **Budgets.** `moduleComposeBudgets` steigt um genau 14 Zeilen:
   `modules/settings/compose` von 308 auf 322 (Speicher, Runtime-Bindung und
   Dienst der Träger-Ebene). `moduleComposeBudgetTotal` steigt von 12450 auf
   12464. Kein anderer Eintrag wächst.
4. **Lesen der Schule.** Die Settings Platform liest `platform.schools` nie.
   In der Tenant-Transaktion einer Schule wählt die lesende Policy der Tabelle
   den Träger der Schule; die Abfrage verlangt zusätzlich, dass
   `app.current_tenant_id` diese Schule ist, und liest sonst nichts. Der
   Batch-Lesepfad über mehrere Schulen fragt Organisation & Tenancy
   (`ListSchoolsByID`) nach dem Träger jeder Schule, bevor er seine
   Admin-Transaktion öffnet.
5. **Benachrichtigung.** `tenant_settings_changed` geht nach dem Commit der
   eigenen Träger-Transaktion an jede Schule des Trägers. Es entsteht kein
   neuer Rückruf nach dem Commit.
6. **Aufbewahrung.** Jede Änderung eines Trägers löscht in derselben
   Transaktion seine Einträge, die älter als die Frist des Schul-Audits sind
   (drei Jahre, #4044). Mit dem Träger gehen alle Einträge.
7. **Einordnung v1.** `beide` bekommen die Rechtstexte der Anmeldung (vier
   Schalter und vier Texte, ohne Anzeigeart und PDF der AGB, die an der Datei
   der Schule hängen) und die Funktionen des Elternportals ohne Sperre (neun
   Schlüssel). Alles andere bleibt `schule`. Lohnarten, Einheiten und
   DATEV-Nummern folgen mit dem Personal-Schnitt: Sie leben auf einer eigenen
   Seite der Schule, die die Herkunft eines Werts noch nicht zeigt. `traeger`
   bekommt in v1 keinen Schlüssel, weil die Werte (Branding, Vertragsweg,
   Kündigungsfrist) noch keinen Verbraucher haben.

## Folgen

- Weitere Tabellen des Trägers nutzen wie der Ferienzeitraum
  `TraegerTxMiddleware`. Nur wer erst nach dem Commit benachrichtigen muss,
  öffnet die Träger-Transaktion selbst, mit eigener Entscheidung und eigenem
  Eintrag im Register. Ein wachsendes Compose-Budget braucht wieder eine
  eigene ADR.
- Die Liste der Schulen, die bei einer Einstellung abweichen (ADR 0086
  Abschnitt 6), liest über Schulgrenzen und gehört deshalb in die Projektion
  `traeger-view`. Sie folgt mit eigener ADR.
- Ein Träger ohne neue Änderung behält ältere Einträge bis zur nächsten
  Änderung oder bis zu seiner Löschung.
