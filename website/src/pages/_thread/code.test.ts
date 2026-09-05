import { describe, expect, it } from 'vitest';
import { codeMetaDescription } from './code';
import type { CodeThreadData } from './model';

describe('codeMetaDescription', () => {
	it('includes the fix when the declaration carries one', () => {
		expect(
			codeMetaDescription(thread('error', 'recovery permanent', 'retry stops', 'register it')),
		).toBe('Code VK0005 · recovery permanent — retry stops · Fix: register it');
	});

	it.each([
		['event', 'log event at warn'],
		['metric', 'metric'],
		['alert', 'alert'],
	] as const)('derives a %s description without an absent fix', (kind, classification) => {
		expect(
			codeMetaDescription(thread(kind, classification, 'the declared consequence', null)),
		).toBe(`Code VK0005 · ${classification} — the declared consequence`);
	});
});

function thread(
	kind: CodeThreadData['kind'],
	classification: string,
	consequence: string,
	fix: string | null,
): CodeThreadData {
	return {
		code: 'VK0005',
		kind,
		solved: fix !== null,
		classification,
		rank: classification,
		introduction: '',
		logLine: '',
		consequence,
		fix,
	};
}
