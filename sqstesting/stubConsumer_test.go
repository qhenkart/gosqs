package sqstesting

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/qhenkart/gosqs"
)

type sample struct {
	Name string `json:"name"`
}

func (s *sample) ModelName() string {
	return "sample"
}

func TestNewMessage(t *testing.T) {
	m := NewStubMessage(t, sample{"name"})
	data, err := json.Marshal(sample{"name"})
	if err != nil {
		t.Fatalf("error while marshalling data %v", err)
	}

	if len(m.body) != len(data) {
		t.Fatalf("did not properly marshal data")
	}
}

func TestNewStubModified(t *testing.T) {
	m := NewStubModified(t, sample{"name"}, map[string]string{"oldName": "old"})
	expected := struct {
		Body    interface{}
		Changes interface{}
	}{
		Body:    sample{"name"},
		Changes: map[string]string{"oldName": "old"},
	}
	data, err := json.Marshal(expected)
	if err != nil {
		t.Fatalf("error while marshalling data %v", err)
	}

	if len(m.body) != len(data) {
		t.Fatalf("did not properly marshal data")
	}
}

func TestRoute(t *testing.T) {
	m := NewStubMessage(t, sample{"name"})
	m.Endpoint = "card_created"

	if m.Route() != "card_created" {
		t.Fatalf("unexpected response, got %s, expected %s", m.Route(), "card_created")
	}
}

func TestDecode(t *testing.T) {
	m := NewStubMessage(t, sample{"name"})
	s := sample{}
	if err := m.Decode(&s); err != nil {
		t.Fatalf("decode error, got %v", err)
	}
	if s.Name != "name" {
		t.Fatalf("unexpected response, got %s, expected %s", s.Name, "name")
	}
}

func TestErrorResponse(t *testing.T) {
	m := NewStubMessage(t, sample{"name"})
	m.ErrorResponse(context.TODO(), gosqs.ErrUnableToDelete)
	if gosqs.ErrUnableToDelete.Error() != m.Err.Error() {
		t.Fatalf("did not attach error to message, got %v, expected %v", m.Err, gosqs.ErrUnableToDelete)
	}
}

func TestMessageSelf(t *testing.T) {
	stub := NewStubConsumer()
	stub.MessageSelf(context.TODO(), "some_event", nil)
	msg := stub.DirectMessages[0]
	if msg.Event != "some_event" {
		t.Fatalf("did not apply message to message array, expected some_event, got %s", msg.Event)
	}
	if msg.QueueName != "self" {
		t.Fatalf("did not apply message to message array, expected self, got %s", msg.QueueName)
	}
}

func TestMessage(t *testing.T) {
	stub := NewStubConsumer()
	stub.Message(context.TODO(), "queueURL", "some_event", nil)
	msg := stub.DirectMessages[0]
	if msg.Event != "some_event" {
		t.Fatalf("expected some_event, got %s", msg.Event)
	}
	if msg.QueueName != "queueURL" {
		t.Fatalf("expected queueURL, got %s", msg.QueueName)
	}
}

func TestMessagePublish(t *testing.T) {
	stub := NewStubDispatcher()
	stub.Message("queueURL", "some_event", nil)
	msg := stub.DirectMessages[0]
	if msg.Event != "some_event" {
		t.Fatalf("expected some_event, got %s", msg.Event)
	}
	if msg.QueueName != "queueURL" {
		t.Fatalf("expected queueURL, got %s", msg.QueueName)
	}
}

func TestCreate(t *testing.T) {
	stub := NewStubDispatcher()
	stub.Create(&sample{})
	msg := stub.DispatcherMessages[0]
	if msg.Event != "sample_created" {
		t.Fatalf("expected sample_created, got %s", msg.Event)
	}

	if stub.EventList[0] != "sample_created" {
		t.Fatalf("expected sample_created, got %s", stub.EventList[0])
	}
}

func TestDelete(t *testing.T) {
	stub := NewStubDispatcher()
	stub.Delete(&sample{})
	msg := stub.DispatcherMessages[0]
	if msg.Event != "sample_deleted" {
		t.Fatalf("expected sample_deleted, got %s", msg.Event)
	}

	if stub.EventList[0] != "sample_deleted" {
		t.Fatalf("expected sample_deleted, got %s", stub.EventList[0])
	}
}

func TestUpdate(t *testing.T) {
	stub := NewStubDispatcher()
	stub.Update(&sample{})
	msg := stub.DispatcherMessages[0]
	if msg.Event != "sample_updated" {
		t.Fatalf("expected sample_updated, got %s", msg.Event)
	}

	if stub.EventList[0] != "sample_updated" {
		t.Fatalf("expected sample_updated, got %s", stub.EventList[0])
	}
}

func TestModified(t *testing.T) {
	stub := NewStubDispatcher()
	stub.Modify(&sample{}, &struct{}{})
	msg := stub.DispatcherMessages[0]
	if msg.Event != "sample_modified" {
		t.Fatalf("expected sample_modified, got %s", msg.Event)
	}

	if stub.EventList[0] != "sample_modified" {
		t.Fatalf("expected sample_modified, got %s", stub.EventList[0])
	}
}

func TestDisp(t *testing.T) {
	stub := NewStubDispatcher()
	stub.Dispatch(&sample{}, "random_event")
	msg := stub.DispatcherMessages[0]
	if msg.Event != "sample_random_event" {
		t.Fatalf("expected sample_random_event, got %s", msg.Event)
	}

	if stub.EventList[0] != "sample_random_event" {
		t.Fatalf("expected sample_random_event, got %s", stub.EventList[0])
	}
}
