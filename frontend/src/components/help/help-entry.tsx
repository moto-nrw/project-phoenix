"use client";

import { useState } from "react";
import { GraduationCap, HeartHandshake, Users, UserRound } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { Button } from "~/components/ui/button";
import { ChoiceTile } from "~/components/ui/choice-tile";
import type { HelpGroupMode, HelpPresenceMode, HelpRole } from "./help-content";

/**
 * Einstieg in die Hilfe: wir fragen, fuer wen die Anleitung ist.
 *
 * Ohne Parameter ist nicht bekannt, wer die Hilfe öffnet. Ein stiller
 * Rueckfall auf die Betreuungskraft zeigte allen dieselbe Anleitung --
 * Leitungsthemen fehlten, Elternthemen standen falsch da. Aus der App
 * heraus setzt `context-help-link` die Rolle mit, dieser Einstieg greift
 * also nur beim direkten Aufruf von `/help`.
 *
 * Schritt 2 erscheint nur für Rollen, deren Anleitung sich mit den
 * Einstellungen der OGS ändert. Eltern und Lehrkräfte arbeiten in eigenen
 * Portalen, in denen NFC, Gruppenmodell und Anwesenheits-Modus keinen
 * Ablauf verändern -- sie danach zu fragen, wäre eine Frage ohne Wirkung.
 */
export const ROLES_WITH_SCHOOL_STEP: readonly HelpRole[] = [
  "caregiver",
  "lead",
];

export interface HelpEntryAnswers {
  readonly role: HelpRole;
  readonly nfcEnabled: boolean | null;
  readonly groupMode: HelpGroupMode;
  readonly presenceMode: HelpPresenceMode;
}

interface RoleOption {
  readonly role: HelpRole;
  readonly label: string;
  readonly description: string;
  readonly icon: LucideIcon;
}

const ROLE_OPTIONS: readonly RoleOption[] = [
  {
    role: "caregiver",
    label: "Betreuungskraft",
    description: "Sie betreuen Kinder in der OGS.",
    icon: HeartHandshake,
  },
  {
    role: "lead",
    label: "Leitung",
    description: "Sie leiten die OGS und das Team.",
    icon: Users,
  },
  {
    role: "parent",
    label: "Elternteil",
    description: "Ihr Kind wird in der OGS betreut.",
    icon: UserRound,
  },
  {
    role: "teacher",
    label: "Lehrkraft",
    description: "Sie arbeiten mit moto schule.",
    icon: GraduationCap,
  },
];

interface SchoolQuestion<TValue> {
  readonly id: string;
  readonly question: string;
  readonly hint?: string;
  readonly options: readonly {
    readonly value: TValue;
    readonly label: string;
  }[];
}

const NFC_QUESTION: SchoolQuestion<boolean | null> = {
  id: "nfc",
  question: "Nutzt Ihre OGS ein NFC-Tablet?",
  hint: "Am Tablet melden sich Kinder mit einem Armband an.",
  options: [
    { value: true, label: "Ja" },
    { value: false, label: "Nein" },
    { value: null, label: "Weiß ich nicht" },
  ],
};

const GROUP_QUESTION: SchoolQuestion<HelpGroupMode> = {
  id: "group",
  question: "Arbeitet Ihre OGS mit festen Gruppen?",
  options: [
    { value: "fixed_groups", label: "Feste Gruppen" },
    { value: "open_care", label: "Offene Betreuung" },
    { value: "unknown", label: "Weiß ich nicht" },
  ],
};

const PRESENCE_QUESTION: SchoolQuestion<HelpPresenceMode> = {
  id: "presence",
  question: "Wie hält Ihre OGS die Anwesenheit fest?",
  options: [
    { value: "detailed", label: "Mit Ort des Kindes" },
    { value: "binary", label: "Nur da oder nicht da" },
    { value: "unknown", label: "Weiß ich nicht" },
  ],
};

function QuestionBlock<TValue>({
  question,
  value,
  onChange,
}: Readonly<{
  question: SchoolQuestion<TValue>;
  value: TValue | undefined;
  onChange: (value: TValue) => void;
}>) {
  return (
    <fieldset className="border-0 p-0">
      <legend className="text-base font-semibold text-gray-950">
        {question.question}
      </legend>
      {question.hint ? (
        <p className="mt-1 text-sm text-gray-600">{question.hint}</p>
      ) : null}
      <div className="mt-3 grid gap-2 sm:grid-cols-3">
        {question.options.map((option) => (
          <ChoiceTile
            key={String(option.value)}
            as="button"
            tone="green"
            selected={option.value === value}
            aria-pressed={option.value === value}
            onClick={() => onChange(option.value)}
            className="min-h-11 justify-center text-center"
          >
            {option.label}
          </ChoiceTile>
        ))}
      </div>
    </fieldset>
  );
}

export function HelpEntry({
  onSubmit,
}: Readonly<{ onSubmit: (answers: HelpEntryAnswers) => void }>) {
  const [role, setRole] = useState<HelpRole | null>(null);
  // `undefined` heisst "noch nicht beantwortet", nicht "weiss ich nicht".
  // Waere "Weiss ich nicht" vorausgewaehlt, sieht die Frage beantwortet aus
  // und niemand merkt, dass er sie haette beantworten koennen.
  const [nfcEnabled, setNfcEnabled] = useState<boolean | null | undefined>(
    undefined,
  );
  const [groupMode, setGroupMode] = useState<HelpGroupMode | undefined>(
    undefined,
  );
  const [presenceMode, setPresenceMode] = useState<
    HelpPresenceMode | undefined
  >(undefined);

  function chooseRole(next: HelpRole) {
    if (ROLES_WITH_SCHOOL_STEP.includes(next)) {
      setRole(next);
      return;
    }
    onSubmit({
      role: next,
      nfcEnabled: null,
      groupMode: "unknown",
      presenceMode: "unknown",
    });
  }

  const showSchoolStep = role !== null;

  return (
    <>
      {/* Gleiche Kopfzeile wie in der Anleitung, damit der Einstieg nicht wie
          eine fremde Seite wirkt. */}
      <header className="relative border-b border-gray-200 bg-white">
        <div className="mx-auto flex h-20 w-full max-w-3xl items-center gap-3 px-4">
          <span className="text-xl font-bold tracking-tight text-gray-950">
            moto Hilfe
          </span>
        </div>
      </header>

      {/* `relative` ist nicht kosmetisch: .moto-dotted-background::before liegt
          absolut positioniert ueber statischem Inhalt und malt das Punktmuster
          sonst quer ueber die Kacheln, die dadurch durchsichtig wirken. */}
      <main className="relative mx-auto w-full max-w-3xl px-4 py-10 sm:py-16">
        <p className="text-moto-green-strong text-sm font-bold tracking-wide uppercase">
          {showSchoolStep ? "Schritt 2 von 2" : "Schritt 1 von 2"}
        </p>

        {showSchoolStep ? (
          <>
            <h1 className="mt-3 text-3xl font-semibold tracking-tight text-balance text-gray-950 sm:text-4xl">
              Wie arbeitet Ihre OGS?
            </h1>
            <p className="mt-3 text-base text-gray-600">
              So zeigen wir Ihnen nur die Schritte, die bei Ihnen vorkommen.
              Wissen Sie etwas nicht? Wählen Sie „Weiß ich nicht“.
            </p>

            {/* Ohne weisse Karte: die weissen Antwort-Kacheln heben sich vom
              grauen Seitengrund ab, auf Weiss verschwaemmen sie. */}
            <div className="mt-8 space-y-8">
              <QuestionBlock
                question={NFC_QUESTION}
                value={nfcEnabled}
                onChange={setNfcEnabled}
              />
              <QuestionBlock
                question={GROUP_QUESTION}
                value={groupMode}
                onChange={setGroupMode}
              />
              <QuestionBlock
                question={PRESENCE_QUESTION}
                value={presenceMode}
                onChange={setPresenceMode}
              />
            </div>

            <div className="mt-6 flex flex-wrap items-center gap-3">
              <Button
                type="button"
                variant="primary"
                size="md"
                onClick={() =>
                  onSubmit({
                    role,
                    nfcEnabled: nfcEnabled ?? null,
                    groupMode: groupMode ?? "unknown",
                    presenceMode: presenceMode ?? "unknown",
                  })
                }
              >
                Zur Anleitung
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="md"
                onClick={() =>
                  onSubmit({
                    role,
                    nfcEnabled: null,
                    groupMode: "unknown",
                    presenceMode: "unknown",
                  })
                }
              >
                Überspringen
              </Button>
              <Button
                type="button"
                variant="ghost"
                size="md"
                className="ms-auto"
                onClick={() => setRole(null)}
              >
                Zurück
              </Button>
            </div>
          </>
        ) : (
          <>
            <h1 className="mt-3 text-3xl font-semibold tracking-tight text-balance text-gray-950 sm:text-4xl">
              Für wen ist die Anleitung?
            </h1>
            <p className="mt-3 text-base text-gray-600">
              Wählen Sie aus, was auf Sie zutrifft. Sie sehen danach nur Ihre
              Themen.
            </p>

            <div className="mt-8 grid gap-3 sm:grid-cols-2">
              {ROLE_OPTIONS.map((option) => {
                const Icon = option.icon;
                return (
                  <ChoiceTile
                    key={option.role}
                    as="button"
                    tone="green"
                    onClick={() => chooseRole(option.role)}
                    className="min-h-11 items-start gap-4 p-4"
                  >
                    <Icon
                      className="text-moto-green-strong mt-0.5 h-6 w-6 shrink-0"
                      aria-hidden="true"
                    />
                    <span className="block">
                      <span className="block text-base font-semibold text-gray-950">
                        {option.label}
                      </span>
                      <span className="mt-1 block text-sm font-normal text-gray-600">
                        {option.description}
                      </span>
                    </span>
                  </ChoiceTile>
                );
              })}
            </div>
          </>
        )}
      </main>
    </>
  );
}
