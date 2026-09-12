<script lang="ts" module>
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import ErrorPost from './error-post.svelte';
	import CopyButton from '../copy-button/copy-button.svelte';

	const { Story } = defineMeta({
		title: 'Board/ErrorPost',
		component: ErrorPost,
		args: {
			code: 'SQL0005',
			rank: 'Permanent error',
			reportHref: 'https://github.com/agentstax/sqlstreams/issues/new',
			actions: null,
		},
	});
</script>

<Story name="Post">
	{#snippet template(args)}
		{@const { children: _children, ...storyProps } = args}
		<ErrorPost {...storyProps}>
			<p>As it arrives in your log or error chain — the values are your call's own.</p>
		</ErrorPost>
	{/snippet}
</Story>

<Story name="WithCopyLink">
	{#snippet template(args)}
		{@const { children: _children, ...storyProps } = args}
		<ErrorPost {...storyProps}>
			{#snippet actions()}
				<CopyButton label="Copy link" text="https://sqlstreams.io/errors/SQL0005/" />
			{/snippet}
			<p>As it arrives in your log or error chain — the values are your call's own.</p>
		</ErrorPost>
	{/snippet}
</Story>
