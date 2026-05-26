import zhAdminBilling from "@/i18n/messages/zh-CN/admin-billing.json";
import zhAdminChannels from "@/i18n/messages/zh-CN/admin-channels.json";
import zhAdminConversation from "@/i18n/messages/zh-CN/admin-conversation.json";
import zhAdminFiles from "@/i18n/messages/zh-CN/admin-files.json";
import zhAdminLogin from "@/i18n/messages/zh-CN/admin-login.json";
import zhAdminLogs from "@/i18n/messages/zh-CN/admin-logs.json";
import zhAdminModels from "@/i18n/messages/zh-CN/admin-models.json";
import zhAdminTools from "@/i18n/messages/zh-CN/admin-tools.json";
import zhAdminUsers from "@/i18n/messages/zh-CN/admin-users.json";
import zhChat from "@/i18n/messages/zh-CN/chat.json";
import zhCommon from "@/i18n/messages/zh-CN/common.json";
import zhErrors from "@/i18n/messages/zh-CN/errors.json";
import zhFiles from "@/i18n/messages/zh-CN/files.json";
import zhGuide from "@/i18n/messages/zh-CN/guide.json";
import zhLogin from "@/i18n/messages/zh-CN/login.json";
import zhRecent from "@/i18n/messages/zh-CN/recent.json";
import zhSettings from "@/i18n/messages/zh-CN/settings.json";
import zhShare from "@/i18n/messages/zh-CN/share.json";
import type { AppLocale } from "@/i18n/config";

export type AppMessages = typeof DEFAULT_MESSAGES;

export const DEFAULT_MESSAGES = {
  common: zhCommon,
  errors: zhErrors,
  login: zhLogin,
  guide: zhGuide,
  chat: zhChat,
  recent: zhRecent,
  share: zhShare,
  files: zhFiles,
  settings: zhSettings,
  adminUsers: zhAdminUsers,
  adminChannels: zhAdminChannels,
  adminConversation: zhAdminConversation,
  adminFiles: zhAdminFiles,
  adminLogin: zhAdminLogin,
  adminModels: zhAdminModels,
  adminBilling: zhAdminBilling,
  adminLogs: zhAdminLogs,
  adminTools: zhAdminTools,
};

type LocaleMessageImports = [
  { default: AppMessages["common"] },
  { default: AppMessages["errors"] },
  { default: AppMessages["login"] },
  { default: AppMessages["guide"] },
  { default: AppMessages["chat"] },
  { default: AppMessages["recent"] },
  { default: AppMessages["share"] },
  { default: AppMessages["files"] },
  { default: AppMessages["settings"] },
  { default: AppMessages["adminUsers"] },
  { default: AppMessages["adminChannels"] },
  { default: AppMessages["adminConversation"] },
  { default: AppMessages["adminFiles"] },
  { default: AppMessages["adminLogin"] },
  { default: AppMessages["adminModels"] },
  { default: AppMessages["adminBilling"] },
  { default: AppMessages["adminLogs"] },
  { default: AppMessages["adminTools"] },
];

function toAppMessages([
  common,
  errors,
  login,
  guide,
  chat,
  recent,
  share,
  files,
  settings,
  adminUsers,
  adminChannels,
  adminConversation,
  adminFiles,
  adminLogin,
  adminModels,
  adminBilling,
  adminLogs,
  adminTools,
]: LocaleMessageImports): AppMessages {
  return {
    common: common.default,
    errors: errors.default,
    login: login.default,
    guide: guide.default,
    chat: chat.default,
    recent: recent.default,
    share: share.default,
    files: files.default,
    settings: settings.default,
    adminUsers: adminUsers.default,
    adminChannels: adminChannels.default,
    adminConversation: adminConversation.default,
    adminFiles: adminFiles.default,
    adminLogin: adminLogin.default,
    adminModels: adminModels.default,
    adminBilling: adminBilling.default,
    adminLogs: adminLogs.default,
    adminTools: adminTools.default,
  };
}

export async function loadLocaleMessages(locale: AppLocale): Promise<AppMessages> {
  if (locale === "zh-CN") {
    return DEFAULT_MESSAGES;
  }

  if (locale === "ja-JP") {
    return toAppMessages(await Promise.all([
      import("@/i18n/messages/ja-JP/common.json"),
      import("@/i18n/messages/ja-JP/errors.json"),
      import("@/i18n/messages/ja-JP/login.json"),
      import("@/i18n/messages/ja-JP/guide.json"),
      import("@/i18n/messages/ja-JP/chat.json"),
      import("@/i18n/messages/ja-JP/recent.json"),
      import("@/i18n/messages/ja-JP/share.json"),
      import("@/i18n/messages/ja-JP/files.json"),
      import("@/i18n/messages/ja-JP/settings.json"),
      import("@/i18n/messages/ja-JP/admin-users.json"),
      import("@/i18n/messages/ja-JP/admin-channels.json"),
      import("@/i18n/messages/ja-JP/admin-conversation.json"),
      import("@/i18n/messages/ja-JP/admin-files.json"),
      import("@/i18n/messages/ja-JP/admin-login.json"),
      import("@/i18n/messages/ja-JP/admin-models.json"),
      import("@/i18n/messages/ja-JP/admin-billing.json"),
      import("@/i18n/messages/ja-JP/admin-logs.json"),
      import("@/i18n/messages/ja-JP/admin-tools.json"),
    ]));
  }

  return toAppMessages(await Promise.all([
    import("@/i18n/messages/en-US/common.json"),
    import("@/i18n/messages/en-US/errors.json"),
    import("@/i18n/messages/en-US/login.json"),
    import("@/i18n/messages/en-US/guide.json"),
    import("@/i18n/messages/en-US/chat.json"),
    import("@/i18n/messages/en-US/recent.json"),
    import("@/i18n/messages/en-US/share.json"),
    import("@/i18n/messages/en-US/files.json"),
    import("@/i18n/messages/en-US/settings.json"),
    import("@/i18n/messages/en-US/admin-users.json"),
    import("@/i18n/messages/en-US/admin-channels.json"),
    import("@/i18n/messages/en-US/admin-conversation.json"),
    import("@/i18n/messages/en-US/admin-files.json"),
    import("@/i18n/messages/en-US/admin-login.json"),
    import("@/i18n/messages/en-US/admin-models.json"),
    import("@/i18n/messages/en-US/admin-billing.json"),
    import("@/i18n/messages/en-US/admin-logs.json"),
    import("@/i18n/messages/en-US/admin-tools.json"),
  ]));
}
