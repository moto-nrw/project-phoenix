"use client";

import { useEffect } from "react";
import { useSession } from "next-auth/react";
import { useRouter } from "next/navigation";
import { UserContextProvider } from "~/lib/usercontext-context";
import { isAdmin } from "~/lib/auth-utils";
import { Loading } from "~/components/ui/loading";
import { DashboardContent } from "~/components/dashboard/dashboard-content";

export default function DashboardPage() {
  const router = useRouter();
  const { data: session, status } = useSession();

  useEffect(() => {
    if (status !== "loading" && !isAdmin(session)) {
      router.replace("/ogs-groups");
    }
  }, [status, session, router]);

  if (status === "loading") {
    return <Loading fullPage={false} />;
  }

  if (!isAdmin(session)) {
    return null;
  }

  return (
    <UserContextProvider>
      <DashboardContent />
    </UserContextProvider>
  );
}
