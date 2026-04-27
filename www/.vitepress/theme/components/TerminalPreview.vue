<script setup lang="ts">
const props = defineProps<{
	locale?: 'zh' | 'en'
}>()

const lines = props.locale === 'en'
	? [
		{ type: 'prompt', text: '$ neocode' },
		{ type: 'meta', text: 'NeoCode v1.2.3 · provider openai · model gpt-4o' },
		{ type: 'prompt', text: '> Refactor the runtime loop and run tests' },
		{ type: 'output', text: 'Reading internal/runtime/loop.go' },
		{ type: 'tool', text: 'filesystem.read_file' },
		{ type: 'tool', text: 'filesystem.edit_file · approved' },
		{ type: 'tool', text: 'bash · go test ./internal/runtime/...' },
		{ type: 'success', text: 'All tests passed · 3 files changed' },
	]
	: [
		{ type: 'prompt', text: '$ neocode' },
		{ type: 'meta', text: 'NeoCode v1.2.3 · provider openai · model gpt-4o' },
		{ type: 'prompt', text: '> 重构 runtime 主循环并运行测试' },
		{ type: 'output', text: '正在阅读 internal/runtime/loop.go' },
		{ type: 'tool', text: 'filesystem.read_file' },
		{ type: 'tool', text: 'filesystem.edit_file · 已审批' },
		{ type: 'tool', text: 'bash · go test ./internal/runtime/...' },
		{ type: 'success', text: '全部测试通过 · 3 个文件已修改' },
	]
</script>

<template>
	<div class="terminal-preview" aria-label="NeoCode terminal session preview">
		<div class="terminal-header">
			<span>neocode session</span>
			<code>local</code>
		</div>
		<div class="terminal-body">
			<div
				v-for="(line, i) in lines"
				:key="i"
				:class="['terminal-line', `terminal-line--${line.type}`]"
			>
				<span class="terminal-prefix">{{ line.type === 'prompt' ? '>' : line.type === 'tool' ? '+' : '=' }}</span>
				<span>{{ line.text }}</span>
			</div>
		</div>
	</div>
</template>

<style scoped>
.terminal-preview {
	overflow: hidden;
	border: 1px solid var(--nc-border);
	border-radius: 8px;
	background: var(--nc-code-bg);
	color: var(--nc-code-text);
	font-family: var(--vp-font-family-mono);
}

.terminal-header {
	display: flex;
	align-items: center;
	justify-content: space-between;
	gap: 16px;
	padding: 10px 14px;
	border-bottom: 1px solid var(--nc-border);
	background: var(--nc-code-toolbar-bg);
	color: var(--nc-muted);
	font-size: 12px;
}

.terminal-header code {
	border: 1px solid var(--nc-border);
	border-radius: 999px;
	padding: 2px 8px;
	background: transparent;
	color: var(--nc-muted);
	font-size: 11px;
}

.terminal-body {
	padding: 14px;
}

.terminal-line {
	display: grid;
	grid-template-columns: 18px minmax(0, 1fr);
	gap: 8px;
	align-items: baseline;
	font-size: 12px;
	line-height: 1.8;
	white-space: pre-wrap;
	overflow-wrap: anywhere;
}

.terminal-prefix {
	color: var(--nc-muted);
	user-select: none;
}

.terminal-line--prompt {
	color: var(--nc-text);
}

.terminal-line--meta,
.terminal-line--output {
	color: var(--nc-muted);
}

.terminal-line--tool {
	color: var(--nc-accent);
}

.terminal-line--success {
	color: var(--nc-success);
}
</style>
