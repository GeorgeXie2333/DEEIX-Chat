import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  APP_LOGO_ASPECT_RATIO,
  APP_LOGO_MASK_SRC,
  appLogoMaskImage,
  appLogoVisualHeight,
  appLogoVisualWidth,
  customAppLogoSource,
} from "./app-logo-assets.ts";

test("app logo uses the SVG as a mask source", () => {
  assert.equal(APP_LOGO_MASK_SRC, "/logo.svg");
  assert.equal(appLogoMaskImage(), 'url("/logo.svg")');
});

test("app logo aspect ratio matches the supplied SVG viewBox", () => {
  assert.equal(APP_LOGO_ASPECT_RATIO, 219.07144 / 43.166336);
});

test("the built-in logo is colorized instead of painted over the current text color", () => {
  assert.equal(customAppLogoSource(""), undefined);
  assert.equal(customAppLogoSource(" /logo.svg "), undefined);
  assert.equal(customAppLogoSource("https://cdn.example.com/logo.svg"), "https://cdn.example.com/logo.svg");
});

test("app logo visual size follows the previous text-logo scale", () => {
  assert.equal(appLogoVisualHeight(24), 13);
  assert.equal(appLogoVisualHeight(48), 20);
  assert.equal(appLogoVisualHeight(72), 30);
  assert.equal(appLogoVisualWidth(48), Math.round(20 * APP_LOGO_ASPECT_RATIO));
});

test("app logo rendering follows the current text color", () => {
  const source = readFileSync(new URL("./app-logo.tsx", import.meta.url), "utf8");

  assert.match(source, /bg-current/);
  assert.match(
    source,
    /maskImage: customLogoSource \? undefined : appLogoMaskImage\(\)/u,
  );
  assert.doesNotMatch(source, /text-foreground/u);
  assert.doesNotMatch(source, /APP_LOGO_LIGHT_SRC|APP_LOGO_DARK_SRC|<Image/u);
});
