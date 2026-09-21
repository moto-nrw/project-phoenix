import { ButtonLink } from "~/components/ui/button";
import { EmptyState } from "~/components/ui/empty-state";
import { Loading } from "~/components/ui/loading";
import { DEMO_WEBSITE_URL } from "~/lib/demo-access";

/**
 * What a visitor of the public demo sees on the way in.
 * "unavailable": the demo school could not be set up; only a new request helps.
 */
export type DemoEntryPhase =
  "opening" | "preparing" | "invalid" | "failed" | "unavailable";

const PROBLEMS: Record<
  Exclude<DemoEntryPhase, "opening" | "preparing">,
  { title: string; description: string }
> = {
  invalid: {
    title: "Dieser Link funktioniert nicht mehr",
    description:
      "Ein Demo-Link gilt 14 Tage. Auf unserer Website bekommen Sie sofort einen neuen.",
  },
  failed: {
    title: "Das hat leider nicht geklappt",
    description:
      "Es liegt nicht an Ihnen. Bitte öffnen Sie den Link noch einmal. Oder fordern Sie einen neuen an.",
  },
  unavailable: {
    title: "Das hat leider nicht geklappt",
    description:
      "Es liegt nicht an Ihnen. Bitte fordern Sie auf unserer Website einen neuen Link an.",
  },
};

// Shared by the waiting room on the main domain and the entry page of the
// demo school (#3463), so both say the same thing in the same words.
export function DemoEntryMessage({
  phase,
}: Readonly<{ phase: DemoEntryPhase }>) {
  if (phase === "opening") return <Loading message="Demo wird geöffnet …" />;
  if (phase === "preparing") {
    return (
      <Loading message="Ihre Demo wird vorbereitet. Einen Moment bitte." />
    );
  }
  const problem = PROBLEMS[phase];
  return (
    <main className="flex min-h-dvh items-center justify-center px-4">
      <EmptyState
        title={problem.title}
        description={problem.description}
        action={
          <ButtonLink href={DEMO_WEBSITE_URL}>Neuen Link anfordern</ButtonLink>
        }
      />
    </main>
  );
}
