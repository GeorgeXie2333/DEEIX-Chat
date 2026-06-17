import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import test from "node:test";

const __dirname = dirname(fileURLToPath(import.meta.url));
const loginPageSource = readFileSync(join(__dirname, "../components/login-page.tsx"), "utf8");

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

