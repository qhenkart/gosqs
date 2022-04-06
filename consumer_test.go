package gosqs

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/service/sqs"
)

type testStruct struct {
	Val string `json:"val"`
}

func test(ctx context.Context, m Message) error {
	return nil
}

func extend(ctx context.Context, m Message) error {
	time.Sleep(2 * time.Second)
	return nil
}

func err(ctx context.Context, m Message) error {
	return ErrGetMessage
}

func retrieveMessage(t *testing.T, c *consumer) Message {
	output, err := c.sqs.ReceiveMessage(&sqs.ReceiveMessageInput{QueueUrl: &c.QueueURL, MessageAttributeNames: []*string{&all}})
	if err != nil {
		t.Fatalf("unable to retrieve message, got: %v", err)
	}

	if len(output.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(output.Messages))
	}

	return newMessage(output.Messages[0])
}

func getConsumer(t *testing.T) *consumer {
	conf := Config{
		Region:   "local",
		Key:      "key",
		Secret:   "secret",
		Env:      "dev",
		Hostname: "http://localhost:4100",
		QueueURL: "http://local.goaws:4100/queue/dev-post-worker",
	}
	sess, err := newSession(conf)
	if err != nil {
		t.Fatalf("could not create session, got %v", err)
	}

	cons := &consumer{
		sqs:               sqs.New(sess),
		env:               conf.Env,
		VisibilityTimeout: 30,
		extensionLimit:    2,
		workerPool:        15,
	}

	cons.sqs.PurgeQueue(&sqs.PurgeQueueInput{QueueUrl: &conf.QueueURL})

	cons.QueueURL = conf.QueueURL
	return cons
}

func TestNewConsumer(t *testing.T) {
	conf := Config{
		Region:   "us-west2",
		Key:      "key",
		Secret:   "secret",
		Hostname: "http://localhost:4100",
		Env:      "dev",
	}
	c, err := NewConsumer(conf, "post-worker")
	if err != nil {
		t.Fatalf("error creating consumer, got %v", err)
	}
	expected := "http://local.goaws:4100/queue/dev-post-worker"
	if c.(*consumer).QueueURL != expected {
		t.Fatalf("did not properly apply http result, expected %s, got %s", expected, c.(*consumer).QueueURL)
	}
}

func TestRegisterHandler(t *testing.T) {
	c := getConsumer(t)
	a := []Adapter{}
	c.RegisterHandler("post_published", test, a...)

	handlers := c.handlers
	if len(handlers) != 1 {
		t.Fatalf("did not apply the handler, expected 1 got %d", len(handlers))
	}

	if _, ok := handlers["post_published"]; !ok {
		t.Fatalf("did not apply the correct handler, expected post_published, got %+v", handlers)
	}
}

func TestMessageSelf(t *testing.T) {
	c := getConsumer(t)

	c.MessageSelf(context.TODO(), "test_event", testStruct{"val"})
	msg := retrieveMessage(t, c)
	if msg.Route() != "test_event" {
		t.Errorf("unexpected route, expected test_event, got %s", msg.Route())
	}

	var ts testStruct
	msg.Decode(&ts)
	if ts.Val != "val" {
		t.Errorf("did not properly apply value body, got %s", ts.Val)
	}
}

func TestMessage(t *testing.T) {
	c := getConsumer(t)

	c.Message(context.TODO(), "post-worker", "test_event", testStruct{"val"})
	msg := retrieveMessage(t, c)
	if msg.Route() != "test_event" {
		t.Errorf("unexpected route, expected test_event, got %s", msg.Route())
	}

	var ts testStruct
	msg.Decode(&ts)
	if ts.Val != "val" {
		t.Errorf("did not properly apply value body, got %s", ts.Val)
	}
}

func TestDeleteMessage(t *testing.T) {
	c := getConsumer(t)

	c.Message(context.TODO(), "post-worker", "test_event", testStruct{"val"})
	msg := retrieveMessage(t, c)
	if msg.Route() != "test_event" {
		t.Errorf("unexpected route, expected test_event, got %s", msg.Route())
	}

	if err := c.delete(msg.(*message)); err != nil {
		t.Fatalf("unable to delete got %v", err)
	}
}

func TestRun(t *testing.T) {
	c := getConsumer(t)
	a := []Adapter{WithRecovery(func() {})}
	c.RegisterHandler("post_published", test, a...)
	c.RegisterHandler("post_event", err, a...)
	c.RegisterHandler("extend", extend, a...)

	if len(c.handlers) != 3 {
		t.Fatalf("did not apply the handler, expected 3 got %d", len(c.handlers))
	}

	t.Run("no_error", func(t *testing.T) {
		c.Message(context.TODO(), "post-worker", "post_published", testStruct{"val"})
		m := retrieveMessage(t, c)
		if err := c.run(m.(*message)); err != nil {
			t.Errorf("should not return an error, got %v", err)
		}
	})

	t.Run("error", func(t *testing.T) {
		c.Message(context.TODO(), "post-worker", "post_event", testStruct{"val"})
		m := retrieveMessage(t, c)
		if err := c.run(m.(*message)); err != ErrGetMessage {
			t.Errorf("unexpected result, expected %v, got %v", ErrGetMessage, err)
		}
	})

	t.Run("no_event", func(t *testing.T) {
		c.Message(context.TODO(), "post-worker", "no_event", testStruct{"val"})
		m := retrieveMessage(t, c)
		if err := c.run(m.(*message)); err != nil {
			t.Errorf("unexpected result, expected %v, got %v", nil, err)
		}
	})

	t.Run("renew_visibility", func(t *testing.T) {
		c.VisibilityTimeout = 11
		c.Message(context.TODO(), "post-worker", "extend", testStruct{"val"})
		m := retrieveMessage(t, c)
		if err := c.run(m.(*message)); err != nil {
			t.Errorf("unexpected result, expected %v, got %v", nil, err)
		}
	})
}

// // TODO:fix `panic: runtime error: invalid memory address or nil pointer dereference`
// func TestRunNoAttributes(t *testing.T) {
// 	c := getConsumer(t)
// 	a := []Adapter{WithRecovery(func() {})}
// 	c.RegisterHandler("", test, a...)

// 	t.Run("no_attributes", func(t *testing.T) {
// 		c.Message(context.TODO(), "post-worker", "", testStruct{"val"})
// 		m := retrieveMessage(t, c)
// 		if err := c.run(m.(*message)); err != nil {
// 			t.Errorf("should not return an error, got %v", err)
// 		}
// 	})
// }
