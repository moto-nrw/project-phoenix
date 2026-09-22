import { ChoiceTile } from "~/components/ui/choice-tile";
import { SectionCard } from "~/components/ui/section-card";
import {
  DEMO_ROLE_CHOICE_HINT,
  DEMO_ROLE_CHOICE_TITLE,
  DEMO_ROLES,
  type DemoRole,
} from "~/lib/demo-access";

// Role cards of the demo entry page (#3467). A link from the website or a
// fair QR code usually brings a role; a link without one asks here.
export function DemoRoleChoice({
  onChoose,
}: {
  readonly onChoose: (role: DemoRole) => void;
}) {
  return (
    <main className="flex min-h-dvh items-center justify-center px-4 py-8">
      <SectionCard
        headingLevel={1}
        title={DEMO_ROLE_CHOICE_TITLE}
        description={DEMO_ROLE_CHOICE_HINT}
        className="w-full max-w-md"
      >
        <div className="flex flex-col gap-3">
          {DEMO_ROLES.map((entry) => (
            <ChoiceTile
              key={entry.role}
              as="button"
              className="flex-col items-start gap-1 p-4"
              onClick={() => onChoose(entry.role)}
            >
              <span className="text-base font-semibold text-gray-900">
                {entry.label}
              </span>
              <span className="font-normal text-gray-600">
                {entry.description}
              </span>
            </ChoiceTile>
          ))}
        </div>
      </SectionCard>
    </main>
  );
}
