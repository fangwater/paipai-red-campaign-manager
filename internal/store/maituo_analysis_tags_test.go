package store

import (
	"slices"
	"testing"

	"paipai-red-campaign-manager/internal/maituo"
)

func TestCompleteNoteTagsReportsMissingFields(t *testing.T) {
	tags := maituo.NoteTags{
		NoteType:            []string{"科普"},
		CoverType:           []string{"单图"},
		CommercialIntensity: []string{"软广"},
		Audience:            []string{"职场人"},
		Progress:            []string{"已发布"},
	}

	completeNoteTags(&tags)
	if tags.Complete || !slices.Equal(tags.MissingFields, []string{"user_scenario"}) {
		t.Fatalf("incomplete tags = %+v", tags)
	}

	tags.UserScenario = []string{"通勤"}
	completeNoteTags(&tags)
	if !tags.Complete || len(tags.MissingFields) != 0 {
		t.Fatalf("complete tags = %+v", tags)
	}
}
