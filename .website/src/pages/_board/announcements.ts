export type Announcement = {
	title: string;
	href: string;
	date: string; // the milestone's HISTORY.md entry date
};

export const announcements: Announcement[] = [
	{
		title: 'A Reference board — one thread per handle, every default checked against the library',
		href: '/reference/',
		date: '2026-09-06',
	},
	{
		title: 'Migration compat gate ships — old binaries stay safe through additive releases',
		href: '/guides/migrations/',
		date: '2026-08-22',
	},
	{
		title: 'Per-stream table families — every stream gets its own message_log',
		href: '/concepts/architecture/',
		date: '2026-08-22',
	},
	{
		title: 'Every error and log event gets a SS code and its own page',
		href: '/errors/',
		date: '2026-08-20',
	},
];
