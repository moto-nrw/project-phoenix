"use client";

import Image from "next/image";
import { signOut, useSession } from "next-auth/react";
import { LogOut } from "lucide-react";
import { Button } from "~/components/ui/button";
import { getUserDisplayName } from "~/lib/auth-utils";
import { schoolPath } from "~/lib/school-url";
import { DELIBERATE_LOGOUT_KEY } from "~/lib/session-cache";
import { createLogger } from "~/lib/logger";

const logger = createLogger({ component: "SchoolShell" });

/**
 * Slim app shell for the school portal ("moto schule", #2207).
 *
 * Deliberately NOT the tenant AppShell: the portal has exactly one surface
 * (the Klassenansicht), so it gets a single header bar — brand, user, logout
 * — instead of a sidebar chrome whose navigation would be empty.
 */
export function SchoolShell({
  children,
}: Readonly<{ children: React.ReactNode }>) {
  const { data: session } = useSession();

  const handleLogout = async () => {
    try {
      sessionStorage.setItem(DELIBERATE_LOGOUT_KEY, "1");
    } catch {
      // sessionStorage unavailable — the login page then shows a harmless
      // session-expired note instead of nothing.
    }
    try {
      await signOut({ redirect: false });
    } catch (err) {
      logger.warn("school_signout_failed", {
        error: err instanceof Error ? err.message : String(err),
      });
    }
    window.location.href = schoolPath("/school/login");
  };

  return (
    <div className="flex min-h-screen flex-col">
      <header className="moto-content-surface sticky top-0 z-20 border-b">
        <div className="mx-auto flex h-14 w-full max-w-6xl items-center justify-between gap-3 px-4 sm:px-6">
          <div className="flex items-center gap-2.5">
            <Image
              src="/images/moto_transparent.webp"
              alt=""
              width={33}
              height={24}
              className="h-6 w-auto object-contain"
              priority
            />
            <span className="[font-family:var(--font-moto)] text-xl leading-none font-bold text-gray-950">
              moto
            </span>
            <span className="rounded-full bg-[#5080D8]/10 px-2 py-0.5 text-xs font-semibold text-[#5080D8]">
              schule
            </span>
          </div>
          <div className="flex items-center gap-2">
            <span className="hidden text-sm text-gray-600 sm:block">
              {getUserDisplayName(session)}
            </span>
            <Button
              type="button"
              variant="ghost"
              size="compact"
              onClick={() => {
                void handleLogout();
              }}
            >
              <LogOut className="h-4 w-4" aria-hidden="true" />
              <span>Abmelden</span>
            </Button>
          </div>
        </div>
      </header>
      <main className="mx-auto w-full max-w-6xl flex-1 px-4 py-5 sm:px-6">
        {children}
      </main>
    </div>
  );
}
