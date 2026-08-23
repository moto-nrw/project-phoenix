// Accept-Flow für Lehrkraft-Einladungen im Schul-Portal (#2207): gleiche
// geteilte Einladungs-Strecke wie /invite, aber nach dem Annehmen geht es
// zum Schul-Login auf diesem Host statt zur Tenant-Subdomain.
import { InvitationPageRoute } from "~/components/auth/invitation-page-route";

export default function SchoolInvitePage() {
  return <InvitationPageRoute redirectToPath="/login" />;
}
