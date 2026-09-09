<script lang="ts" module>
	import { defineMeta } from '@storybook/addon-svelte-csf';
	import SearchResult from './search-result.svelte';

	const { Story } = defineMeta({
		title: 'Board/SearchResult',
		component: SearchResult,
		args: {
			title: 'stream not found [SQL0005]',
			href: '/errors/SQL0005/',
			excerpt: [
				{ text: 'the ', marked: false },
				{ text: 'stream', marked: true },
				{ text: ' named in the call is not registered — register it with ', marked: false },
				{ text: 'Client.Stream(name).Register', marked: true },
				{ text: ' first.', marked: false },
			],
		},
	});
</script>

<Story name="Result" />
