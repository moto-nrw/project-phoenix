import { MotoBrand } from "~/components/auth/auth-shell";
import { ButtonLink } from "~/components/ui/button";

export default function NotFound() {
  return (
    <main className="moto-dotted-background moto-dotted-background--fullscreen flex min-h-dvh flex-col items-center justify-center px-4 py-16">
      <div className="relative flex flex-col items-center text-center">
        <MotoBrand />
        <div aria-hidden="true" className="mt-8 flex items-center">
          <span className="text-[6.5rem] leading-none font-extrabold text-gray-950 sm:text-[10rem]">
            4
          </span>
          <span className="border-moto-green mx-2 inline-block size-[4.6rem] rounded-full border-[1.05rem] sm:mx-3 sm:size-[7rem] sm:border-[1.6rem]" />
          <span className="text-[6.5rem] leading-none font-extrabold text-gray-950 sm:text-[10rem]">
            4
          </span>
        </div>
        <h1 className="mt-8 text-3xl font-semibold text-gray-950 sm:text-4xl">
          Seite nicht gefunden
        </h1>
        <p className="mt-3 max-w-md text-sm leading-6 text-gray-600 sm:text-base">
          Diese Seite gibt es nicht. Vielleicht ist die Adresse falsch.
        </p>
        <ButtonLink href="/" variant="primary" size="base" className="mt-9">
          Zur Startseite
        </ButtonLink>
      </div>
    </main>
  );
}
