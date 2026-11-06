---
status: proposed
---

# Träger teilt vorhandene Dokumente über Organisation & Tenancy

Vorschlag vom 06.11.2026 zu [#4692](https://github.com/moto-nrw/project-phoenix/issues/4692),
Etappe 2 der Spec [#4611](https://github.com/moto-nrw/project-phoenix/issues/4611),
User Stories 98 und 99. Er hält fest, wo die geteilten Dokumente eines Trägers
liegen, wer sie liest und was er in den Registern ändert (ADR 0057 Punkt 5,
ADR 0070).

## Kontext

- Das Träger-Büro hat Dokumente und Listen, die Eltern und Schulen brauchen:
  Hausordnung, Elternbrief, Ansprechpersonen. Bisher erreichen Eltern nur
  Schreiben, die moto selbst erzeugt (Bestätigung, Vertrag; ADR 0126, 0153).
- Briefkopf, Logo und Vorlagen des Trägers liegen bei Organisation & Tenancy in
  Tabellen mit der Träger-Policy (ADR 0105, 0112, 0152).
- Keine Transaktion trägt Schule und Träger zugleich (ADR 0105).
- Eltern lesen Träger-Dokumente im Postfach der Eltern-App in der
  Träger-Transaktion; Zugriff folgt den Rechten der Eltern-Kind-Beziehung.

## Entscheidung

1. **Owner:** Organisation & Tenancy besitzt `platform.traeger_documents`,
   `platform.traeger_document_releases` und `platform.traeger_document_reads`.
   Ein neuer Owner wäre für drei Tabellen ohne eigene Regeln zu klein
   (ADR 0081).
2. **Freigabe:** Zielgruppe Eltern (alle Schulen, eine Schule, eine Gruppe)
   oder Schulen (alle, eine). Eine Freigabe wird zurückgenommen, nie gelöscht.
   Je Dokument und Zielgruppe gibt es eine aktive Freigabe. Ein Dokument ändert
   sich nie.
3. **Schule:** Die Schul-Transaktion liest keine Zeile der Tabellen. Zwei
   `SECURITY DEFINER`-Funktionen antworten für `app.current_tenant_id` mit den
   aktiven Freigaben, die die Schule oder die Eltern an ihr abdecken, und
   zeichnen das Öffnen auf. Lesen darf, wer die Dateien der Schule verwaltet
   (`files:manage`).
4. **Eltern:** Der Workflow `parentportal` bestimmt in der vorhandenen
   Admin-Grenze des Postfachs die Kinder mit `parent_portal.access`, ihre
   Schulen und Gruppen und liest dann in der Träger-Transaktion. Eine
   Mitgliedschaft allein öffnet nichts.
5. **Gruppen:** Der Dienst des Trägers liest die Gruppen einer Schule über
   School Structure in deren Tenant-Transaktion und speichert den Namen mit
   der Freigabe. `school_id` trägt keinen Fremdschlüssel, wie bei den
   Betreuungsverträgen; ein Trigger prüft beim Einfügen, dass die Schule zum
   Träger gehört. So bleibt der Erasure-Plan der Schulen unverändert.
6. **Rechte im Träger-Büro:** Lesen „Übersicht lesen“, Hochladen, Freigeben und
   Zurücknehmen „Anmeldung verwalten“ (Verwaltung). Ein eigenes Recht wäre ein
   Nachtrag zu ADR 0084.

## Register

- Compose-LOC: `modules/organizationtenancy/compose` +10 (die zwei
  Konstruktoren), `workflows/parentportal/compose` +1 (der Port), Summe +11.
- Transaktionsgrenzen: fünf Träger-Transaktionen des Dienstes; die
  Admin-Grenze des Postfachs zieht unverändert nach `inboxAccess` um.
- Pass-through-Register: zwei Bindungen an der Wurzel
  (`parentSharedDocuments`) und eine Gruppen-Bindung.

## Folgen

- Eltern bekommen zu einer neuen Freigabe keine Push-Nachricht.
- Dateien liegen als `BYTEA` in Postgres, höchstens 10 MB, wie Logo und Briefe.
- Eine Freigabe einer gelöschten Schule deckt niemanden mehr ab.
- Einträge des Öffnens löscht die stündliche Bereinigung des Schedulers nach
  drei Jahren für alle Träger, in der Admin-Transaktion wie die Löschfristen
  der Ferienanmeldungen (Port `TraegerDocumentReadRetention`).
