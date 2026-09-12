import { expect, test } from '@playwright/test';

test.use({ hasTouch: true, contextOptions: { reducedMotion: 'no-preference' } });

test.beforeEach(async ({ page }) => {
	await page.addInitScript(() => {
		localStorage.setItem('sqlstreams-board:cookie-notice', '2026-09-12');
	});
	await page.goto('/members/brandon/');
	await expect(page.locator('astro-island[component-url*="member-profile"]')).not.toHaveAttribute(
		'ssr',
	);
	await page.clock.install({ time: new Date('2026-09-12T12:00:00Z') });
	await page.clock.pauseAt(new Date('2026-09-12T12:00:01Z'));
	await page.mouse.move(1, 1);
});

test('four idle seconds start fading and growth without moving links', async ({ page }) => {
	const text = page.locator('.personal-text');
	const website = page.locator('.facts a');
	const originalBounds = await website.boundingBox();
	await page.clock.runFor(3_999);
	await expect(text).toHaveAttribute('data-idle', 'false');
	await page.clock.runFor(50);
	await expect(text).toHaveAttribute('data-idle', 'true');
	await page.clock.fastForward(110_000);
	await expect(page.locator('.personal-text-veil')).toHaveCSS('opacity', '1');
	const scale = await page.locator('.personal-text-track').evaluate((element) => {
		return new DOMMatrix(getComputedStyle(element).transform).a;
	});
	expect(scale).toBeCloseTo(113 / 13, 4);
	expect(await website.boundingBox()).toEqual(originalBounds);
	expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);

	await page.mouse.move(2, 2);
	await expect(text).toHaveAttribute('data-idle', 'false');
	await expect(page.locator('.personal-text-veil')).toHaveCSS('opacity', '0');
	await expect(page.locator('.personal-text-track')).toHaveCSS(
		'transform',
		'matrix(1, 0, 0, 1, 0, 0)',
	);
});

test('larger text scrolls more slowly and activity restores its normal speed', async ({ page }) => {
	const text = page.locator('.personal-text-track');
	await page.clock.runFor(6_000);
	await page.clock.fastForward(55_000);
	const middle = await text.evaluate(async (element) => {
		const animations = element.getAnimations({ subtree: true });
		await Promise.all(animations.map((animation) => animation.ready));
		const scale = new DOMMatrix(getComputedStyle(element).transform).a;
		return animations.map((animation) => animation.playbackRate * scale);
	});
	expect(middle).toHaveLength(2);
	expect(middle[0]).toBeGreaterThan(0);
	expect(middle[0]).toBeLessThan(1);
	expect(middle[1]).toBe(middle[0]);

	await page.clock.fastForward(55_000);
	const full = await text.evaluate(async (element) => {
		const animations = element.getAnimations({ subtree: true });
		await Promise.all(animations.map((animation) => animation.ready));
		const scale = new DOMMatrix(getComputedStyle(element).transform).a;
		return animations.map((animation) => animation.playbackRate * scale);
	});
	expect(full[0]).toBeGreaterThan(0);
	expect(full[0]).toBeLessThan(middle[0]!);
	expect(full[1]).toBe(full[0]);

	await page.mouse.move(2, 2);
	const restored = await text.evaluate(async (element) => {
		const animations = element.getAnimations({ subtree: true });
		await Promise.all(animations.map((animation) => animation.ready));
		return animations.map((animation) => animation.playbackRate);
	});
	expect(restored).toEqual([1, 1]);
});

test('the text widens to the viewport edges without horizontal overflow and restores on activity', async ({
	page,
}) => {
	await page.setViewportSize({ width: 1280, height: 400 });
	const growth = page.locator('.personal-text-growth');
	const original = await growth.boundingBox();
	expect(original).not.toBeNull();
	await page.clock.runFor(4_000);
	await page.clock.fastForward(54_000);
	const middle = await growth.boundingBox();
	expect(middle!.x).toBeGreaterThan(0);
	expect(middle!.x).toBeLessThan(original!.x);
	expect(middle!.width).toBeGreaterThan(original!.width);
	await page.clock.fastForward(56_000);
	await expect.poll(async () => (await growth.boundingBox())!.x).toBeCloseTo(0, 1);
	const full = await growth.boundingBox();
	const viewportWidth = await page.evaluate(() => document.documentElement.clientWidth);
	expect(full!.width).toBeCloseTo(viewportWidth, 1);
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(viewportWidth);

	await page.setViewportSize({ width: 360, height: 400 });
	await expect.poll(async () => (await growth.boundingBox())!.x).toBeCloseTo(0, 1);
	const narrowWidth = await page.evaluate(() => document.documentElement.clientWidth);
	await expect.poll(async () => (await growth.boundingBox())!.width).toBeCloseTo(narrowWidth, 1);
	expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(narrowWidth);

	await page.mouse.move(2, 2);
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'false');
	const restored = await growth.boundingBox();
	const textWindow = await page.locator('.personal-text-window').boundingBox();
	expect(restored!.x).toBeCloseTo(textWindow!.x, 1);
	expect(restored!.width).toBeCloseTo(textWindow!.width, 1);
});

test('keyboard, wheel, and touch each restore the profile', async ({ page }) => {
	const text = page.locator('.personal-text');
	await page.clock.runFor(6_000);
	await expect(text).toHaveAttribute('data-idle', 'true');
	await page.keyboard.press('Tab');
	await expect(text).toHaveAttribute('data-idle', 'false');

	await page.clock.runFor(6_000);
	await expect(text).toHaveAttribute('data-idle', 'true');
	await page.mouse.wheel(0, 10);
	await expect(text).toHaveAttribute('data-idle', 'false');

	await page.clock.runFor(6_000);
	await expect(text).toHaveAttribute('data-idle', 'true');
	await page.touchscreen.tap(5, 5);
	await expect(text).toHaveAttribute('data-idle', 'false');
});

test('reduced motion restores the profile and disables idle animation', async ({ page }) => {
	await page.clock.runFor(6_000);
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'true');
	await page.emulateMedia({ reducedMotion: 'reduce' });
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'false');
	await page.clock.fastForward(120_000);
	await expect(page.locator('.personal-text-veil')).toHaveCSS('display', 'none');
	await expect(page.locator('.personal-text-track')).toHaveCSS('transform', 'none');
	await expect(page.locator('.personal-text-copy').first()).toHaveCSS('animation-name', 'none');
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'false');
});

test('hidden time does not advance the effect and returning starts a fresh delay', async ({
	page,
}) => {
	await page.clock.runFor(6_000);
	await page.evaluate(() => {
		Object.defineProperty(document, 'hidden', { configurable: true, value: true });
		document.dispatchEvent(new Event('visibilitychange'));
	});
	await page.clock.fastForward(120_000);
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'false');
	await page.evaluate(() => {
		Reflect.deleteProperty(document, 'hidden');
		document.dispatchEvent(new Event('visibilitychange'));
	});
	await page.clock.runFor(3_999);
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'false');
	await page.clock.runFor(50);
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'true');
});

test('navigation works on the first tap and returning restarts the profile effect', async ({
	page,
}) => {
	await page.clock.runFor(6_000);
	await page.clock.fastForward(110_000);
	await page.locator('a[href="/search/"]').first().tap();
	await expect(page).toHaveURL(/\/search\//);
	await expect(page.locator('.personal-text-veil')).toHaveCount(0);
	await page.clock.fastForward(120_000);
	await expect(page.locator('.personal-text-veil')).toHaveCount(0);
	await page.clock.resume();
	await page.goBack();
	await expect(page).toHaveURL(/\/members\/brandon\//);
	await expect(page.locator('astro-island[component-url*="member-profile"]')).not.toHaveAttribute(
		'ssr',
	);
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'false');
	await page.clock.runFor(6_000);
	await expect(page.locator('.personal-text')).toHaveAttribute('data-idle', 'true');
});
