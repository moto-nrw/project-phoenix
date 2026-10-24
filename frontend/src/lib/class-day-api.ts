// Report-Vertrag der Klassenansicht (#1772): Tagesansicht einer Klasse;
// geschrieben werden Vormittagsanwesenheit, Übergabe an die OGS und
// Hausaufgaben. Das Backend scopt jeden Abruf auf die zugewiesenen Klassen der
// aufrufenden Person (education.class_teachers) und liefert hier nie
// Kontaktdaten der Sorgeberechtigten.
//
// Seit dem Cutover (#2207 PR 3) gibt es nur noch einen Abrufweg: das
// Schul-Portal über lib/school-class-day-api. Diese Datei trägt deshalb nur
// noch die Typen, die sich Ansicht und Abruf teilen.

import type { HomeworkFeedbackStatus } from "./homework-feedback";

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
  // Anwesenheit am Vormittag (#4790): was die Lehrkraft eingetragen hat
  // (source "teacher"), sonst "absent" aus einer Meldung krank, entschuldigt
  // oder Klassenfahrt (source "report"). Fehlt, solange nichts davon vorliegt.
  morning_attendance?: ClassDayMorningAttendance;
  // Rückmeldung der OGS zu den Hausaufgaben der Klasse an diesem Tag
  // (#4796). Fehlt ohne Rückmeldung und bei Klassenlisteneinträgen.
  homework_feedback?: ClassDayHomeworkFeedback;
  // Übergabe nach dem Unterricht (#4794): die Lehrkraft hat das Kind bei der
  // heutigen Übergabe als „nicht da“ markiert.
  handover_absent?: boolean;
}

interface ClassDayHomeworkFeedback {
  status: HomeworkFeedbackStatus;
  note?: string;
}

/** Ein Hausaufgaben-Eintrag der Klasse an diesem Tag (#4796). */
export interface ClassDayHomework {
  // int64 des Backends, als String.
  id: string;
  subject?: string;
  description: string;
}

export type MorningAttendanceStatus = "present" | "absent";

interface ClassDayMorningAttendance {
  status: MorningAttendanceStatus;
  source: "teacher" | "report";
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
  // Die Lehrkraft darf die Anwesenheit am Vormittag eintragen (#4790): der
  // Tag ist heute und ein Schultag, die OGS hat es nicht ausgeschaltet und
  // die Berechtigung liegt vor.
  can_record_morning_attendance?: boolean;
  // Hausaufgaben der Klasse an diesem Tag (#4796), älteste zuerst. Fehlt,
  // wenn es keine gibt.
  homework?: ClassDayHomework[];
  // Die Lehrkraft darf Hausaufgaben eintragen (Berechtigung liegt vor).
  can_write_homework?: boolean;
  // Übergabe der Klasse an die OGS (#4794). Fehlt, solange die Klasse an
  // dem Tag nicht übergeben ist.
  handover?: ClassDayHandover;
  // Die Lehrkraft darf die Klasse übergeben oder die Übergabe zurücknehmen:
  // der Tag ist heute und ein Schultag, die OGS hat es freigegeben und die
  // Berechtigung liegt vor.
  can_hand_over?: boolean;
}

export interface ClassDayHandover {
  handed_over_at: string; // ISO-Zeitstempel
  // Name der Lehrkraft; fehlt, wenn sie nicht mehr angelegt ist.
  handed_over_by?: string;
  absent_count: number;
}

interface ClassDayArrivalException {
  arrival_time: string; // HH:MM
  reason?: string;
  origin: "ogs" | "school";
}
