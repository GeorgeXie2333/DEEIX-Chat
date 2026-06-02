import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const configSource = readFileSync(
  resolve(__dirname, "../components/sections/chat-model-config.tsx"),
  "utf8",
);

test("ChatModelConfig keeps the local advanced settings popover", () => {
  assert.match(configSource, /resolveAdvancedSettings/);
  assert.match(configSource, /resetAdvancedSettings/);
  assert.match(configSource, /aria-label=\{tComposer\("advancedSettings"\)\}/);
  assert.doesNotMatch(configSource, /DialogTitle[^]*tComposer\("modelOptions"\)/);
});
