package report

import (
	"strings"
	"testing"
)

func TestApplyProjectNotesByPositionAndName(t *testing.T) {
	tasks := []Task{
		{ID: "1", Source: "Smart Parking", ProjectID: "111"},
		{ID: "2", Source: "pidmis-backend"},
		{ID: "3", Source: "Smart Parking", ProjectID: "111"},
		{ID: "4", Source: "Lotteries", ProjectID: "333"},
	}
	// ClickUp lists 111 then 333, then GitHub repo iras-test/pidmis-backend.
	order := ProjectOrder(tasks, []string{"111", "333", "iras-test/pidmis-backend"})
	if strings.Join(order, "|") != "Smart Parking|Lotteries|pidmis-backend" {
		t.Fatalf("order = %v", order)
	}

	warnings := ApplyProjectNotes(tasks, order, NoteColumns{
		Challenges: "Delayed feedback|Unclear specs, Server downtime, extra group",
		FollowUp:   "pidmis-backend=Write tests",
	})

	if tasks[0].Challenges != "• Delayed feedback\n• Unclear specs" {
		t.Errorf("Smart Parking challenges = %q", tasks[0].Challenges)
	}
	if tasks[2].Challenges != "" {
		t.Errorf("only the first row of a project gets the note, got %q", tasks[2].Challenges)
	}
	if tasks[3].Challenges != "Server downtime" {
		t.Errorf("Lotteries challenges = %q", tasks[3].Challenges)
	}
	if tasks[1].Challenges != "extra group" || tasks[1].FollowUp != "Write tests" {
		t.Errorf("pidmis-backend = %q / %q", tasks[1].Challenges, tasks[1].FollowUp)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestApplyProjectNotesKeepsSlotForEmptyList(t *testing.T) {
	tasks := []Task{{ID: "1", Source: "B", ProjectID: "222"}}
	order := ProjectOrder(tasks, []string{"111", "222"})
	warnings := ApplyProjectNotes(tasks, order, NoteColumns{Challenges: "for list 111, for list 222"})
	if tasks[0].Challenges != "for list 222" {
		t.Fatalf("got %q", tasks[0].Challenges)
	}
	if len(warnings) != 1 {
		t.Fatalf("want a warning for list 111, got %v", warnings)
	}
}
