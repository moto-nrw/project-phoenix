---
status: accepted
---

# Dokumentenpostfach der Eltern-App liest die Schreiben des Trägers

Entscheidung vom 27.10.2026 zu [#4671](https://github.com/moto-nrw/project-phoenix/issues/4671),
Etappe 1 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 93 und 96. Sie ist der fünfte Bau-Schnitt der Elternabrechnung
aus [ADR 0088](0088-betreuungsvertrag-und-elternabrechnung-sind-eigene-owner-beim-traeger.md)
nach [ADR 0123](0123-bestaetigungsschreiben-gehen-als-mail-des-traegers-ueber-die-outbox.md).
Sie führt keinen Owner ein und hält fest, was das Postfach im Workflow
`parent-portal`, in Delivery und in den Registern ändert (ADR 0057 Punkt 5,
ADR 0070).

## Kontext

Ein freigegebener Lauf hat je Rechnung ein unveränderliches
Bestätigungsschreiben als PDF (ADR 0121); das Träger-Büro sendet es per Klick
an die Adresse der Familie (ADR 0123). Die Spec will, dass Eltern alle
gesendeten Dokumente in der Eltern-App finden und eine Push-Nachricht
bekommen. Die Rechnung liegt beim Träger ohne Schule; die Ferienanmeldung
kennt kein Elternkonto und verbindet kein Kind mit einem Schulkind (ADR 0110).
Das Elternportal ist Schul-Tenant-bezogen, Push-Abos und Push-Outbox gehören
je einer Schule. ADR 0088 §7 sagt: Ein Elternkonto sieht ein Beitragsdokument
nur als Konto des Zahlers; Mitgliedschaft und Verbindung zum Kind reichen
nicht. Den Zahler gibt es erst mit Etappe 2.

## Entscheidung

1. **Was im Postfach liegt.** Ein Schreiben liegt im Postfach, sobald es
   einmal gesendet ist (`parent_billing.document_mailings`, ADR 0123). Ein
   Klick auf „Senden“ stellt also über Mail und Eltern-App zu (Story 93). Ein
   nur erstelltes Schreiben liegt nicht im Postfach.
2. **Wer es sieht: zwei Bedingungen, keine allein.**
   - *Zahler:* Das Schreiben ist an die aktuelle Adresse des Kontos gerichtet
     (Vergleich ohne Groß- und Kleinschreibung, wie beim Login). Bis es den
     Zahler gibt, ist der Empfänger der Rechnung sein Konto. Das Postfach
     zeigt damit nichts, was nicht schon per Mail an diese Adresse ging.
   - *Beziehung:* Das Konto hält auf einer Beziehung zu einem Kind an einer
     Schule dieses Trägers `parent_portal.enrollments.view`. Nur diese Träger
     liest das Postfach. Das Schreiben ist die Bestätigung einer Anmeldung,
     deshalb das Recht, Anmeldungen zu sehen. Ein Konto ohne dieses Recht an
     irgendeiner Schule bekommt 403 (`care.guardian_permission_denied`), ein
     Schreiben außerhalb seines Postfachs 404.
   Der Workflow `parent-portal` prüft die Beziehung (Gate in
   `internal/care`, wie #4662), die Elternabrechnung die Adresse.
3. **Lesen in der Träger-Transaktion.** Die Elternabrechnung bekommt die
   Capability `parentbilling.ParentDocuments` (Liste, PDF). Der Workflow
   öffnet je Träger dessen Träger-Transaktion (`TraegerRuntime`, gebunden an
   `tenant.WithinTraeger`) und liest über seinen Port `BillingDocuments`; die
   Root übersetzt. Den Träger einer Schule und die Adresse des Kontos liest
   er über eigene Ports in der Admin-Transaktion (Organisation & Tenancy,
   Identity & Access). Das PDF ist das gespeicherte; nichts wird neu
   gerendert.
4. **Zugriffsprotokoll.** Jede Liste und jeder Download steht als `read` mit
   dem Elternkonto in `parent_billing.run_events` (`inbox`: `list` mit den
   Rechnungen bzw. `download` mit der Rechnung). Die Aktionen der Spur
   bleiben unverändert.
5. **Push in derselben Transaktion wie der Versand.** Jeder Versand legt
   zusätzlich einen aufgeschobenen Push des Trägers in Delivery ab
   (`EnqueueDeferredPush` mit `OrganizationID`, Art
   `parent_billing_document`, Schlüssel je Schreiben). Nur der erste Versand
   pusht; ein erneuter Versand trifft auf denselben Schlüssel. Dafür bekommt
   `platform.push_outbox` wie die Mail-Outbox (ADR 0110) die Spalte
   `organization_id`, genau einen Bezug (Schule oder Träger) und die
   Träger-Policy; die Schul-Policy sieht diese Zeilen nie.
6. **Empfänger erst beim Senden.** Der Worker gibt den Push an den Workflow
   (`parentportal.DocumentNotices`, von der Root im Worker gebunden). Er
   sucht das Konto der Adresse, dessen Kinder mit
   `parent_portal.enrollments.view` an Schulen dieses Trägers, und schickt je
   Schule in deren Tenant-Transaktion über den Router der Schule
   (`NotifyEventDurably`) den Push an dieses Konto, mit seiner Zustimmung zum
   neuen Typ „Neues Dokument vom Träger“, in seiner Sprache, ohne Kind und
   Betrag, mit den Kindern als `StudentIDs`. Ein Konto ohne Beziehung bekommt
   nichts; die Mail bleibt sein Weg. Die Push-Zeile des Trägers geht nach der
   Frist der Outbox (30 Tage nach Ende), wie seine Mails.
7. **Grenzen.** Kein Import zwischen Domain-Ownern, keine neue Policy-Regel:
   alle Kanten laufen über Ports, die die Root bindet
   (`roots/owners/parent_portal_documents.go`,
   `api/serve_mounts_parent_billing.go`). Keine Admin-Transaktion schreibt,
   keine Transaktion trägt Schule und Träger zugleich.
8. **Register.**
   - `command-effects.json`: eine geprüfte Bindung (Push des Trägers:
     platform/durable nach ADR 0098).
   - `passthrough.json`: vier Methoden mit Entscheidung 0126.
   - Transaktionsregister: `ParentPortalRuntime.WithinTraeger` als Stelle der
     Träger-Transaktion.
   - Migration 1.15.684: Push-Outbox des Trägers, Index auf Träger und
     Empfänger der Rechnungen.

## Verworfene Optionen

- **Kind der Rechnung mit Schulkind abgleichen (Name, Geburtsdatum):** Die
  Anmeldung verbindet bewusst kein Kind (ADR 0110); ein getippter Name
  schaltete Daten frei.
- **Nur die Adresse prüfen:** ein Konto ohne Elternrecht an einer Schule des
  Trägers (etwa nur Abholung) läse das Postfach; die Beziehungsregel verlangt
  ein Recht.
- **Nur die Beziehung prüfen:** jeder Erziehungsberechtigte derselben Schule
  sähe die Rechnungen anderer Familien des Trägers (ADR 0088 §7).
- **Push gleich beim Versand an Geräte binden:** Die Träger-Transaktion darf
  Push-Abos und Schul-Outbox nicht lesen; zwei RLS-Bezüge in einer
  Transaktion hat ADR 0088 verworfen.
- **Push nach dem Commit im Request:** ein dauerhafter Schreibvorgang in
  einem Nach-Commit-Schritt, den das Register
  `durable_side_effect_notices.json` nicht zulässt.

## Folgen

- Mit dem Zahler (Etappe 2) tritt sein Konto an die Stelle der Adresse; die
  Beziehungsbedingung bleibt.
- Weitere Dokumentarten (Vertrag, Bescheid, Mahnung) können denselben Weg
  nehmen.
- Eltern, die Push schon eingeschaltet haben, bekommen den neuen Typ erst,
  wenn sie ihn in den Einstellungen anschalten; neue Eltern bekommen ihn mit
  der Einrichtung der Benachrichtigungen.
