import assert from "node:assert/strict";
import { existsSync, readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = join(__dirname, "../../..");
const providerSource = readFileSync(join(__dirname, "providers.ts"), "utf8");
const logoSource = readFileSync(join(__dirname, "../components/provider-logo.tsx"), "utf8");
const landingSource = readFileSync(join(__dirname, "../components/landing-page.tsx"), "utf8");

const expectedProviders = [
  "openai",
  "anthropic",
  "google",
  "xai",
  "deepseek",
  "zhipu",
  "alibaba",
  "moonshot",
  "minimax",
  "stepfun",
  "meta",
];
const expectedDemoIcons = ["openai", "claude", "gemini", "grok"];

test("landing providers use the approved order and unique slugs", () => {
  const slugs = [...providerSource.matchAll(/slug:\s*"([^"]+)"/g)].map((match) => match[1]);

  assert.deepEqual(slugs, expectedProviders);
  assert.equal(new Set(slugs).size, slugs.length);
});

test("exactly the first six providers are featured", () => {
  const rows = [...providerSource.matchAll(/\{\s*slug:\s*"([^"]+)"[^}]*featured:\s*(true|false)[^}]*\}/g)]
    .map((match) => ({ slug: match[1], featured: match[2] === "true" }));

  assert.deepEqual(
    rows.filter((row) => row.featured).map((row) => row.slug),
    expectedProviders.slice(0, 6),
  );
});

test("all eleven local provider fallbacks exist", () => {
  for (const slug of expectedProviders) {
    assert.ok(
      existsSync(join(frontendRoot, "public/vendor/lobehub-icons", `${slug}.svg`)),
      `missing local fallback for ${slug}`,
    );
  }
});

test("hero uses all providers in a carousel and model-family icons in the demo", () => {
  assert.match(landingSource, /landing-provider-carousel-track/);
  assert.doesNotMatch(landingSource, /FEATURED_LANDING_PROVIDERS/);

  const demoBlock = landingSource.slice(
    landingSource.indexOf("const DEMO_STEPS"),
    landingSource.indexOf("const CAPABILITY_KEYS"),
  );
  const demoIcons = [...demoBlock.matchAll(/slug:\s*"([^"]+)"/g)].map((match) => match[1]);

  assert.deepEqual(demoIcons, expectedDemoIcons);
  for (const slug of expectedDemoIcons) {
    assert.ok(
      existsSync(join(frontendRoot, "public/vendor/lobehub-icons", `${slug}.svg`)),
      `missing local demo icon fallback for ${slug}`,
    );
  }
});

test("LobeHub icons are pinned, direct, and fall back only once", () => {
  assert.match(providerSource, /LOBEHUB_ICON_VERSION\s*=\s*"1\.90\.0"/);
  assert.doesNotMatch(providerSource, /latest/);
  assert.match(logoSource, /<img/);
  assert.match(logoSource, /current === "cdn" \? "local" : "generic"/);
  assert.doesNotMatch(landingSource, /__sprite\.svg/);
  assert.doesNotMatch(logoSource, /__sprite\.svg/);
});
