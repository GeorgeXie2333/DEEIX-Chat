import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const frontendRoot = join(__dirname, "../../..");
const loginPageSource = readFileSync(join(__dirname, "../components/login-page.tsx"), "utf8");
const loginRoutePageSource = readFileSync(join(frontendRoot, "app/(auth)/login/page.tsx"), "utf8");

function getClassNameAfter(marker) {
  const markerIndex = loginPageSource.indexOf(marker);
  assert.notEqual(markerIndex, -1, `missing ${marker}`);

  const afterMarker = loginPageSource.slice(markerIndex);
  const classMatch = afterMarker.match(/className="([^"]+)"/);
  assert.ok(classMatch, `missing className after ${marker}`);
  return classMatch[1];
}

test("login page owns vertical scrolling inside the fixed app shell", () => {
  const mainClassName = getClassNameAfter("<main");

  assert.match(mainClassName, /\bh-svh\b/, "login main must use a fixed viewport height so its overflow can scroll");
  assert.match(mainClassName, /\boverflow-y-auto\b/, "login main must keep vertical scrolling enabled");
  assert.match(mainClassName, /\boverflow-x-hidden\b/, "login main must prevent horizontal page drift while scrolling");
});

test("login page exposes the password reset flow from the password form", () => {
  assert.match(loginPageSource, /\bpasswordResetEnabled\b/, "login page must read the password reset feature flag");
  assert.match(loginPageSource, /\brequestPasswordResetCode\b/, "login page must wire the reset-code request action");
  assert.match(loginPageSource, /\bonPasswordResetSubmit\b/, "login page must wire the reset submit action");
  assert.match(loginPageSource, /\bupdateResetEmail\b/, "login page must be able to prefill the reset email");
  assert.match(loginPageSource, /t\("forgotPassword"\)/, "password form must show a forgot-password entry point");
  assert.match(loginPageSource, /setMode\("reset-password"\)/, "forgot-password entry point must open reset-password mode");
  assert.match(loginPageSource, /mode === "reset-password" && passwordResetEnabled/, "reset form must only render in reset-password mode when enabled");
  assert.match(loginPageSource, /htmlFor="reset-email"/, "reset form must collect the account email");
  assert.match(loginPageSource, /htmlFor="reset-password"/, "reset form must collect a new password");
  assert.match(loginPageSource, /htmlFor="reset-code"/, "reset form must collect the verification code");
});

test("login route shows default public branding before client configuration loads", () => {
  assert.match(loginRoutePageSource, /data-public-branding-ready/);
  assert.match(loginRoutePageSource, /fallback=\{<LoginRouteFallback\s*\/>\}/);
  assert.doesNotMatch(loginRoutePageSource, /fallback=\{null\}/);
  assert.match(loginRoutePageSource, /aria-busy="true"/);
  assert.match(loginRoutePageSource, /<AppLogo/);
});

test("login page is a focused public-brand authentication card", () => {
  assert.match(loginPageSource, /<PublicBrandSurface>/);
  assert.match(loginPageSource, /<PublicPageHeader showLoginAction=\{false\}\s*\/>/);
  assert.match(loginPageSource, /max-w-\[420px\]/);
  assert.match(loginPageSource, /<LoginCardSkeleton\s*\/>/);
  assert.doesNotMatch(loginPageSource, /LoginLandingCopy|LoginProductPreview|CustomBrandAttribution/);
  assert.doesNotMatch(loginPageSource, /landing\./);
});

test("registration is exposed as tabs only during the primary authentication modes", () => {
  assert.match(loginPageSource, /role="tablist"/);
  assert.match(loginPageSource, /canShowRegisterSwitch && mode !== "reset-password" && !twoFactorChallengeToken/);
  assert.match(loginPageSource, /loginPage\.setMode\(nextMode\)/);
});

test("login mode tabs have an explicit high-contrast selected state", () => {
  assert.match(loginPageSource, /border-primary\/45/);
  assert.match(loginPageSource, /dark:border-primary\/65/);
  assert.match(loginPageSource, /dark:bg-primary\/20/);
  assert.match(loginPageSource, /after:bg-primary/);
  assert.match(loginPageSource, /border-transparent text-muted-foreground/);
});

test("all authentication entry points remain wired after the layout redesign", () => {
  assert.match(loginPageSource, /onSubmit=\{onLoginSubmit\}/);
  assert.match(loginPageSource, /onSubmit=\{onRegisterSubmit\}/);
  assert.match(loginPageSource, /<TurnstileWidget/);
  assert.match(loginPageSource, /requestRegisterCode/);
  assert.match(loginPageSource, /registerDebugCode/);
  assert.match(loginPageSource, /twoFactorChallengeToken/);
  assert.match(loginPageSource, /requestTwoFactorEmailCode/);
  assert.match(loginPageSource, /twoFactorEmailDebugCode/);
  assert.match(loginPageSource, /handleProviderLogin\(provider\.slug\)/);
  assert.match(loginPageSource, /<IdentityProviderIcon/);
  assert.match(loginPageSource, /useLoginPage\(\{ nextPath \}\)/);
});

