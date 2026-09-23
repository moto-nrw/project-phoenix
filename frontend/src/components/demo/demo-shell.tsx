import type { ReactNode } from "react";
import { MotoBrand } from "~/components/auth/moto-brand";
import { Button, ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import { SectionCard } from "~/components/ui/section-card";
import {
  DEMO_ENTRY_NEW_LINK,
  DEMO_ENTRY_OPENING,
  DEMO_ENTRY_PROBLEMS,
  DEMO_ENTRY_RETRY,
  DEMO_WEBSITE_URL,
} from "~/lib/demo-access";

// Page frame of every public demo screen (opening, setup lines, role cards,
// problems): the dotted app background with the moto mark above the card, as
// on the 404 page, so the demo opens in the look of the app.
export function DemoShell({ children }: { readonly children: ReactNode }) {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen flex min-h-dvh flex-col items-center justify-center px-4 py-10">
      {/* `relative` lifts the content above the pattern's ::before layer. */}
      <div className="relative flex w-full max-w-md flex-col items-center gap-8">
        <MotoBrand />
        {children}
      </div>
    </main>
  );
}

/** The short wait before the first status answer; same frame, no overlay. */
export function DemoOpening() {
  return (
    <DemoShell>
      <Loading message={DEMO_ENTRY_OPENING} fullPage={false} />
    </DemoShell>
  );
}

/**
 * A demo link that does not lead in. The waiting room and both entry pages
 * say the same thing (#3463): a failed wait offers another try, a dead or
 * unusable link sends the visitor to the website for a new one.
 */
export function DemoProblem({
  phase,
  onRetry,
}: {
  readonly phase: keyof typeof DEMO_ENTRY_PROBLEMS;
  readonly onRetry: () => void;
}) {
  const problem = DEMO_ENTRY_PROBLEMS[phase];
  return (
    <DemoShell>
      <SectionCard className="w-full">
        <EmptyState
          className="py-4"
          title={problem.title}
          description={problem.description}
          action={
            phase === "failed" ? (
              <Button type="button" onClick={onRetry}>
                {DEMO_ENTRY_RETRY}
              </Button>
            ) : (
              <ButtonLink href={DEMO_WEBSITE_URL}>
                {DEMO_ENTRY_NEW_LINK}
              </ButtonLink>
            )
          }
        />
      </SectionCard>
    </DemoShell>
  );
}
