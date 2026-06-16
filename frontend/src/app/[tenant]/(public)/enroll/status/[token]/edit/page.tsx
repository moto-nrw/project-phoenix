import { EnrollmentEditPage } from "~/components/enrollment/enrollment-edit-page";

interface PageProps {
  readonly params: Promise<{ tenant: string; token: string }>;
}

export default function PublicEnrollmentEditRoute({ params }: PageProps) {
  return <EnrollmentEditPage params={params} />;
}
