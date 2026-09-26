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
  DEMO_PRIVACY_URL,
  DEMO_RECORDING_NOTICE,
  DEMO_RECORDING_PRIVACY_LINK,
  DEMO_WEBSITE_URL,
} from "~/lib/demo-access";

// A titled demo screen groups its header, content and privacy notice in one card.
export function DemoShell({
  children,
  title,
  description,
}: {
  readonly children: ReactNode;
  readonly title?: string;
  readonly description?: string;
}) {
  const Container = title ? SectionCard : "div";
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen flex min-h-dvh flex-col items-center justify-center px-4 py-10">
      {/* `relative` lifts the content above the pattern's ::before layer. */}
      <Container
        className={`relative w-full ${title ? "max-w-lg" : "flex max-w-md flex-col items-center gap-8"}`}
        {...(title
          ? {
              bodyClassName: "flex flex-col gap-6",
              overflow: "visible" as const,
            }
          : {})}
      >
        {title && (
          <span className="border-moto-green/45 bg-moto-demo-green-soft text-moto-demo-green-strong absolute -top-4 right-5 rotate-3 rounded-full border px-4 py-2 text-xs font-semibold shadow-[0_3px_18px_rgba(131,205,45,0.18)] sm:right-7 sm:text-sm">
            Demo
          </span>
        )}
        <header className="space-y-5 text-center">
          <MotoBrand />
          {title && (
            <div className="space-y-2">
              <h1 className="text-2xl font-semibold tracking-tight text-gray-950">
                {title}
              </h1>
              {description && (
                <p className="text-sm leading-6 text-gray-600">{description}</p>
              )}
            </div>
          )}
        </header>
        {children}
        <footer className={title ? "border-t border-gray-200 pt-5" : undefined}>
          <p className="text-center text-xs leading-5 text-gray-500">
            {DEMO_RECORDING_NOTICE}{" "}
            <a
              href={DEMO_PRIVACY_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="underline underline-offset-2 hover:text-gray-700"
            >
              {DEMO_RECORDING_PRIVACY_LINK}
            </a>
          </p>
        </footer>
      </Container>
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
