package feishu

import "testing"

func TestFilterTaskListsByOwnerOrCreator(t *testing.T) {
	lists := []TaskList{
		{TaskListID: "default", Name: "我的任务"},
		{TaskListID: "mine-owner", Name: "我负责", OwnerID: "ou_xxx_me"},
		{TaskListID: "mine-creator", Name: "我创建", CreatorID: "ou_xxx_me"},
		{TaskListID: "mine-member", Name: "我参与", MemberIDs: []string{"ou_other", "ou_xxx_me"}},
		{TaskListID: "other", Name: "他人共享", OwnerID: "ou_other", CreatorID: "ou_other"},
	}

	got := filterTaskListsByOwnerOrCreator(lists, "ou_xxx_me")
	if len(got) != 4 {
		t.Fatalf("expected 4 lists, got %d", len(got))
	}
	if got[0].TaskListID != "default" || got[1].TaskListID != "mine-owner" || got[2].TaskListID != "mine-creator" || got[3].TaskListID != "mine-member" {
		t.Fatalf("unexpected filtered order/content: %#v", got)
	}
}
