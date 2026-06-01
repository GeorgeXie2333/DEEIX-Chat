import { AdminShell } from "@/features/admin/components/admin-shell";
import { AdminOpenAPISettingsPage } from "@/features/admin/components/sections/settings/admin-settings-openapi";

export default function AdminOpenAPISettingsRoute() {
  return (
    <AdminShell activeSection="open-api" basePath="/admin">
      <AdminOpenAPISettingsPage />
    </AdminShell>
  );
}
