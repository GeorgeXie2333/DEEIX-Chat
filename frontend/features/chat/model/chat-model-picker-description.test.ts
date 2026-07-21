import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const pickerSource = readFileSync(
  resolve(__dirname, "../components/sections/chat-model-picker.tsx"),
  "utf8",
);
const optionsSource = readFileSync(
  resolve(__dirname, "../hooks/use-chat-model-options.ts"),
  "utf8",
);

test("chat model options preserve admin model descriptions", () => {
  assert.match(optionsSource, /description:\s*item\.description/);
});

test("model picker renders description info before pricing without replacing selected check", () => {
  assert.match(pickerSource, /import \{[^}]*Info[^}]*\} from "lucide-react"/);
  assert.match(pickerSource, /selected \? <Check/);
  const selectedCheckIndex = pickerSource.indexOf("selected ? <Check");
  const descriptionTriggerIndex = pickerSource.indexOf("ariaLabel={viewDescriptionLabel}");
  const pricingTriggerIndex = pickerSource.indexOf("ariaLabel={viewPricingLabel}");
  assert.ok(descriptionTriggerIndex > selectedCheckIndex);
  assert.ok(descriptionTriggerIndex < pricingTriggerIndex);
});

test("model picker keeps desktop tooltips and adds mobile long press popovers", () => {
  assert.match(pickerSource, /<Tooltip>/);
  assert.match(pickerSource, /isMobile \?/);
  assert.match(pickerSource, /data-model-menu-auxiliary-popover="true"/);
  assert.match(pickerSource, /MODEL_MENU_AUXILIARY_LONG_PRESS_MS/);
  assert.match(pickerSource, /shouldCancelModelMenuAuxiliaryLongPress/);
});

test("model picker popover stops clicks before composer focus handling", () => {
  assert.match(pickerSource, /onClick=\{stopModelMenuClickPropagation\}/);

  const auxiliaryPopoverIndex = pickerSource.indexOf('data-model-menu-auxiliary-popover="true"');
  const auxiliaryPopoverTagEnd = pickerSource.indexOf(">", auxiliaryPopoverIndex);
  assert.ok(auxiliaryPopoverIndex >= 0);
  assert.ok(auxiliaryPopoverTagEnd > auxiliaryPopoverIndex);
  assert.doesNotMatch(
    pickerSource.slice(auxiliaryPopoverIndex, auxiliaryPopoverTagEnd),
    /onClick=/,
  );
});

test("tiered pricing headings render complete closed and open token ranges", () => {
  assert.match(pickerSource, /return `\$\{formatTokenQuantity\(fromTokens\)\}～∞`;/);
  assert.match(pickerSource, /return `\$\{formatTokenQuantity\(fromTokens\)\}～\$\{formatTokenQuantity\(upToTokens\)\}`;/);
});
