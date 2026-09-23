import { DemoShell } from "~/components/demo/demo-shell";
import { CheckIcon, SpinnerIcon } from "~/components/ui/icons";
import { SectionCard } from "~/components/ui/section-card";
import {
  DEMO_SETUP_HINT,
  DEMO_SETUP_LINE_STATE,
  DEMO_SETUP_STEPS,
  type DemoSetupProgress,
  demoSetupTitle,
} from "~/lib/demo-access";

type LineState = keyof typeof DEMO_SETUP_LINE_STATE;

function lineState(index: number, step: number): LineState {
  if (index < step) return "done";
  return index === step ? "running" : "next";
}

function LineIcon({ state }: { readonly state: LineState }) {
  if (state === "done") {
    return <CheckIcon className="text-moto-green-strong h-5 w-5" />;
  }
  if (state === "running") {
    return (
      <SpinnerIcon className="h-4 w-4 text-gray-600 motion-reduce:animate-none" />
    );
  }
  return <span className="h-2 w-2 rounded-full bg-gray-300" />;
}

// Setup screen of the public demo (#3464): names the visitor's OGS and shows
// what is being prepared, line by line. The kit's Loading carries one message
// only; the waiting room and the school's entry page both show this list while
// a demo school is seeded. Screen readers hear the running line.
export function DemoSetupScreen({ schoolName, step }: DemoSetupProgress) {
  const running = DEMO_SETUP_STEPS[step] ?? DEMO_SETUP_STEPS[0];
  return (
    <DemoShell>
      <SectionCard
        headingLevel={1}
        title={demoSetupTitle(schoolName)}
        titleClassName="break-words"
        description={DEMO_SETUP_HINT}
        className="w-full max-w-md"
      >
        <ol className="space-y-3">
          {DEMO_SETUP_STEPS.map((label, index) => {
            const state = lineState(index, step);
            return (
              <li key={label} className="flex items-center gap-3 text-sm">
                <span
                  aria-hidden="true"
                  className="flex h-5 w-5 shrink-0 items-center justify-center"
                >
                  <LineIcon state={state} />
                </span>
                <span
                  className={
                    state === "next"
                      ? "text-gray-400"
                      : "font-medium text-gray-900"
                  }
                >
                  {label}
                </span>
                <span className="sr-only">{DEMO_SETUP_LINE_STATE[state]}</span>
              </li>
            );
          })}
        </ol>
        {/* No aria-busy around it: some screen readers hold back live
            announcements inside a busy region. */}
        <output aria-live="polite" className="sr-only">
          {running} …
        </output>
      </SectionCard>
    </DemoShell>
  );
}
