#!/usr/bin/env node
/**
 * Helper that drives chat.qwen.ai via Playwright to extract the auth token
 * from localStorage after a successful sign-in.
 *
 * Usage:
 *   npm install playwright
 *   npx playwright install chromium
 *   node login.mjs --email you@example.com --password 'your-pass'
 *
 * The token is printed to stdout on success. Save it to QWEN2API_TOKENS or
 * to config.json.
 */

import { chromium } from "playwright";

function parseArgs(argv) {
  const out = {};
  for (let i = 0; i < argv.length; i += 2) {
    if (argv[i].startsWith("--")) {
      out[argv[i].slice(2)] = argv[i + 1];
    }
  }
  return out;
}

const args = parseArgs(process.argv.slice(2));
const email = args.email;
const password = args.password;
const headless = args.headless !== "false";
const timeoutMs = parseInt(args.timeout ?? "60000", 10);

if (!email || !password) {
  console.error("Usage: node login.mjs --email <e> --password <p> [--headless false] [--timeout 60000]");
  process.exit(2);
}

const browser = await chromium.launch({ headless });
try {
  const ctx = await browser.newContext({
    userAgent:
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36",
  });
  const page = await ctx.newPage();
  await page.goto("https://chat.qwen.ai/auth?action=signin", { waitUntil: "domcontentloaded", timeout: timeoutMs });

  // Best-effort field selectors. Site DOM may change; adjust as needed.
  const emailField = page.locator('input[type="email"], input[name="email"], input[placeholder*="mail" i]').first();
  await emailField.waitFor({ timeout: timeoutMs });
  await emailField.fill(email);

  const passwordField = page.locator('input[type="password"]').first();
  await passwordField.fill(password);

  await Promise.all([
    page.waitForURL(/chat\.qwen\.ai\/(?!auth)/, { timeout: timeoutMs }).catch(() => null),
    page.locator('button[type="submit"], button:has-text("Sign in"), button:has-text("登录")').first().click(),
  ]);

  // Give the SPA a moment to set localStorage after auth completes.
  await page.waitForTimeout(2000);
  const token = await page.evaluate(() => window.localStorage.getItem("token"));
  if (!token) {
    console.error("login flow finished but no token was set in localStorage");
    process.exit(1);
  }
  process.stdout.write(token + "\n");
} catch (err) {
  console.error("login failed:", err.message || err);
  process.exit(1);
} finally {
  await browser.close();
}
