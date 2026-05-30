import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const messagesRoot = join(__dirname, "../../../i18n/messages");

const requiredLandingKeys = [
  "headline",
  "description",
  "authTitle",
  "authDescription",
  "registerTitle",
  "registerDescription",
  "capabilityRouting",
  "capabilityFiles",
  "capabilityUsage",
  "proofTitle",
  "proofDescription",
  "previewPanelTitle",
  "previewPanelDescription",
  "previewPanelStatus",
  "previewRouting",
  "previewRoutingDescription",
  "previewContext",
  "previewContextDescription",
  "previewGovernance",
  "previewGovernanceDescription",
  "previewMetricModelsValue",
  "previewMetricSwitchesValue",
  "previewMetricApiValue",
  "previewMetricModels",
  "previewMetricFlows",
  "previewMetricPlane",
];

function readLoginMessages(locale) {
  return JSON.parse(readFileSync(join(messagesRoot, locale, "login.json"), "utf8"));
}

for (const locale of ["zh-CN", "en-US", "ja-JP"]) {
  test(`login landing copy exists for ${locale}`, () => {
    const messages = readLoginMessages(locale);

    assert.equal(typeof messages.landing, "object");
    for (const key of requiredLandingKeys) {
      assert.equal(typeof messages.landing[key], "string", `${locale} missing landing.${key}`);
      assert.ok(messages.landing[key].trim().length > 0, `${locale} landing.${key} must not be empty`);
    }
  });
}
