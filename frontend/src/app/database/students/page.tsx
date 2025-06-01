"use client";

import { DatabasePage } from "@/components/ui/database/database-page";
import { studentsConfig } from "@/lib/database/configs/students.config";

export default function StudentsPage() {
  return <DatabasePage config={studentsConfig} />;
}
