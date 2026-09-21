import { CheckIcon, SpinnerIcon } from "~/components/ui/icons";
import {
  DEMO_SETUP_STEPS,
  type DemoSetupProgress,
  demoSetupTitle,
} from "~/lib/demo-access";

const LINE_STATE = { done: "erledigt", running: "läuft", next: "folgt" };

// Setup screen of the public demo (#3464): names the visitor's OGS and shows
// what is being prepared, line by line. The kit's Loading carries one message
// only; the waiting room and the school's entry page both show this list while
// a demo school is seeded. Screen readers hear the running line.
export function DemoSetupScreen({ schoolName, step }: DemoSetupProgress) {
  const running = DEMO_SETUP_STEPS[step] ?? DEMO_SETUP_STEPS[0];
  return (
    <main className="flex min-h-dvh items-center justify-center px-4 py-8">
      <section
        aria-busy="true"
        aria-labelledby="demo-setup-title"
        className="moto-content-surface w-full max-w-md rounded-2xl border p-6 shadow-sm"
      >
        <h1
          id="demo-setup-title"
          className="text-lg font-semibold break-words text-gray-900"
        >
          {demoSetupTitle(schoolName)}
        </h1>
        <p className="mt-1 text-sm text-gray-500">
          Das dauert meist weniger als eine Minute.
        </p>
        <ol className="mt-5 space-y-3">
          {DEMO_SETUP_STEPS.map((label, index) => {
            const state =
              index < step ? "done" : index === step ? "running" : "next";
            return (
              <li key={label} className="flex items-center gap-3 text-sm">
                <span
                  aria-hidden="true"
                  className="flex h-5 w-5 shrink-0 items-center justify-center"
                >
                  {state === "done" ? (
                    <CheckIcon className="text-moto-green-strong h-5 w-5" />
                  ) : state === "running" ? (
                    <SpinnerIcon className="h-4 w-4 text-gray-600 motion-reduce:animate-none" />
                  ) : (
                    <span className="h-2 w-2 rounded-full bg-gray-300" />
                  )}
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
                <span className="sr-only">{LINE_STATE[state]}</span>
              </li>
            );
          })}
        </ol>
        <output aria-live="polite" className="sr-only">
          {running} …
        </output>
      </section>
    </main>
  );
}
