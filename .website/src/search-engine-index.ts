const excludedPathnames = new Set(['/search', '/whats-new']);

export function isSearchEngineIndexable(pathname: string): boolean {
	const normalized = pathname !== '/' && pathname.endsWith('/') ? pathname.slice(0, -1) : pathname;
	return !excludedPathnames.has(normalized);
}
