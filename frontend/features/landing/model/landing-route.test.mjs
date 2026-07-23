import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = join(__dirname, "../../..");
const appPageSource = readFileSync(join(frontendRoot, "app/page.tsx"), "utf8");
const routeSource = readFileSync(join(__dirname, "../components/landing-route.tsx"), "utf8");
const landingSource = readFileSync(join(__dirname, "../components/landing-page.tsx"), "utf8");
const globalCSSSource = readFileSync(join(frontendRoot, "app/globals.css"), "utf8");
const publicSurfaceSource = readFileSync(
  join(frontendRoot, "shared/components/public-brand-surface.tsx"),
  "utf8",
);
const publicHeaderSource = readFileSync(
  join(frontendRoot, "shared/components/public-page-header.tsx"),
  "utf8",
);

test("root route mounts the landing gate instead of redirecting on the server", () => {
  assert.match(appPageSource, /<LandingRoute\s*\/>/);
  assert.doesNotMatch(appPageSource, /redirect\(/);
  assert.match(routeSource, /resolveAccessToken\(\)/);
  assert.match(routeSource, /LANDING_GATE_TIMEOUT_MS\s*=\s*800/);
  assert.match(routeSource, /router\.replace\("\/chat"\)/);
});

test("root route is visible while public branding refreshes in the background", () => {
  assert.match(appPageSource, /data-public-branding-ready/);
  assert.match(
    globalCSSSource,
    /html\[data-branding-pending\]\s+\[data-public-branding-ready\]\s*\{\s*visibility:\s*visible/,
  );
});

test("landing owns scrolling inside the fixed application shell", () => {
  assert.match(landingSource, /h-svh overflow-x-hidden overflow-y-auto/);
});

test("public theme boundary preserves the saved application preset", () => {
  assert.match(publicSurfaceSource, /dataset\.publicSurface/);
  assert.doesNotMatch(publicSurfaceSource, /setPreset|theme-preset|dataset\.theme/);
  assert.match(publicHeaderSource, /setTheme\(value as Theme\)/);
  assert.doesNotMatch(publicHeaderSource, /setPreset/);
});

test("interactive demo respects autoplay, user intent, viewport, and reduced motion", () => {
  assert.match(landingSource, /4_500/);
  assert.match(landingSource, /setAutoPlay\(false\)/);
  assert.match(landingSource, /IntersectionObserver/);
  assert.match(landingSource, /reduceMotion \|\| !autoPlay \|\| !isInView/);
  assert.doesNotMatch(landingSource, /15ch/);
});

test("root metadata references a static 1200 by 630 social image", () => {
  assert.match(appPageSource, /\/og\/comi-ai-landing\.png/);
  assert.match(appPageSource, /width:\s*1200/);
  assert.match(appPageSource, /height:\s*630/);
});
