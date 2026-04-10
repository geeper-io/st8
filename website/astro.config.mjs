// @ts-check
import { defineConfig } from 'astro/config';
import starlight from '@astrojs/starlight';

// https://astro.build/config
export default defineConfig({
	integrations: [
		starlight({
			title: 'st8',
			description: 'Lightweight versioned state store for dynamic configuration',
			social: [{ icon: 'github', label: 'GitHub', href: 'https://github.com/geeper-io/st8' }],
			customCss: ['./src/styles/custom.css'],
			sidebar: [
				{
					label: 'Getting Started',
					items: [
						{ label: 'Introduction', slug: 'guides/introduction' },
						{ label: 'Quick Start', slug: 'guides/quickstart' },
						{ label: 'Concepts', slug: 'guides/concepts' },
					],
				},
				{
					label: 'Recipes',
					items: [
						{ label: 'Dynamic Configuration', slug: 'guides/dynamic-config' },
						{ label: 'Feature Flags', slug: 'guides/feature-flags' },
						{ label: 'A/B Testing', slug: 'guides/ab-testing' },
						{ label: 'CI/CD Integration', slug: 'guides/ci-cd' },
						{ label: 'OpenFeature (OFREP)', slug: 'guides/openfeature' },
					],
				},
				{
					label: 'Reference',
					items: [
						{ label: 'CLI (st8ctl)', slug: 'reference/cli' },
						{ label: 'Go Client', slug: 'reference/go-client' },
						{ label: 'HTTP API', slug: 'reference/api' },
						{ label: 'Configuration', slug: 'reference/configuration' },
					],
				},
			],
		}),
	],
});
