/**
 * Client-side logout page that invokes the proxy logout API and redirects to login.
 */
"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

export default function LogoutPage() {
  const router = useRouter();

  useEffect(() => {
    const performLogout = async () => {
      try {
        await fetch("/api/auth/logout", {
          method: "POST",
          headers: {
            "Content-Type": "application/json",
          },
        });
      } catch (error) {
        console.error("Logout request failed:", error);
      } finally {
        router.replace("/login");
      }
    };

    void performLogout();
  }, [router]);

  return (
    <div className="flex min-h-screen items-center justify-center">
      <p className="text-sm text-neutral-600">Signing you out…</p>
    </div>
  );
}
