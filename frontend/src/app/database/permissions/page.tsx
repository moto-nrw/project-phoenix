"use client";

import { DatabasePage } from "@/components/ui/database";
import { permissionsConfig } from "@/lib/database/configs/permissions.config";

export default function PermissionsPage() {
  return <DatabasePage config={permissionsConfig} />;
}