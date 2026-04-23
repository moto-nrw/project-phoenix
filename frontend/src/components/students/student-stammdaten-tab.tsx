"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Save } from "lucide-react";
import { Button } from "~/components/ui/button";
import {
  BusStatusSection,
  PersonalInfoSection,
  PickupStatusSection,
} from "./student-form-fields";
import { StudentCommonFormSections } from "./student-common-form-sections";
import {
  handleStudentFormSubmit,
  validateStudentForm,
} from "~/lib/student-form-validation";
import type { Student } from "~/lib/api";

interface StudentStammdatenTabProps {
  student: Student;
  groups: Array<{ value: string; label: string }>;
  onSave: (data: Partial<Student>) => Promise<void>;
}

function buildDraft(student: Student): Partial<Student> {
  return {
    first_name: student.first_name ?? "",
    second_name: student.second_name ?? "",
    school_class: student.school_class ?? "",
    group_id: student.group_id ?? "",
    birthday: student.birthday ?? "",
    health_info: student.health_info ?? "",
    supervisor_notes: student.supervisor_notes ?? "",
    extra_info: student.extra_info ?? "",
    privacy_consent_accepted: student.privacy_consent_accepted ?? false,
    data_retention_days: student.data_retention_days ?? 30,
    bus: student.bus ?? false,
    pickup_status: student.pickup_status ?? "",
  };
}

export function StudentStammdatenTab({
  student,
  groups,
  onSave,
}: StudentStammdatenTabProps) {
  const [formData, setFormData] = useState<Partial<Student>>(() =>
    buildDraft(student),
  );
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    setFormData(buildDraft(student));
    setErrors({});
  }, [student]);

  const originalDraft = useMemo(() => buildDraft(student), [student]);
  const isDirty = useMemo(() => {
    const keys = Object.keys(originalDraft) as Array<keyof Student>;
    return keys.some((key) => originalDraft[key] !== formData[key]);
  }, [originalDraft, formData]);

  const handleChange = useCallback(
    (field: keyof Student, value: string | boolean | number | null) => {
      setFormData((prev) => ({ ...prev, [field]: value }));
      if (errors[field]) {
        setErrors((prev) => {
          const next = { ...prev };
          delete next[field];
          return next;
        });
      }
    },
    [errors],
  );

  const validateForm = useCallback(() => {
    const next = validateStudentForm(formData, {
      firstName: true,
      lastName: true,
      schoolClass: false,
    });
    setErrors(next);
    return Object.keys(next).length === 0;
  }, [formData]);

  const handleSubmit = (event: React.FormEvent) => {
    return handleStudentFormSubmit(
      event,
      formData,
      validateForm,
      onSave,
      setSaving,
      setErrors,
    );
  };

  return (
    <form onSubmit={handleSubmit} noValidate className="space-y-5">
      {errors.submit ? (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3">
          <p className="text-sm text-red-800">{errors.submit}</p>
        </div>
      ) : null}

      <PersonalInfoSection
        formData={formData}
        onChange={handleChange}
        errors={errors}
        groups={groups}
        requiredFields={{
          firstName: true,
          lastName: true,
          schoolClass: false,
        }}
      />

      <StudentCommonFormSections
        formData={formData}
        errors={errors}
        onChange={handleChange}
      />

      <PickupStatusSection
        value={formData.pickup_status}
        onChange={(value) => handleChange("pickup_status", value)}
      />

      <BusStatusSection
        value={formData.bus}
        onChange={(value) => handleChange("bus", value)}
      />

      <div className="sticky bottom-0 -mx-6 -mb-6 flex items-center justify-end gap-2 border-t border-gray-100 bg-white/95 px-6 py-3 backdrop-blur-sm">
        <Button type="submit" variant="primary" disabled={saving || !isDirty}>
          {saving ? (
            <>
              <Loader2 className="mr-1.5 h-4 w-4 animate-spin" aria-hidden />
              Speichern...
            </>
          ) : (
            <>
              <Save className="mr-1.5 h-4 w-4" aria-hidden />
              Speichern
            </>
          )}
        </Button>
      </div>
    </form>
  );
}
