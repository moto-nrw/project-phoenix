import { EnrollmentEditPage } from "~/components/enrollment/enrollment-edit-page";

interface PageProps {
  readonly params: Promise<{ tenant: string; token: string }>;
}

// Reduced Halbjahreswechsel flow (#2251): offerings/weekdays only.
export default function PublicEnrollmentAdjustRoute({ params }: PageProps) {
  return <EnrollmentEditPage params={params} adjustOnly />;
}
