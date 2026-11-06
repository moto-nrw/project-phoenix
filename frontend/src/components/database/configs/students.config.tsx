// Student entity configuration. The Kinder database page creates children
// through StudentCreateModal and edits them on the child page, so the config
// carries only the API path, the names and the list title the page reads.

import { defineEntityConfig } from "@/lib/database/types";
import type { Student } from "~/lib/student-helpers";
import { childrenConcept } from "~/lib/concepts/children";

export const studentsConfig = defineEntityConfig<Student>({
  name: {
    singular: "Kinder",
    plural: "Kinder",
  },

  concept: childrenConcept,

  backUrl: "/database",

  api: {
    basePath: "/api/students",
  },

  form: { sections: [] },

  detail: { sections: [] },

  list: {
    title: "Kinder auswählen",
    description: "Verwalte Kinderdaten und Gruppenzuweisungen",
    searchPlaceholder: "Kinder suchen…",
    item: {
      title: (student: Student) =>
        `${student.first_name} ${student.second_name}`,
    },
  },

  labels: {
    createButton: "Neues Kind erstellen",
    detailModalTitle: "Kinddetails",
    deleteConfirmation:
      "Sind Sie sicher, dass Sie dieses Kind löschen möchten?",
  },
});
