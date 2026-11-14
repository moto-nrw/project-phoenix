---
status: proposed
---

# Ferien-Mittagessen ist eine eigene Position außerhalb der Regelkette

**Status: Vorgeschlagen.** Entscheidung vom 14.11.2026 zu
[#5631](https://github.com/moto-ogs/project-phoenix/issues/5631), Nachtrag zu
Etappe 1 der Spec [#4611](https://github.com/moto-ogs/project-phoenix/issues/4611).
Den Zuschnitt (Pauschale aus der Buchung, eine Rechnung, keine Ermäßigung auf
das Essen, frei wählbar) hat Yannick Wenger am 14.11.2026 im Issue festgelegt;
der umsetzende PR legt die technische Fassung zur Annahme vor. Sie baut auf
[ADR 0115](0115-ferienbeitrag-entsteht-in-der-elternabrechnung-und-wird-bei-bestaetigung-festgehalten.md)
(Ferienbeitrag) und
[ADR 0206](0206-ferienplatz-aenderungen-und-neueinfrieren-haben-einen-workflow.md)
(Workflow der Ferienplätze) auf. Sie führt keinen Owner, keine Tabelle und
keine Route ein.

## Anlass

Die Gemeinde Bissendorf bietet in den Sommerferien Mittagessen an und will es
ab 2027 getrennt vom Ferienbeitrag berechnen, in derselben Ferienabrechnung
(Termin 11.11.2026). Die Mensa der Monatsbeiträge
([ADR 0168](0168-mensa-wird-aus-der-monatsfreigabe-der-schule-im-monatslauf-abgerechnet.md))
passt nicht: Sie hängt an Vertrag, Monatslauf und Monatsfreigabe und zählt
Ferientage nicht.

## Entscheidung

1. **Angebot und Buchung bei Enrollment.** Eine Ferienwoche trägt den Preis
   für ein Essen (`enrollment.holiday_period_weeks.lunch_price_cents`, `NULL`
   ohne Angebot), eine Buchung, ob das Kind isst
   (`holiday_registration_bookings.lunch`). Ein Kind mit Essen isst an jedem
   Betreuungstag seiner Buchung: an den gebuchten Tagen einer Woche nach
   Tagen, an Montag bis Freitag einer ganzen Woche, nie an einem Tag ohne
   Betreuung (Feiertag, Schließtag). Die Zahl rechnet Enrollment
   (`HolidayLunch`); die Elternabrechnung bekommt Zahl und Betrag über den
   Port `HolidayRegistrations`. Das Angebot einer Woche bleibt, solange eine
   Buchung Essen hat (409 `enrollment.holiday_lunch_booked`); der Preis lässt
   sich ändern.
2. **Pauschale, keine Spitzabrechnung.** Gebucht heißt berechnet. Es gibt
   keine Abmeldung einzelner Essen und keine Freigabe nach Anwesenheit.
   Dafür müsste Meal Plan Ferientage und Feriengäste kennen; das ist nicht
   Teil dieses Schnitts.
3. **Neue Zeilenart nach der Kette.** `meal` steht je Kind und gebuchter
   Woche hinter allen Zeilen der Regelkette (ADR 0115 Punkt 3) und außerhalb von ihr: Die
   Geschwisterermäßigung rechnet nur aus der Betreuung, die Befreiung und die
   Kostenübernahme setzen nur die Betreuung auf 0 €, und der Rang der Kinder
   folgt der Betreuung allein. Das Essen zahlt die Familie voll, wie bei der
   Mensa (ADR 0168 Punkt 6). Die Zeile hält die Zahl der Essen
   (`booked_days`) und den Betrag; der Preis für ein Essen ist ihr Quotient.
   Eine Buchung auf der Warteliste hat noch kein Essen.
4. **Festhalten (zu ADR 0115 Punkt 5).** Die Bestätigung hält die Essenszeilen mit dem
   Beitrag fest. Ein späterer Preis, ein später geplanter Schließtag und eine
   Umbuchung des Essens ändern einen festgehaltenen Beitrag nicht, wie beim
   Grundpreis; nur eine nachgerückte Woche wird mit ihrem Essen neu
   berechnet. Wird die Gruppe neu gerechnet, kommt das Essen aus den
   festgehaltenen Zeilen. Den Unterschied nach einer Umbuchung berechnet das
   Träger-Büro als Korrektur des Laufs (ADR 0119).
5. **Rechnung und Exporte.** Es bleibt eine Rechnung je Anmeldung oder Kind.
   Die Position `meal` nennt Woche, Zahl der Essen und Preis für ein Essen
   (`document_positions.booked_days`). Sie bucht auf die Kostenstelle ihrer
   Woche (#6624) und auf ein eigenes Erlöskonto
   (`revenue_accounts.scope = 'holiday_meals'`), ohne eigenes auf das der
   Ferienbetreuung, sonst auf das Standard-Erlöskonto. In den Finanz-Exporten
   ist sie ein eigener Teil (`holiday_meal`): Ermäßigungen der Betreuung
   werden nicht mit dem Essen verrechnet.
6. **Wer was sieht.** Die ausrichtende OGS sieht je Kind, ob es isst, und
   die Zahl der Essen je Betreuungstag, keinen Preis. Die Heimatschule sieht
   bei den Ferienbuchungen ihrer Kinder, ob sie dort essen
   (`enrollment.holiday_home_school_bookings()`, jetzt als wiederholbare
   Definition nach ADR 0223). Eltern sehen das Essen in Anmeldung,
   Eingangsmail, Stand und Beitrag.
7. **Umbuchung über den bestehenden Workflow.** Das Essen einer Buchung ändert
   das Träger-Büro mit derselben Umbuchung wie Woche, Zeitmodell und Tage
   (`PUT …/children/{id}/bookings`, ADR 0206). Die Bindung
   `api.parentBillingPlaceChanges.RebookHolidayChild` reicht dafür ein
   weiteres Feld der Buchung an den Workflow `holiday-allocation` durch.
   Sperrreihenfolge, Transaktion, Rechte, Platzvergabe und Neueinfrieren
   bleiben, wie ADR 0206 sie festlegt: Das Essen belegt keinen Platz und
   ändert keinen festgehaltenen Beitrag. Diese ADR ist der Freigabebeleg der
   geänderten Bindung im Register der Befehlswirkungen (ADR 0083).

## Verworfene Optionen

- **Mensa-Monatslauf (ADR 0168) für die Ferien öffnen.** Er braucht einen
  Vertrag, eine Monatsfreigabe der Schule und Anwesenheit; Feriengäste haben
  keinen Vertrag, und Meal Plan zählt Ferientage nicht.
- **Eigene Rechnung für das Essen.** Der Fachdienst Finanzen will es in
  derselben Ferienabrechnung; eine zweite Rechnung je Familie verdoppelt
  Nummern, Briefe und Zahlungen.
- **Essen als Teil der Regelkette.** Dann träfen Geschwisterermäßigung,
  Befreiung und Kostenübernahme auch das Essen. Das hat die Gemeinde nicht
  entschieden; die Mensa der Monatsbeiträge kennt es auch nicht.
- **Preis je Woche neben dem Preis je Tag.** Die Wochensumme ergibt sich aus
  den Betreuungstagen (5 × 4,30 € = 21,50 €); ein zweiter Preis könnte ihr
  widersprechen.

## Folgen

- Migration `20261114151835_holiday_lunch`: Spalten bei Enrollment, die
  Zeilenart `meal` an Beitragszeilen und Positionen, `booked_days` an den
  Positionen, der Geltungsbereich `holiday_meals` der Erlöskonten.
- Eine Buchung ohne Essen, eine Woche ohne Angebot, ihre Beiträge, Rechnungen
  und Exporte bleiben unverändert.
- Neuer Fehlercode `enrollment.holiday_lunch_booked`.

Offen für das Folgegespräch mit der Gemeinde 2027: ob Geschwisterregel und
Befreiung auch für das Essen gelten sollen, wer das Essen bei Leistungsbezug
zahlt und ob es beim langen Zeitmodell Pflicht ist. Entscheidet die Gemeinde
anders, kommt ein Schalter je Regel in einem eigenen Schnitt. Die Liste der
OGS zählt bei einer ganzen Woche Montag bis Freitag; einen Schließtag der
eigenen Schule zieht sie nicht ab.
