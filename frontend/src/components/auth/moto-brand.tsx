import Image from "next/image";

// The moto mark with its word. Kept out of auth-shell.tsx so the 404 page and
// the demo screens do not pull the whole login shell into their bundle.
export function MotoBrand() {
  return (
    <div className="flex items-center justify-center gap-3">
      <Image
        src="/images/moto_transparent.webp"
        alt=""
        width={44}
        height={32}
        className="h-8 w-11 object-contain"
        priority
      />
      <span className="[font-family:var(--font-moto)] text-[2rem] leading-none font-bold text-gray-950">
        moto
      </span>
    </div>
  );
}
