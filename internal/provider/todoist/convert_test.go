package todoist

import "testing"

func TestToModelTaskWithSection(t *testing.T) {
	task := &Task{
		ID:        ID("task-1"),
		ProjectID: ID("project-1"),
		SectionID: ID("section-1"),
		Content:   "demo",
		Priority:  1,
	}

	got := toModelTaskWithSection(task, "Board A")
	if got == nil {
		t.Fatal("expected task")
	}
	if got.Metadata == nil || got.Metadata.CustomFields == nil {
		t.Fatal("expected metadata custom fields")
	}
	if got.Metadata.CustomFields[todoistSectionIDField] != "section-1" {
		t.Fatalf("unexpected section id: %#v", got.Metadata.CustomFields[todoistSectionIDField])
	}
	if got.Metadata.CustomFields[todoistSectionNameField] != "Board A" {
		t.Fatalf("unexpected section name: %#v", got.Metadata.CustomFields[todoistSectionNameField])
	}
}
