import { AppSettingsPanel } from "@/features/settings/components/app-settings-panel";
import { SettingsOpenAPI } from "@/features/settings/components/sections/settings-openapi";

export default function SettingsOpenAPIPage() {
  return (
    <AppSettingsPanel activeSection="openapi" basePath="/setting">
      <SettingsOpenAPI />
    </AppSettingsPanel>
  );
}
