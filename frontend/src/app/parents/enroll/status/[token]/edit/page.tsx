import { EnrollmentEditPage } from "~/components/enrollment/enrollment-edit-page";

interface PageProps {
  readonly params: Promise<{ token: string }>;
}

export default function ParentEnrollmentEditRoute({ params }: PageProps) {
  return <EnrollmentEditPage params={params} />;
}
