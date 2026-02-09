#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

if [ -f .env ]; then
  set -a
  # shellcheck disable=SC1091
  source .env
  set +a
  echo "✅ 已读取 .env"
fi

if ! command -v node >/dev/null; then
  echo "❌ 未找到 node，请先安装" >&2
  exit 1
fi

if ! command -v npx >/dev/null; then
  echo "❌ 未找到 npx，请先安装" >&2
  exit 1
fi

UI_BASE_URL="${UI_BASE_URL:-${BASE_URL:-https://console.svc.plus}}"
LOGIN_EMAIL="${LOGIN_EMAIL:-admin@svc.plus}"
LOGIN_PASSWORD="${SUPERADMIN_PASSWORD:-}"

if [ -z "$LOGIN_PASSWORD" ]; then
  echo "❌ 缺少 SUPERADMIN_PASSWORD（可写入 .env）" >&2
  exit 1
fi

TMP_DIR="${TMPDIR:-/tmp}"
PLAYWRIGHT_TEST_DIR="${TMP_DIR}/xcontrol-playwright-login"
mkdir -p "$PLAYWRIGHT_TEST_DIR"

cat <<'TEST' > "$PLAYWRIGHT_TEST_DIR/login.spec.mjs"
import { test, expect } from '@playwright/test';

test.use({ screenshot: 'only-on-failure' });

test('superadmin login', async ({ page }) => {
  const baseUrl = process.env.UI_BASE_URL || process.env.BASE_URL || 'https://console.svc.plus';
  const email = process.env.LOGIN_EMAIL || 'admin@svc.plus';
  const password = process.env.SUPERADMIN_PASSWORD;

  if (!password) {
    throw new Error('missing SUPERADMIN_PASSWORD');
  }

  await page.goto(`${baseUrl}/login`, { waitUntil: 'domcontentloaded' });

  const emailByRole = page.getByRole('textbox', { name: /email|邮箱|账号|用户名|identifier/i });
  const emailBySelector = page.locator(
    'input[type="email"], input[name="email"], input[name="identifier"], input[placeholder*="邮箱"], input[placeholder*="Email"], input[type="text"]'
  );
  const emailInput = (await emailByRole.count()) > 0 ? emailByRole.first() : emailBySelector.first();
  await expect(emailInput).toBeVisible({ timeout: 15000 });
  await emailInput.fill(email);

  const passwordInput = page.locator(
    'input[type="password"], input[name="password"], input[placeholder*="密码"], input[placeholder*="Password"]'
  );
  await expect(passwordInput).toBeVisible({ timeout: 15000 });
  await passwordInput.fill(password);

  const submitBtn = page.locator(
    'button[type="submit"], button:has-text("登录"), button:has-text("Log in"), button:has-text("Sign in")'
  );
  await expect(submitBtn).toBeVisible({ timeout: 15000 });
  await submitBtn.click();

  await page.waitForLoadState('networkidle');
  await expect(page).not.toHaveURL(/login/);
});
TEST

cd "$PLAYWRIGHT_TEST_DIR"

if [ ! -f package.json ]; then
  cat <<'PKG' > package.json
{
  "name": "xcontrol-playwright-login",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "devDependencies": {
    "@playwright/test": "^1.49.0"
  }
}
PKG
fi

npm install --silent
npx playwright install chromium

UI_BASE_URL="$UI_BASE_URL" BASE_URL="$UI_BASE_URL" LOGIN_EMAIL="$LOGIN_EMAIL" SUPERADMIN_PASSWORD="$LOGIN_PASSWORD" \
  npx playwright test login.spec.mjs

echo "✅ UI 登录测试通过"
