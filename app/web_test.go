package app

import (
	"testing"

	"github.com/krishnan/datastore-tui/datastore/client"
	"github.com/krishnan/datastore-tui/datastore/model"
)

// The expected URLs below follow the format of real Cloud Console links,
// with placeholder project/kind/property values.

func TestConsoleEntityURL(t *testing.T) {
	key := &model.Key{Path: []model.PathElement{{Kind: "Task", ID: 1234567890}}}
	got := consoleEntityURL("my-project", key)
	want := "https://console.cloud.google.com/datastore/databases/-default-/entities;kind=Task;ns=__$DEFAULT$__/edit;key=0%252F%257C4%252FTask%257C13%252Fid%253A1234567890?project=my-project"
	if got != want {
		t.Fatalf("consoleEntityURL =\n%s\nwant\n%s", got, want)
	}
}

func TestConsoleQueryURL(t *testing.T) {
	ref := &model.Key{Path: []model.PathElement{{Kind: "User", ID: 42}}}
	got, ok := consoleQueryURL("my-project", "", "Task", []client.PropertyFilter{
		{Property: "owner", Op: client.OpEqual, Value: model.KeyValueOf(ref)},
		{Property: "status", Op: client.OpEqual, Value: model.StringValue("DONE")},
	})
	want := "https://console.cloud.google.com/datastore/databases/-default-/entities;kind=Task;ns=__$DEFAULT$__/query/kind;queryModel=2%7CWH%7C1%7C5%2Fowner%7CEQ%7CKEY%7C20%2FS2V5KFVzZXIsIDQyKQ..%7CWH%7C1%7C6%2Fstatus%7CEQ%7CSTR%7C4%2FDONE?project=my-project"
	if !ok || got != want {
		t.Fatalf("consoleQueryURL = %v,\n%s\nwant\n%s", ok, got, want)
	}
}

func TestConsoleQueryURL_UnsupportedFilterFallsBackToUnfiltered(t *testing.T) {
	got, ok := consoleQueryURL("p", "ns1", "Task", []client.PropertyFilter{
		{Property: "n", Op: client.OpGreaterThan, Value: model.IntegerValue(1)},
	})
	want := "https://console.cloud.google.com/datastore/databases/-default-/entities;kind=Task;ns=ns1/query/kind?project=p"
	if ok || got != want {
		t.Fatalf("consoleQueryURL = %v, %s; want false, %s", ok, got, want)
	}
}
