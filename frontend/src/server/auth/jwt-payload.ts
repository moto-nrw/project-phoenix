export interface JwtPayload {
  id: string | number;
  exp?: number;
  token?: string;
  sub?: string;
  username?: string;
  first_name?: string;
  last_name?: string;
  email?: string;
  roles?: string[];
  permissions?: string[];
  is_admin?: boolean;
  tenant_id?: number;
  org_id?: number;
  scope?: string;
  read_only?: boolean;
  acting_admin_id?: number;
}
