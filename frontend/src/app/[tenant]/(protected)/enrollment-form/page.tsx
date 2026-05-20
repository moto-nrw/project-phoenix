import { EnrollmentFormEditor } from "~/components/enrollment/enrollment-form-editor";
import { PageHeaderWithSearch } from "~/components/ui/page-header";

export default function EnrollmentFormPage() {
  return (
    <div className="-mt-1.5 w-full">
      <PageHeaderWithSearch title="Anmeldeformulare" />
      <EnrollmentFormEditor />
    </div>
  );
}
