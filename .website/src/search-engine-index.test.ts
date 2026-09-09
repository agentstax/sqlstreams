import { describe, expect, it } from 'vitest';
import { isSearchEngineIndexable } from './search-engine-index';

describe('isSearchEngineIndexable', () => {
	it.each(['/search', '/search/', '/whats-new', '/whats-new/'])('excludes %s', (pathname) => {
		expect(isSearchEngineIndexable(pathname)).toBe(false);
	});

	it.each(['/', '/quickstart/', '/errors/SS0005/'])('includes %s', (pathname) => {
		expect(isSearchEngineIndexable(pathname)).toBe(true);
	});
});
