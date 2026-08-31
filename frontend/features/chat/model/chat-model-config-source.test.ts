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
  assert.doesNotMatch(configSource, /DialogTitle[\s\S]*tComposer\("modelOptions"\)/);
});

test("ChatModelConfig hides temperature from rendered advanced settings", () => {
  assert.match(configSource, /settings\.filter\(\(setting\) => setting\.kind !== "temperature"\)/);
  assert.match(configSource, /if \(visibleSettings\.length === 0 && customSettings\.length === 0\)/);
  assert.match(configSource, /visibleSettings\.map\(\(setting\) =>/);
});

test("ChatModelConfig popover matches the compact tools menu surface", () => {
  assert.match(
    configSource,
    /className="w-\[min\(calc\(100vw-2rem\),13rem\)\] overflow-hidden rounded-xl border-\[0\.5px\] border-border p-1\.5 shadow-xs"/,
  );
});

test("ChatModelConfig keeps select controls compact enough for labels", () => {
  assert.match(configSource, /grid-cols-\[minmax\(0,1fr\)_6\.5rem\]/);
});

test("ChatModelConfig renders server controls and restores backend defaults", () => {
  assert.match(configSource, /resolveServerOptionControls/);
  assert.match(configSource, /customSettings\.map/);
  assert.match(configSource, /valueType === "boolean"/);
  assert.match(configSource, /valueType === "select"/);
  assert.match(configSource, /onDefaultOptionsRestore/);
  assert.match(configSource, /lockedOptionPaths/);
});
