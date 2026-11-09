---
status: accepted
---

# Demo-Betrieb löscht ausgemusterte Demo-Schulen nach 90 Tagen aus der Datenbank

**Status: Akzeptiert.** Der Product Owner hat Frist, Löschweg und Umgang mit
dem Altbestand am 2026-11-09 entschieden. Ergänzt
[ADR 0092](0092-mandanten-datenlebenszyklus-nutzt-einen-gemeinsamen-katalog.md)
um einen zweiten Einstieg in die Schullöschung und baut auf
[ADR 0148](0148-demo-betrieb-loescht-die-dateien-ausgemusterter-demo-schulen.md)
auf. Entscheidung für [#5559](https://github.com/moto-nrw/project-phoenix/issues/5559).

## Kontext

Der Demo-Betrieb legt eine Demo-Schule still, wenn ihr letzter Demo-Zugang
abgelaufen ist (#3470), löscht den Besuch (ADR 0062) und danach die
hochgeladenen Dateien (ADR 0148). Die übrigen Datenbankzeilen der Schule
blieben ausgeblendet liegen, auch alles, was der Besucher selbst eingetippt
hat: Ordner- und Dateinamen, Freitexte, Anmeldungen, Warteschlangen- und
Auditzeilen. Sie gingen erst mit einem Neuaufbau der Demo-Datenbank.

Die Schullöschung (#3837, Migration 1.15.481) löscht eine Schule vollständig:
`platform.school_erasure_preflight` sperrt alle Schultabellen, prüft Plan und
Fremdschlüssel gegen das aktuelle Schema, und `platform.school_erasure_delete`
löscht in Planreihenfolge, prüft danach mit
`platform.school_erasure_assert_empty`, dass keine Zeile bleibt, und entfernt
zuletzt die Schulwurzel. ADR 0092 lässt den Einstieg nur über das CLI von
Organisation & Tenancy mit Operator-Auftrag zu.

## Entscheidung

1. **Frist: 90 Tage ab Stilllegung** (`platform.schools.deleted_at`). Die
   Schullöschung löscht auch `audit.auth_events` der Schule. Für
   Anmeldeereignisse eines Demo-Besuchs gelten nach der Entscheidung vom
   02.11.2026 (#5543) 90 Tage; eine kürzere Frist würde sie vorzeitig
   löschen.
2. **Fällig** ist eine Schule nur, wenn alle Bedingungen gelten:
   - Herkunftsnachweis in `platform.demo_school_origins` (ADR 0148). Name,
     E-Mail-Muster und Soft Delete allein reichen nicht.
   - Datei-Vermerk in `platform.demo_school_file_erasures` (ADR 0148): Die
     Datenbanklöschung nimmt der Dateiphase nichts weg, was sie noch braucht.
   - Stilllegung vor mindestens 90 Tagen.
   - Keine Demo-Bestellung nennt die Schule mehr (`tenant_id` oder
     `visitor_school_ids`). Das schließt die Standschule, eine Schule im
     Aufbau und eine Schule aus, deren Besuch noch nicht gelöscht ist.
3. **Zweiter Einstieg in die Schullöschung, nur für den Demo-Betrieb.**
   `platform.erase_retired_demo_school(school_id)` ist `SECURITY DEFINER`
   mit festem Suchpfad und nur für `phoenix_demo` ausführbar; ebenso die
   Liste `platform.retired_demo_schools_due_for_database_erasure(limit)`.
   Die Funktion prüft die Fälligkeit selbst unter Zeilensperre der Schule,
   nimmt dieselbe Advisory-Sperre wie die Operator-Löschung und ruft deren
   unveränderte Preflight- und Löschfunktion auf. Auftrag, Nachweisdatei und
   externes Ledger der Schullöschung nutzt sie nicht: Die Demo-Umgebung hat
   keine Kundenschule und keinen Auftraggeber. Die Rollen der Server und
   `phoenix_school_erasure` erhalten nichts.
4. **Eine Schule je Aufruf, eine Transaktion je Schule.** Jede Löschung läuft
   in einer Untertransaktion. Ein Fehler rollt alles zurück und liefert nur
   den SQLSTATE, nie eine Meldung mit möglichen Personenwerten. Die Schule
   bleibt fällig; der Lauf meldet einen Fehler, der Sidecar wird `unhealthy`
   und die nächste Abfrage versucht es erneut. `lock_timeout` begrenzt jede
   Sperrwartezeit auf 100 ms wie im übrigen stündlichen Demo-Lauf. Ist eine
   Tabelle länger gesperrt, ist die Schule „busy“: Nichts ändert sich, der
   Lauf gilt nicht als fehlgeschlagen, und die Schule wartet auf den nächsten
   Lauf. Pro Lauf höchstens 10 Schulen in zufälliger Reihenfolge, damit eine
   dauerhaft scheiternde Schule die anderen nicht aufhält.
5. **Nachweis am Datei-Vermerk.** Zwei neue Spalten `db_erased_at` und
   `db_row_counts` (Anzahl je Tabelle, keine Werte) setzt nur die Funktion.
   Ein Trigger verhindert, dass `phoenix_demo` beim Anlegen eines
   Datei-Vermerks eine Datenbanklöschung vortäuscht; `UPDATE` hat die Rolle
   nicht. Der Vermerk hat keinen Fremdschlüssel und überdauert die Schule
   (ADR 0148).
6. **Aufruf im Demo-Prozess.** `demoExpiry.run` ruft den Schritt
   `EraseRetiredDemoSchoolDatabases` nach erfolgreicher Dateiphase auf.
   Organisation & Tenancy bietet beide Schritte als eine Fähigkeit
   `DemoSchoolErasure` an (vorher `DemoSchoolFiles`); nur `cmd/devtools`
   verdrahtet sie, und dieses Binary liegt nur im Demo-Image (ADR 0060).
7. **Keine Policy-Änderung.** Es entsteht kein neues Paket über eine
   Owner-Grenze, kein neues Datenobjekt und kein Import einer fremden
   Implementierung: Adapter, Anwendung und Compose liegen bei Organisation &
   Tenancy, die Tabellen `platform.demo_school_file_erasures` und
   `platform.demo_school_origins` gehören ihr bereits. Die owner-übergreifende
   Löschung bleibt in der benannten Datenbankfunktion der Ausnahme aus
   ADR 0092.
8. **Altbestand:** Ausgeblendete Demo-Schulen ohne Herkunftsnachweis, deren
   Bestellung schon vor Migration 1.15.735 gelöscht war, löscht die
   Automatik nicht. Sie gehen mit einem einmaligen, von einem Menschen
   ausgeführten Neuaufbau der Demo-Datenbank und des Upload-Volumes. Kein
   Code rät die Herkunft.

## Was bleibt

Nach der Löschung hält keine Tabelle des Löschplans eine Zeile der Schule, und
die Schulwurzel ist weg. Außerhalb des Plans nennen nur diese Tabellen die
Schul-ID; jede ist im Wächtertest
`TestRetiredDemoSchoolIsErasedFromTheDatabaseAfterNinetyDaysOnly`
klassifiziert, eine neue Tabelle mit `tenant_id` oder `school_id` lässt ihn
scheitern:

- Gelöscht schon mit dem Besuch (ADR 0062), im Test als leer verlangt:
  Betreuungsverträge mit Spur, Monatsrechnungen mit Positionen,
  Jahresbescheide, Mensa-Monatsfreigaben.
- Bleiben beim Träger wie nach jeder Schullöschung, ohne Personenbezug:
  Vertragsvorlagen, Tarife, Preislisten, Mensa-Tarife, Schulen eines
  Abrechnungslaufs, Abrechnungszählungen (eigene Frist), Träger-Freigaben.
- Lesebestätigungen von Träger-Dokumenten mit eigener Frist von drei Jahren;
  das Besuchskonto ist schon anonymisiert.
- Vermerk und Herkunftsnachweis selbst (Schul-ID, Zeiten, Anzahlen).

Konten werden nicht gelöscht, nur die Zuordnungen zur Schule. Ein Konto, das
auch eine andere Schule nutzt, bleibt dort unverändert; ein Konto ohne
Schule anonymisiert der bestehende stündliche Lauf (`auth.erase_unused_accounts`).

## Vertrauensgrenze

`phoenix_demo` kann über `platform.demo_school_states.visitor_school_ids`
beliebige Schul-IDs eintragen, und der Trigger macht daraus einen
Herkunftsnachweis. Der Nachweis schützt gegen HTTP-Schreibwege und gegen
eine wiederhergestellte echte Schule ohne Bestellung, nicht gegen eine
fehlerhafte oder kompromittierte Demo-Steuerung. Diese könnte nach 90 Tagen
eine fremde, ausgeblendete Schule der Demo-Datenbank löschen lassen. Das ist
hinnehmbar, weil die Demo eine eigene Datenbank hat (ADR 0029), Produktion
und Staging keinen Demo-Sidecar haben und dort keine Demo-Bestellung
existiert.

## Betrachtete Optionen

- **Operator-CLI mit Demo-Modus.** Verworfen: Die Löschung hinge an einem
  Menschen; abgelaufene Demo-Schulen sammelten sich wieder an.
- **Frist direkt nach der Dateiphase.** Verworfen: widerspricht den 90 Tagen
  der Anmeldeereignisse (#5543).
- **Eigene Demo-Löschfunktion statt der Schullöschung.** Verworfen: Ein
  zweiter Löschplan liefe neben dem geprüften Plan der Schullöschung
  auseinander.

## Folgen

- Die Demo-Datenbank wächst nicht mehr mit ausgemusterten Schulen.
- Jede Löschung sperrt kurz alle Schultabellen der Demo-Datenbank
  (`SHARE ROW EXCLUSIVE`); gleichzeitige Schreibzugriffe aktiver
  Demo-Schulen warten so lange.
- Ein neuer Fremdschlüssel oder eine neue Mandantentabelle ohne Plan lässt
  die Preflight-Prüfung scheitern; die Schule bleibt dann fällig und der
  Sidecar `unhealthy`, bis der Plan nachgezogen ist.
