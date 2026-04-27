<script setup lang="ts">
import { computed } from 'vue'
import { withBase } from 'vitepress'

const props = defineProps<{
	locale?: 'zh' | 'en'
}>()

type LinkItem = {
	text: string
	href: string
	external?: boolean
	variant?: 'primary' | 'secondary'
}

type ValueItem = {
	title: string
	description: string
}

type LandingContent = {
	eyebrow: string
	title: string
	description: string
	actions: LinkItem[]
	values: ValueItem[]
}

const contents: Record<'zh' | 'en', LandingContent> = {
	zh: {
		eyebrow: 'NeoCode Coding Agent',
		title: '终端里的本地 AI 编码助手',
		description:
			'本地运行，贴合终端工作流。从理解代码、修改文件到运行验证，NeoCode 帮你把一次代码任务完整推进。',
		actions: [
			{ text: '立即安装', href: '/guide/install', variant: 'primary' },
			{ text: '查看示例', href: '/guide/examples', variant: 'secondary' },
			{ text: 'GitHub', href: 'https://github.com/1024XEngineer/neo-code', external: true, variant: 'secondary' },
		],
		values: [
			{ title: '本地运行', description: '配置和工作区留在你的机器上。' },
			{ title: '终端原生', description: '直接融入 shell 和 TUI 工作流。' },
			{ title: '工具可控', description: '写文件、跑命令前保留确认边界。' },
		],
	},
	en: {
		eyebrow: 'NeoCode Coding Agent',
		title: 'A local AI coding agent for your terminal',
		description:
			'Run locally, stay in your terminal workflow, and move a code task from understanding to edits and verification.',
		actions: [
			{ text: 'Install now', href: '/en/guide/install', variant: 'primary' },
			{ text: 'See examples', href: '/en/guide/examples', variant: 'secondary' },
			{ text: 'GitHub', href: 'https://github.com/1024XEngineer/neo-code', external: true, variant: 'secondary' },
		],
		values: [
			{ title: 'Local first', description: 'Config and workspace stay on your machine.' },
			{ title: 'Terminal native', description: 'Designed for shell and TUI workflows.' },
			{ title: 'Controlled tools', description: 'File edits and commands keep approval boundaries.' },
		],
	},
}

const currentLocale = computed<'zh' | 'en'>(() => (props.locale === 'en' ? 'en' : 'zh'))
const content = computed(() => contents[currentLocale.value])

// hrefFor 负责统一处理站内链接的 base 前缀，同时保留外部链接原始地址。
function hrefFor(item: LinkItem) {
	if (item.external) {
		return item.href
	}
	return withBase(item.href)
}
</script>

<template>
	<div class="home-landing">
		<section class="home-hero" aria-labelledby="home-title">
			<p class="home-eyebrow">{{ content.eyebrow }}</p>
			<h1 id="home-title">{{ content.title }}</h1>
			<p class="home-hero__lead">{{ content.description }}</p>

			<div class="home-actions">
				<a
					v-for="action in content.actions"
					:key="action.text"
					class="home-action"
					:class="`home-action--${action.variant || 'secondary'}`"
					:href="hrefFor(action)"
					:target="action.external ? '_blank' : undefined"
					:rel="action.external ? 'noreferrer' : undefined"
				>
					{{ action.text }}
				</a>
			</div>

			<div class="home-values" aria-label="NeoCode highlights">
				<div v-for="value in content.values" :key="value.title" class="home-value">
					<strong>{{ value.title }}</strong>
					<span>{{ value.description }}</span>
				</div>
			</div>
		</section>
	</div>
</template>
