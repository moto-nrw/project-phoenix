---
status: accepted
---

# Ferienanmeldung kennt das Elternkonto, und Eltern ohne Schule bekommen ein Konto am Träger

Entscheidung vom 11.11.2026 zu [#6673](https://github.com/moto-nrw/project-phoenix/issues/6673).
Sie ändert [ADR 0110](0110-oeffentliche-ferienanmeldung-schreibt-in-der-traeger-transaktion.md)
(Folgen: „die Anmeldung kennt kein Elternkonto“),
[ADR 0113](0113-vorbefuellung-der-ferienanmeldung-liest-im-elternportal.md)
(Folgen, Punkt 2) und ergänzt
[ADR 0120](0120-feriengast-ist-ein-befristetes-kind-der-ausrichtenden-ogs.md) und
[ADR 0126](0126-dokumentenpostfach-der-eltern-app-liest-die-schreiben-des-traegers.md).
Sie führt keinen neuen Owner ein.

## Kontext

Ein Anmeldelink, zwei Wege: Eltern mit Eltern-App melden sich dort an, alle
anderen füllen das Formular einmal aus. Am Ende sollen möglichst alle Eltern
ein Elternkonto haben und dort den Stand ihrer Ferienanmeldung sehen. Bisher
speichert die Anmeldung kein Konto (ADR 0110), und ein Elternkonto gibt es
nur über die Einladung einer Schule mit Profil und Beziehung zu einem Kind.
Die Eltern-Sitzung verlangt eine Schule, an der das Konto die Rolle
Erziehungsberechtigte hält und die Eltern-App gebucht ist. Eltern eines
Feriengasts (ADR 0120) haben nichts davon.

## Entscheidung

1. **Konto an der Anmeldung.** `enrollment.holiday_registrations` bekommt
   `parent_account_id` (Fremdschlüssel auf `auth.accounts`, `ON DELETE SET
   NULL`) und `parent_account_requested`. Die Verknüpfung nennt nur das
   Konto. Sie verbindet weiterhin kein Kind mit einem Schulkind: ADR 0110
   Punkt 4 gilt unverändert, Name und Geburtsdatum schalten nichts frei.
2. **Sofort bei angemeldeten Eltern, mit Nachweis.** Die Vorbefüllung
   (ADR 0113) gibt dem angemeldeten Konto einen versiegelten Nachweis mit:
   Konto, Träger und Zeitraum, 24 Stunden gültig, versiegelt wie der
   Ferienwunsch (ADR 0113, Nachtrag). Die öffentliche Anmeldung öffnet ihn
   und trägt das Konto ein. Das Konto kommt nie als Zahl aus der Anfrage.
   Ohne gültigen Nachweis bleibt die Anmeldung ohne Konto.
3. **Sonst über die bestätigte Adresse.** Bei jeder Anmeldung am
   Elternportal und beim Einrichten des Kontos übernimmt das Konto die
   Anmeldungen ohne Konto, deren Elternadresse seiner Adresse gleicht (ohne
   Groß- und Kleinschreibung). Die Adresse eines Kontos ist bestätigt: Ein
   Konto entsteht nur über einen Link, der an sie ging. Der Workflow
   Elternportal ruft dafür Enrollments Fähigkeit in einer
   Admin-Transaktion auf; die Abfrage nennt Konto und Adresse im Filter.
4. **Die Frage am Ende.** Wer das Formular ohne Eltern-Sitzung abschickt,
   beantwortet „Möchten Sie ein Elternkonto anlegen?“. Bei „Ja“ legt
   Enrollment in der Transaktion der Anmeldung eine zweite Mail des Trägers
   in die Outbox (Art `holiday_parent_account_offer`). Sie trägt den Link
   zum Einrichten und den Weg für alle, die schon ein Konto haben
   (anmelden). Antwort und Mail sind für jede Adresse gleich: Das Formular
   verrät nicht, ob es ein Konto gibt.
5. **Link ohne Tabelle.** Identity & Access stellt den Schlüssel des Links
   aus und löst ihn ein. Er ist ein signiertes Token mit Adresse, Träger und
   Ablauf nach 30 Tagen, das keine Sitzung ist und nur an den Routen
   `/auth/holiday-parent-setups/{token}` gilt. Er wird nirgends gespeichert.
   Mehrfaches Einlösen ist folgenlos: Gibt es das Konto schon, bleibt sein
   Passwort unverändert.
6. **Einrichten.** `GET` nennt Adresse, Träger und ob es das Konto schon
   gibt; wer den Link hat, hat das Postfach. `POST` legt in einer
   Admin-Transaktion das Konto mit Passwort an, wenn es keines gibt, und
   trägt in beiden Fällen den Zugang am Träger ein
   (`auth.traeger_parent_access`: Konto, Träger). Ein Konto, das so
   entsteht, hat keine Schule, kein Profil und keine Beziehung zu einem
   Kind.
7. **Eltern-Sitzung am Träger.** Die Eltern-Anmeldung prüft zuerst wie
   bisher die Schulen des Kontos. Hält das Konto an keiner aktiven Schule
   die Rolle, aber einen Zugang an einem aktiven Träger, bekommt es eine
   Eltern-Sitzung der neuen Art `traeger_parent`. Ein Konto, dessen
   Schulen die Eltern-App nicht gebucht haben, bleibt abgewiesen wie
   bisher: Die Sitzung am Träger gäbe ihm sonst an der Buchungssperre
   vorbei Zugriff auf seine Kinder (#4895).
   Sie nennt den Träger und keine Schule (wie die Träger-Sitzung aus
   ADR 0084), folgt keiner Modulbuchung einer Schule und keinem zweiten
   Faktor. Auffrischen und jede Prüfung der Sitzung lesen den Zugang und
   den Träger neu; fehlt einer, endet die Sitzung. Bekommt das Konto später
   eine Schule, öffnet die nächste Anmeldung die gewöhnliche Eltern-Sitzung.
8. **Keine Rechte an einer Schule.** Die Sitzung trägt wie jede
   Eltern-Sitzung keine Schule und keine Rechte. Jede Route des
   Elternportals, die Daten einer Schule liest oder schreibt, geht weiter
   von den Beziehungen des Kontos mit ihren `parent_portal.*`-Rechten aus.
   Ein Konto ohne Beziehung bekommt dort leere Listen oder 403. Das gilt
   auch für die ausrichtende OGS: Nachrichten, Abholung und Check-in
   bleiben verschlossen. Mandantentrennung und Beziehungsrechte bleiben
   unberührt.
9. **Was das Konto sieht.** `GET /parent/me/holiday-registration-entries`
   liefert die Anmeldungen mit diesem Konto: Träger, Zeitraum, Eingang,
   Stand und je Kind Vorname und Wochen mit Zeitmodell und Platz. Keine
   Gesundheitsangaben, keine Anschrift, keine Kontakte. Enrollment liest
   sie über die neue Fähigkeit `HolidayParentRegistrations` in der
   Admin-Transaktion, mit dem Konto im Filter, mengenbasiert. Das
   Dokumentenpostfach (ADR 0126) liest die Träger, bei denen das Konto
   eine Anmeldung hat, wenn die Beziehungen des Kontos keinen Träger
   öffnen; die Bedingung der Adresse bleibt. Ein Konto mit Schulkind bei
   einem Träger und Ferienanmeldung bei einem anderen liest dessen
   Schreiben dort noch nicht: Das zweite Lesen kostete das Postfach ein
   Statement über sein Budget. Die Mail bleibt sein Weg.
10. **Löschfristen.** Die Verknüpfung steht in der Zeile der Anmeldung und
    geht mit ihr: nach der Frist, die die Anmeldung trägt (ADR 0118,
    Nachtrag #6691; Spalte `retention_days`). Das Konto folgt den Regeln
    für Elternkonten. Der Zugang am Träger geht mit dem Konto oder dem
    Träger.
11. **Annahme-Mail.** Die Annahme-Mail (#6672) bietet allen ohne Konto
    denselben Link an, auch für Anmeldungen vor dieser Entscheidung.
    Umgesetzt wie der Link zur Eltern-App derselben Mail: Die Mail trägt
    den Träger (`organization_id` in der Nutzlast); erst wenn Delivery sie
    sendet, prüft die Wurzel die Adresse. Hält ein Konto der Adresse die
    Rolle Erziehungsberechtigte an einer aktiven Schule oder einen Zugang
    an einem aktiven Träger, zeigt die Mail „In der Eltern-App ansehen“.
    Sonst zeigt sie „Elternkonto einrichten“ mit dem signierten Link dieser
    Entscheidung (Punkt 5). Eine Mail, die vor dieser Änderung in die
    Outbox kam, nennt keinen Träger und bietet keinen Link.

## Verworfene Optionen

- **Konto als Zahl im Formular mitschicken:** Die öffentliche Route hat
  keine Sitzung; jede Zahl wäre frei wählbar.
- **Öffentliche Route liest die Eltern-Sitzung:** zwei Arten von Anfragen an
  einer Route ohne Sitzung, und der Träger-Host bekäme das Eltern-Cookie.
- **Nur über die Adresse zuordnen, ohne Verknüpfung:** Ändert das Konto
  seine Adresse, verliert es seine Anmeldungen.
- **Tabelle für Einrichtungslinks:** Die Träger-Transaktion der Anmeldung
  schreibt keine Konten-Tabellen; ein signiertes Token braucht keine.
- **Schule als Halt der Sitzung (ausrichtende OGS):** gäbe dem Konto eine
  Mitgliedschaft an einer Schule, an der es kein Kind hat.
- **Geheimer Status-Link statt Konto (#6676, Option A):** bleibt als
  Ergänzung möglich, ersetzt aber kein Konto für Dokumente und Rechnung.

## Folgen

- Die Nutzungsanalyse zählt das Einrichten des Kontos; die Leseroute zählt
  sie nicht.
- Push-Nachrichten gehören einer Schule. Ein Konto ohne Schule bekommt
  keine; die Mail bleibt sein Weg (wie ADR 0126 Punkt 6).
- Das Zurücksetzen des Passworts führt ein Konto mit Zugang am Träger zum
  Elternportal.
- `docs/agents/contracts.md` nennt die neue Art der Sitzung.
