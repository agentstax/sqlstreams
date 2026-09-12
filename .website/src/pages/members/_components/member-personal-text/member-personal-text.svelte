<script lang="ts">
	import { onMount } from 'svelte';
	import { MemberPersonalTextState } from './member-personal-text-state.svelte';

	type Props = {
		// the phrase the personal text repeats without ever completing
		personalText: string;
	};

	let { personalText }: Props = $props();

	// each copy must be wider than any viewport, so the loop never shows a gap
	const repeated = `${personalText} `.repeat(20);
	const state = new MemberPersonalTextState();
	let text: HTMLDivElement;
	let textWindow: HTMLDivElement;

	onMount(() => state.start(text, textWindow));
</script>

<div
	class="personal-text"
	data-idle={state.progress > 0}
	style:--personal-text-progress={state.progress}
	style:--personal-text-scale={state.scale}
	style:--personal-text-left-inset={`${state.leftInset}px`}
	style:--personal-text-right-inset={`${state.rightInset}px`}
>
	<div class="personal-text-veil" aria-hidden="true"></div>
	<span class="personal-text-label">Personal text</span>
	<div class="personal-text-window" bind:this={textWindow}>
		<div class="personal-text-growth">
			<div class="personal-text-track" bind:this={text}>
				<span class="personal-text-copy">{repeated}</span>
				<span class="personal-text-copy" aria-hidden="true">{repeated}</span>
			</div>
		</div>
	</div>
</div>

<style src="./member-personal-text.css"></style>
