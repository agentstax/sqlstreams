import { describe, expect, it } from 'vitest';
import { fillText, logAttributes } from './placeholders';

// The lines below are the shapes the library actually emits, so a change to
// its rendering shows up here first.
describe('logAttributes', () => {
	const names = ['stream_id', 'group_id', 'message_id'];

	it("reads slog's text handler", () => {
		const line =
			'time=2026-08-26T10:11:12.345-05:00 level=WARN msg="message dead-lettered -- unrecoverable, will not be retried" code=SQL0029 group_id=2 stream_id=1 message_id=42';

		expect(logAttributes(line, names)).toEqual(
			new Map([
				['stream_id', '1'],
				['group_id', '2'],
				['message_id', '42'],
			]),
		);
	});

	it('reads a quoted text-handler value', () => {
		expect(logAttributes('level=WARN stream="orders and refunds"', ['stream'])).toEqual(
			new Map([['stream', 'orders and refunds']]),
		);
	});

	it('reads a JSON log line', () => {
		const line = '{"level":"WARN","code":"SQL0029","stream_id":1,"group_id":2,"message_id":42}';

		expect(logAttributes(line, names)).toEqual(
			new Map([
				['stream_id', '1'],
				['group_id', '2'],
				['message_id', '42'],
			]),
		);
	});

	it('reads the Error() one-liner', () => {
		const line =
			'stream not found: stream "orders", version 3 -- register it with Client.Stream(name).Register first [SQL0005]';

		expect(logAttributes(line, ['stream', 'version'])).toEqual(
			new Map([
				['stream', 'orders'],
				['version', '3'],
			]),
		);
	});

	it('does not read a problem-line word as a value', () => {
		expect(logAttributes('stream not found [SQL0005]', ['stream'])).toEqual(new Map());
	});

	it('leaves a name the line never carried absent', () => {
		expect(logAttributes('level=WARN code=SQL0029 stream_id=1', names)).toEqual(
			new Map([['stream_id', '1']]),
		);
	});

	it('returns nothing for an empty line', () => {
		expect(logAttributes('', names)).toEqual(new Map());
	});

	// stream_id must not be satisfied by the stream= pair sitting beside it
	it('matches a whole name, never a prefix of one', () => {
		expect(logAttributes('stream=orders group=billing', ['stream_id'])).toEqual(new Map());
	});
});

describe('fillText', () => {
	it('substitutes a name the values carry', () => {
		const segments = fillText(
			'register "{schedule}" with Client.Scheduler(name).Register first',
			new Map([['schedule', 'nightly-rollup']]),
		);

		expect(segments).toEqual([
			{ text: 'register "', kind: 'plain' },
			{ text: 'nightly-rollup', kind: 'value' },
			{ text: '" with Client.Scheduler(name).Register first', kind: 'plain' },
		]);
	});

	it('leaves a name nothing filled as a visible blank', () => {
		const segments = fillText(
			'migrate the {owner_kind} schema up from {version} to {build_version}',
			new Map([['owner_kind', 'stream']]),
		);

		expect(segments).toEqual([
			{ text: 'migrate the ', kind: 'plain' },
			{ text: 'stream', kind: 'value' },
			{ text: ' schema up from ', kind: 'plain' },
			{ text: '{version}', kind: 'placeholder' },
			{ text: ' to ', kind: 'plain' },
			{ text: '{build_version}', kind: 'placeholder' },
		]);
	});

	it('returns one plain segment for a fix with no placeholders', () => {
		const segments = fillText('choose a different name', new Map());

		expect(segments).toEqual([{ text: 'choose a different name', kind: 'plain' }]);
	});

	it('returns no segments for empty text', () => {
		expect(fillText('', new Map())).toEqual([]);
	});

	it('marks every blank when nothing fills', () => {
		const segments = fillText('migrate the {owner_kind} schema up from {version}', new Map());

		expect(segments).toEqual([
			{ text: 'migrate the ', kind: 'plain' },
			{ text: '{owner_kind}', kind: 'placeholder' },
			{ text: ' schema up from ', kind: 'plain' },
			{ text: '{version}', kind: 'placeholder' },
		]);
	});
});
