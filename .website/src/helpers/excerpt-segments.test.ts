import { describe, expect, it } from 'vitest';
import { excerptSegments } from './excerpt-segments';

describe('excerptSegments', () => {
	it('returns one unmarked segment for plain text', () => {
		expect(excerptSegments('plain text with no matches.')).toEqual([
			{ text: 'plain text with no matches.', marked: false },
		]);
	});

	it('splits marked words out of the surrounding text', () => {
		expect(excerptSegments('the <mark>stream</mark> was not found')).toEqual([
			{ text: 'the ', marked: false },
			{ text: 'stream', marked: true },
			{ text: ' was not found', marked: false },
		]);
	});

	it('handles marks at the start and end', () => {
		expect(excerptSegments('<mark>stream</mark> not <mark>found</mark>')).toEqual([
			{ text: 'stream', marked: true },
			{ text: ' not ', marked: false },
			{ text: 'found', marked: true },
		]);
	});

	it('decodes entities in and around marks', () => {
		expect(excerptSegments('a &quot;stream&quot; &amp; its <mark>&lt;log&gt;</mark>')).toEqual([
			{ text: 'a "stream" & its ', marked: false },
			{ text: '<log>', marked: true },
		]);
	});

	it('returns no segments for an empty excerpt', () => {
		expect(excerptSegments('')).toEqual([]);
	});

	it('keeps an unclosed mark as one marked segment', () => {
		expect(excerptSegments('tail <mark>match')).toEqual([
			{ text: 'tail ', marked: false },
			{ text: 'match', marked: true },
		]);
	});
});
