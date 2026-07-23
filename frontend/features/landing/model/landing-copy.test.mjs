import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const messagesRoot = join(__dirname, "../../../i18n/messages");

const requiredStringPaths = [
  "nav.ariaLabel",
  "nav.home",
  "nav.language",
  "nav.login",
  "theme.label",
  "theme.light",
  "theme.dark",
  "theme.system",
  "hero.eyebrow",
  "hero.headline",
  "hero.headlineLine1",
  "hero.headlineLine2",
  "hero.description",
  "hero.cta",
  "hero.providerCount",
  "hero.featuredProviders",
  "demo.label",
  "demo.round",
  "demo.controlsLabel",
  "demo.userInitial",
  "demo.userLabel",
  "demo.userPrompt",
  "demo.steps.gpt.role",
  "demo.steps.gpt.title",
  "demo.steps.gpt.body",
  "demo.steps.claude.role",
  "demo.steps.claude.title",
  "demo.steps.claude.body",
  "demo.steps.gemini.role",
  "demo.steps.gemini.title",
  "demo.steps.gemini.body",
  "demo.steps.grok.role",
  "demo.steps.grok.title",
  "demo.steps.grok.body",
  "providers.eyebrow",
  "providers.title",
  "providers.description",
  "providers.ongoing",
  "capabilities.eyebrow",
  "capabilities.title",
  "capabilities.switching.title",
  "capabilities.switching.description",
  "capabilities.connection.title",
  "capabilities.connection.description",
  "capabilities.pricing.title",
  "capabilities.pricing.description",
  "finalCta.title",
  "finalCta.description",
  "finalCta.button",
  "footer.copyright",
  "footer.docs",
];

function readMessages(locale, name) {
  return JSON.parse(readFileSync(join(messagesRoot, locale, `${name}.json`), "utf8"));
}

function valueAtPath(value, path) {
  return path.split(".").reduce((current, segment) => current?.[segment], value);
}

for (const locale of ["zh-CN", "en-US", "ja-JP"]) {
  test(`landing copy is complete for ${locale}`, () => {
    const messages = readMessages(locale, "landing");

    for (const path of requiredStringPaths) {
      const value = valueAtPath(messages, path);
      assert.equal(typeof value, "string", `${locale} missing landing.${path}`);
      assert.ok(value.trim().length > 0, `${locale} landing.${path} must not be empty`);
    }
  });

  test(`authentication copy is separate from landing copy for ${locale}`, () => {
    const messages = readMessages(locale, "login");

    assert.equal(messages.landing, undefined, `${locale} login.json must not retain marketing copy`);
    for (const key of [
      "authTitle",
      "authDescription",
      "registerTitle",
      "registerDescription",
      "resetDescription",
      "twoFactorTitle",
      "twoFactorDescription",
    ]) {
      assert.equal(typeof messages[key], "string", `${locale} missing login.${key}`);
      assert.ok(messages[key].trim().length > 0, `${locale} login.${key} must not be empty`);
    }
  });
}

test("localized hero headlines match the approved wording", () => {
  const zh = readMessages("zh-CN", "landing");
  const en = readMessages("en-US", "landing");
  const ja = readMessages("ja-JP", "landing");

  assert.equal(zh.hero.headlineLine1, "一个入口，");
  assert.equal(zh.hero.headlineLine2, "用遍主流 AI");
  assert.equal(en.hero.headline, "Every leading AI, in one place.");
  assert.equal(ja.hero.headline, "主要なAIを、ひとつの入口で。");
});
