<script lang="ts" module>
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import LogLinePaste from './log-line-paste.svelte';

	const { Story } = defineMeta({
		title: 'Board/LogLinePaste',
		component: LogLinePaste,
		args: {
			placeholders: ['stream_id', 'group_id', 'message_id'],
			exampleText:
				'level=WARN msg="message dead-lettered -- unrecoverable, will not be retried" code=SS0029 group_id=2 stream_id=1 message_id=42',
		},
	});
</script>

<!-- the state module is a page singleton, so the stories share one pasted line -->
<Story name="Nothing pasted" />

<Story
	name="One placeholder"
	args={{
		placeholders: ['stream'],
		exampleText:
			'stream not found: stream "orders", version 3 -- register it with Client.Stream(name).Register first [SS0005]',
	}}
/>

<Story
	name="No placeholders"
	args={{
		placeholders: [],
		exampleText: 'level=WARN msg="manager row suspended" code=SS0035 worker=janitor',
	}}
/>
