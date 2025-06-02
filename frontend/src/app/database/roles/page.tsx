'use client';

import { DatabasePage } from '@/components/ui/database';
import { rolesConfig } from '@/lib/database/configs/roles.config';

export default function RolesPage() {
  return <DatabasePage config={rolesConfig} />;
}