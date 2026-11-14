---
status: accepted
---

# Ferienanmeldung kennt das Elternkonto, und Eltern ohne Schule bekommen ein Konto am Träger

Entscheidung vom 11.11.2026 zu [#6673](https://github.com/moto-ogs/project-phoenix/issues/6673).
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
   Elternportal übernimmt das Konto die Anmeldungen ohne Konto, deren
   Elternadresse seiner Adresse gleicht (ohne Groß- und Kleinschreibung).
   Nach dem Einrichten geschieht das bei der ersten Anmeldung: Das
   Einrichten selbst übernimmt nichts, Identity & Access hängt nicht von
   Enrollment ab. Die Adresse eines Kontos ist bestätigt: Ein
   Konto entsteht nur über einen Link, der an sie ging. Der Workflow
   Elternportal ruft dafür Enrollments Fähigkeit in einer
   Admin-Transaktion auf; die Abfrage nennt Konto und Adresse im Filter.
4. **Die Frage am Ende.** Wer das Formular ohne Eltern-Sitzung abschickt,
   beantwortet „Möchten Sie ein Elternkonto anlegen?“. Bei „Ja“ legt
   Enrollment in der Transaktion der Anmeldung eine zweite Mail des Trägers
   in die Outbox (Art `holiday_parent_account_offer`). Sie nennt den
   Träger und trägt keinen Link: Erst wenn Delivery sie sendet, prüft die
   Wurzel die Adresse nach der Regel der Annahme-Mail (Punkt 11). Hat die
   Adresse ein Elternkonto, zeigt die Mail nur die Anmeldung in der
   Eltern-App; sonst „Elternkonto einrichten“ mit einem Link, der beim
   Senden entsteht, und leise darunter die Anmeldung. Die Mail geht nur an
   die Person, der die Adresse gehört. Das Formular verrät nichts: Seine
   Antwort ist für jede Adresse gleich.
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
    für Elternkonten: Die stündliche Löschung unbenutzter Konten
    (`auth.account_is_unused`) hält es, solange etwas es braucht, wie das
    Elternprofil das Konto eines Schulelternteils hält. Ein Zugang am
    Träger hält das Konto nur an einem aktiven, nicht gelöschten Träger,
    und dort nur in den 30 Tagen des Einrichtungslinks oder solange eine
    Ferienanmeldung dieses Trägers das Konto oder, noch nicht übernommen,
    seine Adresse nennt. Geht die letzte solche Anmeldung mit ihrer Frist,
    löscht die nächste Stunde das Konto, wenn nichts anderes es hält. Der
    Zugang selbst geht mit dem Konto oder dem Träger.
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

## Nachtrag 2026-11-14: Anmeldung mit Code für Eltern (#7051)

Punkt 7 hielt fest, dass die Eltern-Sitzung keinem zweiten Faktor folgt.
Das gilt nicht mehr. Eltern schalten die Anmeldung mit Code per E-Mail
freiwillig ein, nach dem Muster des Caterers (ADR 0077, Nachtrag #4497).

1. **Freiwillig, für alle Eltern.** Keine Schule und kein Träger kann den
   Code verlangen, auch nicht für einzelne Vorgänge wie Bankverbindung oder
   Vertrag. Dafür gäbe es einen Schritt-Faktor je Vorgang; das bleibt eine
   eigene Entscheidung, wenn ein Vorgang ihn braucht.
2. **Der Faktor gehört dem Konto.** Es ist derselbe Code per E-Mail wie an
   Schule und Träger. Ein Konto, das ihn aus seiner Arbeit an einer Schule
   hat, wird auch im Eltern-Portal gefragt; Ausschalten im Eltern-Portal
   schaltet ihn für das Konto aus, und eine Schule oder ein Träger, die ihn
   verlangen, fordern bei der nächsten Anmeldung dort die Einrichtung.
3. **Ein gemerktes Gerät gilt für das Eltern-Konto im Eltern-Portal**, nicht
   je Schule: Die Eltern-Sitzung nennt keine Schule. Das Gerät trägt
   `parent_portal`, nennt weder Schule noch Träger und gilt weder im
   Caterer-Portal noch umgekehrt. Beide Sitzungsarten (Schule und Träger)
   lesen dieselben Geräte.
4. **Zurücksetzen macht das moto-Team**, über das vorhandene Zurücksetzen
   eines Kontos an einer Schule des Kontos (Operator-Portal, Konten, Grund
   im Audit, Hinweis-Mail). Eltern wenden sich an ihre Schule. Eine Schule
   setzt nicht selbst zurück: Ein Eltern-Konto gehört oft zu mehreren
   Schulen, und das Zurücksetzen gilt für das ganze Konto (#6862). Ein
   Konto ohne Schule (nur Zugang am Träger) kann den Code deshalb nicht
   einschalten; die Einstellungen sagen das.
5. **Prüfung der Sitzung.** Ausstellen und Erneuern verlangen den Nachweis,
   solange das Konto den Faktor hat. Die Prüfung bei jedem Seitenaufruf liest
   ihn nicht (Budget `identity.session_validate.parent`): Eine Sitzung von
   vor dem Einschalten endet bei ihrer nächsten Erneuerung, spätestens nach
   der Lebensdauer des Zugangstokens.
6. **Sprache.** Code-Mail und Hinweis auf ein gemerktes Gerät gehen in der
   Sprache, die Eltern im Portal gewählt haben.

