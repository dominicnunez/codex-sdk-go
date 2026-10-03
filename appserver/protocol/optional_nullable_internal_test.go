package protocol

import (
	"encoding/json"
	"testing"
)

func TestOptionalNullableOwnedCopies(t *testing.T) {
	for _, input := range []string{`{}`, `{"desktop":null}`, `{"desktop":{}}`, `{"desktop":{"nested":{"value":1}}}`} {
		var source Config
		if err := json.Unmarshal([]byte(input), &source); err != nil {
			t.Fatal(err)
		}
		snapshot := cloneArbitraryValue(source)
		if snapshot.Desktop.Present != source.Desktop.Present {
			t.Fatal("presence lost")
		}
		if source.Desktop.Value == nil {
			if snapshot.Desktop.Value != nil {
				t.Fatal("null changed")
			}
			continue
		}
		(*source.Desktop.Value)["source"] = json.RawMessage(`true`)
		if _, found := (*snapshot.Desktop.Value)["source"]; found {
			t.Fatal("copied map aliases input")
		}
		(*snapshot.Desktop.Value)["snapshot"] = json.RawMessage(`false`)
		if _, found := (*source.Desktop.Value)["snapshot"]; found {
			t.Fatal("input map aliases snapshot")
		}
		if raw := (*snapshot.Desktop.Value)["nested"]; len(raw) > 0 {
			raw[0] = '!'
			if (*source.Desktop.Value)["nested"][0] == '!' {
				t.Fatal("nested raw JSON aliases snapshot")
			}
		}
	}
	values := []string{"original"}
	source := PluginSummary{EligiblePlanTypes: OptionalNullable[[]string]{Present: true, Value: &values}}
	snapshot := cloneArbitraryValue(source)
	values[0] = "source"
	if (*snapshot.EligiblePlanTypes.Value)[0] != "original" {
		t.Fatal("slice snapshot aliases input")
	}
	(*snapshot.EligiblePlanTypes.Value)[0] = "snapshot"
	if values[0] != "source" {
		t.Fatal("input slice aliases snapshot")
	}
	for _, value := range []OptionalNullable[[]string]{
		{}, {Value: &values}, {Present: true}, {Present: true, Value: new([]string)},
	} {
		cloned := cloneArbitraryValue(value)
		if cloned.Present != value.Present {
			t.Fatal("constructed presence changed")
		}
		data, err := json.Marshal(struct {
			Field OptionalNullable[[]string] `json:"field,omitzero"`
		}{value})
		if err != nil {
			t.Fatal(err)
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(data, &object); err != nil {
			t.Fatal(err)
		}
		_, present := object["field"]
		if present != value.Present {
			t.Fatalf("constructed membership=%v, want %v", present, value.Present)
		}
		if value.Present && string(object["field"]) != "null" {
			t.Fatalf("constructed null=%s, want null", object["field"])
		}
	}
}

func TestOptionalNullableNestedCopies(t *testing.T) {
	days := []ScheduledTaskWeekday{ScheduledTaskWeekdayMonday}
	hourly := &HourlyScheduledTaskSchedule{IntervalHours: 1, Days: OptionalNullable[[]ScheduledTaskWeekday]{Present: true, Value: &days}}
	tasks := []ScheduledTaskSummary{{Key: "original", Name: "Task", Prompt: "data", Schedule: hourly}}
	source := PluginDetail{ScheduledTasks: OptionalNullable[[]ScheduledTaskSummary]{Present: true, Value: &tasks}}
	snapshot := cloneArbitraryValue(source)
	days[0] = ScheduledTaskWeekdayTuesday
	snapshotTasks := *snapshot.ScheduledTasks.Value
	snapshotHourly := snapshotTasks[0].Schedule.(*HourlyScheduledTaskSchedule)
	if (*snapshotHourly.Days.Value)[0] != ScheduledTaskWeekdayMonday {
		t.Fatal("nested day slice aliases input")
	}
	snapshotTasks[0].Key = "copy"
	snapshotHourly.IntervalHours = 2
	if tasks[0].Key != "original" || hourly.IntervalHours != 1 {
		t.Fatal("nested schedule or task aliases copy")
	}
	snapshotHourly.Days.Present = false
	if !hourly.Days.Present {
		t.Fatal("presence state aliases copy")
	}
}
