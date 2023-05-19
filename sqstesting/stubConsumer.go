package sqstesting

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/qhenkart/gosqs"
)

// StubMessage provides a stub framework for consumer unit tests
type StubMessage struct {
	body     []byte
	Err      error
	Endpoint string
}

// NewStubMessage returns an encoded stubmessage that is ready to emulate the sqs messenger
func NewStubMessage(t *testing.T, in interface{}) *StubMessage {
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("error while marshalling data %v", err)
	}
	sm := &StubMessage{
		body: data,
	}

	return sm
}

// NewStubModified returns an encoded stubmessage that is ready to emulate the sqs messenger for modification messages
func NewStubModified(t *testing.T, in interface{}, changes interface{}) *StubMessage {
	payload := struct {
		Body    interface{}
		Changes interface{}
	}{
		Body:    in,
		Changes: changes,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("error while marshalling data %v", err)
	}
	sm := &StubMessage{
		body: data,
	}

	return sm
}

// Route returns the target endpoint of the message
func (sm *StubMessage) Route() string {
	return sm.Endpoint
}

// Decode decodes the message into the provided interface
func (sm *StubMessage) Decode(out interface{}) error {
	return json.Unmarshal(sm.body, &out)
}

// DecodeModified decodes the message into a provided interface along with changed values
func (sm *StubMessage) DecodeModified(body interface{}, changes interface{}) error {
	s := struct {
		Body    interface{}
		Changes interface{}
	}{
		Body:    body,
		Changes: changes,
	}
	return sm.Decode(&s)
}

// ErrorResponse applies an error to the stub message and returns
func (sm *StubMessage) ErrorResponse(ctx context.Context, err error) error {
	sm.Err = err
	return err
}

// Success returns nil
func (sm *StubMessage) Success(ctx context.Context) error {
	return nil
}

// Attribute returns a fake attribute
func (sm *StubMessage) Attribute(key string) string {
	return ""
}

// StubConsumer provides a stub framework for consumer unit tests
//
// SNS messages event names will go into the DispatcherMessages string array
//
// Direct Messages to SQS will go into a map[string]string which defines
// the queueName as the key and the event as the value. If a message is
// being sent to itself, then the key will be "self"
type StubConsumer struct {
	DirectMessages []SentMessage
	EventList      []string
}

// NewStubConsumer provides a stub consumer/publisher to place into the handler or context
func NewStubConsumer() *StubConsumer {
	return &StubConsumer{
		DirectMessages: make([]SentMessage, 0),
		EventList:      make([]string, 0),
	}
}

type SentMessage struct {
	QueueName string
	Event     string
	Body      interface{}
}

// Consume satisfies the Consumer interface
func (c *StubConsumer) Consume() {}

// MessageSelf saves the message into the local map with the queue name listed as "self"
// satisfies the Consumer interface
func (c *StubConsumer) MessageSelf(ctx context.Context, event string, body interface{}) {
	sm := SentMessage{
		QueueName: "self",
		Event:     event,
		Body:      body,
	}
	c.DirectMessages = append(c.DirectMessages, sm)
	c.EventList = append(c.EventList, sm.Event)

}

// Message saves the message into the local map and satisfies the Consumer interface
func (c *StubConsumer) Message(ctx context.Context, queue, event string, body interface{}) {
	sm := SentMessage{
		QueueName: queue,
		Event:     event,
		Body:      body,
	}
	c.DirectMessages = append(c.DirectMessages, sm)
	c.EventList = append(c.EventList, sm.Event)
}

// RegisterHandler satisfies the Consumer interface
func (c *StubConsumer) RegisterHandler(name string, h gosqs.Handler, a ...gosqs.Adapter) {}

// StubPublisher provides a stub framework for service unit tests
//
// SNS messages event names will go into the DispatcherMessages string array
//
// Direct Messages to SQS will go into a map[string]string which defines
// the queueName as the key and the event as the value. If a message is
// being sent to itself, then the key will be "self"
type StubPublisher struct {
	DirectMessages     []SentMessage
	DispatcherMessages []SentMessage
	EventList          []string
}

// NewStubDispatcher provides a stub publisher to place into the handler or context
func NewStubDispatcher() *StubPublisher {
	return &StubPublisher{
		DispatcherMessages: make([]SentMessage, 0),
		EventList:          make([]string, 0),
		DirectMessages:     make([]SentMessage, 0),
	}
}

// Create saves the message in the dispatcher array and satisfies the Consumer interface
func (c *StubPublisher) Create(n gosqs.Notifier) {
	sm := SentMessage{
		Event: fmt.Sprintf("%s_%s", n.ModelName(), "created"),
		Body:  n,
	}
	c.DispatcherMessages = append(c.DispatcherMessages, sm)
	c.EventList = append(c.EventList, sm.Event)
}

// Delete saves the message in the dispatcher array and satisfies the Consumer interface
func (c *StubPublisher) Delete(n gosqs.Notifier) {
	sm := SentMessage{
		Event: fmt.Sprintf("%s_%s", n.ModelName(), "deleted"),
		Body:  n,
	}
	c.DispatcherMessages = append(c.DispatcherMessages, sm)
	c.EventList = append(c.EventList, sm.Event)
}

// Update saves the message in the dispatcher array and satisfies the Consumer interface
func (c *StubPublisher) Update(n gosqs.Notifier) {
	sm := SentMessage{
		Event: fmt.Sprintf("%s_%s", n.ModelName(), "updated"),
		Body:  n,
	}
	c.DispatcherMessages = append(c.DispatcherMessages, sm)
	c.EventList = append(c.EventList, sm.Event)
}

// Modify saves the message in the dispatcher array and satisfies the Consumer interface
func (c *StubPublisher) Modify(n gosqs.Notifier, changes interface{}) {
	sm := SentMessage{
		Event: fmt.Sprintf("%s_%s", n.ModelName(), "modified"),
		Body:  n,
	}
	c.DispatcherMessages = append(c.DispatcherMessages, sm)
	c.EventList = append(c.EventList, sm.Event)
}

// Dispatch saves the message in the dispatcher array and satisfies the Consumer interface
func (c *StubPublisher) Dispatch(n gosqs.Notifier, event string) {
	sm := SentMessage{
		Event: fmt.Sprintf("%s_%s", n.ModelName(), event),
		Body:  n,
	}
	c.DispatcherMessages = append(c.DispatcherMessages, sm)
	c.EventList = append(c.EventList, sm.Event)
}

// Message saves the message into the local map and satisfies the Consumer interface
func (c *StubPublisher) Message(queue, event string, body interface{}) {
	sm := SentMessage{
		QueueName: queue,
		Event:     event,
		Body:      body,
	}
	c.DirectMessages = append(c.DirectMessages, sm)
	c.EventList = append(c.EventList, sm.Event)
}
