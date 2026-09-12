const idleDelayMs = 4_000;
const growthDurationMs = 110_000;

export class MemberPersonalTextState {
	progress = $state(0);
	scale = $derived(1 + (100 / 13) * this.progress ** 2);
	leftInset = $state(0);
	rightInset = $state(0);

	start(text: HTMLElement, textWindow: HTMLElement): () => void {
		const listeners = new AbortController();
		const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
		const bounds = new ResizeObserver(() => {
			const rectangle = textWindow.getBoundingClientRect();
			this.leftInset = rectangle.left;
			this.rightInset = document.documentElement.clientWidth - rectangle.right;
		});
		bounds.observe(textWindow);
		bounds.observe(document.documentElement);
		let timer = 0;
		let frame = 0;
		let started = 0;

		const advance = () => {
			this.setProgress(Math.min(1, (performance.now() - started) / growthDurationMs), text);
			if (this.progress < 1) frame = requestAnimationFrame(advance);
		};

		const reset = () => {
			window.clearTimeout(timer);
			cancelAnimationFrame(frame);
			this.setProgress(0, text);
			if (listeners.signal.aborted || document.hidden || motion.matches) return;

			timer = window.setTimeout(() => {
				started = performance.now();
				frame = requestAnimationFrame(advance);
			}, idleDelayMs);
		};

		const stop = () => {
			bounds.disconnect();
			listeners.abort();
			reset();
		};

		// Input counts as activity; layout-induced scroll events do not.
		for (const event of [
			'pointermove',
			'pointerdown',
			'keydown',
			'wheel',
			'touchmove',
			'focusin',
		]) {
			window.addEventListener(event, reset, {
				capture: true,
				passive: true,
				signal: listeners.signal,
			});
		}
		document.addEventListener('visibilitychange', reset, { signal: listeners.signal });
		window.addEventListener('pageshow', reset, { signal: listeners.signal });
		motion.addEventListener('change', reset, { signal: listeners.signal });
		document.addEventListener('astro:before-swap', stop, { signal: listeners.signal });
		reset();

		return stop;
	}

	private setProgress(progress: number, text: HTMLElement): void {
		this.progress = progress;
		// Compensate for enlargement while progressively reducing pixel speed.
		const playbackRate = 1 / this.scale ** 1.9;
		for (const animation of text.getAnimations({ subtree: true })) {
			animation.updatePlaybackRate(playbackRate);
		}
	}
}
