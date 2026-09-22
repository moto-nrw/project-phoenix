"use client";

import {
  CaretDownIcon,
  CaretUpIcon,
  CheckCircleIcon,
  CircleIcon,
  ListChecksIcon,
  MinusCircleIcon,
} from "@phosphor-icons/react";
import { Button, ButtonLink } from "~/components/ui/button";
import { ProgressBar } from "~/components/ui/progress-bar";
import { TileCard } from "~/components/ui/tile-card";
import type { HelpTopicId } from "~/lib/help-topics";
import { LOCATION_COLORS } from "~/lib/location-helper";
import type {
  SchoolSetupStep,
  SchoolSetupStepKey,
} from "~/lib/school-setup-api";
import {
  fillTopics,
  NEXT_HELP_GROUPS,
  SCHOOL_SETUP_STEP_CONTENT,
  type SchoolSetupNextTopic,
} from "./school-setup-steps";
import { SETUP_TOURS } from "./setup-tours";

// Auf dem Handy links: rechts unten sitzt dort der grüne „+“-Knopf der
// Datenseiten (DatabaseCreateAction), über der unteren Navigation.
const BEACON_POSITION =
  "fixed bottom-20 left-4 z-40 sm:left-auto sm:right-6 sm:bottom-6";
const PANEL_POSITION =
  "fixed right-4 bottom-20 left-4 z-40 sm:left-auto sm:right-6 sm:bottom-6";

interface BeaconProps {
  readonly open: number;
  readonly onOpen: () => void;
}

/** Der Knopf unten rechts, der die Checkliste öffnet. */
export function SchoolSetupBeacon({ open, onOpen }: BeaconProps) {
  return (
    <div className={BEACON_POSITION}>
      {/* Wie ein Helfer-Knopf zum Aufklappen: Symbol, runder Zähler, Pfeil
          nach oben und ein Schatten, der ihn über die Seite hebt. */}
      <Button
        type="button"
        variant="success"
        size="md"
        onClick={onOpen}
        aria-label={
          open === 0
            ? "Erste Schritte öffnen, alles erledigt"
            : `Erste Schritte öffnen, ${open} offen`
        }
        className="gap-2 rounded-full py-2.5 pr-3 pl-4 shadow-lg hover:shadow-xl"
      >
        <ListChecksIcon size={20} weight="bold" aria-hidden />
        <span>Erste Schritte</span>
        <span
          aria-hidden
          className="flex h-6 min-w-6 items-center justify-center rounded-full bg-white px-1.5 text-xs font-semibold text-gray-900"
        >
          {open === 0 ? "✓" : open}
        </span>
        <CaretUpIcon size={16} weight="bold" aria-hidden />
      </Button>
    </div>
  );
}

function StepIcon({ step }: Readonly<{ step: SchoolSetupStep }>) {
  if (step.done) {
    return (
      <CheckCircleIcon
        size={20}
        weight="fill"
        style={{ color: LOCATION_COLORS.GROUP_ROOM }}
        aria-hidden
      />
    );
  }
  if (step.skipped) {
    return <MinusCircleIcon size={20} className="text-gray-400" aria-hidden />;
  }
  return <CircleIcon size={20} className="text-gray-400" aria-hidden />;
}

function TopicList({
  title,
  topics,
  helpHref,
  helpGroupHref,
}: Readonly<{
  title: string;
  topics: readonly SchoolSetupNextTopic[];
  helpHref: (topic: HelpTopicId) => string;
  helpGroupHref: (group: string) => string;
}>) {
  if (topics.length === 0) return null;
  const hrefFor = (topic: SchoolSetupNextTopic) =>
    "topic" in topic.link
      ? helpHref(topic.link.topic)
      : helpGroupHref(topic.link.group);
  return (
    <section className="flex flex-col gap-2" aria-label={title}>
      <h4 className="text-sm font-semibold text-gray-900">{title}</h4>
      <ul className="flex flex-col gap-2">
        {topics.map((topic) => (
          <li key={topic.title}>
            <TileCard href={hrefFor(topic)}>
              <span className="block text-sm font-semibold text-gray-900">
                {topic.title}
              </span>
              <span className="block text-sm text-gray-600">
                {topic.description}
              </span>
            </TileCard>
          </li>
        ))}
      </ul>
    </section>
  );
}

function stepStatus(step: SchoolSetupStep): string {
  if (step.done) return "Erledigt";
  if (step.skipped) return "Übersprungen";
  return "Offen";
}

interface ChecklistProps {
  readonly steps: readonly SchoolSetupStep[];
  readonly expanded: SchoolSetupStepKey | null;
  readonly busy: boolean;
  readonly error: string | null;
  /** Ein ruhiger Hinweis, etwa warum eine Tour endete. */
  readonly notice: string | null;
  readonly helpHref: (topic: HelpTopicId) => string;
  readonly helpGroupHref: (group: string) => string;
  readonly onExpand: (step: SchoolSetupStepKey | null) => void;
  readonly onOpenBasics: () => void;
  readonly onStartTour: (step: SchoolSetupStepKey) => void;
  readonly onSkip: (step: SchoolSetupStepKey, skipped: boolean) => void;
  readonly onComplete: () => void;
  readonly onCollapse: () => void;
  readonly onDismiss: () => void;
}

/**
 * Die Checkliste der ersten Schritte (#2832): unten rechts, über allen
 * Seiten. Der nächste offene Schritt ist aufgeklappt und führt per Tour
 * dorthin; sind alle erledigt oder übersprungen, gratuliert sie und bietet
 * den Abschluss an.
 */
export function SchoolSetupChecklist({
  steps,
  expanded,
  busy,
  error,
  notice,
  helpHref,
  helpGroupHref,
  onExpand,
  onOpenBasics,
  onStartTour,
  onSkip,
  onComplete,
  onCollapse,
  onDismiss,
}: ChecklistProps) {
  const finished = steps.filter((step) => step.done || step.skipped).length;
  const allFinished = finished === steps.length;

  return (
    <section
      aria-label="Erste Schritte mit moto"
      className={`${PANEL_POSITION} moto-popover-surface flex max-h-[calc(100dvh-7rem)] flex-col overflow-hidden rounded-xl border sm:w-96`}
    >
      <header className="flex flex-col gap-2 border-b border-gray-100 p-4">
        <div className="flex items-center justify-between gap-2">
          <h2 className="text-base font-semibold text-gray-900">
            Erste Schritte mit moto
          </h2>
          <Button
            type="button"
            variant="ghost"
            size="icon"
            onClick={onCollapse}
            aria-label="Checkliste einklappen"
          >
            <CaretDownIcon size={18} aria-hidden />
          </Button>
        </div>
        <p className="text-sm text-gray-600">
          {finished} von {steps.length} erledigt
        </p>
        <ProgressBar
          value={finished}
          max={steps.length}
          label={`${finished} von ${steps.length} Schritten erledigt`}
        />
      </header>

      <div className="flex-1 overflow-y-auto p-2">
        {allFinished ? (
          <div className="flex flex-col gap-3 p-2">
            <div className="flex flex-col gap-1">
              <h3 className="text-base font-semibold text-gray-900">
                Herzlichen Glückwunsch!
              </h3>
              {/* Ein Eintrag hakt einen Schritt ab: Jetzt kommen die übrigen
                  Daten, nicht „fertig“. */}
              <p className="text-sm text-gray-700">
                Die ersten Einträge stehen. Legen Sie jetzt die übrigen Daten
                Ihrer OGS an. Die Anleitungen zeigen, wie es geht.
              </p>
            </div>
            <TopicList
              title="Jetzt die übrigen Daten anlegen"
              topics={fillTopics(steps)}
              helpHref={helpHref}
              helpGroupHref={helpGroupHref}
            />
            <TopicList
              title="Danach"
              topics={NEXT_HELP_GROUPS}
              helpHref={helpHref}
              helpGroupHref={helpGroupHref}
            />
            <p className="text-sm text-gray-600">
              Mit „Abschließen“ verschwindet die Checkliste für alle. Die
              Anleitungen bleiben unter „Hilfe“.
            </p>
            <Button
              type="button"
              variant="primary"
              size="md"
              disabled={busy}
              onClick={onComplete}
            >
              Abschließen
            </Button>
          </div>
        ) : (
          <ol className="flex flex-col gap-1">
            {steps.map((step) => {
              const content = SCHOOL_SETUP_STEP_CONTENT[step.key];
              const isExpanded = expanded === step.key;
              const panelId = `school-setup-step-${step.key}`;
              // Die Tour ergibt erst Sinn, wenn der vorausgesetzte Schritt
              // wirklich erledigt ist (etwa: erst ein Kind, dann Eltern).
              const required = content.requires;
              const blockedBy =
                required &&
                !steps.find((candidate) => candidate.key === required.step)
                  ?.done
                  ? required
                  : null;
              return (
                <li key={step.key} className="rounded-lg">
                  <button
                    type="button"
                    className="flex w-full items-center gap-3 rounded-lg px-2 py-2 text-left hover:bg-gray-50"
                    aria-expanded={isExpanded}
                    aria-controls={panelId}
                    onClick={() => onExpand(isExpanded ? null : step.key)}
                  >
                    <StepIcon step={step} />
                    <span className="flex-1">
                      <span
                        className={`block text-sm font-medium ${
                          step.done
                            ? "text-gray-500 line-through"
                            : "text-gray-900"
                        }`}
                      >
                        {content.title}
                      </span>
                      <span className="sr-only">{stepStatus(step)}</span>
                    </span>
                    {isExpanded ? (
                      <CaretUpIcon
                        size={16}
                        className="text-gray-400"
                        aria-hidden
                      />
                    ) : (
                      <CaretDownIcon
                        size={16}
                        className="text-gray-400"
                        aria-hidden
                      />
                    )}
                  </button>
                  {isExpanded && (
                    <div
                      id={panelId}
                      className="flex flex-col gap-3 px-2 pb-3 pl-10"
                    >
                      <div className="flex flex-col gap-1">
                        <p className="text-sm text-gray-700">
                          {content.description}
                        </p>
                        {content.hints.map((hint) => (
                          <p key={hint} className="text-sm text-gray-600">
                            {hint}
                          </p>
                        ))}
                        {step.skipped && (
                          <p className="text-sm font-medium text-gray-900">
                            Übersprungen.
                          </p>
                        )}
                        {blockedBy && (
                          <p className="text-sm font-medium text-gray-900">
                            {blockedBy.reason}
                          </p>
                        )}
                      </div>
                      {error && (
                        <p
                          className="text-moto-red-strong text-sm"
                          role="alert"
                        >
                          {error}
                        </p>
                      )}
                      {notice && (
                        <p
                          className="text-sm font-medium text-gray-900"
                          role="status"
                        >
                          {notice}
                        </p>
                      )}
                      <div className="flex flex-wrap gap-2">
                        {step.key === "basics" ? (
                          <Button
                            type="button"
                            variant="primary"
                            size="compact"
                            onClick={onOpenBasics}
                          >
                            {step.done
                              ? "Antworten ändern"
                              : "Fragen beantworten"}
                          </Button>
                        ) : (
                          !step.done &&
                          (blockedBy ? (
                            <Button
                              type="button"
                              variant="primary"
                              size="compact"
                              onClick={() => onExpand(blockedBy.step)}
                            >
                              Zu „
                              {SCHOOL_SETUP_STEP_CONTENT[blockedBy.step].title}“
                            </Button>
                          ) : (
                            SETUP_TOURS[step.key] && (
                              <Button
                                type="button"
                                variant="primary"
                                size="compact"
                                onClick={() => onStartTour(step.key)}
                              >
                                Zeig es mir
                              </Button>
                            )
                          ))
                        )}
                        {content.helpTopic && (
                          <ButtonLink
                            href={helpHref(content.helpTopic)}
                            variant="outline"
                            size="compact"
                          >
                            Anleitung lesen
                          </ButtonLink>
                        )}
                        {step.key !== "basics" && !step.done && (
                          <Button
                            type="button"
                            variant="ghost"
                            size="compact"
                            disabled={busy}
                            onClick={() => onSkip(step.key, !step.skipped)}
                          >
                            {step.skipped
                              ? "Doch nicht überspringen"
                              : "Überspringen"}
                          </Button>
                        )}
                      </div>
                    </div>
                  )}
                </li>
              );
            })}
          </ol>
        )}
      </div>

      <footer className="border-t border-gray-100 p-2">
        <Button
          type="button"
          variant="ghost"
          size="compact"
          disabled={busy}
          onClick={onDismiss}
        >
          Nicht mehr anzeigen
        </Button>
        <p className="px-2 pb-1 text-xs text-gray-500">
          Die Anleitungen zu allen Schritten finden Sie jederzeit unter „Hilfe“.
        </p>
      </footer>
    </section>
  );
}
