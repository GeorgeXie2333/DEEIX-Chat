import zhAdminAnnouncements from "@/i18n/messages/zh-CN/admin-announcements.json";
import zhAdminBilling from "@/i18n/messages/zh-CN/admin-billing.json";
import zhAdminContentModeration from "@/i18n/messages/zh-CN/admin-content-moderation.json";
import zhAdminConversation from "@/i18n/messages/zh-CN/admin-conversation.json";
import zhAdminFiles from "@/i18n/messages/zh-CN/admin-files.json";
import zhAdminGroups from "@/i18n/messages/zh-CN/admin-groups.json";
import zhAdminLogin from "@/i18n/messages/zh-CN/admin-login.json";
import zhAdminLogs from "@/i18n/messages/zh-CN/admin-logs.json";
import zhAdminModels from "@/i18n/messages/zh-CN/admin-models.json";
import zhAdminOpenAPI from "@/i18n/messages/zh-CN/admin-openapi.json";
import zhAdminPrompts from "@/i18n/messages/zh-CN/admin-prompts.json";
import zhAdminStatistics from "@/i18n/messages/zh-CN/admin-statistics.json";
import zhAdminTools from "@/i18n/messages/zh-CN/admin-tools.json";
import zhAdminUpstreams from "@/i18n/messages/zh-CN/admin-upstreams.json";
import zhAdminUsers from "@/i18n/messages/zh-CN/admin-users.json";
import zhAnnouncements from "@/i18n/messages/zh-CN/announcements.json";
import zhChat from "@/i18n/messages/zh-CN/chat.json";
import zhCommon from "@/i18n/messages/zh-CN/common.json";
import zhConversation from "@/i18n/messages/zh-CN/conversation.json";
import zhErrors from "@/i18n/messages/zh-CN/errors.json";
import zhFiles from "@/i18n/messages/zh-CN/files.json";
import zhGuide from "@/i18n/messages/zh-CN/guide.json";
import zhKnowledgeBases from "@/i18n/messages/zh-CN/knowledge-bases.json";
import zhLanding from "@/i18n/messages/zh-CN/landing.json";
import zhLogin from "@/i18n/messages/zh-CN/login.json";
import zhPrompts from "@/i18n/messages/zh-CN/prompts.json";
import zhRecent from "@/i18n/messages/zh-CN/recent.json";
import zhSettings from "@/i18n/messages/zh-CN/settings.json";
import zhShare from "@/i18n/messages/zh-CN/share.json";
import type { AppLocale } from "@/i18n/config";
import { replaceDefaultBrandTitle } from "@/shared/config/branding";

const BASE_MESSAGES = {
  common: zhCommon,
  conversation: zhConversation,
  errors: zhErrors,
  login: zhLogin,
  landing: zhLanding,
  prompts: zhPrompts,
  guide: zhGuide,
  chat: zhChat,
  announcements: zhAnnouncements,
  recent: zhRecent,
  share: zhShare,
  files: zhFiles,
  knowledgeBases: zhKnowledgeBases,
  settings: zhSettings,
  adminAnnouncements: zhAdminAnnouncements,
  adminBilling: zhAdminBilling,
  adminConversation: zhAdminConversation,
  adminFiles: zhAdminFiles,
  adminGroups: zhAdminGroups,
  adminLogin: zhAdminLogin,
  adminLogs: zhAdminLogs,
  adminModels: zhAdminModels,
  adminOpenAPI: zhAdminOpenAPI,
  adminPrompts: zhAdminPrompts,
  adminStatistics: zhAdminStatistics,
  adminTools: zhAdminTools,
  adminUpstreams: zhAdminUpstreams,
  adminUsers: zhAdminUsers,
  adminContentModeration: zhAdminContentModeration,
};

export type AppMessages = typeof BASE_MESSAGES;

export function applyBrandingToMessages(messages: AppMessages, brandTitle: string): AppMessages {
  return {
    ...messages,
    guide: {
      ...messages.guide,
      userWelcomeTitle: replaceDefaultBrandTitle(messages.guide.userWelcomeTitle, brandTitle),
    },
    recent: {
      ...messages.recent,
      allConversationsDescription: replaceDefaultBrandTitle(messages.recent.allConversationsDescription, brandTitle),
    },
    login: {
      ...messages.login,
      title: replaceDefaultBrandTitle(messages.login.title, brandTitle),
    },
    share: {
      ...messages.share,
      signInToContinue: replaceDefaultBrandTitle(messages.share.signInToContinue, brandTitle),
    },
    chat: {
      ...messages.chat,
      placeholder: replaceDefaultBrandTitle(messages.chat.placeholder, brandTitle),
    },
    settings: {
      ...messages.settings,
      accountPage: {
        ...messages.settings.accountPage,
        securityDialog: {
          ...messages.settings.accountPage.securityDialog,
          email: {
            ...messages.settings.accountPage.securityDialog.email,
            description: {
              ...messages.settings.accountPage.securityDialog.email.description,
              change: replaceDefaultBrandTitle(
                messages.settings.accountPage.securityDialog.email.description.change,
                brandTitle,
              ),
            },
          },
        },
      },
    },
  };
}

export const DEFAULT_MESSAGES: AppMessages = BASE_MESSAGES;

type LocaleMessageModule = { default: unknown };
type LocaleMessageImports = readonly [LocaleMessageModule, ...LocaleMessageModule[]];

function mergeLocaleValue<T>(base: T, localized: unknown): T {
  if (base === null || typeof base !== "object" || Array.isArray(base)) {
    return (localized === undefined ? base : localized) as T;
  }
  const localizedRecord = localized !== null && typeof localized === "object" && !Array.isArray(localized)
    ? localized as Record<string, unknown>
    : {};
  return Object.fromEntries(
    Object.entries(base as Record<string, unknown>).map(([key, value]) => [
      key,
      mergeLocaleValue(value, localizedRecord[key]),
    ]),
  ) as T;
}

function toAppMessages([
  common,
  conversation,
  errors,
  login,
  landing,
  prompts,
  guide,
  chat,
  announcements,
  recent,
  share,
  files,
  knowledgeBases,
  settings,
  adminAnnouncements,
  adminBilling,
  adminConversation,
  adminFiles,
  adminGroups,
  adminLogin,
  adminLogs,
  adminModels,
  adminOpenAPI,
  adminPrompts,
  adminStatistics,
  adminTools,
  adminUpstreams,
  adminUsers,
  adminContentModeration,
]: LocaleMessageImports): AppMessages {
  return mergeLocaleValue(BASE_MESSAGES, {
    common: common.default,
    conversation: conversation.default,
    errors: errors.default,
    login: login.default,
    landing: landing.default,
    prompts: prompts.default,
    guide: guide.default,
    chat: chat.default,
    announcements: announcements.default,
    recent: recent.default,
    share: share.default,
    files: files.default,
    knowledgeBases: knowledgeBases.default,
    settings: settings.default,
    adminAnnouncements: adminAnnouncements.default,
    adminBilling: adminBilling.default,
    adminConversation: adminConversation.default,
    adminFiles: adminFiles.default,
    adminGroups: adminGroups.default,
    adminLogin: adminLogin.default,
    adminLogs: adminLogs.default,
    adminModels: adminModels.default,
    adminOpenAPI: adminOpenAPI.default,
    adminPrompts: adminPrompts.default,
    adminStatistics: adminStatistics.default,
    adminTools: adminTools.default,
    adminUpstreams: adminUpstreams.default,
    adminUsers: adminUsers.default,
    adminContentModeration: adminContentModeration.default,
  });
}

export async function loadLocaleMessages(locale: AppLocale): Promise<AppMessages> {
  if (locale === "zh-CN") {
    return DEFAULT_MESSAGES;
  }

  if (locale === "ja-JP") {
    return toAppMessages(await Promise.all([
      import("@/i18n/messages/ja-JP/common.json"),
      import("@/i18n/messages/ja-JP/conversation.json"),
      import("@/i18n/messages/ja-JP/errors.json"),
      import("@/i18n/messages/ja-JP/login.json"),
      import("@/i18n/messages/ja-JP/landing.json"),
      import("@/i18n/messages/ja-JP/prompts.json"),
      import("@/i18n/messages/ja-JP/guide.json"),
      import("@/i18n/messages/ja-JP/chat.json"),
      import("@/i18n/messages/ja-JP/announcements.json"),
      import("@/i18n/messages/ja-JP/recent.json"),
      import("@/i18n/messages/ja-JP/share.json"),
      import("@/i18n/messages/ja-JP/files.json"),
      import("@/i18n/messages/en-US/knowledge-bases.json"),
      import("@/i18n/messages/ja-JP/settings.json"),
      import("@/i18n/messages/ja-JP/admin-announcements.json"),
      import("@/i18n/messages/ja-JP/admin-billing.json"),
      import("@/i18n/messages/ja-JP/admin-conversation.json"),
      import("@/i18n/messages/ja-JP/admin-files.json"),
      import("@/i18n/messages/ja-JP/admin-groups.json"),
      import("@/i18n/messages/ja-JP/admin-login.json"),
      import("@/i18n/messages/ja-JP/admin-logs.json"),
      import("@/i18n/messages/ja-JP/admin-models.json"),
      import("@/i18n/messages/ja-JP/admin-openapi.json"),
      import("@/i18n/messages/ja-JP/admin-prompts.json"),
      import("@/i18n/messages/ja-JP/admin-statistics.json"),
      import("@/i18n/messages/ja-JP/admin-tools.json"),
      import("@/i18n/messages/ja-JP/admin-upstreams.json"),
      import("@/i18n/messages/ja-JP/admin-users.json"),
      import("@/i18n/messages/en-US/admin-content-moderation.json"),
    ]));
  }

  return toAppMessages(await Promise.all([
    import("@/i18n/messages/en-US/common.json"),
    import("@/i18n/messages/en-US/conversation.json"),
    import("@/i18n/messages/en-US/errors.json"),
    import("@/i18n/messages/en-US/login.json"),
    import("@/i18n/messages/en-US/landing.json"),
    import("@/i18n/messages/en-US/prompts.json"),
    import("@/i18n/messages/en-US/guide.json"),
    import("@/i18n/messages/en-US/chat.json"),
    import("@/i18n/messages/en-US/announcements.json"),
    import("@/i18n/messages/en-US/recent.json"),
    import("@/i18n/messages/en-US/share.json"),
    import("@/i18n/messages/en-US/files.json"),
    import("@/i18n/messages/en-US/knowledge-bases.json"),
    import("@/i18n/messages/en-US/settings.json"),
    import("@/i18n/messages/en-US/admin-announcements.json"),
    import("@/i18n/messages/en-US/admin-billing.json"),
    import("@/i18n/messages/en-US/admin-conversation.json"),
    import("@/i18n/messages/en-US/admin-files.json"),
    import("@/i18n/messages/en-US/admin-groups.json"),
    import("@/i18n/messages/en-US/admin-login.json"),
    import("@/i18n/messages/en-US/admin-logs.json"),
    import("@/i18n/messages/en-US/admin-models.json"),
    import("@/i18n/messages/en-US/admin-openapi.json"),
    import("@/i18n/messages/en-US/admin-prompts.json"),
    import("@/i18n/messages/en-US/admin-statistics.json"),
    import("@/i18n/messages/en-US/admin-tools.json"),
    import("@/i18n/messages/en-US/admin-upstreams.json"),
    import("@/i18n/messages/en-US/admin-users.json"),
    import("@/i18n/messages/en-US/admin-content-moderation.json"),
  ]));
}
