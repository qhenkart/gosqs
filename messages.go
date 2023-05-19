package gosqs

import (
	"context"
	"encoding/json"

	"github.com/aws/aws-sdk-go/service/sqs"
)

// Message serves as the message interface for handling the message
type Message interface {
	// Route returns the event name that is used for routing within a worker, e.g. post_published
	Route() string
	// Decode will unmarshal the message into a supplied output using json
	Decode(out interface{}) error
	// DecodeModified is used for decoding the modification message, it will populate the body with the actual message and a
	// map[string]interface{} to view original values from that message
	DecodeModified(out interface{}, changes interface{}) error
	// Attribute will return the custom attribute that was sent through out the request.
	Attribute(key string) string
}

// message serves as a wrapper for sqs.Message as well as controls the error handling channel
type message struct {
	*sqs.Message
	err chan error
}

func newMessage(m *sqs.Message) *message {
	return &message{m, make(chan error, 1)}
}

func (m *message) body() []byte {
	return []byte(*m.Message.Body)
}

// Route returns the event name that is used for routing within a worker, e.g. post_published
func (m *message) Route() string {
	return *m.MessageAttributes["route"].StringValue
}

// Decode will unmarshal the message into a supplied output using json
func (m *message) Decode(out interface{}) error {
	return json.Unmarshal(m.body(), &out)
}

// DecodeModified is used for decoding the modification message, it will populate the body with the actual message and a
// map[string]interface{} to view original values from that message
func (m *message) DecodeModified(body, changes interface{}) error {
	s := struct {
		Body    interface{}
		Changes interface{}
	}{
		Body:    body,
		Changes: changes,
	}

	return m.Decode(&s)
}

// ErrorResponse is used to determine for error handling within the handler. When an error occurs,
// this function should be returned.
func (m *message) ErrorResponse(ctx context.Context, err error) error {
	go func() {
		m.err <- err
	}()
	return err
}

// Success is used to determine that a handler was successful in processing the message and the message should
// now be consumed. This will delete the message from the queue
func (m *message) Success(ctx context.Context) error {
	go func() {
		m.err <- nil
	}()

	return nil
}

// Attribute will return the attrubute that was sent with the request.
func (m *message) Attribute(key string) string {
	id, ok := m.MessageAttributes[key]
	if !ok {
		return ""
	}

	return *id.StringValue
}
