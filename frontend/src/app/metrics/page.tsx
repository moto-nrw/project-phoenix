"use client";

import { useSession } from "next-auth/react";
import { redirect } from "next/navigation";
import {
  PageHeader,
  SectionTitle,
  ActivityStats,
} from "~/components/dashboard";

export default function MetricsPage() {
  const { status } = useSession({
    required: true,
    onUnauthenticated() {
      redirect("/login");
    },
  });

  if (status === "loading") {
    return (
      <div className="flex min-h-screen items-center justify-center">
        <p>Loading...</p>
      </div>
    );
  }

  return (
    <div className="min-h-screen">
      {/* Header */}
      <PageHeader title="Statistiken" backUrl="/dashboard" />

      {/* Main Content */}
      <main className="container mx-auto p-4 py-8">
        {/* Stats Section */}
        <div className="mb-12">
          <SectionTitle title="Aktuelle Statistiken" />
          <div className="mx-auto grid max-w-6xl grid-cols-1 gap-6 md:grid-cols-2">
            <ActivityStats />
            {/* Other stats widgets can be added here */}
          </div>
        </div>
      </main>
    </div>
  );
}
