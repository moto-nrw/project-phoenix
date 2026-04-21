import { redirect } from "next/navigation";

// The five Schulverwaltung tabs were split into top-level operator routes
// (organizations, schools, accounts, devices, persons). Keep this path as a
// redirect so bookmarks and external links continue to resolve.
export default function OperatorProvisioningRedirect() {
  redirect("/operator/organizations");
}
