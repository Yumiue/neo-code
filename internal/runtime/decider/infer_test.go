package decider

import "testing"

func TestInferTaskKind(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		goal string
		want TaskKind
	}{
		{
			name: "todo plan",
			goal: "请创建 todo 列表并规划后续任务",
			want: TaskKindTodoState,
		},
		{
			name: "workspace write",
			goal: "创建文件 test.txt 并写入 1",
			want: TaskKindWorkspaceWrite,
		},
		{
			name: "review read only",
			goal: "review README.md 并总结风险",
			want: TaskKindReadOnly,
		},
		{
			name: "mixed write and review",
			goal: "edit main.go then review changes",
			want: TaskKindMixed,
		},
		{
			name: "subagent explicit",
			goal: "用 subagent 创建 test1.txt，内容为 1",
			want: TaskKindSubAgent,
		},
		{
			name: "chat answer fallback",
			goal: "什么是 NeoCode",
			want: TaskKindChatAnswer,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := InferTaskKind(tt.goal)
			if got != tt.want {
				t.Fatalf("InferTaskKind(%q) = %q, want %q", tt.goal, got, tt.want)
			}
		})
	}
}
