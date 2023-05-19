package example

import (
	"context"

	"github.com/qhenkart/gosqs"
)

func initWorker(c gosqs.Config) {
	// create the connection to AWS or the emulator
	consumer, err := gosqs.NewConsumer(c, "post-worker")
	if err != nil {
		panic(err)
	}

	h := Consumer{
		consumer,
	}

	// add any adapters and middleware, you can also create your own adapters following gosqs.Handler function composition.
	// These will be run before the final message handler
	a := []gosqs.Adapter{gosqs.WithMiddleware(func(ctx context.Context, m gosqs.Message) error {
		// add middleware functionality or authorization middleware etc
		return nil
	})}

	// register the event listeners
	h.RegisterHandlers(a...)

	// begin message consumption
	go h.Consume()
}

// Consumer a wrapper for the gosqs consumer
type Consumer struct {
	gosqs.Consumer
}

// RegisterHandlers listens to the specific event types from the queue
func (c *Consumer) RegisterHandlers(adapters ...gosqs.Adapter) {
	c.RegisterHandler("post_created", c.CreatePost, adapters...)
	c.RegisterHandler("some_new_message", c.Test, adapters...)
}

// Test example event handler
func (c *Consumer) Test(ctx context.Context, m gosqs.Message) error {
	out := map[string]interface{}{}

	if err := m.Decode(&out); err != nil {
		// returning an error will cause the message to try again until it is sent to the Dead-Letter-Queue
		return err
	}

	// returning nil means the message was successfully processed and it will be deleted from the queue
	return nil
}

// CreatePost an example of what an event listener looks like
func (c *Consumer) CreatePost(ctx context.Context, m gosqs.Message) error {
	var p Post
	if err := m.Decode(&p); err != nil {
		return err
	}

	// send a message to the same queue
	c.MessageSelf(ctx, "some_new_message", &p)

	//forward the message to another queue
	c.Message(ctx, "notification-worker", "some_new_message", &p)

	return nil
}
