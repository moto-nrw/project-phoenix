"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import {
  ArrowRight,
  CalendarPlus,
  CalendarRange,
  Check,
  ChevronDown,
  ClipboardList,
  type LucideIcon,
} from "lucide-react";

import {
  computeTimetableSetup,
  type TimetableEnrollmentStatus,
} from "~/lib/timetable-helpers";

interface TimetableSetupGuideProps {
  readonly hasActivePeriod: boolean;
  readonly activePeriodLabel: string | null;
  readonly enrollmentStatus: TimetableEnrollmentStatus;
  readonly enrollmentLabel: string | null;
  readonly hasPlan: boolean;
  readonly plannedCount: number;
  /** Opens "Schuljahre & Ferien" (or the create modal when none exists). */
  readonly onManagePeriods: () => void;
  /** Opens the create-appointment modal. */
  readonly onCreateEvent: () => void;
  /** Tenant-relative href to the enrollment admin overview. */
  readonly enrollmentHref: string;
}

interface GuideStep {
  readonly key: "period" | "enrollment" | "plan";
  readonly title: string;
  readonly description: string;
  readonly done: boolean;
  readonly meta: string;
  readonly action: string;
  readonly icon: LucideIcon;
  readonly onClick: (() => void) | null;
  readonly href: string | null;
  readonly optional: boolean;
  readonly applicable: boolean;
}

export function TimetableSetupGuide({
  hasActivePeriod,
  activePeriodLabel,
  enrollmentStatus,
  enrollmentLabel,
  hasPlan,
  plannedCount,
  onManagePeriods,
  onCreateEvent,
  enrollmentHref,
}: TimetableSetupGuideProps) {
  const setup = computeTimetableSetup({
    hasActivePeriod,
    enrollment: enrollmentStatus,
    hasPlan,
  });
  const { setupComplete } = setup;
  const [expanded, setExpanded] = useState(!setupComplete);

  useEffect(() => {
    if (!setupComplete) setExpanded(true);
  }, [setupComplete]);

  const steps: GuideStep[] = [
    {
      key: "period",
      title: "Planungszeitraum festlegen",
      description:
        "Lege fest, bis wann dein Betreuungsplan gilt — zum Beispiel ein Schuljahr oder Halbjahr.",
      done: setup.periodDone,
      meta: setup.periodDone
        ? (activePeriodLabel ?? "Aktiv")
        : "Noch nicht festgelegt",
      action: setup.periodDone ? "Zeitraum anpassen" : "Zeitraum festlegen",
      icon: CalendarRange,
      onClick: onManagePeriods,
      href: null,
      optional: false,
      applicable: true,
    },
    {
      key: "enrollment",
      title: "Mit der Anmeldung verknüpfen",
      description:
        "Verbinde den Plan mit der Online-Anmeldung, damit angemeldete Kinder direkt auftauchen.",
      done: setup.enrollmentDone,
      meta:
        enrollmentStatus === "unknown"
          ? "Optional"
          : enrollmentStatus === "active"
            ? (enrollmentLabel ?? "Anmeldung aktiv")
            : "Keine aktive Anmeldephase",
      action: setup.enrollmentDone
        ? "Anmeldung öffnen"
        : "Anmeldung einrichten",
      icon: ClipboardList,
      onClick: null,
      href: enrollmentHref,
      optional: true,
      applicable: setup.enrollmentApplicable,
    },
    {
      key: "plan",
      title: "Erste Woche planen",
      description:
        "Erstelle deine ersten Termine — zum Beispiel Mensa, Lernzeit oder eine AG.",
      done: setup.planDone,
      meta:
        plannedCount > 0
          ? `${plannedCount} ${plannedCount === 1 ? "Termin" : "Termine"} geplant`
          : "Noch keine Termine",
      action: "Termin erstellen",
      icon: CalendarPlus,
      onClick: onCreateEvent,
      href: null,
      optional: false,
      applicable: true,
    },
  ];

  const visibleSteps = steps.filter((step) => step.applicable);
  const summary = `${setup.completedSteps} von ${setup.totalSteps} Schritten erledigt`;

  const nextActionStep =
    steps.find((step) => !step.done && !step.optional) ??
    steps.find((step) => !step.done && step.optional && step.applicable) ??
    steps[steps.length - 1]!;

  const contentExpanded = !setupComplete || expanded;

  return (
    <section className="moto-content-surface overflow-hidden rounded-2xl border shadow-sm backdrop-blur-md">
      {setupComplete && (
        <button
          type="button"
          onClick={() => setExpanded((value) => !value)}
          aria-expanded={expanded}
          className="group flex w-full items-center justify-between gap-4 px-5 py-4 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
        >
          <span className="flex min-w-0 items-center gap-3.5">
            <span
              className="h-2.5 w-2.5 shrink-0 rounded-full bg-[#83CD2D]"
              aria-hidden="true"
            />
            <span className="min-w-0">
              <span className="block text-xs font-medium tracking-wide text-gray-500 uppercase">
                Einrichtung
              </span>
              <span className="mt-0.5 block text-base font-semibold text-gray-900">
                Betreuungsplan eingerichtet
              </span>
              <span className="mt-0.5 block text-sm text-gray-500">
                {summary}. Neue Termine legst du jederzeit über „Termin
                erstellen" an.
              </span>
            </span>
          </span>
          <span
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-gray-400 transition-colors group-hover:bg-gray-100 group-hover:text-gray-700"
            aria-hidden="true"
          >
            <ChevronDown
              className={`h-4 w-4 transition-transform duration-200 ${expanded ? "rotate-180" : ""}`}
              aria-hidden="true"
            />
          </span>
        </button>
      )}
      <div
        className="grid transition-[grid-template-rows] duration-200 ease-out motion-reduce:transition-none"
        style={{ gridTemplateRows: contentExpanded ? "1fr" : "0fr" }}
      >
        <div className="min-h-0 overflow-hidden">
          <div
            className={`h-px bg-gray-100 transition-opacity duration-150 ${setupComplete && expanded ? "opacity-100" : "opacity-0"}`}
            aria-hidden="true"
          />
          <div className="grid lg:grid-cols-[minmax(0,1fr)_20rem]">
            <div className="p-4 sm:p-5">
              <div className="border-b border-gray-100 pb-4">
                <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                  <div>
                    <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                      Einrichtung
                    </p>
                    <h2 className="mt-1 text-base font-semibold text-gray-900">
                      {setupComplete
                        ? "Einrichtung abgeschlossen"
                        : "Betreuungsplan einrichten"}
                    </h2>
                    <p className="mt-1 max-w-2xl text-sm text-gray-600">
                      {setupComplete
                        ? "Dein Plan ist startklar. Lege weitere Termine an oder passe den Zeitraum an, wenn sich etwas ändert."
                        : "Folge den Schritten: Zeitraum festlegen, optional mit der Anmeldung verknüpfen und die erste Woche planen."}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2 text-xs">
                    <StatusPill
                      label={setupComplete ? "Startklar" : "In Einrichtung"}
                      tone={setupComplete ? "success" : "neutral"}
                    />
                    <StatusPill
                      label={`${plannedCount} ${plannedCount === 1 ? "Termin" : "Termine"}`}
                      tone={plannedCount > 0 ? "info" : "neutral"}
                    />
                  </div>
                </div>
                <div className="mt-4">
                  <div className="flex items-center justify-between gap-3 text-xs text-gray-500">
                    <span>{summary}</span>
                    <span>{setup.progressPercent}%</span>
                  </div>
                  <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-gray-100">
                    <div
                      className="h-full rounded-full bg-gray-900 transition-[width]"
                      style={{ width: `${setup.progressPercent}%` }}
                    />
                  </div>
                </div>
              </div>

              <ol className="mt-2 divide-y divide-gray-100">
                {visibleSteps.map((step) => (
                  <li key={step.key}>
                    <StepRow step={step} />
                  </li>
                ))}
              </ol>
            </div>

            <aside className="moto-dotted-background moto-dotted-background--split border-t border-gray-100 p-4 sm:p-5 lg:border-t-0 lg:border-l">
              <div className="relative z-10 space-y-4">
                <div>
                  <p className="text-xs font-semibold tracking-wide text-gray-500 uppercase">
                    Startpunkt
                  </p>
                  <h3 className="mt-1 text-base font-semibold text-gray-900">
                    {setupComplete ? "Plan ausbauen" : nextActionStep.title}
                  </h3>
                  <p className="mt-1 text-sm text-gray-600">
                    {setupComplete
                      ? "Lege weitere Termine an oder passe deinen Planungszeitraum an."
                      : nextActionStep.description}
                  </p>
                </div>

                {setupComplete ? (
                  <button
                    type="button"
                    onClick={onCreateEvent}
                    className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    Termin erstellen
                  </button>
                ) : nextActionStep.href ? (
                  <Link
                    href={nextActionStep.href}
                    className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    {nextActionStep.action}
                  </Link>
                ) : (
                  <button
                    type="button"
                    onClick={nextActionStep.onClick ?? undefined}
                    className="inline-flex h-10 w-full items-center justify-center rounded-lg bg-gray-900 px-4 text-sm font-medium text-white transition-colors hover:bg-gray-700 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none"
                  >
                    {nextActionStep.action}
                  </button>
                )}

                <div className="moto-content-surface rounded-xl border p-3">
                  <p className="text-xs font-semibold text-gray-900">
                    Fortschritt
                  </p>
                  <div className="mt-3 space-y-2">
                    {visibleSteps.map((step) => (
                      <div
                        key={step.key}
                        className="flex items-center justify-between gap-3 px-2 py-1.5 text-xs"
                      >
                        <span className="min-w-0 truncate text-gray-700">
                          {step.title}
                        </span>
                        <span
                          className={`h-2.5 w-2.5 shrink-0 rounded-full ${step.done ? "bg-[#83CD2D]" : "bg-gray-300"}`}
                          aria-hidden="true"
                        />
                      </div>
                    ))}
                  </div>
                </div>

                <div className="text-xs leading-relaxed text-gray-500">
                  Pflicht sind Planungszeitraum und erste Termine. Die
                  Verknüpfung mit der Anmeldung ist optional und sorgt dafür,
                  dass angemeldete Kinder automatisch im Plan auftauchen.
                </div>
              </div>
            </aside>
          </div>
        </div>
      </div>
    </section>
  );
}

function StepRow({ step }: Readonly<{ step: GuideStep }>) {
  const StepIcon = step.icon;
  const rowClass =
    "group grid w-full gap-3 py-3 text-left transition-colors hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none sm:grid-cols-[2.25rem_1fr_auto] sm:items-center sm:px-2";
  const inner = (
    <>
      <span
        className={`flex h-9 w-9 items-center justify-center rounded-full border ${
          step.done
            ? "border-[#83CD2D]/30 bg-[#83CD2D]/15 text-[#5A8B1F]"
            : "border-gray-200 bg-white text-gray-500"
        }`}
        aria-hidden="true"
      >
        {step.done ? (
          <Check className="h-4 w-4" />
        ) : (
          <StepIcon className="h-4 w-4" />
        )}
      </span>
      <span className="min-w-0">
        <span className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium text-gray-900">
            {step.title}
          </span>
          {step.optional && (
            <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-medium text-gray-500">
              Optional
            </span>
          )}
        </span>
        <span className="mt-0.5 block text-sm text-gray-600">
          {step.description}
        </span>
      </span>
      <span className="flex flex-wrap items-center gap-2 sm:justify-end">
        <span
          className={`inline-flex items-center gap-1.5 text-[11px] font-medium whitespace-nowrap ${
            step.done ? "text-[#5A8B1F]" : "text-gray-500"
          }`}
        >
          <span
            className={`h-1.5 w-1.5 rounded-full ${step.done ? "bg-[#83CD2D]" : "bg-gray-300"}`}
            aria-hidden="true"
          />
          {step.meta}
        </span>
        <span className="inline-flex items-center gap-1 rounded-lg px-2 py-1 text-xs font-medium text-gray-700 transition-colors group-hover:bg-gray-100">
          {step.action}
          <ArrowRight className="h-3 w-3" aria-hidden="true" />
        </span>
      </span>
    </>
  );

  if (step.href) {
    return (
      <Link href={step.href} className={rowClass}>
        {inner}
      </Link>
    );
  }
  return (
    <button
      type="button"
      onClick={step.onClick ?? undefined}
      className={rowClass}
    >
      {inner}
    </button>
  );
}

function StatusPill({
  label,
  tone,
}: Readonly<{
  label: string;
  tone: "success" | "info" | "neutral";
}>) {
  const className =
    tone === "success"
      ? "bg-[#83CD2D]/15 text-[#5A8B1F]"
      : tone === "info"
        ? "bg-[#5080D8]/10 text-[#4070C8]"
        : "bg-gray-100 text-gray-600";
  return (
    <span
      className={`rounded-full px-2 py-0.5 text-[11px] font-medium ${className}`}
    >
      {label}
    </span>
  );
}
