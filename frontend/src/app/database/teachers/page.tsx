"use client";

import { DatabasePage } from "@/components/ui/database/database-page";
import { teachersConfig } from "@/lib/database/configs/teachers.config";

export default function TeachersPage() {
  return <DatabasePage config={teachersConfig} />;
}