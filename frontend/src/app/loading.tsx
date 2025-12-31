"use client";
// app/loading.tsx
// Root-level loading - catches ALL route transitions

import { Loading } from "~/components/ui/loading";

export default function RootLoadingPage() {
  return <Loading message="Laden..." />;
}
