---
status: proposed
---

# Demo-Betrieb löscht die Dateien ausgemusterter Demo-Schulen

**Status: Vorgeschlagen.** Die Freigabe erteilt der Product Owner. Ergänzt
[ADR 0062](0062-demo-visitor-erasure-is-a-database-workflow.md) und
[ADR 0092](0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md);
Entscheidung für [#5544](https://github.com/moto-nrw/project-phoenix/issues/5544).

## Kontext

Der Demo-Betrieb legt eine Demo-Schule still, wenn ihr letzter Demo-Zugang
abgelaufen ist (#3470), und löscht danach den Besuch aus der Datenbank
(`platform.erase_closed_demo_visitors`, ADR 0062). Die Dateien, die ein Besuch
hochgeladen hat (Dateiablage, Anhänge, Dokumente, Fotos, Anmeldeunterlagen),
blieben im Upload-Volume `phoenix-demo-uploads` liegen. Die Dateiablage hat
keine eigene Frist, und der nächtliche Lauf ist in der Demo nicht zwingend an.

Die Schullöschung (#3837, ADR 0092) kennt bereits alle Upload-Namensräume
einer Schule (`files` im Datenkatalog) und entfernt sie geprüft:
`objects.Local.TenantObjects` löscht nach Namensraum, nie nach gespeichertem
Dateinamen, folgt keinem Symlink und nennt keinen Dateinamen in Fehlern. Sie
ist aber ein operatorgeführtes Verfahren mit Auftrag, Nachweis und externem
Ledger.

## Entscheidung

1. **Der Demo-Prozess löscht stündlich die Upload-Namensräume ausgemusterter
   Demo-Schulen.** Der Schritt läuft in `demoExpiry.run` nach
   `RetireDemoSchools`. Organization & Tenancy bietet ihn als
   `DemoSchoolFiles` an und nutzt dafür den Dateikatalog, die Prüfung
   `validateErasureFiles` und dieselbe Schleife „löschen, nachzählen, 0
   verlangen“ wie die Schullöschung (`tenantFiles`, `removeAndVerify`). Den
   Auftrag, den Nachweis und das Ledger der Schullöschung nutzt er nicht: Die
   Demo-Umgebung hat keine Kundenschule und keinen Auftraggeber.
2. **Der Arbeitsvorrat kommt nur aus der Datenbank.** Fällig ist jede Schule,
   die seit mindestens einer Stunde ausgeblendet ist, noch keinen Vermerk in
   `platform.demo_school_file_erasures` hat und nicht die Schule einer noch
   wartenden oder laufenden Bestellung ist. Die Standschule ist nie fällig.
   Die Bestellung selbst ist kein Vorrat, denn ADR 0062 löscht sie mit dem
   Besuch. Pro Lauf höchstens 50 Schulen.
3. **Ein Vermerk erst nach vollständiger Löschung.** Pro Schule werden alle
   Namensräume gelöscht und neu gezählt; erst bei 0 schreibt eine einzige
   Anweisung den Vermerk mit den gelöschten Anzahlen und prüft dabei erneut,
   ob die Schule fällig ist. Schlägt eine Schule fehl, entsteht kein Vermerk,
   die anderen Schulen laufen weiter, und der Lauf bleibt fällig. Der Sidecar
   meldet sich dann als `unhealthy`.
4. **Keine Transaktion und keine Zeilensperre.** Owner dürfen keine neue
   Transaktion beginnen (`test/transaction_boundaries.json` wächst nicht).
   Nur ein Demo-Prozess läuft, das sichert die bestehende Demo-Lease. Ein
   zweiter Läufer würde dieselben Namensräume nur noch einmal leer zählen.
   Nichts wartet auf eine Sperre.
5. **Der Sidecar bekommt das Upload-Volume.** `demo-runtime` mountet
   `phoenix-demo-uploads` unter `/app/public/uploads` und startet mit
   `--uploads-root /app/public/uploads` (Pflicht-Flag, Startfehler ohne
   Verzeichnis). Server und Sidecar laufen aus demselben Image als `appuser`
   (UID 1001), dem die Dateien gehören. `scripts/check-runtime-env.py` erlaubt
   genau dieses eine Volume, kein weiteres, keine Ports und keine Env-Datei.
6. **Eine neue Policy-Regel.** `dev-tools.compose.delivery-objects` erlaubt
   `cmd/devtools` (`dev-tools`/`compose`) nur das Paket
   `modules/delivery/objects` (`delivery-platform`/`public`), nur in
   Produktion. Der Akzeptor `reviewedDemoUploadsRule` lässt genau diese
   Regel in Epoche 96 zu. Die Bindung liegt wie in `cmd/tenant.go` beim
   Prozess, der das Volume kennt.
7. **Die Vermerk-Tabelle** gehört Organization & Tenancy, hat keinen Bezug zu
   Personen und keinen Fremdschlüssel auf `platform.schools`: Die
   Schullöschung prüft den Fingerabdruck aller eingehenden Fremdschlüssel und
   würde sonst nicht mehr laufen. `phoenix_demo` erhält nur `SELECT` und
   `INSERT`, die Server-Rollen nichts.

## Betrachtete Optionen

- **Dateilöschaufträge der Owner (`*_file_cleanup`).** Verworfen: Sie decken
  nur Dateien mit Datenbankzeile und nur einige Owner ab. Verwaiste Dateien
  blieben.
- **Dateiphase der Schullöschung über Auftrag und Ledger.** Verworfen: Sie
  braucht einen Operator, einen Auftrag je Schule und ein externes Ledger;
  stündlich und ohne Auftraggeber passt das nicht.
- **Demo-Datenbank und Volume neu aufbauen.** Verworfen als Regelweg: Es
  nimmt allen aktiven Besuchen ihre Schule und braucht einen Menschen.

## Folgen

- Die Datenbankzeilen einer ausgemusterten Demo-Schule bleiben ausgeblendet,
  bis die Demo-Datenbank neu aufgebaut wird; nur ihre Dateien gehen.
- `TenantObjects` entfernt beim Löschen auch den geleerten Schulordner
  (`<art>/<schule>/`), damit kein Pfad mehr die Schule nennt. Das gilt auch
  für die Schullöschung; ihr Nachzählen bleibt 0.
- Ein neuer Namensraum im Datenkatalog bleibt ohne geprüften Löschweg
  ungültig (`validateErasureFiles`); der Demo-Prozess startet dann nicht.
- Der Sidecar kann jetzt Dateien des Demo-Servers löschen. Das ist seine
  einzige Schreibmöglichkeit auf dem Volume; Staging und Produktion haben
  keinen Sidecar.
