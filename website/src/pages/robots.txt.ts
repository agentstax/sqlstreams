import type { APIContext, APIRoute } from 'astro';

export const GET = (({ site }: APIContext): Response => {
	if (site === undefined) {
		throw new Error('robots.txt requires the configured site URL');
	}

	const sitemapUrl = new URL('/sitemap-index.xml', site);
	const body = `User-agent: *\nAllow: /\n\nSitemap: ${sitemapUrl.href}\n`;

	return new Response(body, {
		headers: { 'Content-Type': 'text/plain; charset=utf-8' },
	});
}) satisfies APIRoute;
