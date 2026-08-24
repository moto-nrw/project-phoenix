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
}
