import { EmptyState } from "~/components/ui/empty-state";
import { MotoBrand } from "~/components/auth/moto-brand";
import { ButtonLink } from "~/components/ui/button";

export default function NotFound() {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen flex min-h-dvh flex-col items-center justify-center px-4 py-16">
      <div className="relative flex flex-col items-center">
        <MotoBrand />
        <EmptyState
          titleAs="h1"
          title="Seite nicht gefunden"
          description="Diese Seite gibt es nicht. Vielleicht ist die Adresse falsch."
          className="py-0 pt-8"
          icon={
            <div className="flex items-center">
              <span className="text-[6.5rem] leading-none font-extrabold text-gray-950 sm:text-[10rem]">
                4
              </span>
              <span className="border-moto-green mx-2 inline-block size-[4.6rem] rounded-full border-[1.05rem] sm:mx-3 sm:size-[7rem] sm:border-[1.6rem]" />
              <span className="text-[6.5rem] leading-none font-extrabold text-gray-950 sm:text-[10rem]">
                4
              </span>
            </div>
          }
          action={
            <ButtonLink href="/" variant="primary" size="base">
              Zur Startseite
            </ButtonLink>
          }
        />
      </div>
    </main>
  );
}
