"use client";

import { CareOfferingsEditor } from "~/components/enrollment/care-offerings-editor";
import { Loading } from "~/components/ui/loading";
import { useRequireAdmin } from "~/lib/hooks/use-require-admin";

export default function CareOfferingsPage() {
  const { isReady } = useRequireAdmin();
  if (!isReady) return <Loading fullPage={false} />;

  return (
    <div className="-mt-1.5 w-full">
      <CareOfferingsEditor />
    </div>
  );
}
