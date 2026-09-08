import { expect, test } from '@playwright/test';

// PGlite downloads and boots a full Postgres; its waits dwarf the default
const sandboxBootTimeout = 120_000;

test('the home page renders with a board style applied', async ({ page }) => {
	await page.goto('/');
	await expect(page).toHaveTitle('Vulkan Board Index | Vulkan Docs');
	await expect(page.locator('html')).toHaveAttribute('data-board-style', /classic|night/);
});

test('a thread suffixes its document title without changing its heading', async ({ page }) => {
	await page.goto('/quickstart/');
	await expect(page).toHaveTitle('Quickstart | Vulkan Docs');
	await expect(page.getByRole('heading', { level: 1 })).toHaveText('Quickstart');
});

test('utility pages stay outside the search-engine index and sitemap', async ({
	page,
	request,
}) => {
	for (const pathname of ['/search/', '/whats-new/']) {
		await page.goto(pathname);
		await expect(page.locator('meta[name="robots"]')).toHaveAttribute('content', 'noindex');
	}

	await page.goto('/quickstart/');
	await expect(page.locator('meta[name="robots"]')).toHaveCount(0);
	await expect(page.locator('meta[name="keywords"]')).toHaveCount(0);

	const indexResponse = await request.get('/sitemap-index.xml');
	expect(indexResponse.ok()).toBe(true);
	const indexXml = await indexResponse.text();
	const sitemapLocations = [...indexXml.matchAll(/<loc>([^<]+)<\/loc>/g)]
		.map((match) => match[1])
		.filter((location): location is string => location !== undefined);
	const sitemapPaths = sitemapLocations.map((location) => new URL(location).pathname);
	expect(sitemapPaths.length).toBeGreaterThan(0);

	const sitemapXml = (
		await Promise.all(sitemapPaths.map(async (pathname) => (await request.get(pathname)).text()))
	).join('\n');
	expect(sitemapXml).not.toContain('/search/');
	expect(sitemapXml).not.toContain('/whats-new/');
	expect(sitemapXml).not.toContain('/tags/');
});

test('the sandbox boots and its panels show rows', async ({ page }) => {
	test.setTimeout(sandboxBootTimeout + 30_000);
	await page.goto('/');
	await expect(page.locator('.sandbox-region table').first()).toBeVisible({
		timeout: sandboxBootTimeout,
	});
});

test('search finds threads and back returns to the results', async ({ page }) => {
	await page.goto('/search/?q=topic');
	const results = page.locator('.search-results a');
	await expect(results.first()).toBeVisible({ timeout: 20_000 });

	await results.first().click();
	await expect(page).not.toHaveURL(/\/search/, { timeout: 15_000 });

	// one back returns to the query and its results (the history entry keeps
	// the client router's state, so the router must handle the popstate)
	await page.goBack();
	await expect(page).toHaveURL(/\/search\/\?q=topic/);
	await expect(results.first()).toBeVisible({ timeout: 20_000 });
});

test('the editor swaps in over the static shell and registers edits', async ({ page }) => {
	await page.goto('/');

	// the swap needs only hydration and an idle tick, not a booted database,
	// so this waits on CodeMirror alone
	const panel = page.locator('.sql-panel').first();
	await expect(panel.locator('.cm-editor')).toBeVisible({ timeout: 30_000 });
	await expect(panel.locator('.editor-area pre')).toHaveCount(0);

	// typing must reach the panel state -- the chip leaves "auto re-runs"
	// only through its setSql
	await panel.locator('.cm-content').click();
	await page.keyboard.type(' -- edited');
	await expect(panel.locator('.panel-chip')).toHaveText(/edited/);
});

// raw (decompressed) bytes, measured 2026-08-28 at 82,080 -- headroom for
// small growth, a failing build for a heavy chunk in the initial payload
const initialJsCeilingBytes = 96_000;

test('the homepage JS below the sandbox gate stays under the ceiling', async ({ page }) => {
	const scriptBytes: Promise<number>[] = [];
	const scriptUrls: string[] = [];
	page.on('response', (response) => {
		if (response.request().resourceType() !== 'script') return;
		scriptUrls.push(response.url());
		scriptBytes.push(response.body().then((body) => body.length));
	});

	// below the 761px sandbox gate the island never hydrates, so networkidle
	// is a stable moment and the sum is the JS every phone reader pays
	await page.setViewportSize({ width: 640, height: 900 });
	await page.goto('/', { waitUntil: 'networkidle' });

	expect(scriptUrls.filter((url) => /pglite/i.test(url))).toEqual([]);
	const totalBytes = (await Promise.all(scriptBytes)).reduce((sum, bytes) => sum + bytes, 0);
	expect(totalBytes).toBeLessThan(initialJsCeilingBytes);
});

test('leaving a booted sandbox keeps the next page alive', async ({ page }) => {
	test.setTimeout(sandboxBootTimeout + 60_000);
	await page.goto('/');
	await expect(page.locator('.sandbox-region table').first()).toBeVisible({
		timeout: sandboxBootTimeout,
	});

	// closing PGlite with a statement in flight once wedged the main thread
	// for good; typing and searching proves the destination page still runs
	await page.locator('a[href="/search/"]').first().click();

	// an island still carrying ssr has not hydrated: typing into it submits
	// the form natively and the query is lost
	await expect(page.locator('astro-island[component-url*="board-search"]:not([ssr])')).toBeAttached(
		{ timeout: 15_000 },
	);
	await page.locator('input[type="search"]').fill('topic', { timeout: 15_000 });
	await page.keyboard.press('Enter');
	await expect(page.locator('.search-results a').first()).toBeVisible({ timeout: 20_000 });
});
