"use client";

import { useState } from "react";
import { Alert } from "~/components/ui/alert";
import { Button } from "~/components/ui/button";
import { ContactlessPaymentIcon, QuestionIcon } from "@phosphor-icons/react";
import { SegmentedControl } from "~/components/ui/segmented-control";
import { Tooltip } from "~/components/ui/tooltip";
import type { SchoolSetupBasics } from "~/lib/school-setup-api";

/** Die Antworten des ersten Schritts, wie das Formular sie abschickt. */
export interface SchoolSetupBasicsAnswers {
  presenceMode: SchoolSetupBasics["presenceMode"];
  timetableEnabled: boolean;
  groupMode: SchoolSetupBasics["groupMode"];
  parentAppUsed: boolean;
}

interface SchoolSetupBasicsFormProps {
  readonly basics: SchoolSetupBasics;
  readonly saving: boolean;
  readonly error: string | null;
  readonly onSubmit: (answers: SchoolSetupBasicsAnswers) => void;
}

type YesNo = "yes" | "no";

const YES_NO = [
  { value: "yes", label: "Ja" },
  { value: "no", label: "Nein" },
] as const;

function Question({
  id,
  label,
  info,
  hint,
  notice,
  children,
}: Readonly<{
  id: string;
  label: string;
  /** Kurze Erklärung hinter dem Fragezeichen: was die Antwort bedeutet. */
  info: string;
  hint?: readonly string[];
  /** Ein Hinweis, der die Wahl einschränkt: als Info-Kasten statt Kleingedrucktem. */
  notice?: { title: string; message: string };
  children: React.ReactNode;
}>) {
  return (
    <fieldset className="flex flex-col gap-2" aria-labelledby={id}>
      <legend
        id={id}
        className="flex items-center gap-1.5 text-sm font-medium text-gray-900"
      >
        {label}
        <Tooltip content={info} bubbleClassName="w-72">
          <QuestionIcon
            className="h-4 w-4 text-gray-500"
            aria-label={`Was heißt das? ${label}`}
          />
        </Tooltip>
      </legend>
      {children}
      {hint?.map((line) => (
        <p key={line} className="text-sm text-gray-600">
          {line}
        </p>
      ))}
      {notice && (
        <Alert
          type="info"
          title={notice.title}
          message={notice.message}
          announce="off"
        />
      )}
    </fieldset>
  );
}

/**
 * Der erste Schritt (#2832): vier Fragen auf einer Seite. Gruppen und
 * Betreuungsplan sind normale Einstellungen; die Anwesenheitsart darf die
 * Schule nur hier festlegen, danach nur noch über das moto-Team.
 */
export function SchoolSetupBasicsForm({
  basics,
  saving,
  error,
  onSubmit,
}: SchoolSetupBasicsFormProps) {
  const [presenceMode, setPresenceMode] = useState(basics.presenceMode);
  const [timetable, setTimetable] = useState<YesNo>(
    basics.timetableEnabled ? "yes" : "no",
  );
  const [groupMode, setGroupMode] = useState(basics.groupMode);
  const [parentApp, setParentApp] = useState<YesNo | null>(
    basics.parentAppUsed === null ? null : basics.parentAppUsed ? "yes" : "no",
  );

  const [missingAnswer, setMissingAnswer] = useState(false);

  return (
    <form
      className="flex flex-col gap-5"
      onSubmit={(event) => {
        event.preventDefault();
        if (parentApp === null) {
          setMissingAnswer(true);
          return;
        }
        setMissingAnswer(false);
        onSubmit({
          presenceMode,
          timetableEnabled: timetable === "yes",
          groupMode,
          parentAppUsed: parentApp === "yes",
        });
      }}
    >
      <Question
        id="setup-presence"
        info="„Anwesend oder abwesend“: moto zeigt nur, ob ein Kind da ist. „Auch den Raum“: moto zeigt zusätzlich, in welchem Raum das Kind gerade ist."
        label="Was soll moto erfassen?"
        notice={{
          title: "Nur jetzt wählbar",
          message:
            "Später können Sie die Anwesenheitsart nur über das moto-Team ändern.",
        }}
      >
        <SegmentedControl
          items={[
            { value: "binary", label: "Anwesend oder abwesend" },
            { value: "detailed", label: "Auch den Raum" },
          ]}
          value={presenceMode}
          onChange={setPresenceMode}
          fullWidth
        />
      </Question>

      <Question
        id="setup-groups"
        label="Arbeiten Sie mit festen Gruppen?"
        info="Feste Gruppen: Jedes Kind gehört zu einer Gruppe mit eigener Leitung und eigenem Raum. Offene Betreuung: Die Kinder gehören zu keiner festen Gruppe und bewegen sich frei."
      >
        <SegmentedControl
          items={[
            { value: "fixed_groups", label: "Feste Gruppen" },
            { value: "open_care", label: "Offene Betreuung" },
          ]}
          value={groupMode}
          onChange={setGroupMode}
          fullWidth
        />
      </Question>

      <Question
        id="setup-timetable"
        info="Im Betreuungsplan planen Sie AGs und Angebote für die Woche: wann, wo und mit wem. Ohne Betreuungsplan fehlt dieser Bereich in moto."
        label="Planen Sie Angebote mit dem Betreuungsplan?"
      >
        <SegmentedControl
          items={YES_NO}
          value={timetable}
          onChange={setTimetable}
          fullWidth
        />
      </Question>

      <Question
        id="setup-parents"
        info="In der Eltern-App melden Eltern ihr Kind krank und lesen Nachrichten der OGS. Dafür laden Sie die Eltern per E-Mail ein."
        label="Sollen Eltern die Eltern-App nutzen?"
        hint={["Bei „Nein“ fällt der Schritt „Erste Eltern einladen“ weg."]}
      >
        <SegmentedControl
          items={YES_NO}
          value={parentApp}
          onChange={setParentApp}
          fullWidth
        />
      </Question>

      {/* Ein freiwilliges Angebot, keine Einschränkung: deshalb eine ruhige
          Zeile am Ende statt eines Hinweiskastens an der Frage. */}
      <p className="flex items-start gap-2 text-sm text-gray-600">
        <ContactlessPaymentIcon
          size={18}
          className="mt-0.5 shrink-0 text-gray-500"
          aria-hidden
        />
        <span>
          Sie möchten die Anwesenheit mit NFC-Armbändern erfassen? Dann melden
          Sie sich beim moto-Team.
        </span>
      </p>

      {missingAnswer && (
        <Alert
          type="error"
          message="Bitte sagen Sie noch, ob Eltern die Eltern-App nutzen sollen."
        />
      )}
      {error && <Alert type="error" message={error} />}

      <div className="flex justify-end">
        <Button type="submit" variant="primary" size="md" disabled={saving}>
          {saving ? "Wird gespeichert…" : "Speichern und weiter"}
        </Button>
      </div>
    </form>
  );
}
