// Report-Vertrag der Klassenansicht (#1772): read-only Tagesansicht einer
// Klasse. Das Backend scopt jeden Abruf auf die zugewiesenen Klassen der
// aufrufenden Person (education.class_teachers) und liefert hier nie
// Kontaktdaten der Sorgeberechtigten.
//
// Seit dem Cutover (#2207 PR 3) gibt es nur noch einen Abrufweg: das
// Schul-Portal über lib/school-class-day-api. Diese Datei trägt deshalb nur
// noch die Typen, die sich Ansicht und Abruf teilen.

export interface ClassDayRow {
  student_id: number;
  first_name: string;
  last_name: string;
  // Klassenlisteneintrag (#2382): Kind des Klassenverbands OHNE OGS-Datensatz
  // ("Keine Betreuung"). student_id ist dann 0, list_entry_id trägt die ID —
  // als String, weil JSON-Zahlen oberhalb von 2^53 im Client runden würden.
  list_entry?: boolean;
  list_entry_id?: string;
  group_name?: string;
  registered: boolean;
  stays_today: boolean;
  offerings: string[];
  arrival?: string;
  pickup?: string;
  departure?: string;
  status?: "sick" | "excused" | "class_trip" | "cancelled" | "";
  // Abweichung vom üblichen Wochenplan (#2294): pickup_changed markiert eine
  // Abholzeit, die heute von der Regelzeit abweicht, pickup_regular nennt
  // die Regelzeit (leer, wenn der Plan an dem Wochentag keine hat).
  pickup_changed?: boolean;
  pickup_regular?: string;
  // Zeitpunkt, seit dem die Abweichung bekannt ist (ISO-Zeitstempel). Nur bei
  // Zeilen mit Abweichung gesetzt: Status-Meldezeit, sonst der Eintrag der
  // Tages-Ausnahme.
  reported_at?: string;
  // Hinweise der Lehrkraft für heute an die OGS, nur für dieses Kind (#4795).
  day_hints?: ClassDayHint[];
}

/** Ein Hinweis der Lehrkraft für einen Tag an die OGS (#4795). */
export interface ClassDayHint {
  body: string;
  author_name?: string;
  // true: der Hinweis ist von der angemeldeten Lehrkraft und darf geändert
  // oder gelöscht werden.
  own: boolean;
  updated_at: string;
}

interface ClassDayTotals {
  students: number;
  staying: number;
  leaving: number;
  absent: number;
  // Klassenlisteneinträge (#2382) unter students: weder "bleiben" noch
  // "gehen", sondern der neutrale "Keine Betreuung"-Anteil des Verbands.
  list_entries: number;
}

export interface ClassDayReport {
  school_class: string;
  date: string; // YYYY-MM-DD
  weekday: string; // "mon".."fri", "" am Wochenende
  school_day: boolean;
  phase_name?: string;
  // false: keine Anmeldephase deckt den Tag ab — Bleiben/Gehen ist dann
  // unbekannt, NICHT "alle gehen nach Hause".
  enrollment_known: boolean;
  totals: ClassDayTotals;
  rows: ClassDayRow[];
  // Tagesausnahme der ganzen Klasse an diesem Tag (#2962/#2970), egal ob
  // OGS oder Schule sie eingetragen hat. Fehlt, wenn es keine gibt.
  class_arrival_exception?: ClassDayArrivalException;
  // Hinweise der Lehrkraft für heute an die OGS für die ganze Klasse (#4795).
  class_day_hints?: ClassDayHint[];
  // true: die Lehrkraft darf heute einen Hinweis schreiben (Recht und
  // Freigabe der OGS). Fehlt bei älteren Antworten.
  can_write_day_hints?: boolean;
}

interface ClassDayArrivalException {
  arrival_time: string; // HH:MM
  reason?: string;
  origin: "ogs" | "school";
}
