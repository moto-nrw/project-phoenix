"use client";

import { ArrowLeft } from "lucide-react";
import { useRouter } from "next/navigation";

const buttonClassName =
  "inline-flex h-10 min-w-0 items-center gap-2 rounded-lg border border-gray-200 bg-white px-3 text-sm font-semibold text-gray-700 shadow-sm transition-colors hover:border-gray-300 hover:bg-gray-50 focus-visible:ring-2 focus-visible:ring-gray-400 focus-visible:outline-none";

export function HelpBackButton() {
  const router = useRouter();

  const handleClick = () => {
    if (document.referrer) {
      const referrer = new URL(document.referrer);
      if (referrer.origin === window.location.origin) {
        router.back();
        return;
      }
    }

    router.push("/");
  };

  return (
    <button type="button" onClick={handleClick} className={buttonClassName}>
      <ArrowLeft className="h-4 w-4 shrink-0" aria-hidden="true" />
      <span className="truncate">Zurück zur App</span>
    </button>
  );
}

export { buttonClassName as helpBackButtonClassName };
