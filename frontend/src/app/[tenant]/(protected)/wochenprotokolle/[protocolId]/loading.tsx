import { TenantPage } from "~/components/ui/tenant-page";

/** Route-level loading UI of one weekly protocol. */
export default function WeekProtocolLoading() {
  return (
    <TenantPage
      title="Wochenprotokoll"
      back
      backHref="/wochenprotokolle"
      backLabel="Zurück zu den Wochenprotokollen"
      loading
    />
  );
}
