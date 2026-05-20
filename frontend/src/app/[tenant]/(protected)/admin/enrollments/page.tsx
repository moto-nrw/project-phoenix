import { AdminEnrollmentsList } from "~/components/enrollment/admin-enrollments-list";
import { PageHeaderWithSearch } from "~/components/ui/page-header";

export default function AdminEnrollmentsPage() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Anmeldungen" />
      <AdminEnrollmentsList />
    </div>
  );
}
