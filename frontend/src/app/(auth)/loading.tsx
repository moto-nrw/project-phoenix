"use client";
// app/(auth)/loading.tsx
// Next.js will automatically show this during route transitions

import { Loading } from "~/components/ui/loading";

export default function LoadingPage() {
  return <Loading message="Laden..." />;
}
